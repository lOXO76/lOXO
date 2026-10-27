import { describe, expect, it } from 'vitest'

import { resolveCatalog, resolveEndpoints, siteRoot } from '../endpoints'
import {
  CLIENT_DEFINITIONS,
  base64Utf8,
  evaluateClient,
  findClientDefinition,
  listCcSwitchApps
} from '../registry'

const baseInput = {
  baseUrl: 'https://api.example.com',
  apiKey: 'sk-test-key',
  siteName: 'Sub2API',
  models: [],
  defaultModel: '',
  platform: 'anthropic' as const
}

const group = (overrides: Partial<{ platform: string; claude_code_only: boolean; allow_messages_dispatch: boolean }> = {}) => ({
  platform: 'anthropic' as const,
  claude_code_only: false,
  allow_messages_dispatch: false,
  ...overrides
})

describe('siteRoot', () => {
  it('去掉尾部斜杠与 /v1 后缀', () => {
    expect(siteRoot('https://api.example.com/')).toBe('https://api.example.com')
    expect(siteRoot('https://api.example.com/v1')).toBe('https://api.example.com')
    expect(siteRoot('https://api.example.com/v1/')).toBe('https://api.example.com')
  })
})

describe('resolveEndpoints', () => {
  it('默认平台同时提供 anthropic / chat / responses', () => {
    const endpoints = resolveEndpoints('https://api.example.com', 'anthropic')
    expect(endpoints.anthropic).toBe('https://api.example.com')
    expect(endpoints.chat).toBe('https://api.example.com/v1')
    expect(endpoints.responses).toBe('https://api.example.com/v1')
  })

  it('antigravity 的 Anthropic 走 /antigravity，OpenAI 兼容端点仍走 /v1', () => {
    const endpoints = resolveEndpoints('https://api.example.com', 'antigravity')
    expect(endpoints).toEqual({
      anthropic: 'https://api.example.com/antigravity',
      chat: 'https://api.example.com/v1',
      responses: 'https://api.example.com/v1'
    })
  })

  it('gemini 不提供 responses 端点', () => {
    const endpoints = resolveEndpoints('https://api.example.com', 'gemini')
    expect(endpoints.responses).toBeUndefined()
    expect(endpoints.gemini).toBe('https://api.example.com/v1beta')
  })
})

describe('resolveCatalog', () => {
  it('给出 models.dev 目录名', () => {
    expect(resolveCatalog('openai')).toBe('openai')
    expect(resolveCatalog('gemini')).toBe('google')
    expect(resolveCatalog('composite')).toBeUndefined()
  })
})

describe('evaluateClient', () => {
  const claudeCode = findClientDefinition('claude-code')!
  const codex = findClientDefinition('codex')!

  it('claude_code_only 分组只放行 Claude Code', () => {
    const clamped = group({ claude_code_only: true })
    expect(evaluateClient(claudeCode, clamped).allowed).toBe(true)
    expect(evaluateClient(codex, clamped)).toEqual({ allowed: false, reason: 'claudeCodeOnly' })
  })

  it('openai 分组未开启 messages dispatch 时禁用 Claude Code', () => {
    const openaiGroup = group({ platform: 'openai' })
    expect(evaluateClient(claudeCode, openaiGroup)).toEqual({
      allowed: false,
      reason: 'messagesDispatchOff'
    })
  })

  it('openai 分组开启 messages dispatch 后放行 Claude Code', () => {
    const openaiGroup = group({ platform: 'openai', allow_messages_dispatch: true })
    expect(evaluateClient(claudeCode, openaiGroup).allowed).toBe(true)
  })

  it('gemini 分组禁用 Claude Code 但允许 Codex', () => {
    const geminiGroup = group({ platform: 'gemini' })
    expect(evaluateClient(claudeCode, geminiGroup)).toEqual({
      allowed: false,
      reason: 'platformMismatch'
    })
    expect(evaluateClient(codex, geminiGroup).allowed).toBe(true)
  })

  it('typesafe 分组对所有客户端都不可用', () => {
    const typesafe = group({ platform: 'typesafe' })
    for (const definition of CLIENT_DEFINITIONS) {
      expect(evaluateClient(definition, typesafe).allowed).toBe(false)
    }
  })
})

