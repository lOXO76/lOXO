import type { GroupPlatform } from '@/types'
import {
  CC_SWITCH_APPS,
  CC_SWITCH_USAGE_SCRIPT,
  buildCcSwitchAppDeeplink,
  resolveCcSwitchAppTarget,
  type CcSwitchApp
} from '@/utils/ccswitchImport'
import { buildClaudeCodeCommands, buildCodexCommands } from './commands'
import { resolveCatalog, resolveEndpoints, siteRoot } from './endpoints'
import type {
  ClientImportId,
  GenerateInput,
  ImportMode,
  ImportResult,
  ModelMode,
  ResolvedEndpoints
} from './types'

/**
 * 不提供通用文本对话协议的平台：video 只有视频生成接口，typesafe 只有
 * /v1/systemone。其余平台（含后端平台清单里新登记的）都能走 OpenAI 兼容端点。
 */
const NON_TEXT_PLATFORMS: GroupPlatform[] = ['video', 'typesafe']

/**
 * 客户端需要的上游协议：
 * - anthropic：Anthropic Messages（/v1/messages），Claude Code 必须
 * - openai：OpenAI 兼容（chat/completions 或 responses），网关按分组平台转换
 */
export type ClientProtocol = 'anthropic' | 'openai'

export interface ClientDefinition {
  id: ClientImportId
  mode: ImportMode
  modelMode: ModelMode
  protocol: ClientProtocol
  generate(input: GenerateInput): ImportResult
}

// ───────────────────────── 通用工具 ─────────────────────────

/** UTF-8 安全的 base64：直接 btoa(中文) 会抛 InvalidCharacterError */
export function base64Utf8(value: string): string {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return btoa(binary)
}

const pretty = (value: unknown): string => JSON.stringify(value, null, 2)

/** TOML 基本字符串转义：反斜杠必须先于引号处理 */
const escapeToml = (value: string): string =>
  value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')

/**
 * 把默认模型排到最前。Chatbox 没有独立的默认模型字段，
 * 官方约定的默认模型就是 models 数组的第一项。
 */
function withDefaultFirst(models: string[], defaultModel: string): string[] {
  if (!defaultModel) return models
  return [defaultModel, ...models.filter((model) => model !== defaultModel)]
}

/** 生成器实际使用的模型列表：空列表时回退到默认模型 */
function effectiveModels(input: GenerateInput): string[] {
  if (input.models.length > 0) return input.models
  return input.defaultModel ? [input.defaultModel] : []
}

// ───────────────────────── Claude Code ─────────────────────────

/** 原生提供 claude-* 模型的平台，Claude Code 不指定模型也能直接用 */
const CLAUDE_NATIVE_PLATFORMS: GroupPlatform[] = ['anthropic', 'antigravity', 'composite']

/** 本站在 Claude Code env 中管理的模型键：没取值时要删掉，避免沿用上次导入的模型 */
const CLAUDE_MODEL_ENV_KEYS = [
  'ANTHROPIC_MODEL',
  'ANTHROPIC_DEFAULT_OPUS_MODEL',
  'ANTHROPIC_DEFAULT_SONNET_MODEL',
  'ANTHROPIC_DEFAULT_HAIKU_MODEL'
]

