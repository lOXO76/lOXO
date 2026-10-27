import type { GroupPlatform } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'
export const GROK_CC_SWITCH_MODEL = 'grok-4.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
}

/**
 * Balance query CC Switch runs against the imported provider. CC Switch fills
 * `{{baseUrl}}` with the provider's base URL as stored — Codex and Grok imports
 * carry a trailing `/v1` (see `withV1Endpoint`), Claude ones do not, and users
 * may edit it either way afterwards — then evaluates the script, so the URL
 * strips an existing `/v1` instead of blindly appending one (`/v1/v1/usage`
 * is a 404 and CC Switch shows "query failed").
 */
export const CC_SWITCH_USAGE_SCRIPT = `({
    request: {
      url: "{{baseUrl}}".replace(/\\/+$/, "").replace(/\\/v1$/, "") + "/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`

function withV1Endpoint(baseUrl: string): string {
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, '')
  return normalizedBaseUrl.endsWith('/v1') ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`
}

function withoutTrailingSlashes(baseUrl: string): string {
  return baseUrl.replace(/\/+$/, '')
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl.replace(/\/+$/, '')}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        // CC Switch's Codex provider appends the OpenAI-compatible path itself.
        // Passing /v1 here can make the client request /v1/v1/....
        endpoint: withoutTrailingSlashes(baseUrl),
        model: OPENAI_CC_SWITCH_CODEX_MODEL
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    case 'grok':
      return {
        app: 'grokbuild',
        endpoint: withV1Endpoint(baseUrl),
        model: GROK_CC_SWITCH_MODEL
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(input.platform, input.clientType, input.baseUrl)
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}

// ───────────────────── 按目标应用导入（导入客户端页） ─────────────────────

/**
 * CC Switch 深链 provider 导入支持的目标应用。
 *
 * claude / codex / gemini / opencode / openclaw 是稳定版就支持的；
 * grokbuild / hermes / pi 需要较新的 CC Switch（pi 需要 v4.0.10+）。
 * agy（Antigravity 桌面端）暂不提供：它不走本站的 API Key。
 */
export type CcSwitchApp =
  | 'claude'
  | 'codex'
  | 'gemini'
  | 'grokbuild'
  | 'opencode'
  | 'openclaw'
  | 'hermes'
  | 'pi'

export const CC_SWITCH_APPS: readonly CcSwitchApp[] = [
  'claude',
  'codex',
  'gemini',
  'grokbuild',
  'opencode',
  'openclaw',
  'hermes',
  'pi'
]

/** CC Switch 的上游协议格式（apiFormat，v4.0.10+ 生效；旧版按应用默认格式处理）。 */
export type CcSwitchApiFormat = 'anthropic' | 'openai_chat' | 'openai_responses' | 'gemini_native'

export interface CcSwitchAppTarget {
  endpoint: string
  apiFormat: CcSwitchApiFormat
  model?: string
  /** Pi 的深链必须带 model，否则 CC Switch 拒绝导入 */
  requiresModel: boolean
}

/**
 * 计算某个目标应用在本站该平台下应使用的端点与协议。
 *
 * 端点规则：
 * - Anthropic 协议不带 /v1（Claude Code 自己拼 /v1/messages）；antigravity 走 /antigravity
 * - Codex 用站点根：网关同时注册了 /responses 与 /v1/responses，根地址无论
 *   CC Switch 是否追加 /v1 都能命中，带 /v1 反而可能变成 /v1/v1
 * - Grok Build 沿用现有导入（固定一个 /v1）
 * - OpenCode / OpenClaw / Hermes / Pi 走 OpenAI Chat，SDK 期望 base URL 带 /v1
 */
export function resolveCcSwitchAppTarget(
  app: CcSwitchApp,
  platform: GroupPlatform | null | undefined,
  baseUrl: string,
  model: string
): CcSwitchAppTarget {
  const root = withoutTrailingSlashes(baseUrl).replace(/\/v1$/i, '')
  const trimmedModel = model.trim()
  switch (app) {
    case 'claude':
      return {
        endpoint: platform === 'antigravity' ? `${root}/antigravity` : root,
        apiFormat: 'anthropic',
        model: trimmedModel || undefined,
        requiresModel: false
      }
    case 'codex':
      return {
        endpoint: root,
        apiFormat: 'openai_responses',
        model: trimmedModel || (platform === 'openai' ? OPENAI_CC_SWITCH_CODEX_MODEL : undefined),
        requiresModel: false
      }
    case 'gemini':
      return {
        endpoint: platform === 'antigravity' ? `${root}/antigravity` : root,
        apiFormat: 'gemini_native',
        model: trimmedModel || undefined,
        requiresModel: false
      }
    case 'grokbuild':
      return {
        endpoint: withV1Endpoint(root),
        apiFormat: 'openai_responses',
        model: trimmedModel || GROK_CC_SWITCH_MODEL,
        requiresModel: false
      }
    case 'pi':
      return {
        endpoint: withV1Endpoint(root),
        apiFormat: 'openai_chat',
        model: trimmedModel || undefined,
        requiresModel: true
      }
    default:
      return {
        endpoint: withV1Endpoint(root),
        apiFormat: 'openai_chat',
        model: trimmedModel || undefined,
        requiresModel: false
      }
  }
}

export interface CcSwitchAppDeeplinkInput {
  app: CcSwitchApp
  target: CcSwitchAppTarget
  providerName: string
  homepage: string
  apiKey: string
  usageScript: string
}

export function buildCcSwitchAppDeeplink(input: CcSwitchAppDeeplinkInput): string {
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', input.app],
    ['name', input.providerName],
    ['homepage', input.homepage],
    ['endpoint', input.target.endpoint],
    ['apiKey', input.apiKey],
    ['apiFormat', input.target.apiFormat],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]
  if (input.target.model) {
    entries.splice(2, 0, ['model', input.target.model])
  }
  // URLSearchParams 负责把 base64 里的 + / = 转义，CC Switch 文档明确要求这一点
  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
