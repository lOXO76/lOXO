import { apiClient } from './client'

export interface CheckInRecord {
  date: string
  reward: number
}

export interface CheckInStatus {
  enabled: boolean
  captcha_required: boolean
  checked_in_today: boolean
  today_reward: number
  today_date: string
  streak_days: number
  next_reset_at: string
  /** 所有档位合并后的奖励下限（不含各档权重/概率） */
  reward_min: number
  /** 所有档位合并后的奖励上限 */
  reward_max: number
  recent_records: CheckInRecord[]
}

export interface CheckInResult {
  reward: number
  balance: number
  check_in_date: string
  streak_days: number
}

/** 月历中某一天：reward 为 null 表示当天未签到。 */
export interface CheckInCalendarDay {
  date: string
  reward: number | null
}

export interface CheckInCalendar {
  /** YYYY-MM */
  month: string
  /** 服务器时区的今天，YYYY-MM-DD */
  today: string
  days: CheckInCalendarDay[]
  checked_days: number
  total_reward: number
}

export interface CheckInPayload {
  turnstile_token?: string
  tencent_ticket?: string
  tencent_randstr?: string
}

export async function getCheckInStatus(): Promise<CheckInStatus> {
  const { data } = await apiClient.get<CheckInStatus>('/user/check-in')
  return data
}

/** 查询某自然月的签到记录；省略 month 时后端返回当前月。 */
export async function getCheckInCalendar(month?: string): Promise<CheckInCalendar> {
  const { data } = await apiClient.get<CheckInCalendar>('/user/check-in/calendar', {
    params: month ? { month } : undefined,
  })
  return data
}

export async function performCheckIn(payload: CheckInPayload = {}): Promise<CheckInResult> {
  const { data } = await apiClient.post<CheckInResult>('/user/check-in', payload)
  return data
}

export const checkInAPI = {
  getCheckInStatus,
  getCheckInCalendar,
  performCheckIn,
}

export default checkInAPI