describe('generate', () => {
  it('Claude Code 写入 env 与 settings.json', () => {
    const result = findClientDefinition('claude-code')!.generate({
      ...baseInput,
      defaultModel: 'claude-opus-4',
      models: ['claude-opus-4']
    })
    expect(result.files.map((file) => file.path)).toEqual(['~/.claude/settings.json'])
    const settings = JSON.parse(result.files[0].content)
    expect(settings.env.ANTHROPIC_BASE_URL).toBe('https://api.example.com')
    expect(settings.env.ANTHROPIC_AUTH_TOKEN).toBe('sk-test-key')
    expect(settings.env.ANTHROPIC_MODEL).toBe('claude-opus-4')
  })

  it('非 Anthropic 分组未选模型时提示选择默认模型', () => {
    const claudeCode = findClientDefinition('claude-code')!
    const hint = (input: typeof baseInput & { platform: string; defaultModel?: string }) =>
      claudeCode.generate(input as never).steps.some((step) => step.text.includes('建议选择一个默认模型'))
    expect(hint({ ...baseInput, platform: 'kimi' })).toBe(true)
    expect(hint({ ...baseInput, platform: 'kimi', defaultModel: 'kimi-k2' })).toBe(false)
    expect(hint({ ...baseInput, platform: 'anthropic' })).toBe(false)
  })

  it('Claude Code 一键命令：合并 env、清理冲突键与未取值的模型键', () => {
    const withModel = findClientDefinition('claude-code')!.generate({ ...baseInput, defaultModel: 'claude-opus-4' })
    const bash = withModel.commands!.find((command) => command.shell === 'bash')!.content
    const powershell = withModel.commands!.find((command) => command.shell === 'powershell')!.content
    expect(withModel.commands!.map((command) => command.shell)).toEqual(['bash', 'powershell'])
    // 合并而非覆盖：先读原文件、带时间戳备份
    expect(bash).toContain('JSON.parse(raw)')
    expect(bash).toContain('.bak-')
    expect(bash).toContain('"ANTHROPIC_AUTH_TOKEN":"sk-test-key"')
    // 粘贴执行时不能退出用户的终端
    expect(bash).not.toMatch(/\bexit\b/)
    expect(bash).toContain('python3 -')
    expect(powershell).toContain('ConvertTo-Json -Depth 32')
    expect(powershell).toContain("'ANTHROPIC_AUTH_TOKEN' = 'sk-test-key'")

    const noModel = findClientDefinition('claude-code')!.generate(baseInput)
    const removeLine = noModel.commands![0].content.match(/const remove = (.*);/)![1]
    expect(JSON.parse(removeLine)).toEqual([
      'ANTHROPIC_API_KEY',
      'ANTHROPIC_MODEL',
      'ANTHROPIC_DEFAULT_OPUS_MODEL',
      'ANTHROPIC_DEFAULT_SONNET_MODEL',
      'ANTHROPIC_DEFAULT_HAIKU_MODEL'
    ])
  })

  it('PowerShell 命令对单引号做转义', () => {
    const result = findClientDefinition('claude-code')!.generate({ ...baseInput, apiKey: "sk-te'st" })
    const powershell = result.commands!.find((command) => command.shell === 'powershell')!.content
    expect(powershell).toContain("'ANTHROPIC_AUTH_TOKEN' = 'sk-te''st'")
  })

  it('Codex 密钥放在 auth.json，config.toml 不内联也不依赖环境变量', () => {
    const result = findClientDefinition('codex')!.generate({
      ...baseInput,
      platform: 'openai',
      defaultModel: 'gpt-5.5'
    })
    const config = result.files.find((file) => file.path === '~/.codex/config.toml')!
    expect(config.content).toContain('requires_openai_auth = true')
    expect(config.content).not.toContain('env_key')
    expect(config.content).not.toContain('sk-test-key')
    const auth = result.files.find((file) => file.path === '~/.codex/auth.json')!
    expect(JSON.parse(auth.content).OPENAI_API_KEY).toBe('sk-test-key')
  })

  it('Codex 一键命令：先备份再写入，PowerShell 用无 BOM 的 UTF-8', () => {
    const result = findClientDefinition('codex')!.generate({ ...baseInput, platform: 'openai' })
    const bash = result.commands!.find((command) => command.shell === 'bash')!.content
    const powershell = result.commands!.find((command) => command.shell === 'powershell')!.content
    expect(bash.indexOf('.bak-')).toBeLessThan(bash.indexOf('cat > ~/.codex/config.toml'))
    expect(bash).toContain("<<'SUB2API_EOF'")
    expect(bash).toContain('chmod 600 ~/.codex/auth.json')
    expect(bash).not.toMatch(/\bexit\b/)
    expect(powershell).toContain('New-Object Text.UTF8Encoding $false')
    expect(powershell).not.toContain('Set-Content')
  })

  it('Chatbox 把默认模型排到第一位', () => {
    const result = findClientDefinition('chatbox')!.generate({
      ...baseInput,
      models: ['a-model', 'b-model'],
      defaultModel: 'b-model'
    })
    const config = JSON.parse(result.files[0].content)
    expect(config.settings.models.map((model: { modelId: string }) => model.modelId)).toEqual([
      'b-model',
      'a-model'
    ])
    expect(config.settings.apiHost).toBe('https://api.example.com')
    // base64 必须 URL 编码后再拼进 query，否则 + / = 会被客户端解码破坏
    expect(result.deeplink).toBe(
      `chatbox://provider/import?config=${encodeURIComponent(base64Utf8(result.files[0].content))}`
    )
    const encoded = new URL(result.deeplink!).searchParams.get('config')!
    expect(JSON.parse(atob(encoded)).settings.apiKey).toBe('sk-test-key')
  })

  it('Magpie 链接带 key，gemini 分组不传 responses', () => {
    const result = findClientDefinition('magpie')!.generate({
      ...baseInput,
      platform: 'gemini',
      keyId: 7,
      keyName: 'team',
      models: ['gemini-2.5-pro']
    })
    expect(result.deeplink).toBeTruthy()
    const url = new URL(result.deeplink!)
    expect(url.searchParams.get('key')).toBe('sk-test-key')
    expect(url.searchParams.get('responses')).toBeNull()
    expect(url.searchParams.get('id')).toBe('sub2api-7')
    expect(url.searchParams.get('models')).toBe('gemini-2.5-pro')
    expect(url.searchParams.get('catalog')).toBe('google')
  })

  it('Pi 的 anthropic 平台分支用 anthropic-messages', () => {
    const result = findClientDefinition('pi')!.generate({
      ...baseInput,
      platform: 'anthropic',
      models: ['claude-opus-4'],
      defaultModel: 'claude-opus-4'
    })
    const modelsJson = JSON.parse(result.files[0].content)
    expect(modelsJson.providers.sub2api.api).toBe('anthropic-messages')
    expect(modelsJson.providers.sub2api.baseUrl).toBe('https://api.example.com')
    const settings = JSON.parse(result.files[1].content)
    expect(settings.defaultProvider).toBe('sub2api')
    expect(settings.defaultModel).toBe('claude-opus-4')
  })

  it('Pi 的 openai 平台分支用 openai-completions 与 /v1 端点', () => {
    const result = findClientDefinition('pi')!.generate({
      ...baseInput,
      platform: 'openai',
      models: ['gpt-5.5'],
      defaultModel: 'gpt-5.5'
    })
    const modelsJson = JSON.parse(result.files[0].content)
    expect(modelsJson.providers.sub2api.api).toBe('openai-completions')
    expect(modelsJson.providers.sub2api.baseUrl).toBe('https://api.example.com/v1')
  })
})

