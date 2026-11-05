import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const mocks = vi.hoisted(() => ({ getUserBalanceHistory: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/utils/format', () => ({ formatDateTime: () => 'date' }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => (params ? `${key}(${Object.values(params).join(',')})` : key)
  })
}))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
function result(id: number) {
  return { items: [{ id, type: 'admin_balance', value: id, notes: `History ${id}` }], total: 1, total_recharged: id }
}
function deferred() {
  let resolve!: (value: ReturnType<typeof result>) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<ReturnType<typeof result>>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
async function openDialog() {
  const wrapper = mount(UserBalanceHistoryModal, {
    props: { show: false, user: { id: 1, email: 'one@example.com', balance: 1 } as AdminUser },
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Icon: true, Select: true } }
  })
  await wrapper.setProps({ show: true })
  return wrapper
}

describe('UserBalanceHistoryModal request ordering', () => {
  it('keeps the new user history when an old response finishes later', async () => {
    const old = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockResolvedValueOnce(result(20))
    const wrapper = await openDialog()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, user: { id: 2, email: 'two@example.com', balance: 2 } as AdminUser })
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.text()).toContain('History 20')
    expect(wrapper.text()).not.toContain('History 10')
    expect(wrapper.text()).toContain('$20.00')
  })

  it('does not end the current filter loading state when an old request finishes', async () => {
    const old = deferred()
    const current = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = await openDialog()
    wrapper.findComponent({ name: 'Select' }).vm.$emit('change', 'balance')
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('History 10')
    current.resolve(result(20))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
    expect(wrapper.text()).toContain('History 20')
  })

  it.each(['close', 'unmount'])('ignores failures after %s', async (action) => {
    const pending = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(pending.promise)
    const wrapper = await openDialog()
    if (action === 'close') await wrapper.setProps({ show: false })
    else wrapper.unmount()
    pending.reject(new Error('Stale error'))
    await flushPromises()
    expect(console.error).not.toHaveBeenCalled()
  })

  it('still reports current failures and ends loading', async () => {
    const error = new Error('Current error')
    mocks.getUserBalanceHistory.mockRejectedValueOnce(error)
    const wrapper = await openDialog()
    await flushPromises()
    expect(console.error).toHaveBeenCalledWith('Failed to load balance history:', error)
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
  })
})

describe('UserBalanceHistoryModal check-in credit', () => {
  const checkIn = { id: -5, type: 'check_in', code: 'CHECKIN-2026-10-27', value: 0.2783, notes: '', used_at: '2026-10-27T00:31:05Z', created_at: '2026-10-27T00:31:05Z' }
  const affiliate = { id: -5, type: 'affiliate_balance', code: 'AFF-5', value: 3, notes: '', used_at: '2026-10-26T10:00:00Z', created_at: '2026-10-26T10:00:00Z' }

  it('labels check-in credit distinctly from recharges', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({ items: [checkIn, affiliate], total: 2, total_recharged: 0 })
    const wrapper = await openDialog()
    await flushPromises()

    // 签到与返利都使用负数 ID，必须两条都渲染（列表 key 带类型）
    expect(wrapper.text()).toContain('redeem.balanceAddedCheckIn')
    expect(wrapper.text()).toContain('redeem.balanceAddedAffiliate')
    // 来源标注签到额度与签到日期，小额奖励保留 4 位小数
    expect(wrapper.text()).toContain('redeem.checkInSource(2026-10-27)')
    expect(wrapper.text()).toContain('+$0.2783')
    expect(wrapper.text()).toContain('+$3.00')
    // 签到额度使用日历图标与琥珀色，和充值的绿色美元图标区分
    const icons = wrapper.findAllComponents({ name: 'Icon' })
    expect(icons.map((icon) => icon.attributes('name'))).toEqual(expect.arrayContaining(['calendar', 'dollar']))
    expect(wrapper.find('p.text-amber-600').text()).toBe('+$0.2783')
  })

  it('offers check-in credit as its own type filter', async () => {
    mocks.getUserBalanceHistory.mockResolvedValue({ items: [], total: 0, total_recharged: 0 })
    const wrapper = await openDialog()
    await flushPromises()
    const options = wrapper.findComponent({ name: 'Select' }).props('options') as Array<{ value: string; label: string }>
    expect(options).toContainEqual({ value: 'check_in', label: 'admin.users.typeCheckIn' })

    const select = wrapper.findComponent({ name: 'Select' })
    select.vm.$emit('update:modelValue', 'check_in')
    select.vm.$emit('change', 'check_in')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 1, expect.any(Number), 'check_in')
  })
})

describe('UserBalanceHistoryModal task reward credit', () => {
  const taskReward = {
    id: -42,
    type: 'task_reward',
    code: 'TASK-42',
    value: 0.5,
    notes: '浏览 XX 官网 30 秒',
    used_at: '2026-11-05T10:00:30Z',
    created_at: '2026-11-05T10:00:30Z',
  }
  const checkIn = { id: -5, type: 'check_in', code: 'CHECKIN-2026-11-05', value: 0.2783, notes: '', used_at: '2026-11-05T00:31:05Z', created_at: '2026-11-05T00:31:05Z' }

  it('labels task rewards distinctly from recharges and check-ins', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({ items: [taskReward, checkIn], total: 2, total_recharged: 0 })
    const wrapper = await openDialog()
    await flushPromises()

    // 任务奖励与签到都使用负数 ID，两条都要渲染
    expect(wrapper.text()).toContain('admin.rewardTasks.balanceHistory.title')
    expect(wrapper.text()).toContain('redeem.balanceAddedCheckIn')
    // 来源显示任务标题（notes），且不在上方 notes 行重复展示（整页只出现一次）
    expect(wrapper.text()).toContain('admin.rewardTasks.balanceHistory.source(浏览 XX 官网 30 秒)')
    expect(wrapper.text().split('浏览 XX 官网 30 秒')).toHaveLength(2)
    // 小额奖励最多保留 4 位小数
    expect(wrapper.text()).toContain('+$0.50')
    // 任务奖励使用奖杯图标与天蓝色，和签到的日历图标 + 琥珀色区分
    const icons = wrapper.findAllComponents({ name: 'Icon' })
    expect(icons.map((icon) => icon.attributes('name'))).toEqual(expect.arrayContaining(['trophy', 'calendar']))
    expect(wrapper.find('p.text-sky-600').text()).toBe('+$0.50')
    expect(wrapper.find('p.text-amber-600').text()).toBe('+$0.2783')
  })

  it('falls back to a plain source label when the task title snapshot is empty', async () => {
    mocks.getUserBalanceHistory.mockResolvedValueOnce({
      items: [{ ...taskReward, notes: '' }],
      total: 1,
      total_recharged: 0,
    })
    const wrapper = await openDialog()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.rewardTasks.balanceHistory.sourceFallback')
  })

  it('offers task reward as its own type filter', async () => {
    mocks.getUserBalanceHistory.mockResolvedValue({ items: [], total: 0, total_recharged: 0 })
    const wrapper = await openDialog()
    await flushPromises()
    const options = wrapper.findComponent({ name: 'Select' }).props('options') as Array<{ value: string; label: string }>
    expect(options).toContainEqual({ value: 'task_reward', label: 'admin.rewardTasks.balanceHistory.typeTaskReward' })

    const select = wrapper.findComponent({ name: 'Select' })
    select.vm.$emit('update:modelValue', 'task_reward')
    select.vm.$emit('change', 'task_reward')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 1, expect.any(Number), 'task_reward')
  })
})