function generateClaudeCode(input: GenerateInput): ImportResult {
  const { anthropic = '' } = resolveEndpoints(input.baseUrl, input.platform)
  const model = input.defaultModel

  const env: Record<string, string> = {
    ANTHROPIC_BASE_URL: anthropic,
    ANTHROPIC_AUTH_TOKEN: input.apiKey,
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1'
  }
  if (model) {
    for (const key of CLAUDE_MODEL_ENV_KEYS) env[key] = model
  }

  // ANTHROPIC_API_KEY 与 ANTHROPIC_AUTH_TOKEN 同时存在时 Claude Code 会报认证冲突
  const remove = ['ANTHROPIC_API_KEY', ...CLAUDE_MODEL_ENV_KEYS.filter((key) => !(key in env))]

  return {
    commands: buildClaudeCodeCommands(env, remove),
    files: [
      {
        path: '~/.claude/settings.json',
        content: pretty({
          $schema: 'https://json.schemastore.org/claude-code-settings.json',
          env
        }),
        hint: '手动放置时，若文件已存在请把 env 内的键合并进去，不要整份覆盖'
      }
    ],
    manual: [],
    steps: [
      { text: '一键命令只改写 ~/.claude/settings.json 中 env 的相关键，其余配置保留，原文件会先备份为 settings.json.bak-时间戳' },
      // Claude Code 默认请求 claude-* 模型名。非 Anthropic 分组的账号未必配置了
      // 这类模型映射，不指定模型时请求可能找不到可用账号。
      ...(!model && !CLAUDE_NATIVE_PLATFORMS.includes(input.platform)
        ? [{ text: '该分组不是 Anthropic 平台，建议选择一个默认模型，否则 Claude Code 会请求 claude-* 模型名，可能没有可用账号' }]
        : [])
    ]
  }
}

// ───────────────────────── Codex ─────────────────────────

/**
 * 密钥写在 auth.json，由 requires_openai_auth = true 读取，与 UseKeyModal 默认的
 * legacy 模式同一种写法。不用 env_key：一条命令没法可靠地持久化环境变量，
 * 用户重开终端后 Codex 就找不到密钥了。
 */
function generateCodex(input: GenerateInput): ImportResult {
  const { chat = '' } = resolveEndpoints(input.baseUrl, input.platform)
  const model = input.defaultModel || 'gpt-5.5'

  const config = `model_provider = "sub2api"
model = "${escapeToml(model)}"
review_model = "${escapeToml(model)}"
disable_response_storage = true
network_access = "enabled"

[model_providers.sub2api]
name = "${escapeToml(input.siteName || 'Sub2API')}"
base_url = "${escapeToml(chat)}"
wire_api = "responses"
requires_openai_auth = true`
  const auth = pretty({ OPENAI_API_KEY: input.apiKey })

  return {
    commands: buildCodexCommands(config, auth),
    files: [
      { path: '~/.codex/config.toml', content: config },
      {
        path: '~/.codex/auth.json',
        content: auth,
        hint: '覆盖 auth.json 会让 Codex 原有的 ChatGPT 登录失效'
      }
    ],
    manual: [],
    steps: [
      { text: '一键命令会整份写入 ~/.codex/config.toml 与 auth.json，原文件先备份为 *.bak-时间戳；原配置里的 MCP 等设置需要从备份中自行合并回来' },
      { text: '写入后重新打开 Codex 会话即可生效' }
    ]
  }
}

// ───────────────────────── Pi Agent ─────────────────────────

/** 走 Anthropic Messages 的平台；其余走 OpenAI Chat Completions */
const PI_ANTHROPIC_PLATFORMS: GroupPlatform[] = [
  'anthropic',
  'antigravity',
  'composite',
  'deepseek',
  'minimax'
]

function generatePi(input: GenerateInput): ImportResult {
  const endpoints = resolveEndpoints(input.baseUrl, input.platform)
  const useAnthropic = PI_ANTHROPIC_PLATFORMS.includes(input.platform)
  const models = effectiveModels(input)

  const modelsJson = pretty({
    providers: {
      sub2api: {
        name: input.siteName || 'Sub2API',
        baseUrl: useAnthropic ? endpoints.anthropic : endpoints.chat,
        api: useAnthropic ? 'anthropic-messages' : 'openai-completions',
        apiKey: input.apiKey,
        models: models.map((id) => ({ id, name: id }))
      }
    }
  })

  const settingsJson = pretty({
    defaultProvider: 'sub2api',
    ...(input.defaultModel ? { defaultModel: input.defaultModel } : {})
  })

  return {
    files: [
      { path: '~/.pi/agent/models.json', content: modelsJson },
      {
        path: '~/.pi/agent/settings.json',
        content: settingsJson,
        hint: '已存在时请合并 defaultProvider / defaultModel 两个键'
      }
    ],
    manual: [],
    steps: [{ text: 'Pi 在 /model 时重新读取配置，不需要重启进程' }]
  }
}

