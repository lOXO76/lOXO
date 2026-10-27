import type { ImportCommand } from './types'

/**
 * 一键配置命令：用户复制到终端粘贴执行，把配置写到客户端读取的位置。
 *
 * 约束：
 * - 不能整份覆盖用户已有的配置（Claude Code 的 settings.json 里常有权限、
 *   hooks 等），能合并的只合并本站管理的键，覆盖前一律带时间戳备份
 * - 粘贴到交互式终端执行，不能用 exit / set -e，否则会关掉用户的终端
 * - 输出用英文：Windows PowerShell 5.1 的控制台对 UTF-8 中文容易乱码
 */

/** heredoc 结束标记：带引号时 shell 不做任何展开，内容原样写入 */
const EOF = 'SUB2API_EOF'

/** PowerShell 单引号字符串：只需把 ' 写成 '' */
function psQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

// ───────────────────────── Claude Code ─────────────────────────

/**
 * 合并写入 ~/.claude/settings.json 的 env：set 中的键写入，remove 中的键删除。
 * remove 用来清理本站管理但这次没有取值的键（比如上次导入设置过、这次没选的
 * 模型），以及会与 ANTHROPIC_AUTH_TOKEN 冲突的 ANTHROPIC_API_KEY。
 */
export function buildClaudeCodeCommands(
  set: Record<string, string>,
  remove: string[]
): ImportCommand[] {
  const setJson = JSON.stringify(set)
  const removeJson = JSON.stringify(remove)

  const nodeScript = `const fs = require("fs"), os = require("os"), path = require("path");
const set = ${setJson};
const remove = ${removeJson};
const dir = path.join(os.homedir(), ".claude");
const file = path.join(dir, "settings.json");
fs.mkdirSync(dir, { recursive: true });
let cfg = {};
if (fs.existsSync(file)) {
  const raw = fs.readFileSync(file, "utf8");
  const backup = file + ".bak-" + Date.now();
  fs.writeFileSync(backup, raw);
  console.log("Backed up to " + backup);
  try { cfg = JSON.parse(raw); } catch (e) { console.log("settings.json is not valid JSON, rewriting it"); }
}
if (!cfg || typeof cfg !== "object" || Array.isArray(cfg)) cfg = {};
const env = cfg.env && typeof cfg.env === "object" && !Array.isArray(cfg.env) ? cfg.env : {};
Object.assign(env, set);
for (const key of remove) delete env[key];
cfg.env = env;
fs.writeFileSync(file, JSON.stringify(cfg, null, 2) + "\\n");
console.log("Claude Code configured: " + file);`

  const pythonScript = `import json, os, shutil, time
set_ = json.loads(r'''${setJson}''')
remove = json.loads(r'''${removeJson}''')
d = os.path.join(os.path.expanduser("~"), ".claude")
f = os.path.join(d, "settings.json")
os.makedirs(d, exist_ok=True)
cfg = {}
if os.path.exists(f):
    b = f + ".bak-" + time.strftime("%Y%m%d%H%M%S")
    shutil.copyfile(f, b)
    print("Backed up to " + b)
    try:
        with open(f, encoding="utf-8") as h:
            cfg = json.load(h)
    except Exception:
        print("settings.json is not valid JSON, rewriting it")
if not isinstance(cfg, dict):
    cfg = {}
env = cfg.get("env") if isinstance(cfg.get("env"), dict) else {}
env.update(set_)
for k in remove:
    env.pop(k, None)
cfg["env"] = env
with open(f, "w", encoding="utf-8") as h:
    json.dump(cfg, h, indent=2, ensure_ascii=False)
    h.write("\\n")
print("Claude Code configured: " + f)`

  // Claude Code 的原生安装包不依赖 Node，所以 node 不一定存在：依次尝试
  // node、python3，都没有就提示改用下载文件。
  const bash = `if command -v node >/dev/null 2>&1; then
node - <<'${EOF}'
${nodeScript}
${EOF}
elif command -v python3 >/dev/null 2>&1; then
python3 - <<'${EOF}'
${pythonScript}
${EOF}
else
echo "Neither node nor python3 was found; download settings.json from this page instead."
fi`

  const psSet = Object.entries(set)
    .map(([key, value]) => `  ${psQuote(key)} = ${psQuote(value)}`)
    .join('\n')
  const psRemove = remove.map(psQuote).join(', ')

  // Windows 用 PowerShell 自带的 JSON 处理，不依赖 node。ConvertTo-Json 默认
  // 只展开 2 层，必须显式给足 Depth，否则深层配置会被截成字符串。
  const powershell = `$dir = Join-Path $HOME '.claude'
$file = Join-Path $dir 'settings.json'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$cfg = New-Object PSObject
if (Test-Path $file) {
  $raw = [IO.File]::ReadAllText($file)
  $backup = "$file.bak-" + (Get-Date -Format 'yyyyMMddHHmmss')
  [IO.File]::WriteAllText($backup, $raw)
  Write-Host "Backed up to $backup"
  try { $parsed = $raw | ConvertFrom-Json; if ($parsed -is [PSCustomObject]) { $cfg = $parsed } } catch { Write-Host 'settings.json is not valid JSON, rewriting it' }
}
if (-not ($cfg.PSObject.Properties['env'] -and $cfg.env -is [PSCustomObject])) {
  $cfg | Add-Member -NotePropertyName env -NotePropertyValue (New-Object PSObject) -Force
}
$set = [ordered]@{
${psSet}
}
foreach ($key in $set.Keys) { $cfg.env | Add-Member -NotePropertyName $key -NotePropertyValue $set[$key] -Force }
foreach ($key in @(${psRemove})) { $cfg.env.PSObject.Properties.Remove($key) }
[IO.File]::WriteAllText($file, ($cfg | ConvertTo-Json -Depth 32), (New-Object Text.UTF8Encoding $false))
Write-Host "Claude Code configured: $file"`

  return [
    { shell: 'bash', content: bash },
    { shell: 'powershell', content: powershell }
  ]
}

