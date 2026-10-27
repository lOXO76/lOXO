import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import CheckInStatsView from '../CheckInStatsView.vue'

const mocks = vi.hoisted(() => ({
  getStats: vi.fn(),
  listRecords: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/checkIns', () => {
  const api = { getStats: mocks.getStats, listRecords: mocks.listRecords }
  return { checkInsAPI: api, default: api }
})
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => `dt:${value}` }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key}(${Object.values(params).join(',')})` : key),
    }),
  }
})

const stats = {
  days: 30,
  summary: {
    today_date: '2026-10-27',
    today_users: 12,
    today_reward: 3.4567,
    last_7_days_users: 40,
    total_check_ins: 321,
    total_users: 88,
    total_reward: 120.5,
    average_reward: 0.3754,
    period_check_ins: 200,
    period_reward: 75.25,
    period_start_date: '2026-09-28',
    check_in_enabled: true,
    captcha_enabled: false,
    configured_tiers: 2,
    configured_reward_min: 0.1,
    configured_reward_max: 5,
  },
  daily: [
    { date: '2026-10-26', users: 10, reward: 3.1 },
    { date: '2026-10-27', users: 12, reward: 3.4567 },
  ],
  tiers: [
    { tier_index: 0, count: 140, reward: 42, actual_percent: 70, observed_min: 0.1, observed_max: 0.49, configured_min: 0.1, configured_max: 0.5, configured_percent: 70 },
    { tier_index: 1, count: 0, reward: 0, actual_percent: 0, observed_min: 0, observed_max: 0, configured_min: 0.5, configured_max: 5, configured_percent: 30 },
    { tier_index: 3, count: 60, reward: 33.25, actual_percent: 30, observed_min: 0.5, observed_max: 0.6, configured_min: null, configured_max: null, configured_percent: null },
  ],
}

const records = {
  items: [
    { id: 9, user_id: 5, user_email: 'a@example.com', username: 'alice', check_in_date: '2026-10-27', reward: 0.2783, tier_index: 0, created_at: '2026-10-27T00:31:05Z' },
    { id: 8, user_id: 6, user_email: '', username: '', check_in_date: '2026-10-26', reward: 2.5, tier_index: 1, created_at: '2026-10-26T08:00:00Z' },
  ],
  total: 2,
  page: 1,
  page_size: 20,
  pages: 1,
}

function mountView() {
  return mount(CheckInStatsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        RouterLink: RouterLinkStub,
        CheckInDailyChart: { props: ['title', 'values'], template: '<div class="chart-stub">{{ title }}:{{ values.join(",") }}</div>' },
        DataTable: {
          props: ['data', 'columns'],
          template: `<div>
            <span v-for="column in columns" :key="column.key" class="column">{{ column.label }}</span>
            <div v-for="row in data" :key="row.id" class="record-row">
              <slot name="cell-user" :row="row" /><slot name="cell-reward" :row="row" /><slot name="cell-tier_index" :row="row" />
            </div>
          </div>`,
        },
      },
    },
  })
}

describe('CheckInStatsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    mocks.getStats.mockResolvedValue(stats)
    mocks.listRecords.mockResolvedValue(records)
  })
  afterEach(() => vi.useRealTimers())

  it('renders summary tiles and labels amounts as check-in credit', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(mocks.getStats).toHaveBeenCalledWith(30)
    const summary = wrapper.get('[data-testid="check-in-summary"]').text()
    expect(summary).toContain('admin.checkIns.summary.todayUsers')
    expect(summary).toContain('12')
    expect(summary).toContain('admin.checkIns.summary.todayReward $3.46')
    expect(summary).toContain('$75.25')
    expect(summary).toContain('$120.50')
    expect(summary).toContain('admin.checkIns.summary.averageReward $0.3754')
    expect(wrapper.get('[data-testid="check-in-state"]').text()).toContain('admin.checkIns.state.configuredRange(2,$0.10,$5.00)')

    // 两张单系列趋势图（不使用双纵轴）
    expect(wrapper.get('[data-testid="chart-users"]').text()).toContain('admin.checkIns.charts.dailyUsers:10,12')
    expect(wrapper.get('[data-testid="chart-reward"]').text()).toContain('admin.checkIns.charts.dailyReward:3.1,3.4567')

    // 明细金额列明确为「签到额度」
    expect(wrapper.findAll('.column').map((node) => node.text())).toContain('admin.checkIns.records.reward')
    const rows = wrapper.findAll('.record-row')
    expect(rows[0].text()).toContain('a@example.com')
    expect(rows[0].text()).toContain('$0.2783')
    expect(rows[1].text()).toContain('admin.checkIns.records.deletedUser')
    expect(rows[1].text()).toContain('#2')
  })

  it('compares configured probability with the actual share per tier', async () => {
    const wrapper = mountView()
    await flushPromises()

    const first = wrapper.get('[data-testid="tier-row-0"]')
    expect(first.text()).toContain('#1')
    expect(first.text()).toContain('$0.10 ~ $0.50')
    expect(first.text()).toContain('70%')

    // 当前配置中但周期内未抽中的档位
    expect(wrapper.get('[data-testid="tier-row-1"]').text()).toContain('30%')
    // 已不在当前配置中的历史档位
    const legacy = wrapper.get('[data-testid="tier-row-3"]')
    expect(legacy.text()).toContain('admin.checkIns.tiers.legacy')
    expect(legacy.findAll('[title]').length).toBe(0)
  })

  it('reloads stats when the period changes', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="range-7"]').trigger('click')
    await flushPromises()
    expect(mocks.getStats).toHaveBeenLastCalledWith(7)
    expect(wrapper.get('[data-testid="range-7"]').attributes('aria-pressed')).toBe('true')
  })

  it('debounces record search and restarts from the first page', async () => {
    const wrapper = mountView()
    await flushPromises()
    vi.useFakeTimers()

    const input = wrapper.get('[data-testid="records-search"]')
    await input.setValue('ali')
    await input.trigger('input')
    await input.setValue('alice')
    await input.trigger('input')
    expect(mocks.listRecords).toHaveBeenCalledTimes(1)

    vi.advanceTimersByTime(300)
    await flushPromises()
    expect(mocks.listRecords).toHaveBeenCalledTimes(2)
    expect(mocks.listRecords).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, search: 'alice', sort_by: 'created_at', sort_order: 'desc' }))
  })
})