// ───────────────────────── Chatbox ─────────────────────────

function generateChatbox(input: GenerateInput): ImportResult {
  const root = siteRoot(input.baseUrl)
  const models = withDefaultFirst(effectiveModels(input), input.defaultModel)

  const config = {
    id: 'sub2api',
    name: input.siteName || 'Sub2API',
    type: 'openai',
    iconUrl: '',
    urls: { website: root },
    settings: {
      // Chatbox 自己在 apiHost 后面拼 apiPath，所以这里只给站点根
      apiHost: root,
      apiPath: '/v1/chat/completions',
      apiKey: input.apiKey,
      models: models.map((modelId) => ({
        modelId,
        nickname: modelId,
        type: 'chat',
        capabilities: ['tool_use']
      }))
    }
  }

  const content = pretty(config)

  return {
    files: [{ path: 'chatbox-provider.json', content }],
    // base64 里的 + / = 在 query 值中必须转义，否则会被客户端按 URL 解码成
    // 空格等字符，密钥或模型名随之损坏。
    deeplink: `chatbox://provider/import?config=${encodeURIComponent(base64Utf8(content))}`,
    manual: [],
    steps: [
      { text: '需要 Chatbox 1.15.1 及以上版本，点击「一键导入」后在确认框里选择导入' },
      { text: '列表第一位即默认模型（Chatbox 没有独立的默认模型设置项）' }
    ]
  }
}

// ───────────────────────── Magpie ─────────────────────────

function slugify(value: string): string {
  const slug = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return slug || 'sub2api'
}

/** Magpie 按字节截断 provider 名称，中文会被截成半个字符，这里先按 UTF-8 字节裁剪 */
function clampUtf8(value: string, maxBytes = 80): string {
  let out = ''
  let bytes = 0
  for (const char of value) {
    const size = new TextEncoder().encode(char).length
    if (bytes + size > maxBytes) break
    out += char
    bytes += size
  }
  return out
}

function generateMagpie(input: GenerateInput): ImportResult {
  const resolved = resolveEndpoints(input.baseUrl, input.platform)
  // Magpie 对 antigravity 只接受 /antigravity 下的 Anthropic 端点（与上游
  // sub2api 的 Magpie 导入规则一致），不能把 /v1 的兼容端点一并带过去。
  const endpoints: ResolvedEndpoints = input.platform === 'antigravity'
    ? { anthropic: resolved.anthropic }
    : resolved
  const root = siteRoot(input.baseUrl)

  // provider id 决定 Magpie 的去重行为：同一个 key 重复导入应当提示替换，
  // 不同 key 必须被视为不同 provider，否则会互相覆盖。
  const idSuffix = input.keyId !== undefined ? `-${input.keyId}` : ''
  const providerId = `${slugify(input.siteName || root)}${idSuffix}`

  const displayName = clampUtf8(
    [input.siteName || 'Sub2API', input.keyName].filter(Boolean).join('-')
  )

  const params = new URLSearchParams({
    name: displayName,
    id: providerId,
    key: input.apiKey,
    website: root
  })
  if (endpoints.anthropic) params.set('anthropic', endpoints.anthropic)
  if (endpoints.chat) params.set('chat', endpoints.chat)
  if (endpoints.responses) params.set('responses', endpoints.responses)
  if (endpoints.gemini) params.set('gemini', endpoints.gemini)

  const catalog = resolveCatalog(input.platform)
  if (catalog) params.set('catalog', catalog)
  if (input.models.length > 0) params.set('models', input.models.join(','))

  return {
    files: [],
    deeplink: `magpie://import?${params.toString()}`,
    manual: [
      endpoints.anthropic
        ? { label: 'Anthropic', value: endpoints.anthropic, mono: true }
        : null,
      endpoints.chat ? { label: 'Chat', value: endpoints.chat, mono: true } : null,
      endpoints.responses
        ? { label: 'Responses', value: endpoints.responses, mono: true }
        : null,
      { label: 'API Key', value: input.apiKey, mono: true }
    ].filter((field): field is NonNullable<typeof field> => field !== null),
    steps: [{ text: '未安装 Magpie 时链接会引导下载；重复导入同一 provider id 会提示替换' }]
  }
}

