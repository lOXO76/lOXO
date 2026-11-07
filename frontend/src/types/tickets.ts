/**
 * 工单系统共享类型。
 * 与后端契约 `.codex-tasks/ticket-system/SPEC.md` 第 7 节逐字段一致；修改前请先同步后端。
 */

export type TicketStatus = 'pending_admin' | 'pending_user' | 'resolved' | 'closed'
export type TicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export type TicketCategory = 'general' | 'billing' | 'technical' | 'account' | 'feature' | 'other'
export type TicketAuthorType = 'user' | 'admin' | 'system'
export type TicketVia = 'web' | 'agent' | 'system'
export type TicketAgentKeyScope = 'user' | 'admin'
export type TicketAgentKeyStatus = 'active' | 'disabled'
export type TicketNotifyChannel = 'email' | 'feishu'

export const TICKET_STATUSES: TicketStatus[] = ['pending_admin', 'pending_user', 'resolved', 'closed']
export const TICKET_PRIORITIES: TicketPriority[] = ['low', 'normal', 'high', 'urgent']
export const TICKET_CATEGORIES: TicketCategory[] = ['general', 'billing', 'technical', 'account', 'feature', 'other']

export interface TicketSummary {
  id: number
  ticket_no: string
  subject: string
  category: TicketCategory
  priority: TicketPriority
  status: TicketStatus
  user_id: number
  assignee_id: number | null
  /** 对当前查看方是否未读（用户端 = user_unread，管理端 = admin_unread） */
  unread: boolean
  message_count: number
  last_message_at: string
  last_message_by: TicketAuthorType
  created_at: string
  updated_at: string
  closed_at: string | null
}

export interface AdminTicketSummary extends TicketSummary {
  user_email: string
  user_username: string
  assignee_name: string | null
  user_unread: boolean
  admin_unread: boolean
}

export interface TicketMessage {
  id: number
  ticket_id: number
  author_type: TicketAuthorType
  /** 用户端接口中 admin 消息恒为 null */
  author_id: number | null
  /** 用户端接口中 admin 消息恒为 "Support" */
  author_name: string
  via: TicketVia
  internal: boolean
  /** 纯文本，渲染时禁止 v-html。system 消息形如 `status_changed:resolved`，前端按 key 做 i18n */
  body: string
  created_at: string
}

export interface TicketDetail {
  ticket: TicketSummary
  messages: TicketMessage[]
}

export interface AdminTicketDetail {
  ticket: AdminTicketSummary
  messages: TicketMessage[]
}

export interface TicketStats {
  pending_admin: number
  pending_user: number
  resolved: number
  closed: number
  unread: number
}

export interface TicketPublicConfig {
  enabled: boolean
  agent_enabled: boolean
  categories: TicketCategory[]
  priorities: TicketPriority[]
  max_subject_length: number
  max_body_length: number
  max_open_tickets_per_user: number
  agent_default_qps: number
  agent_max_qps: number
  agent_user_total_qps: number
  agent_max_keys_per_user: number
}

export interface CreateTicketRequest {
  subject: string
  category: TicketCategory
  priority: TicketPriority
  body: string
}

export interface TicketListQuery {
  page?: number
  page_size?: number
  status?: TicketStatus | ''
  keyword?: string
}

export interface AdminTicketListQuery extends TicketListQuery {
  priority?: TicketPriority | ''
  category?: TicketCategory | ''
  user_id?: number
  assignee_id?: number
  unread_only?: boolean
}

export interface AdminTicketUpdateRequest {
  status?: TicketStatus
  priority?: TicketPriority
  /** 0 表示取消指派 */
  assignee_id?: number
}

export interface TicketAssignee {
  id: number
  email: string
  username: string
}

export interface TicketNotifyPreference {
  email_enabled: boolean
  /** 只读：账号邮箱，可能为空串 */
  email: string
  feishu_enabled: boolean
  feishu_webhook_configured: boolean
  feishu_webhook_url_masked: string
  feishu_secret_configured: boolean
}

/** 字段缺省/null = 不变；feishu_webhook_url / feishu_secret 传空串 = 清除 */
export interface TicketNotifyPreferenceUpdate {
  email_enabled?: boolean
  feishu_enabled?: boolean
  feishu_webhook_url?: string | null
  feishu_secret?: string | null
}

export interface TicketAgentKey {
  id: number
  name: string
  key_prefix: string
  scope: TicketAgentKeyScope
  qps_limit: number
  status: TicketAgentKeyStatus
  last_used_at: string | null
  last_used_ip: string | null
  expires_at: string | null
  created_at: string
}

export interface TicketAgentKeyCreated extends TicketAgentKey {
  /** 完整明文，仅在创建响应里出现一次 */
  secret: string
}

export interface AdminTicketAgentKey extends TicketAgentKey {
  user_id: number
  user_email: string
}

export interface CreateTicketAgentKeyRequest {
  name: string
  scope?: TicketAgentKeyScope
  qps_limit?: number
  expires_at?: string | null
}

export interface UpdateTicketAgentKeyRequest {
  name?: string
  qps_limit?: number
  status?: TicketAgentKeyStatus
}

export interface AdminTicketAgentKeyListQuery {
  page?: number
  page_size?: number
  user_id?: number
  scope?: TicketAgentKeyScope | ''
  status?: TicketAgentKeyStatus | ''
  keyword?: string
}

export interface TicketSettings {
  enabled: boolean
  admin_email_enabled: boolean
  admin_notify_emails: string[]
  admin_feishu_enabled: boolean
  admin_feishu_webhook_configured: boolean
  admin_feishu_webhook_url_masked: string
  admin_feishu_secret_configured: boolean
  notify_throttle_seconds: number
  max_open_tickets_per_user: number
  create_per_hour_per_user: number
  messages_per_minute_per_user: number
  max_subject_length: number
  max_body_length: number
  agent_enabled: boolean
  agent_default_qps: number
  agent_max_qps: number
  agent_user_total_qps: number
  agent_admin_total_qps: number
  agent_max_keys_per_user: number
  agent_failed_auth_per_minute_per_ip: number
  /** 单个公网 IP 每分钟访问 Agent API 的总请求上限，0 = 关闭 */
  agent_ip_requests_per_minute: number
}

type TicketSettingsReadOnlyKeys =
  | 'admin_feishu_webhook_configured'
  | 'admin_feishu_webhook_url_masked'
  | 'admin_feishu_secret_configured'

/** admin_feishu_webhook_url / admin_feishu_secret：缺省/null = 不变；空串 = 清除 */
export type TicketSettingsUpdate = Partial<Omit<TicketSettings, TicketSettingsReadOnlyKeys>> & {
  admin_feishu_webhook_url?: string | null
  admin_feishu_secret?: string | null
}

export interface TicketNotifyTestRequest {
  channel: TicketNotifyChannel
}
