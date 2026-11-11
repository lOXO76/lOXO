import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RedeemView from '../RedeemView.vue'

const { redeem, getHistory, refreshUser, fetchActiveSubscriptions, showError, showWarning, showSuccess } = vi.hoisted(() => ({
  redeem: vi.fn(),
  getHistory: vi.fn(),
  refreshUser: vi.fn(),
  fetchActiveSubscriptions: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  redeemAPI: { redeem, getHistory },
  authAPI: { getPublicSettings: vi.fn().mockResolvedValue({}) },
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { balance: 10, concurrency: 2 }, refreshUser }),
}))
vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({ fetchActiveSubscriptions }),
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountView() {
  return mount(RedeemView, {
    global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
  })
}

async function submitCode() {
  const wrapper = mountView()
  await flushPromises()
  await wrapper.get('input#code').setValue('SPRING-GIFT-2026')
  await wrapper.get('form').trigger('submit')
  await flushPromises()
  return wrapper
}

describe('RedeemView multi-use codes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getHistory.mockResolvedValue({ items: [], total: 0 })
    refreshUser.mockResolvedValue({ balance: 15, concurrency: 2 })
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // 这两类错误说明兑换码本身有效，不能提示用户「检查兑换码后重试」。
  it.each([
    ['REDEEM_CODE_USER_LIMIT_REACHED', 'redeem.errors.userLimitReached'],
    ['REDEEM_CODE_USER_QUOTA_EXHAUSTED', 'redeem.errors.userQuotaExhausted'],
  ])('explains the %s limit instead of asking to check the code', async (reason, messageKey) => {
    redeem.mockRejectedValue({ status: 409, code: 409, reason, message: 'limit reached' })
    const wrapper = await submitCode()

    expect(wrapper.text()).toContain(messageKey)
    expect(wrapper.text()).not.toContain('redeem.failedToRedeem')
    expect(showError).toHaveBeenCalledWith('redeem.redeemFailed')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(refreshUser).not.toHaveBeenCalled()
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('SPRING-GIFT-2026')
    wrapper.unmount()
  })

  it('keeps the generic failure message for other errors', async () => {
    redeem.mockRejectedValue({ status: 409, code: 409, reason: 'REDEEM_CODE_USED', message: 'redeem code already used' })
    const wrapper = await submitCode()

    expect(wrapper.text()).toContain('redeem.failedToRedeem')
    expect(wrapper.text()).not.toContain('redeem.errors.')
    wrapper.unmount()
  })

  it('shows a multi-use redemption like a regular balance redemption', async () => {
    redeem.mockResolvedValue({ type: 'balance', value: 5, message: 'Code applied', parent_code: 'SPRING-GIFT-2026' })
    getHistory.mockResolvedValue({
      total: 1,
      items: [{
        id: 1,
        code: 'f3a9c1d2e4b5a6978877665544332211',
        parent_code: 'SPRING-GIFT-2026',
        type: 'balance',
        value: 5,
        used_at: '2026-03-08T00:00:00Z',
      }],
    })
    const wrapper = await submitCode()

    expect(redeem).toHaveBeenCalledWith('SPRING-GIFT-2026')
    expect(showSuccess).toHaveBeenCalledWith('redeem.codeRedeemSuccess')
    expect(wrapper.text()).toContain('redeem.balanceAddedRedeem')
    expect(wrapper.text()).toContain('+$5.00')
    // 历史里显示用户实际输入的兑换码，而不是内部流水号。
    expect(wrapper.text()).toContain('SPRING-G...')
    expect(wrapper.text()).not.toContain('f3a9c1d2...')
    wrapper.unmount()
  })
})
