/**
 * 用户端工单系统接口封装。
 *
 * 契约见 `.codex-tasks/ticket-system/SPEC.md` 第 6.1 节；共享类型见 `@/types/tickets`。
 * 响应信封（`{code, message, data}`）已由 `apiClient` 拦截器拆包，这里直接返回 `data`。
 */
import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'
import type {
  CreateTicketAgentKeyRequest,
  CreateTicketRequest,
  TicketAgentKey,
  TicketAgentKeyCreated,
  TicketDetail,
  TicketListQuery,
  TicketMessage,
  TicketNotifyChannel,
  TicketNotifyPreference,
  TicketNotifyPreferenceUpdate,
  TicketPublicConfig,
  TicketSummary,
  UpdateTicketAgentKeyRequest,
} from '@/types/tickets'

/** 去掉 query 中的空值，避免把 `status=` / `keyword=` 传给后端。 */
function cleanParams(query: TicketListQuery | undefined): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(query ?? {}) as Array<[string, unknown]>) {
    if (value === undefined || value === null || value === '') continue
    params[key] = value
  }
  return params
}

// ==================== 公开配置 ====================

/** 工单系统公开配置（系统关闭时仍可访问，返回 enabled:false）。 */
export async function getTicketConfig(): Promise<TicketPublicConfig> {
  const { data } = await apiClient.get<TicketPublicConfig>('/user/ticket-config')
  return data
}

// ==================== 工单 ====================

export async function listTickets(query: TicketListQuery = {}): Promise<PaginatedResponse<TicketSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<TicketSummary>>('/user/tickets', {
    params: cleanParams(query),
  })
  return data
}

export async function getTicketUnreadCount(): Promise<number> {
  const { data } = await apiClient.get<{ count: number }>('/user/tickets/unread-count')
  return data?.count ?? 0
}

export async function createTicket(payload: CreateTicketRequest): Promise<TicketDetail> {
  const { data } = await apiClient.post<TicketDetail>('/user/tickets', payload)
  return data
}

export async function getTicket(id: number): Promise<TicketDetail> {
  const { data } = await apiClient.get<TicketDetail>(`/user/tickets/${id}`)
  return data
}

export async function sendTicketMessage(id: number, body: string): Promise<TicketMessage> {
  const { data } = await apiClient.post<TicketMessage>(`/user/tickets/${id}/messages`, { body })
  return data
}

export async function closeTicket(id: number): Promise<TicketSummary> {
  const { data } = await apiClient.post<TicketSummary>(`/user/tickets/${id}/close`)
  return data
}

export async function reopenTicket(id: number): Promise<TicketSummary> {
  const { data } = await apiClient.post<TicketSummary>(`/user/tickets/${id}/reopen`)
  return data
}

// ==================== 通知偏好 ====================

export async function getTicketNotifyPreferences(): Promise<TicketNotifyPreference> {
  const { data } = await apiClient.get<TicketNotifyPreference>('/user/ticket-notify-preferences')
  return data
}

export async function updateTicketNotifyPreferences(
  payload: TicketNotifyPreferenceUpdate,
): Promise<TicketNotifyPreference> {
  const { data } = await apiClient.put<TicketNotifyPreference>('/user/ticket-notify-preferences', payload)
  return data
}

export async function sendTicketNotifyTest(channel: TicketNotifyChannel): Promise<{ sent: boolean }> {
  const { data } = await apiClient.post<{ sent: boolean }>('/user/ticket-notify-preferences/test', { channel })
  return data
}

// ==================== Agent 密钥 ====================

export async function listTicketAgentKeys(): Promise<TicketAgentKey[]> {
  const { data } = await apiClient.get<TicketAgentKey[]>('/user/ticket-agent-keys')
  return data ?? []
}

export async function createTicketAgentKey(payload: CreateTicketAgentKeyRequest): Promise<TicketAgentKeyCreated> {
  const { data } = await apiClient.post<TicketAgentKeyCreated>('/user/ticket-agent-keys', payload)
  return data
}

export async function updateTicketAgentKey(
  id: number,
  payload: UpdateTicketAgentKeyRequest,
): Promise<TicketAgentKey> {
  const { data } = await apiClient.patch<TicketAgentKey>(`/user/ticket-agent-keys/${id}`, payload)
  return data
}

export async function deleteTicketAgentKey(id: number): Promise<boolean> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/user/ticket-agent-keys/${id}`)
  return data?.deleted === true
}
