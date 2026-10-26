import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CheckInView from '../CheckInView.vue'

const { getCheckInStatus, getCheckInCalendar, performCheckIn, refreshUser, showError, showInfo, showSuccess, fetchPublicSettings } = vi.hoisted(() => ({
  getCheckInStatus: vi.fn(),
  getCheckInCalendar: vi.fn(),
  performCheckIn: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
  fetchPublicSettings: vi.fn(),
}))

const appStore = {
  publicSettingsLoaded: true,
  cachedPublicSettings: {
    check_in_enabled: true,
    turnstile_enabled: false,
    turnstile_site_key: '',
    tencent_captcha_enabled: false,
    tencent_captcha_app_id: '',
    aliyun_captcha_enabled: false,
    aliyun_captcha_scene_id: '',
    aliyun_captcha_prefix: '',
  },
  fetchPublicSettings,
  showError,
  showInfo,
  showSuccess,
}

vi.mock('@/api/checkIn', () => ({ getCheckInStatus, getCheckInCalendar, performCheckIn }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } }) }
})
vi.mock('@/utils/format', () => ({ formatCurrency: (amount: number | null | undefined) => `$${Number(amount ?? 0).toFixed(2)}` }))

const baseStatus = {
  enabled: true,
  captcha_required: false,
  checked_in_today: false,
  today_reward: 0,
  today_date: '2026-10-26',
  streak_days: 3,
  next_reset_at: '2026-10-27T00:00:00+08:00',
  reward_min: 0.1,
  reward_max: 5,
  recent_records: [{ date: '2026-10-25', reward: 0.42 }],
}

// 2026-10-01 是周四，配合周一起始的网格应有 3 个前导空格。
const octoberCalendar = {
  month: '2026-10',
  today: '2026-10-26',
  days: [
    { date: '2026-10-24', reward: 0.31 },
    { date: '2026-10-25', reward: 0.42 },
  ],
  checked_days: 2,
  total_reward: 0.73,
}

const septemberCalendar = {
  month: '2026-09',
  today: '2026-10-26',
  days: [{ date: '2026-09-30', reward: 1.5 }],
  checked_days: 1,
  total_reward: 1.5,
}

function mountView() {
  return mount(CheckInView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        CaptchaChallenge: true,
      },
    },
  })
}

describe('CheckInView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appStore.publicSettingsLoaded = true
    getCheckInStatus.mockResolvedValue({ ...baseStatus })
    getCheckInCalendar.mockImplementation((month?: string) =>
      Promise.resolve(month === '2026-09' ? { ...septemberCalendar } : { ...octoberCalendar }),
    )
    performCheckIn.mockResolvedValue({ reward: 1.2345, balance: 11.23, check_in_date: '2026-10-26', streak_days: 4 })
    refreshUser.mockResolvedValue({})
  })

  it('shows a friendly empty state when check-in is disabled', async () => {
    getCheckInStatus.mockResolvedValue({ ...baseStatus, enabled: false })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="check-in-disabled"]').text()).toContain('checkIn.disabledTitle')
    expect(wrapper.find('[data-testid="check-in-button"]').exists()).toBe(false)
  })

  it('renders the already checked-in state and hides the action button', async () => {
    getCheckInStatus.mockResolvedValue({ ...baseStatus, checked_in_today: true, today_reward: 1.25 })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="check-in-today-reward"]').text()).toContain('$1.25')
    expect(wrapper.find('[data-testid="check-in-button"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="check-in-today-done"]').exists()).toBe(true)
  })

  it('never exposes tier weights or probabilities', async () => {
    const wrapper = mountView()
    await flushPromises()

    const text = wrapper.text()
    expect(text).not.toContain('%')
    // 只保留聚合后的奖励区间提示
    expect(wrapper.get('[data-testid="check-in-range"]').text()).toContain('checkIn.rewardRange')
  })

  it('renders a month calendar with signed days highlighted', async () => {
    const wrapper = mountView()
    await flushPromises()

    const grid = wrapper.get('[data-testid="calendar-grid"]')
    const cells = grid.findAll('[data-date]')
    expect(cells).toHaveLength(31) // 2026-10 有 31 天
    expect(wrapper.get('[data-testid="check-in-calendar"]').text()).toContain('checkIn.calendarSummary')

    // 已签到的日期带标记并显示当天金额，未签到的普通日期没有
    const signed = grid.findAll('[data-signed="true"]')
    expect(signed.map((cell) => cell.attributes('data-date'))).toEqual(['2026-10-24', '2026-10-25'])
    expect(grid.get('[data-date="2026-10-25"]').text()).toContain('$0.42')
    // 今天用描边标出
    expect(grid.get('[data-date="2026-10-26"]').classes().join(' ')).toContain('ring-2')
  })

  it('can page back to the previous month', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="calendar-prev"]').trigger('click')
    await flushPromises()

    expect(getCheckInCalendar).toHaveBeenLastCalledWith('2026-09')
    const cells = wrapper.get('[data-testid="calendar-grid"]').findAll('[data-date]')
    expect(cells).toHaveLength(30) // 2026-09 有 30 天
    expect(cells[29].attributes('data-date')).toBe('2026-09-30')
  })

  it('checks in successfully, refreshes status and the user balance', async () => {
    getCheckInStatus
      .mockResolvedValueOnce({ ...baseStatus })
      .mockResolvedValueOnce({ ...baseStatus, checked_in_today: true, today_reward: 1.2345, streak_days: 4 })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="check-in-button"]').trigger('click')
    await flushPromises()

    expect(performCheckIn).toHaveBeenCalledWith({})
    expect(refreshUser).toHaveBeenCalledWith({ fresh: true })
    expect(getCheckInStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="check-in-loading"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="check-in-today-done"]').exists()).toBe(true)
    expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('checkIn.successDescription'))
  })

  it('handles CHECK_IN_ALREADY_DONE by notifying and refreshing status', async () => {
    getCheckInStatus
      .mockResolvedValueOnce({ ...baseStatus })
      .mockResolvedValueOnce({ ...baseStatus, checked_in_today: true, today_reward: 0.5 })
    performCheckIn.mockRejectedValueOnce({ reason: 'CHECK_IN_ALREADY_DONE' })
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="check-in-button"]').trigger('click')
    await flushPromises()

    expect(showInfo).toHaveBeenCalledWith('checkIn.errors.alreadyDone')
    expect(getCheckInStatus).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="check-in-today-done"]').exists()).toBe(true)
  })
})