// ───────────────────────── Cherry Studio ─────────────────────────

function generateCherryStudio(input: GenerateInput): ImportResult {
  const endpoints = resolveEndpoints(input.baseUrl, input.platform)
  const models = effectiveModels(input)

  const fields = [
    { label: '服务商名称', value: input.siteName || 'Sub2API' },
    endpoints.chat
      ? { label: '端点 · OpenAI', value: endpoints.chat, mono: true }
      : null,
    endpoints.anthropic
      ? { label: '端点 · Anthropic', value: endpoints.anthropic, mono: true }
      : null,
    { label: 'API 密钥', value: input.apiKey, mono: true },
    models.length > 0
      ? { label: '模型 ID', value: models.join(', '), mono: true }
      : null
  ].filter((field): field is NonNullable<typeof field> => field !== null)

  return {
    files: [],
    manual: fields,
    steps: [
      { text: '设置 → 模型服务 → 添加服务商，类型选 OpenAI（或 Anthropic）' },
      { text: 'API 地址只填站点根地址，Cherry Studio 会自行补 /v1/chat/completions' },
      { text: '填完点「获取模型列表」勾选模型，并打开右上角启用开关' }
    ]
  }
}

// ───────────────────────── WorkBuddy ─────────────────────────

function generateWorkBuddy(input: GenerateInput): ImportResult {
  const { chat = '' } = resolveEndpoints(input.baseUrl, input.platform)

  return {
    files: [],
    manual: [
      { label: '接口类型', value: 'OpenAI 兼容' },
      { label: 'Base URL', value: chat, mono: true },
      { label: 'API Key', value: input.apiKey, mono: true },
      ...(input.defaultModel
        ? [{ label: '模型 ID', value: input.defaultModel, mono: true }]
        : [])
    ],
    steps: [
      { text: 'WorkBuddy 未公开配置文件格式，请在客户端的自定义模型 / OpenAI 兼容入口逐项填写' }
    ]
  }
}

// ───────────────────────── zcode ─────────────────────────

function generateZcode(input: GenerateInput): ImportResult {
  const { chat = '' } = resolveEndpoints(input.baseUrl, input.platform)

  return {
    files: [],
    manual: [
      { label: '接口类型', value: 'OpenAI 兼容（Chat Completions）' },
      { label: 'Base URL', value: chat, mono: true },
      { label: 'API Key', value: input.apiKey, mono: true },
      ...(input.defaultModel
        ? [{ label: '模型 ID', value: input.defaultModel, mono: true }]
        : [])
    ],
    steps: [
      { text: 'zcode 未公开配置文件导入格式，请在其「自定义模型」里填写上表中的端点与密钥' }
    ]
  }
}

// ───────────────────────── DeepSeek Hermes ─────────────────────────

function generateDeepSeekHermes(input: GenerateInput): ImportResult {
  const { chat = '' } = resolveEndpoints(input.baseUrl, input.platform)
  const root = siteRoot(input.baseUrl)

  return {
    files: [
      {
        path: 'Terminal (macOS / Linux)',
        content: `export DEEPSEEK_BASE_URL="${root}"\nexport DEEPSEEK_API_KEY="${input.apiKey}"`,
        hint: '写入 shell profile 后需要重开终端'
      }
    ],
    manual: [
      { label: 'DEEPSEEK_BASE_URL', value: root, mono: true },
      { label: 'DEEPSEEK_API_KEY', value: input.apiKey, mono: true },
      { label: 'OpenAI 兼容端点', value: chat, mono: true },
      ...(input.defaultModel
        ? [{ label: '模型 ID', value: input.defaultModel, mono: true }]
        : [])
    ],
    steps: [{ text: 'Hermes 读取环境变量；也可通过 Magpie 的 openai-chat 目标接入' }]
  }
}

// ───────────────────────── 平台能力 ─────────────────────────

export interface ClientAvailabilityGroup {
  platform: GroupPlatform
  claude_code_only: boolean
  allow_messages_dispatch?: boolean
}

