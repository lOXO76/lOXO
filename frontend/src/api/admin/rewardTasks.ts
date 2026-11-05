/**
 * Admin Reward Tasks API endpoints
 * 任务中心：任务 CRUD、接入站点、完成记录与开放接口回调日志。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

// 用户端 `@/api/rewardTasks` 由另一工人并行编写，此处先声明同名类型并在本文件导出，
// 两边定义完全一致；合并后若需要可改为从 `@/api/rewardTasks` import。
export type RewardTaskType = 'browse' | 'register' | 'recharge'
export type RewardTaskVerifyMode = 'timer' | 'sdk' | 'callback'
export type RewardTaskPeriod = 'once' | 'daily' | 'weekly' | 'monthly'
export type RewardTaskRecordStatus = 'in_progress' | 'completed'

export interface AdminRewardTask {
  id: number
  title: string
  description: string
  task_type: RewardTaskType
  verify_mode: RewardTaskVerifyMode
  partner_id: number | null
  partner_name: string
  target_url: string
  icon_url: string
  reward_amount: number
  period: RewardTaskPeriod
  browse_seconds: number
  /** 无操作暂停阈值（秒）：0 = 不检测，否则 5~3600；仅 browse + sdk 任务使用 */
  idle_timeout_seconds: number
  min_recharge_amount: number
  max_completions: number
  completed_count: number
  in_progress_count: number
  total_reward: number
  starts_at: string | null
  ends_at: string | null
  enabled: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export type AdminRewardTaskPayload = Omit<
  AdminRewardTask,
  'id' | 'partner_name' | 'completed_count' | 'in_progress_count' | 'total_reward' | 'created_at' | 'updated_at'
>

export interface AdminRewardTaskListParams {
  page?: number
  page_size?: number
  task_type?: RewardTaskType | ''
  enabled?: 'true' | 'false' | ''
  search?: string
}

export interface AdminRewardTaskStats {
  total_tasks: number
  enabled_tasks: number
  completed_today: number
  reward_today: number
  completed_total: number
  reward_total: number
  partners: number
}

export interface AdminRewardTaskPartner {
  id: number
  name: string
  app_key: string
  secret_hint: string
  allowed_domains: string[]
  contact: string
  notes: string
  enabled: boolean
  task_count: number
  last_event_at: string | null
  created_at: string
  updated_at: string
  /** 仅创建 / 重置密钥的响应里带一次性明文密钥 */
  app_secret?: string
}

export interface AdminRewardTaskPartnerPayload {
  name: string
  allowed_domains: string[]
  contact: string
  notes: string
  enabled: boolean
}

export interface AdminRewardTaskRecord {
  id: number
  task_id: number
  task_title: string
  task_type: RewardTaskType
  user_id: number
  user_email: string
  username: string
  period_key: string
  status: RewardTaskRecordStatus
  progress_seconds: number
  recharge_total: number
  external_user_id: string
  started_at: string
  ticket_expires_at: string
  completed_at: string | null
  reward_amount: number
}

export type RewardTaskEventResult = 'accepted' | 'completed' | 'ignored' | 'rejected'

export interface AdminRewardTaskEvent {
  id: number
  partner_id: number
  partner_name: string
  event_id: string
  event_type: string
  ticket: string
  task_id: number | null
  user_id: number | null
  external_user_id: string
  amount: number
  order_id: string
  result: RewardTaskEventResult
  error_code: string
  remote_ip: string
  payload: unknown
  created_at: string
}

export async function listTasks(params: AdminRewardTaskListParams): Promise<PaginatedResponse<AdminRewardTask>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminRewardTask>>('/admin/reward-tasks', { params })
  return data
}

export async function createTask(payload: AdminRewardTaskPayload): Promise<AdminRewardTask> {
  const { data } = await apiClient.post<AdminRewardTask>('/admin/reward-tasks', payload)
  return data
}

export async function updateTask(id: number, payload: AdminRewardTaskPayload): Promise<AdminRewardTask> {
  const { data } = await apiClient.put<AdminRewardTask>(`/admin/reward-tasks/${id}`, payload)
  return data
}

export async function deleteTask(id: number): Promise<void> {
  await apiClient.delete(`/admin/reward-tasks/${id}`)
}

export async function getStats(): Promise<AdminRewardTaskStats> {
  const { data } = await apiClient.get<AdminRewardTaskStats>('/admin/reward-task-stats')
  return data
}

export async function getSettings(): Promise<{ enabled: boolean }> {
  const { data } = await apiClient.get<{ enabled: boolean }>('/admin/reward-task-settings')
  return data
}

export async function updateSettings(enabled: boolean): Promise<{ enabled: boolean }> {
  const { data } = await apiClient.put<{ enabled: boolean }>('/admin/reward-task-settings', { enabled })
  return data
}

export async function listPartners(): Promise<AdminRewardTaskPartner[]> {
  const { data } = await apiClient.get<AdminRewardTaskPartner[]>('/admin/reward-task-partners')
  return data
}

export async function createPartner(payload: AdminRewardTaskPartnerPayload): Promise<AdminRewardTaskPartner> {
  const { data } = await apiClient.post<AdminRewardTaskPartner>('/admin/reward-task-partners', payload)
  return data
}

export async function updatePartner(id: number, payload: AdminRewardTaskPartnerPayload): Promise<AdminRewardTaskPartner> {
  const { data } = await apiClient.put<AdminRewardTaskPartner>(`/admin/reward-task-partners/${id}`, payload)
  return data
}

export async function resetPartnerSecret(id: number): Promise<AdminRewardTaskPartner> {
  const { data } = await apiClient.post<AdminRewardTaskPartner>(`/admin/reward-task-partners/${id}/reset-secret`)
  return data
}

export async function deletePartner(id: number): Promise<void> {
  await apiClient.delete(`/admin/reward-task-partners/${id}`)
}

export async function listRecords(params: {
  page?: number
  page_size?: number
  task_id?: number
  status?: RewardTaskRecordStatus | ''
  search?: string
}): Promise<PaginatedResponse<AdminRewardTaskRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminRewardTaskRecord>>('/admin/reward-task-records', { params })
  return data
}

export async function listEvents(params: {
  page?: number
  page_size?: number
  partner_id?: number
  result?: RewardTaskEventResult | ''
}): Promise<PaginatedResponse<AdminRewardTaskEvent>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminRewardTaskEvent>>('/admin/reward-task-events', { params })
  return data
}

export const rewardTasksAdminAPI = {
  listTasks,
  createTask,
  updateTask,
  deleteTask,
  getStats,
  getSettings,
  updateSettings,
  listPartners,
  createPartner,
  updatePartner,
  resetPartnerSecret,
  deletePartner,
  listRecords,
  listEvents,
}

export default rewardTasksAdminAPI
