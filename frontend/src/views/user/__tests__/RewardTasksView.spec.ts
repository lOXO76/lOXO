import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import RewardTasksView from '../RewardTasksView.vue'
import type { RewardTaskCurrent, UserRewardTask } from '@/api/rewardTasks'

const { listRewardTasks, startRewardTask, claimRewardTask, listMyRewardTaskRecords, refreshUser, showError, showSuccess, showInfo } = vi.hoisted(() => ({
  listRewardTasks: vi.fn(),
  startRewardTask: vi.fn(),
  claimRewardTask: vi.fn(),
  listMyRewardTaskRecords: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

vi.mock('@/api/rewardTasks', () => ({ listRewardTasks, startRewardTask, claimRewardTask, listMyRewardTaskRecords }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess, showInfo }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key} ${JSON.stringify(params)}` : key,
      te: (key: string) => /^rewardTasks\.errors\.(REWARD_|ENTERPRISE_)/.test(key),
      locale: { value: 'en-US' },
    }),
  }
})

const now = new Date('2026-11-05T10:00:00Z').getTime()
const future = new Date(now + 3600_000).toISOString()
const past = new Date(now - 1000).toISOString()

function current(overrides: Partial<RewardTaskCurrent> = {}): RewardTaskCurrent {
  return {
    status: 'in_progress', period_key: 'once', started_at: past, ticket_expires_at: future,
    claimable_at: past, progress_seconds: 12, recharge_total: 30, completed_at: null, reward_amount: 0,
    ...overrides,
  }
}

function task(overrides: Partial<UserRewardTask> = {}): UserRewardTask {
  return {
    id: 1, title: 'Visit a partner', description: 'Read their introduction', task_type: 'browse', verify_mode: 'timer',
    target_url: 'https://example.com/path', icon_url: '', reward_amount: 0.5, period: 'once',
    browse_seconds: 30, idle_timeout_seconds: 15, min_recharge_amount: 0, ends_at: null, quota_exhausted: false, next_reset_at: null,
    current: null, ...overrides,
  }
}

function taskList(tasks: UserRewardTask[] = [task()], enabled = true, count = 0) {
  return { enabled, tasks, summary: { completed_count: count, total_reward: count * 0.5 } }
}

const wrappers: VueWrapper[] = []
function mountView() {
  const wrapper = mount(RewardTasksView, {
    global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Pagination: true } },
  })
  wrappers.push(wrapper)
  return wrapper
}

function visibility(state: 'hidden' | 'visible') {
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: state })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('RewardTasksView', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval', 'setTimeout', 'clearTimeout'] })
    vi.setSystemTime(now)
    vi.clearAllMocks()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
    listRewardTasks.mockResolvedValue(taskList())
    listMyRewardTaskRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    startRewardTask.mockResolvedValue({ jump_url: 'https://example.com/start', current: current() })
    claimRewardTask.mockResolvedValue({ reward_amount: 0.5, balance: 10, current: current({ status: 'completed', reward_amount: 0.5 }) })
    refreshUser.mockResolvedValue({})
    vi.spyOn(window, 'open').mockReturnValue({ location: { href: '' }, opener: window, close: vi.fn() } as unknown as Window)
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('shows a disabled empty state and lifetime totals', async () => {
    listRewardTasks.mockResolvedValue(taskList([], false, 3))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="tasks-disabled"]').text()).toContain('rewardTasks.disabledTitle')
    expect(wrapper.get('[data-testid="total-reward"]').text()).toContain('$1.50')
    expect(wrapper.find('[data-testid="task-1"]').exists()).toBe(false)
  })

  it('shows loading errors with a working retry, and the enabled empty state', async () => {
    listRewardTasks.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce(taskList([]))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="tasks-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="tasks-empty"]').text()).toContain('rewardTasks.emptyTitle')
  })

  it('renders initial, expired, quota, completed and automatic-mode actions', async () => {
    listRewardTasks.mockResolvedValue(taskList([
      task(),
      task({ id: 2, current: current({ ticket_expires_at: past }) }),
      task({ id: 3, quota_exhausted: true }),
      task({ id: 4, current: current({ status: 'completed', reward_amount: 0.1234 }), period: 'daily', next_reset_at: future }),
      task({ id: 5, verify_mode: 'sdk', current: current() }),
      task({ id: 6, task_type: 'register', verify_mode: 'callback', current: current() }),
      task({ id: 7, task_type: 'recharge', verify_mode: 'callback', min_recharge_amount: 100, current: current() }),
    ]))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="start-1"]').text()).toContain('rewardTasks.actions.start')
    expect(wrapper.get('[data-testid="task-2"]').text()).toContain('rewardTasks.expired')
    expect(wrapper.get('[data-testid="start-2"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="start-3"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="task-4"]').text()).toContain('$0.1234')
    expect(wrapper.get('[data-testid="task-4"]').text()).toContain('rewardTasks.nextReset')
    expect(wrapper.get('[data-testid="task-5"] [role="progressbar"]').attributes('aria-valuenow')).toBe('12')
    expect(wrapper.get('[data-testid="start-5"]').text()).toContain('rewardTasks.actions.continueBrowsing')
    expect(wrapper.get('[data-testid="task-6"]').text()).toContain('rewardTasks.registerHint')
    expect(wrapper.get('[data-testid="task-7"]').text()).toContain('rewardTasks.rechargeProgress')
    expect(wrapper.get('[data-testid="start-7"]').text()).toContain('rewardTasks.actions.continue')
  })

  it.each([15, 45])('shows the configured SDK inactivity threshold of %s seconds', async (seconds) => {
    listRewardTasks.mockResolvedValue(taskList([task({ verify_mode: 'sdk', idle_timeout_seconds: seconds, current: current() })]))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="task-1"]').text()).toContain(`rewardTasks.sdkHint {"seconds":${seconds}}`)
    expect(wrapper.get('[data-testid="task-1"]').text()).not.toContain('rewardTasks.sdkHintNoIdle')
  })

  it('uses the no-idle SDK hint when inactivity detection is disabled', async () => {
    listRewardTasks.mockResolvedValue(taskList([task({ verify_mode: 'sdk', idle_timeout_seconds: 0, current: current() })]))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="task-1"]').text()).toContain('rewardTasks.sdkHintNoIdle')
    expect(wrapper.get('[data-testid="task-1"]').text()).not.toContain('"seconds":0')
  })

  it('counts down from the server claimable_at rather than locally restarting the timer', async () => {
    listRewardTasks.mockResolvedValue(taskList([task({ current: current({ claimable_at: new Date(now + 2500).toISOString() }) })]))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="claim-1"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="claim-1"]').text()).toContain('3')
    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.get('[data-testid="claim-1"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="claim-1"]').text()).toContain('rewardTasks.actions.claim')
    expect(wrapper.get('[data-testid="start-1"]').text()).toContain('rewardTasks.actions.reopen')
  })

  it('opens a blank window synchronously, then navigates it using start.jump_url', async () => {
    let resolveStart!: (value: unknown) => void
    startRewardTask.mockImplementation(() => new Promise(resolve => { resolveStart = resolve }))
    const wrapper = mountView()
    await flushPromises()
    const popup = window.open('about:blank', '_blank')!
    vi.mocked(window.open).mockClear()
    await wrapper.get('[data-testid="start-1"]').trigger('click')
    expect(window.open).toHaveBeenCalledWith('about:blank', '_blank')
    expect(vi.mocked(window.open).mock.invocationCallOrder[0]).toBeLessThan(startRewardTask.mock.invocationCallOrder[0])
    expect(popup.location.href).toBe('')
    resolveStart({ jump_url: 'https://example.com/?reward_ticket=rt_abc', current: current() })
    await flushPromises()
    expect(popup.location.href).toContain('reward_ticket=rt_abc')
    expect(popup.opener).toBeNull()
  })

  it('offers a clickable safe fallback when a popup is blocked, without navigating this page', async () => {
    vi.mocked(window.open).mockReturnValue(null)
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="start-1"]').trigger('click')
    await flushPromises()
    const link = wrapper.get('[data-testid="blocked-link-1"]')
    expect(link.attributes('href')).toBe('https://example.com/start')
    expect(link.attributes('rel')).toContain('noopener')
    expect(showInfo).toHaveBeenCalledWith('rewardTasks.popupBlocked')
  })

  it('keeps the start result when a background refresh replaces the original task object', async () => {
    let resolveStart!: (value: unknown) => void
    startRewardTask.mockImplementation(() => new Promise(resolve => { resolveStart = resolve }))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="start-1"]').trigger('click')
    listRewardTasks.mockResolvedValue(taskList([task()]))
    visibility('visible')
    await flushPromises()
    resolveStart({ jump_url: 'https://example.com/start', current: current() })
    await flushPromises()
    expect(wrapper.get('[data-testid="claim-1"]').text()).toContain('rewardTasks.actions.claim')
  })

  it('rejects non-http jump URLs and closes the unused window', async () => {
    startRewardTask.mockResolvedValueOnce({ jump_url: 'javascript:alert(1)', current: current() })
    const wrapper = mountView()
    await flushPromises()
    const popup = vi.mocked(window.open).getMockImplementation()!('about:blank', '_blank')!
    await wrapper.get('[data-testid="start-1"]').trigger('click')
    await flushPromises()
    expect(popup.location.href).toBe('')
    expect(popup.close).toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('rewardTasks.errors.invalidJumpUrl')
  })

  it('claims a reward, refreshes records and balance, and keeps the completed result', async () => {
    listRewardTasks.mockResolvedValueOnce(taskList([task({ current: current() })]))
      .mockResolvedValue(taskList([task({ current: current({ status: 'completed', reward_amount: 0.5 }) })], true, 1))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="claim-1"]').trigger('click')
    await flushPromises()
    expect(claimRewardTask).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledWith(expect.stringContaining('$0.50'))
    expect(refreshUser).toHaveBeenCalledWith({ fresh: true })
    expect(listMyRewardTaskRecords).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="claim-1"]').exists()).toBe(false)
  })

  it.each([
    'REWARD_TASKS_DISABLED', 'REWARD_TASK_NOT_FOUND', 'REWARD_TASK_UNAVAILABLE', 'REWARD_TASK_ALREADY_COMPLETED',
    'REWARD_TASK_QUOTA_EXHAUSTED', 'REWARD_TASK_NOT_STARTED', 'REWARD_TASK_TIMER_NOT_ELAPSED',
    'REWARD_TASK_CLAIM_NOT_SUPPORTED', 'REWARD_TICKET_EXPIRED', 'ENTERPRISE_MEMBER_WALLET_DISABLED',
  ])('maps %s to a translated error and closes the unused window', async (reason) => {
    startRewardTask.mockRejectedValueOnce({ reason, metadata: { remaining_seconds: 8 } })
    const wrapper = mountView()
    await flushPromises()
    const popup = vi.mocked(window.open).getMockImplementation()!('about:blank', '_blank')!
    await wrapper.get('[data-testid="start-1"]').trigger('click')
    await flushPromises()
    expect(showError).toHaveBeenCalledWith(expect.stringContaining(`rewardTasks.errors.${reason}`))
    expect(popup.close).toHaveBeenCalled()
  })

  it('polls automatic tasks every ten seconds, pauses when hidden and refreshes on return', async () => {
    listRewardTasks.mockResolvedValue(taskList([task({ verify_mode: 'sdk', current: current() })]))
    const wrapper = mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(listRewardTasks).toHaveBeenCalledTimes(2)
    visibility('hidden')
    await vi.advanceTimersByTimeAsync(30_000)
    expect(listRewardTasks).toHaveBeenCalledTimes(2)
    visibility('visible')
    await flushPromises()
    expect(listRewardTasks).toHaveBeenCalledTimes(3)
    wrapper.unmount()
    wrappers.splice(wrappers.indexOf(wrapper), 1)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(listRewardTasks).toHaveBeenCalledTimes(3)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('refreshes the balance and records after an automatic completion appears', async () => {
    listRewardTasks.mockResolvedValueOnce(taskList([task({ verify_mode: 'sdk', current: current() })]))
      .mockResolvedValue(taskList([task({ verify_mode: 'sdk', current: current({ status: 'completed' }) })], true, 1))
    mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(10_000)
    await flushPromises()
    expect(refreshUser).toHaveBeenCalledWith({ fresh: true })
    expect(listMyRewardTaskRecords).toHaveBeenCalledTimes(2)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not poll timer-only tasks or automatically claim an elapsed timer', async () => {
    listRewardTasks.mockResolvedValue(taskList([task({ current: current() })]))
    mountView()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(20_000)
    expect(listRewardTasks).toHaveBeenCalledTimes(1)
    expect(claimRewardTask).not.toHaveBeenCalled()
  })

  it('loads another records page and renders descriptions strictly as text', async () => {
    listRewardTasks.mockResolvedValue(taskList([task({ description: '<img src=x onerror=alert(1)>' })]))
    listMyRewardTaskRecords.mockResolvedValue({
      items: [{ id: 1, task_id: 1, task_title: 'Historical title', task_type: 'browse', period_key: 'once', reward_amount: 0.0001, completed_at: past }],
      total: 21, page: 1, page_size: 20, pages: 2,
    })
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('[data-testid="task-1"]').text()).toContain('<img src=x onerror=alert(1)>')
    expect(wrapper.get('[data-testid="task-1"]').find('img').exists()).toBe(false)
    expect(wrapper.get('[data-testid="task-records"]').text()).toContain('$0.0001')
    wrapper.getComponent({ name: 'Pagination' }).vm.$emit('update:page', 2)
    await flushPromises()
    expect(listMyRewardTaskRecords).toHaveBeenLastCalledWith(2, 20)
  })
})
