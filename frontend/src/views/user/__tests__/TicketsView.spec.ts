import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getTicketConfig, listTickets, getTicketUnreadCount, getTicketNotifyPreferences, listTicketAgentKeys, showSuccess, showError, showInfo, showWarning } = vi.hoisted(() => ({
  getTicketConfig: vi.fn(),
  listTickets: vi.fn(),
  getTicketUnreadCount: vi.fn(),
  getTicketNotifyPreferences: vi.fn(),
  listTicketAgentKeys: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showWarning: vi.fn(),
}))

vi.mock('@/api/tickets', () => ({
  getTicketConfig,
  listTickets,
  getTicketUnreadCount,
  createTicket: vi.fn(),
  getTicket: vi.fn(),
  sendTicketMessage: vi.fn(),
  closeTicket: vi.fn(),
  reopenTicket: vi.fn(),
  getTicketNotifyPreferences,
  updateTicketNotifyPreferences: vi.fn(),
  sendTicketNotifyTest: vi.fn(),
  listTicketAgentKeys,
  createTicketAgentKey: vi.fn(),
  updateTicketAgentKey: vi.fn(),
  deleteTicketAgentKey: vi.fn(),
}))

const appStore = {
  publicSettingsLoaded: true,
  cachedPublicSettings: { ticket_system_enabled: true },
  showSuccess,
  showError,
  showInfo,
  showWarning,
}

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 1, role: 'user' } }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  // vitest 下 i18n 被别名到 runtime 版本，`t()` 原样返回 key；带参数时拼接参数值，
  // 便于断言提示词里拼进了站点 origin / docs 地址。
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? [key, ...Object.values(params).map(value => String(value))].join(' ') : key,
      locale: { value: 'en-US' },
    }),
  }
})

const routerPush = vi.fn()
vi.mock('vue-router', () => ({
  useRouter: () => ({ push: routerPush }),
  useRoute: () => ({ params: {} }),
  RouterLink: { props: ['to'], template: '<a :href="String(to)"><slot /></a>' },
}))

import TicketsView from '../TicketsView.vue'

const baseConfig = {
  enabled: true,
  agent_enabled: true,
  categories: ['general', 'billing', 'technical', 'account', 'feature', 'other'],
  priorities: ['low', 'normal', 'high', 'urgent'],
  max_subject_length: 200,
  max_body_length: 5000,
  max_open_tickets_per_user: 5,
  agent_default_qps: 2,
  agent_max_qps: 10,
  agent_user_total_qps: 5,
  agent_max_keys_per_user: 5,
}

function ticketSummary(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    ticket_no: 'TK20261106A1B2C3',
    subject: '无法调用模型',
    category: 'technical',
    priority: 'high',
    status: 'pending_admin',
    user_id: 1,
    assignee_id: null,
    unread: false,
    message_count: 2,
    last_message_at: '2026-11-06T10:00:00Z',
    last_message_by: 'user',
    created_at: '2026-11-06T09:00:00Z',
    updated_at: '2026-11-06T10:00:00Z',
    closed_at: null,
    ...overrides,
  }
}

function mountView() {
  return mount(TicketsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: true,
      },
    },
  })
}

describe('TicketsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appStore.cachedPublicSettings = { ticket_system_enabled: true }
    getTicketUnreadCount.mockResolvedValue(0)
    getTicketNotifyPreferences.mockResolvedValue({
      email_enabled: true,
      email: 'me@example.com',
      feishu_enabled: false,
      feishu_webhook_configured: false,
      feishu_webhook_url_masked: '',
      feishu_secret_configured: false,
    })
    listTicketAgentKeys.mockResolvedValue([])
    listTickets.mockResolvedValue({ items: [ticketSummary()], total: 1, page: 1, page_size: 10, pages: 1 })
  })

  it('shows the disabled state when the ticket system is off', async () => {
    getTicketConfig.mockResolvedValue({ ...baseConfig, enabled: false })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="tickets-disabled"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="tickets-tab-list"]').exists()).toBe(false)
    // 关闭时不请求列表
    expect(listTickets).not.toHaveBeenCalled()
  })

  it('renders the three tabs and the ticket list, including the unread marker', async () => {
    getTicketConfig.mockResolvedValue({ ...baseConfig })
    listTickets.mockResolvedValue({
      items: [ticketSummary({ unread: true }), ticketSummary({ id: 2, subject: '账单疑问', unread: false })],
      total: 2,
      page: 1,
      page_size: 10,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="tickets-tab-list"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="tickets-tab-notify"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="tickets-tab-agent"]').exists()).toBe(true)

    expect(listTickets).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('[data-testid="ticket-row-1"]').length).toBe(1)
    expect(wrapper.findAll('[data-testid="ticket-unread-dot"]').length).toBe(1)
    // 纯文本渲染：标题原样输出
    expect(wrapper.text()).toContain('无法调用模型')
  })

  it('lazy-mounts the notification and agent tabs only after they are selected', async () => {
    getTicketConfig.mockResolvedValue({ ...baseConfig })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="notify-email"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="agent-prompt"]').exists()).toBe(false)

    await wrapper.get('[data-testid="tickets-tab-notify"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="notify-email"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notify-feishu-webhook"]').exists()).toBe(true)

    await wrapper.get('[data-testid="tickets-tab-agent"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="agent-prompt"]').exists()).toBe(true)
    expect(listTicketAgentKeys).toHaveBeenCalledTimes(1)
    // 提示词里带上当前站点 origin 与 docs.md 地址
    const prompt = wrapper.get('[data-testid="agent-prompt-text"]').text()
    expect(prompt).toContain(window.location.origin)
    expect(prompt).toContain('/agent/docs.md')
    expect(prompt).toContain('lxt_REPLACE_WITH_YOUR_KEY')
  })
})