/** 分组能否提供通用文本对话（OpenAI 兼容 chat/responses） */
export function supportsTextProtocol(platform: GroupPlatform): boolean {
  return !NON_TEXT_PLATFORMS.includes(platform)
}

/**
 * 分组能否提供 Anthropic Messages（/v1/messages），与网关行为一致：
 * - anthropic / antigravity / composite 走 Anthropic 网关，原生支持
 * - grok 与多协议供应商（kimi、智谱、DeepSeek、MiniMax、OpenCode Go、
 *   Command Code、Cline 等）由 OpenAI 网关直通，无需开关
 *   （见 openai_gateway_handler.go 的 allowOpenAICompatibleMessagesDispatch）
 * - openai 分组需要管理员开启「允许 /v1/messages 调度」，否则网关返回 403
 * - gemini 分组不提供 Claude Code 接入（与 UseKeyModal 一致）
 */
export function supportsAnthropicMessages(group: ClientAvailabilityGroup): boolean {
  if (!supportsTextProtocol(group.platform)) return false
  if (group.platform === 'gemini') return false
  if (group.platform === 'openai') return Boolean(group.allow_messages_dispatch)
  return true
}

/**
 * 该分组在 CC Switch 中可选的目标应用。
 *
 * claude_code_only 分组只能导入到 CC Switch 的 Claude Code；Gemini CLI 只对
 * gemini/antigravity 分组开放（gemini_native 协议），Grok Build 只对 grok 分组开放。
 */
export function listCcSwitchApps(group: ClientAvailabilityGroup): CcSwitchApp[] {
  if (!supportsTextProtocol(group.platform)) return []
  if (group.claude_code_only) return ['claude']
  return CC_SWITCH_APPS.filter((app) => {
    switch (app) {
      case 'claude':
        return supportsAnthropicMessages(group)
      case 'gemini':
        return group.platform === 'gemini' || group.platform === 'antigravity'
      case 'grokbuild':
        return group.platform === 'grok'
      default:
        return true
    }
  })
}

// ───────────────────────── CC Switch ─────────────────────────

const CC_SWITCH_APP_LABELS: Record<CcSwitchApp, string> = {
  claude: 'Claude Code',
  codex: 'Codex',
  gemini: 'Gemini CLI',
  grokbuild: 'Grok Build',
  opencode: 'OpenCode',
  openclaw: 'OpenClaw',
  hermes: 'Hermes',
  pi: 'Pi'
}

export function ccSwitchAppLabel(app: CcSwitchApp): string {
  return CC_SWITCH_APP_LABELS[app]
}

function generateCcSwitch(input: GenerateInput): ImportResult {
  const app = input.ccSwitchApp
  if (!app) {
    return { files: [], manual: [], steps: [{ text: '请先选择要导入到 CC Switch 的哪个应用' }] }
  }

  const target = resolveCcSwitchAppTarget(app, input.platform, input.baseUrl, input.defaultModel)
  const root = siteRoot(input.baseUrl)
  // CC Switch 不对 provider 去重，名字带上密钥名，方便用户区分多次导入
  const providerName = [input.siteName || 'Sub2API', input.keyName].filter(Boolean).join('-')

  const manual = [
    { label: '目标应用', value: CC_SWITCH_APP_LABELS[app] },
    { label: 'Endpoint', value: target.endpoint, mono: true },
    { label: 'API 格式', value: target.apiFormat, mono: true },
    ...(target.model ? [{ label: '模型', value: target.model, mono: true }] : []),
    { label: 'API Key', value: input.apiKey, mono: true }
  ]

  const steps = [
    { text: '点击「一键导入」唤起 CC Switch，在确认框里核对信息后导入' },
    { text: 'apiFormat 需要 CC Switch v4.0.10 及以上；旧版本会按该应用的默认协议处理' }
  ]
  if (app === 'pi') {
    steps.push({ text: 'Pi 需要 CC Switch v4.0.10 及以上，且必须选择一个默认模型' })
  }
  if (app === 'grokbuild' || app === 'hermes') {
    steps.push({ text: '该应用需要较新的 CC Switch 版本，若导入失败请先升级 CC Switch' })
  }

  // Pi 缺模型时 CC Switch 会直接拒绝导入，不生成链接，避免用户点了没反应
  if (target.requiresModel && !target.model) {
    return {
      files: [],
      manual,
      steps: [{ text: 'Pi 必须指定模型：请先在上方选择默认模型' }, ...steps.slice(1)]
    }
  }

  return {
    files: [],
    deeplink: buildCcSwitchAppDeeplink({
      app,
      target,
      providerName,
      homepage: root,
      apiKey: input.apiKey,
      // 余额查询脚本与 KeysView 的 CCS 导入共用，保证两处行为一致
      usageScript: CC_SWITCH_USAGE_SCRIPT
    }),
    manual,
    steps
  }
}

