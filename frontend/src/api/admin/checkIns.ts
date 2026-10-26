/**
 * Admin Check-in API endpoints
 * 每日签到统计与签到明细。金额均为「签到额度」（签到奖励发放到余额的部分，非充值）。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface CheckInStatsSummary {
  today_date: string
  today_users: number
  today_reward: number
  last_7_days_users: number
  total_check_ins: number
  total_users: number
  total_reward: number
  average_reward: number
  period_check_ins: number
  period_reward: number
  period_start_date: string
  check_in_enabled: boolean
  captcha_enabled: boolean
  configured_tiers: number
  configured_reward_min: number
  configured_reward_max: number
}

export interface CheckInDailyStat {
  date: string
  users: number
  reward: number
}

/** 统计周期内档位的实际抽中情况；configured_* 来自当前配置，历史档位可能为 null。 */
export interface CheckInTierStat {
  tier_index: number
  count: number
  reward: number
  actual_percent: number
  observed_min: number
  observed_max: number
  configured_min: number | null
  configured_max: number | null
  configured_percent: number | null
}

export interface CheckInStats {
  days: number
  summary: CheckInStatsSummary
  daily: CheckInDailyStat[]
  tiers: CheckInTierStat[]
}

export interface AdminCheckInRecord {
  id: number
  user_id: number
  user_email: string
  username: string
  check_in_date: string
  reward: number
  tier_index: number
  created_at: string
}

export interface ListCheckInRecordsParams {
  page?: number
  page_size?: number
  search?: string
  /** YYYY-MM-DD，服务器时区签到日（含当天） */
  start_date?: string
  end_date?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export async function getStats(days: number): Promise<CheckInStats> {
  const { data } = await apiClient.get<CheckInStats>('/admin/check-ins/stats', { params: { days } })
  return data
}

export async function listRecords(params: ListCheckInRecordsParams): Promise<PaginatedResponse<AdminCheckInRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminCheckInRecord>>('/admin/check-ins/records', { params })
  return data
}

export const checkInsAPI = {
  getStats,
  listRecords,
}

export default checkInsAPI
