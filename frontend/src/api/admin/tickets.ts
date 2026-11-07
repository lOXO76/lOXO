/**
 * Admin Tickets API endpoints
 * 工单系统管理端：工单列表/详情/回复/流转、全局设置、Agent 密钥、指派人。
 * 契约见 `.codex-tasks/ticket-system/SPEC.md` 第 6.2 节；类型以 `@/types/tickets` 为准。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type {
  AdminTicketAgentKey,
  AdminTicketAgentKeyListQuery,
  AdminTicketDetail,
  AdminTicketListQuery,
  AdminTicketSummary,
  AdminTicketUpdateRequest,
  TicketAssignee,
  TicketMessage,
  TicketNotifyChannel,
  TicketNotifyTestRequest,
  TicketSettings,
  TicketSettingsUpdate,
  TicketStats,
} from '@/types/tickets'

export type {
  AdminTicketAgentKey,
  AdminTicketAgentKeyListQuery,
  AdminTicketDetail,
  AdminTicketListQuery,
  AdminTicketSummary,
  AdminTicketUpdateRequest,
  TicketAssignee,
  TicketMessage,
  TicketNotifyChannel,
  TicketSettings,
  TicketSettingsUpdate,
  TicketStats,
}

export interface AdminTicketMessageRequest {
  body: string
  internal?: boolean
}

/** 去掉空字符串 / undefined / null，避免把「未筛选」当成筛选条件发给后端。 */
function cleanParams<T extends Record<string, unknown>>(params: T): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(params)) {
    if (value === '' || value === undefined || value === null) continue
    result[key] = value
  }
  return result
}

export async function listTickets(params: AdminTicketListQuery = {}): Promise<PaginatedResponse<AdminTicketSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminTicketSummary>>('/admin/tickets', {
    params: cleanParams(params as Record<string, unknown>),
  })
  return data
}

export async function getStats(): Promise<TicketStats> {
  const { data } = await apiClient.get<TicketStats>('/admin/tickets/stats')
  return data
}

export async function getTicket(id: number): Promise<AdminTicketDetail> {
  const { data } = await apiClient.get<AdminTicketDetail>(`/admin/tickets/${id}`)
  return data
}

export async function addMessage(id: number, payload: AdminTicketMessageRequest): Promise<TicketMessage> {
  const { data } = await apiClient.post<TicketMessage>(`/admin/tickets/${id}/messages`, payload)
  return data
}

export async function updateTicket(id: number, payload: AdminTicketUpdateRequest): Promise<AdminTicketSummary> {
  const { data } = await apiClient.patch<AdminTicketSummary>(`/admin/tickets/${id}`, payload)
  return data
}

export async function getSettings(): Promise<TicketSettings> {
  const { data } = await apiClient.get<TicketSettings>('/admin/ticket-settings')
  return data
}

export async function updateSettings(payload: TicketSettingsUpdate): Promise<TicketSettings> {
  const { data } = await apiClient.put<TicketSettings>('/admin/ticket-settings', payload)
  return data
}

export async function testNotification(channel: TicketNotifyChannel): Promise<{ sent: boolean }> {
  const payload: TicketNotifyTestRequest = { channel }
  const { data } = await apiClient.post<{ sent: boolean }>('/admin/ticket-settings/test-notification', payload)
  return data
}

export async function listAgentKeys(
  params: AdminTicketAgentKeyListQuery = {},
): Promise<PaginatedResponse<AdminTicketAgentKey>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminTicketAgentKey>>('/admin/ticket-agent-keys', {
    params: cleanParams(params as Record<string, unknown>),
  })
  return data
}

export async function updateAgentKey(
  id: number,
  payload: { status?: AdminTicketAgentKey['status']; qps_limit?: number },
): Promise<AdminTicketAgentKey> {
  const { data } = await apiClient.patch<AdminTicketAgentKey>(`/admin/ticket-agent-keys/${id}`, payload)
  return data
}

export async function deleteAgentKey(id: number): Promise<{ deleted: boolean }> {
  const { data } = await apiClient.delete<{ deleted: boolean }>(`/admin/ticket-agent-keys/${id}`)
  return data
}

export async function listAssignees(): Promise<TicketAssignee[]> {
  const { data } = await apiClient.get<TicketAssignee[]>('/admin/ticket-assignees')
  return data
}

export const ticketsAdminAPI = {
  listTickets,
  getStats,
  getTicket,
  addMessage,
  updateTicket,
  getSettings,
  updateSettings,
  testNotification,
  listAgentKeys,
  updateAgentKey,
  deleteAgentKey,
  listAssignees,
}

export default ticketsAdminAPI