// ───────────────────────── 注册表 ─────────────────────────

/** 展示顺序：终端类在前，GUI 客户端在后，CC Switch 作为聚合导入放最后 */
export const CLIENT_DEFINITIONS: ClientDefinition[] = [
  { id: 'claude-code', mode: 'command', modelMode: 'single', protocol: 'anthropic', generate: generateClaudeCode },
  { id: 'codex', mode: 'command', modelMode: 'single', protocol: 'openai', generate: generateCodex },
  { id: 'pi', mode: 'file', modelMode: 'multi', protocol: 'openai', generate: generatePi },
  { id: 'cherry-studio', mode: 'manual', modelMode: 'multi', protocol: 'openai', generate: generateCherryStudio },
  { id: 'chatbox', mode: 'deeplink', modelMode: 'multi', protocol: 'openai', generate: generateChatbox },
  { id: 'magpie', mode: 'deeplink', modelMode: 'multi', protocol: 'openai', generate: generateMagpie },
  { id: 'workbuddy', mode: 'manual', modelMode: 'single', protocol: 'openai', generate: generateWorkBuddy },
  { id: 'zcode', mode: 'manual', modelMode: 'single', protocol: 'openai', generate: generateZcode },
  { id: 'deepseek-hermes', mode: 'manual', modelMode: 'single', protocol: 'openai', generate: generateDeepSeekHermes },
  { id: 'cc-switch', mode: 'deeplink', modelMode: 'single', protocol: 'openai', generate: generateCcSwitch }
]

/** 客户端被禁用的原因，对应 i18n 的 clientImport.disabled.* */
export type ClientBlockReason =
  | 'claudeCodeOnly'
  | 'platformMismatch'
  | 'messagesDispatchOff'

export interface ClientAvailability {
  allowed: boolean
  reason?: ClientBlockReason
}

/**
 * 判断某个客户端能否用于该分组。
 *
 * claude_code_only 分组的优先级最高：网关侧会对非 Claude Code 请求返回
 * ErrClaudeCodeOnly（见 gateway_scheduling.go 的 resolveGatewayGroup），
 * 因此页面必须把其他客户端全部禁用，而不是让用户配好之后请求失败。
 * CC Switch 例外：它仍可导入到自身的 Claude Code 应用。
 */
export function evaluateClient(
  definition: ClientDefinition,
  group: ClientAvailabilityGroup
): ClientAvailability {
  if (!supportsTextProtocol(group.platform)) {
    return { allowed: false, reason: 'platformMismatch' }
  }

  if (definition.id === 'cc-switch') {
    return listCcSwitchApps(group).length > 0
      ? { allowed: true }
      : { allowed: false, reason: 'platformMismatch' }
  }

  if (group.claude_code_only) {
    return definition.id === 'claude-code'
      ? { allowed: true }
      : { allowed: false, reason: 'claudeCodeOnly' }
  }

  if (definition.protocol === 'anthropic' && !supportsAnthropicMessages(group)) {
    return {
      allowed: false,
      reason: group.platform === 'openai' ? 'messagesDispatchOff' : 'platformMismatch'
    }
  }
  return { allowed: true }
}

export function findClientDefinition(id: ClientImportId): ClientDefinition | undefined {
  return CLIENT_DEFINITIONS.find((definition) => definition.id === id)
}
