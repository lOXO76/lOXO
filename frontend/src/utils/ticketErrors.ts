/**
 * 工单系统错误码 → 友好提示。
 *
 * 后端用 `reason` 返回稳定错误码（SPEC 6.4），这里统一映射到 `tickets.errors.<code>`，
 * 未识别的错误码回退到服务端 message，再回退到通用文案。
 */
import { extractApiErrorCode, extractApiErrorMetadata, extractApiErrorMessage } from './apiError'

/** SPEC 6.4 全部错误码（用户端可能遇到的都列在这里）。 */
export const TICKET_ERROR_CODES = [
  'TICKET_SYSTEM_DISABLED',
  'TICKET_NOT_FOUND',
  'TICKET_INVALID_INPUT',
  'TICKET_CLOSED',
  'TICKET_INVALID_TRANSITION',
  'TICKET_OPEN_LIMIT',
  'TICKET_CREATE_RATE_LIMITED',
  'TICKET_MESSAGE_RATE_LIMITED',
  'TICKET_NOTIFY_TEST_RATE_LIMITED',
  'TICKET_NOTIFY_FAILED',
  'NOTIFY_WEBHOOK_INVALID',
  'AGENT_API_DISABLED',
  'AGENT_KEY_MISSING',
  'AGENT_KEY_INVALID',
  'AGENT_KEY_DISABLED',
  'AGENT_KEY_EXPIRED',
  'AGENT_KEY_LIMIT',
  'AGENT_QPS_INVALID',
  'AGENT_SCOPE_FORBIDDEN',
  'AGENT_RATE_LIMITED',
  'AGENT_AUTH_BLOCKED',
  'AGENT_RATE_LIMIT_UNAVAILABLE',
] as const

const TICKET_ERROR_CODE_SET = new Set<string>(TICKET_ERROR_CODES)

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

/** 该错误是否属于工单系统的已知错误码。 */
export function isTicketErrorCode(code: string | undefined): boolean {
  return !!code && TICKET_ERROR_CODE_SET.has(code)
}

/**
 * 把 API 错误翻译成一段可直接展示的文案。
 *
 * - 命中已知 reason → `tickets.errors.<reason>` 的文案，并把后端 metadata 作为插值参数
 *   （例如开放工单上限的 `max_open_tickets_per_user`）。
 * - 未知 reason → 服务端 message（若可读），否则 `tickets.errors.fallback`。
 */
export function resolveTicketErrorMessage(
  error: unknown,
  t: TranslateFn,
  fallbackKey = 'tickets.errors.fallback',
): string {
  const code = extractApiErrorCode(error)
  if (isTicketErrorCode(code)) {
    const key = `tickets.errors.${code}`
    const translated = t(key, { ...(extractApiErrorMetadata(error) ?? {}) })
    if (translated !== key) return translated
  }
  const message = extractApiErrorMessage(error, '')
  if (message && message !== '[object Object]' && message !== 'Network error. Please check your connection.') {
    return message
  }
  return t(fallbackKey)
}
