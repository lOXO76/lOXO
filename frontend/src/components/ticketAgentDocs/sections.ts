// 「工单 Agent 接入」文档页的十个章节（顺序即目录顺序）
export interface TicketAgentDocsSection {
  /** 锚点 id（页面内为 ticket-agent-docs-<id>） */
  id: string
  /** 章节标题的 i18n key */
  titleKey: string
}

export const TICKET_AGENT_DOCS_SECTIONS: TicketAgentDocsSection[] = [
  { id: 'overview', titleKey: 'ticketAgentDocs.sections.overview' },
  { id: 'createKey', titleKey: 'ticketAgentDocs.sections.createKey' },
  { id: 'auth', titleKey: 'ticketAgentDocs.sections.auth' },
  { id: 'endpoints', titleKey: 'ticketAgentDocs.sections.endpoints' },
  { id: 'rateLimits', titleKey: 'ticketAgentDocs.sections.rateLimits' },
  { id: 'idempotency', titleKey: 'ticketAgentDocs.sections.idempotency' },
  { id: 'errors', titleKey: 'ticketAgentDocs.sections.errors' },
  { id: 'polling', titleKey: 'ticketAgentDocs.sections.polling' },
  { id: 'examples', titleKey: 'ticketAgentDocs.sections.examples' },
  { id: 'prompt', titleKey: 'ticketAgentDocs.sections.prompt' },
]

export function ticketAgentDocsAnchor(id: string): string {
  return `ticket-agent-docs-${id}`
}

/** HTTP 方法徽章配色（docs 页与接口参考共用）。 */
export function methodBadgeClass(method: string): string {
  if (method === 'POST') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (method === 'PATCH') return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  return 'bg-primary-100 text-primary-700 dark:bg-primary-500/15 dark:text-primary-300'
}

/** scope 徽章文案的 i18n key。 */
export function scopeLabelKey(scope: 'all' | 'user' | 'admin'): string {
  if (scope === 'user') return 'ticketAgentDocs.endpoints.scopeUser'
  if (scope === 'admin') return 'ticketAgentDocs.endpoints.scopeAdmin'
  return 'ticketAgentDocs.endpoints.scopeAll'
}
