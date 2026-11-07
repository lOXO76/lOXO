/**
 * 工单文本的长度工具。
 *
 * 后端（Go `utf8.RuneCountInString`）与契约（SPEC 第 3 节 `max_subject_length` /
 * `max_body_length`）都按 **Unicode code point** 计数，而 JS 的 `string.length`
 * 数的是 UTF-16 code unit——emoji 之类会被算成 2，且 `slice` 可能切出孤立代理对
 * （后端存成 `�`）。所以前端统一用 `Array.from(...)` 计数与截断。
 */

/** 按 Unicode code point 统计长度。 */
export function countTicketChars(value: string | null | undefined): number {
  if (!value) return 0
  return Array.from(value).length
}

/** 按 Unicode code point 截断，不会切出半个代理对。 */
export function truncateTicketText(value: string | null | undefined, max: number): string {
  if (!value) return ''
  const limit = Math.trunc(max)
  if (!Number.isFinite(limit) || limit <= 0) return ''
  const chars = Array.from(value)
  return chars.length <= limit ? value : chars.slice(0, limit).join('')
}

/** 是否在 code point 上限内（空串视为合法，必填由业务单独校验）。 */
export function isTicketTextWithinLimit(value: string | null | undefined, max: number): boolean {
  return countTicketChars(value) <= Math.trunc(max)
}

/** 文本是否全是空白（必填校验用，避免只输入不可见字符）。 */
export function isBlankTicketText(value: string | null | undefined): boolean {
  return !value || value.trim() === ''
}
