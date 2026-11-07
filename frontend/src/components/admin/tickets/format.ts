/**
 * 管理端工单界面的共享小工具：状态/优先级徽章、系统消息 key 解析、时间格式化。
 * 消息正文一律纯文本渲染，这里不产出任何 HTML。
 */

import { formatDateTime, formatRelativeTime } from '@/utils/format'
import type { TicketPriority, TicketStatus } from '@/types/tickets'

export const TICKET_STATUS_BADGE: Record<TicketStatus, string> = {
  pending_admin: 'badge-warning',
  pending_user: 'badge-primary',
  resolved: 'badge-success',
  closed: 'badge-gray',
}

export const TICKET_PRIORITY_BADGE: Record<TicketPriority, string> = {
  low: 'badge-gray',
  normal: 'badge-primary',
  high: 'badge-warning',
  urgent: 'badge-danger',
}

/** 状态 / 优先级 / 分类的 i18n key（调用方用 t() 取值）。 */
export function statusLabelKey(status: TicketStatus): string {
  return `admin.tickets.statuses.${status}`
}

export function priorityLabelKey(priority: TicketPriority): string {
  return `admin.tickets.priorities.${priority}`
}

export function categoryLabelKey(category: string): string {
  return `admin.tickets.categories.${category}`
}

/** 系统消息解析结果：`status_changed:resolved` → { kind: 'status_changed', status: 'resolved' } */
export interface ParsedSystemMessage {
  kind: 'status_changed' | 'unknown'
  status?: TicketStatus
  raw: string
}

const STATUS_VALUES: TicketStatus[] = ['pending_admin', 'pending_user', 'resolved', 'closed']

export function parseSystemMessage(body: string): ParsedSystemMessage {
  const raw = (body || '').trim()
  const [kind, value] = raw.split(':', 2)
  if (kind === 'status_changed' && value && (STATUS_VALUES as string[]).includes(value)) {
    return { kind: 'status_changed', status: value as TicketStatus, raw }
  }
  return { kind: 'unknown', raw }
}

/** 最后消息时间的展示：7 天内用相对时间，更早用日期。 */
export function formatTicketTime(value: string | null | undefined): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  const diffMs = Date.now() - date.getTime()
  if (diffMs >= 0 && diffMs < 7 * 24 * 60 * 60 * 1000) return formatRelativeTime(value)
  return formatDateTime(value)
}

export function formatTicketDateTime(value: string | null | undefined): string {
  if (!value) return '-'
  return formatDateTime(value)
}