// ───────────────────────── Codex ─────────────────────────

/**
 * 写入 ~/.codex/config.toml 与 auth.json。TOML 没法在 shell 里可靠合并，所以
 * 整份写入，但先带时间戳备份原文件。auth.json 被覆盖后 ChatGPT 登录态会失效，
 * 备份可用于恢复。
 */
export function buildCodexCommands(configToml: string, authJson: string): ImportCommand[] {
  const bash = `mkdir -p ~/.codex
SUB2API_TS=$(date +%Y%m%d%H%M%S)
[ -f ~/.codex/config.toml ] && cp ~/.codex/config.toml ~/.codex/config.toml.bak-$SUB2API_TS
[ -f ~/.codex/auth.json ] && cp ~/.codex/auth.json ~/.codex/auth.json.bak-$SUB2API_TS
cat > ~/.codex/config.toml <<'${EOF}'
${configToml}
${EOF}
cat > ~/.codex/auth.json <<'${EOF}'
${authJson}
${EOF}
chmod 600 ~/.codex/auth.json
echo "Codex configured: ~/.codex (previous files saved as *.bak-$SUB2API_TS)"`

  // 写文件用无 BOM 的 UTF-8：Windows PowerShell 5.1 的 Set-Content -Encoding utf8
  // 会带 BOM，Codex 的 TOML 解析器会因此报错。
  const powershell = `$dir = Join-Path $HOME '.codex'
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$ts = Get-Date -Format 'yyyyMMddHHmmss'
foreach ($name in 'config.toml', 'auth.json') {
  $path = Join-Path $dir $name
  if (Test-Path $path) { Copy-Item $path "$path.bak-$ts" -Force }
}
$toml = @'
${configToml}
'@
$auth = @'
${authJson}
'@
$utf8 = New-Object Text.UTF8Encoding $false
[IO.File]::WriteAllText((Join-Path $dir 'config.toml'), $toml, $utf8)
[IO.File]::WriteAllText((Join-Path $dir 'auth.json'), $auth, $utf8)
Write-Host "Codex configured: $dir (previous files saved as *.bak-$ts)"`

  return [
    { shell: 'bash', content: bash },
    { shell: 'powershell', content: powershell }
  ]
}