describe('多协议供应商分组', () => {
  // 网关对这些平台的 /v1/messages 不要求 Messages 调度开关
  // （openai_gateway_handler.go 的 allowOpenAICompatibleMessagesDispatch），
  // 因此导入页必须放行全部客户端，而不是只给 OpenAI 兼容的那几个。
  it.each(['kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'command_code', 'cline', 'grok'])(
    '%s 分组放行全部客户端（含 Claude Code）',
    (platform) => {
      for (const definition of CLIENT_DEFINITIONS) {
        expect(evaluateClient(definition, group({ platform })), definition.id).toEqual({ allowed: true })
      }
    }
  )

  it('后端新登记、前端未内置的平台默认放行 OpenAI 兼容客户端', () => {
    const codex = findClientDefinition('codex')!
    expect(evaluateClient(codex, group({ platform: 'some_new_provider' })).allowed).toBe(true)
  })

  it('video 分组对所有客户端都不可用', () => {
    for (const definition of CLIENT_DEFINITIONS) {
      expect(evaluateClient(definition, group({ platform: 'video' })).allowed).toBe(false)
    }
  })

  it('Codex 为 antigravity 分组生成 /v1 的 base_url，而不是空串', () => {
    const result = findClientDefinition('codex')!.generate({ ...baseInput, platform: 'antigravity' })
    expect(result.files[0].content).toContain('base_url = "https://api.example.com/v1"')
  })

  it('Magpie 对 antigravity 只传 /antigravity 下的 Anthropic 端点', () => {
    const result = findClientDefinition('magpie')!.generate({ ...baseInput, platform: 'antigravity' })
    const url = new URL(result.deeplink!)
    expect(url.searchParams.get('anthropic')).toBe('https://api.example.com/antigravity')
    expect(url.searchParams.get('chat')).toBeNull()
    expect(url.searchParams.get('responses')).toBeNull()
  })
})

describe('CC Switch', () => {
  const ccSwitch = findClientDefinition('cc-switch')!
  const params = (deeplink: string) => new URL(deeplink).searchParams

  it('按分组平台筛选可选应用', () => {
    expect(listCcSwitchApps(group({ platform: 'anthropic' })))
      .toEqual(['claude', 'codex', 'opencode', 'openclaw', 'hermes', 'pi'])
    expect(listCcSwitchApps(group({ platform: 'gemini' })))
      .toEqual(['codex', 'gemini', 'opencode', 'openclaw', 'hermes', 'pi'])
    expect(listCcSwitchApps(group({ platform: 'grok' })))
      .toEqual(['claude', 'codex', 'grokbuild', 'opencode', 'openclaw', 'hermes', 'pi'])
    expect(listCcSwitchApps(group({ platform: 'antigravity' })))
      .toEqual(['claude', 'codex', 'gemini', 'opencode', 'openclaw', 'hermes', 'pi'])
    expect(listCcSwitchApps(group({ platform: 'kimi' })))
      .toEqual(['claude', 'codex', 'opencode', 'openclaw', 'hermes', 'pi'])
  })

  it('openai 分组未开 Messages 调度时不提供 Claude Code 应用', () => {
    expect(listCcSwitchApps(group({ platform: 'openai' }))).not.toContain('claude')
    expect(listCcSwitchApps(group({ platform: 'openai', allow_messages_dispatch: true }))).toContain('claude')
  })

  it('claude_code_only 分组只能导入到 CC Switch 的 Claude Code', () => {
    const clamped = group({ claude_code_only: true })
    expect(evaluateClient(ccSwitch, clamped)).toEqual({ allowed: true })
    expect(listCcSwitchApps(clamped)).toEqual(['claude'])
  })

  it('非文本平台不提供 CC Switch', () => {
    expect(evaluateClient(ccSwitch, group({ platform: 'video' })).allowed).toBe(false)
    expect(listCcSwitchApps(group({ platform: 'typesafe' }))).toEqual([])
  })

  it('Claude Code 使用 anthropic 协议与不带 /v1 的根地址；antigravity 走 /antigravity', () => {
    const anthropic = params(ccSwitch.generate({ ...baseInput, ccSwitchApp: 'claude' }).deeplink!)
    expect(anthropic.get('app')).toBe('claude')
    expect(anthropic.get('apiFormat')).toBe('anthropic')
    expect(anthropic.get('endpoint')).toBe('https://api.example.com')

    const antigravity = params(
      ccSwitch.generate({ ...baseInput, platform: 'antigravity', ccSwitchApp: 'claude' }).deeplink!
    )
    expect(antigravity.get('endpoint')).toBe('https://api.example.com/antigravity')
  })

  it('Codex 用站点根地址，避免 CC Switch 再拼一次 /v1', () => {
    const codex = params(
      ccSwitch.generate({ ...baseInput, baseUrl: 'https://api.example.com/v1', platform: 'openai', ccSwitchApp: 'codex' }).deeplink!
    )
    expect(codex.get('endpoint')).toBe('https://api.example.com')
    expect(codex.get('apiFormat')).toBe('openai_responses')
    expect(codex.get('model')).toBe('gpt-5.5')
  })

  it('OpenCode / Hermes 走 OpenAI Chat 与 /v1 端点，并带上默认模型', () => {
    const opencode = params(
      ccSwitch.generate({ ...baseInput, platform: 'kimi', defaultModel: 'kimi-k2', ccSwitchApp: 'opencode' }).deeplink!
    )
    expect(opencode.get('apiFormat')).toBe('openai_chat')
    expect(opencode.get('endpoint')).toBe('https://api.example.com/v1')
    expect(opencode.get('model')).toBe('kimi-k2')
  })

  it('Pi 未选模型时不生成链接，选了模型后才生成', () => {
    const missing = ccSwitch.generate({ ...baseInput, ccSwitchApp: 'pi' })
    expect(missing.deeplink).toBeUndefined()
    expect(missing.steps[0].text).toContain('Pi 必须指定模型')

    const ready = params(ccSwitch.generate({ ...baseInput, defaultModel: 'claude-opus-4', ccSwitchApp: 'pi' }).deeplink!)
    expect(ready.get('app')).toBe('pi')
    expect(ready.get('model')).toBe('claude-opus-4')
  })

  it('链接携带 API Key 与可解码的余额查询脚本，名称带密钥名便于区分', () => {
    const link = params(
      ccSwitch.generate({ ...baseInput, keyName: 'team', ccSwitchApp: 'codex' }).deeplink!
    )
    expect(link.get('apiKey')).toBe('sk-test-key')
    expect(link.get('name')).toBe('Sub2API-team')
    expect(atob(link.get('usageScript')!)).toContain('/v1/usage')
  })

  it('未选择目标应用时提示先选择', () => {
    const result = ccSwitch.generate(baseInput)
    expect(result.deeplink).toBeUndefined()
    expect(result.steps[0].text).toContain('请先选择')
  })
})
