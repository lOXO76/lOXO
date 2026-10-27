import type { GroupPlatform } from '@/types'
import type { ResolvedEndpoints } from './types'

/**
 * 站点根地址：去掉尾部斜杠，并把用户误配的 /v1 后缀剥掉。
 *
 * 管理端「API 地址」既可能填 https://api.example.com，也可能填
 * https://api.example.com/v1；两种写法都要能生成正确的客户端端点。
 */
export function siteRoot(baseUrl: string): string {
  return (baseUrl || '')
    .trim()
    .replace(/\/+$/, '')
    .replace(/\/v1$/i, '')
    .replace(/\/+$/, '')
}

function withV1(base: string): string {
  return `${base}/v1`
}

/**
 * 各平台对外暴露的端点，按客户端能用的协议分组。
 *
 * 与网关路由保持一致（见 backend/internal/server/routes/gateway.go）：
 * - antigravity 分组的 Anthropic Messages 走 /antigravity；OpenAI 兼容的
 *   chat/responses 仍走 /v1，由网关的 antigravity 兼容层转换
 * - gemini 分组没有 /v1/responses 桥接，因此不给 responses 端点
 * - 其余平台同时提供 Anthropic Messages 与 OpenAI 兼容端点
 */
export function resolveEndpoints(baseUrl: string, platform: GroupPlatform): ResolvedEndpoints {
  const root = siteRoot(baseUrl)
  if (!root) return {}
  switch (platform) {
    case 'antigravity':
      return { anthropic: `${root}/antigravity`, chat: withV1(root), responses: withV1(root) }
    case 'gemini':
      return { anthropic: root, chat: withV1(root), gemini: `${root}/v1beta` }
    default:
      return { anthropic: root, chat: withV1(root), responses: withV1(root) }
  }
}

/**
 * models.dev 的目录名，供 Magpie 显示模型名称、上下文窗口与推理档位。
 * 未登记的平台不传 catalog，Magpie 会退回按模型 ID 原样展示。
 */
export function resolveCatalog(platform: GroupPlatform): string | undefined {
  switch (platform) {
    case 'openai':
      return 'openai'
    case 'gemini':
      return 'google'
    case 'grok':
      return 'xai'
    case 'kimi':
      return 'moonshotai'
    case 'zhipu':
      return 'zhipuai'
    case 'deepseek':
      return 'deepseek'
    case 'minimax':
      return 'minimax'
    case 'opencode_go':
      return 'opencode-go'
    case 'antigravity':
      return 'google'
    default:
      return undefined
  }
}
