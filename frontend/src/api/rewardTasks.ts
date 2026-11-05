import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export type RewardTaskType = 'browse' | 'register' | 'recharge'
export type RewardTaskVerifyMode = 'timer' | 'sdk' | 'callback'
export type RewardTaskPeriod = 'once' | 'daily' | 'weekly' | 'monthly'
export type RewardTaskRecordStatus = 'in_progress' | 'completed'

export interface RewardTaskCurrent {
  status: RewardTaskRecordStatus
  period_key: string
  started_at: string
  ticket_expires_at: string
  claimable_at: string | null
  progress_seconds: number
  recharge_total: number
  completed_at: string | null
  reward_amount: number
}

export interface UserRewardTask {
  id: number; title: string; description: string
  task_type: RewardTaskType; verify_mode: RewardTaskVerifyMode
  target_url: string; icon_url: string; reward_amount: number; period: RewardTaskPeriod
  browse_seconds: number; idle_timeout_seconds: number; min_recharge_amount: number; ends_at: string | null
  quota_exhausted: boolean; next_reset_at: string | null; current: RewardTaskCurrent | null
}

export interface UserRewardTaskList {
  enabled: boolean
  tasks: UserRewardTask[]
  summary: { completed_count: number; total_reward: number }
}

export interface StartRewardTaskResult { jump_url: string; current: RewardTaskCurrent }
export interface ClaimRewardTaskResult { reward_amount: number; balance: number; current: RewardTaskCurrent }
export interface UserRewardTaskRecord {
  id: number; task_id: number; task_title: string; task_type: RewardTaskType
  period_key: string; reward_amount: number; completed_at: string
}

export async function listRewardTasks(): Promise<UserRewardTaskList> {
  const { data } = await apiClient.get<UserRewardTaskList>('/user/reward-tasks')
  return data
}

export async function startRewardTask(id: number): Promise<StartRewardTaskResult> {
  const { data } = await apiClient.post<StartRewardTaskResult>(`/user/reward-tasks/${id}/start`)
  return data
}

export async function claimRewardTask(id: number): Promise<ClaimRewardTaskResult> {
  const { data } = await apiClient.post<ClaimRewardTaskResult>(`/user/reward-tasks/${id}/claim`)
  return data
}

export async function listMyRewardTaskRecords(page: number, pageSize: number): Promise<PaginatedResponse<UserRewardTaskRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<UserRewardTaskRecord>>('/user/reward-tasks/records', {
    params: { page, page_size: pageSize },
  })
  return data
}
