import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { getTicket, getTicketConfig, sendTicketMessage, closeTicket, reopenTicket, getTicketUnreadCount, showSuccess, showError, showInfo, showWarning, testRoute } = vi.hoisted(() => ({
  getTicket: vi.fn(),
  getTicketConfig: vi.fn(),
  sendTicketMessage: vi.fn(),
  closeTicket: vi.fn(),
  reopenTicket: vi.fn(),
  getTicketUnreadCount: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showWarning: vi.fn(),
  // 路由参数需要可变 + 响应式，用 reactive 包一层（在下面的 mock 工厂里创建）。
  testRoute: { current: null as null | { params: { id: string } } },
}))

vi.mock('@/api/tickets', () => ({
  getTicket,
  getTicketConfig,
  sendTicketMessage,
  closeTicket,
  reopenTicket,
  getTicketUnreadCount,
  listTickets: vi.fn().mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 }),
  createTicket: vi.fn(),
  getTicketNotifyPreferences: vi.fn(),
  updateTicketNotifyPreferences: vi.fn(),
  sendTicketNotifyTest: vi.fn(),
  listTicketAgentKeys: vi.fn().mockResolvedValue([]),
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
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } }) }
})
vi.mock('vue-router', async () => {
  const { reactive } = await import('vue')
  const route = reactive({ params: { id: '7' } })
  testRoute.current = route
  return {
    useRoute: () => route,
    useRouter: () => ({ push: vi.fn() }),
    RouterLink: { props: ['to'], template: '<a :href="String(to)"><slot /></a>' },
  }
})

import TicketDetailView from '../TicketDetailView.vue'

const baseTicket = {
  id: 7,
  ticket_no: 'TK20261106XYZ999',
  subject: '扣费异常',
  category: 'billing',
  priority: 'normal',
  status: 'pending_user',
  user_id: 1,
  assignee_id: null,
  unread: false,
  message_count: 3,
  last_message_at: '2026-11-06T10:00:00Z',
  last_message_by: 'admin',
  created_at: '2026-11-06T09:00:00Z',
  updated_at: '2026-11-06T10:00:00Z',
  closed_at: null,
}

const messages = [
  {
    id: 1,
    ticket_id: 7,
    author_type: 'user',
    author_id: 1,
    author_name: 'me',
    via: 'web',
    internal: false,
    body: '这是一段用户内容 <script>alert(1)</script>',
    created_at: '2026-11-06T09:00:00Z',
  },
  {
    id: 2,
    ticket_id: 7,
    author_type: 'admin',
    author_id: null,
    author_name: 'Support',
    via: 'agent',
    internal: false,
    body: '已收到，正在排查。',
    created_at: '2026-11-06T09:30:00Z',
  },
  {
    id: 3,
    ticket_id: 7,
    author_type: 'system',
    author_id: null,
    author_name: '',
    via: 'system',
    internal: false,
    body: 'status_changed:resolved',
    created_at: '2026-11-06T10:00:00Z',
  },
]

function mountView() {
  return mount(TicketDetailView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: true,
      },
    },
  })
}

describe('TicketDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // 路由参数在多个用例之间共享，每个用例重置回第一张工单。
    if (testRoute.current) testRoute.current.params.id = '7'
    getTicketUnreadCount.mockResolvedValue(0)
    getTicketConfig.mockResolvedValue({
      enabled: true,
      agent_enabled: true,
      categories: ['general', 'billing', 'technical', 'account', 'feature', 'other'],
      priorities: ['low', 'normal', 'high', 'urgent'],
      max_subject_length: 200,
      max_body_length: 120,
      max_open_tickets_per_user: 5,
      agent_default_qps: 2,
      agent_max_qps: 10,
      agent_user_total_qps: 5,
      agent_max_keys_per_user: 5,
    })
    getTicket.mockResolvedValue({ ticket: { ...baseTicket }, messages: messages.map(message => ({ ...message })) })
    sendTicketMessage.mockResolvedValue(messages[0])
  })

  it('renders the chat stream as plain text, tags agent messages and localizes system events', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getTicket).toHaveBeenCalledWith(7)

    const bodies = wrapper.findAll('[data-testid="ticket-message-body"]')
    expect(bodies.length).toBe(2)
    // 禁止 v-html：脚本标签按纯文本显示
    expect(bodies[0].text()).toContain('<script>alert(1)</script>')

    // via=agent 的小标签
    expect(wrapper.findAll('[data-testid="ticket-message-agent-tag"]').length).toBe(1)

    // system 消息走 i18n key 渲染（测试环境 t() 返回 key）
    const system = wrapper.get('[data-testid="ticket-message-system"]')
    expect(system.text()).toContain('tickets.detail.systemEvents.status_changed')
  })

  it('sends a reply through the API and reloads the ticket', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="ticket-reply-input"]').setValue('补充信息：账单号 123')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(sendTicketMessage).toHaveBeenCalledWith(7, '补充信息：账单号 123')
    expect(getTicket.mock.calls.length).toBeGreaterThanOrEqual(2)
    expect((wrapper.get('[data-testid="ticket-reply-input"]').element as HTMLTextAreaElement).value).toBe('')
  })

  it('hides the reply box when the ticket is closed', async () => {
    getTicket.mockResolvedValue({
      ticket: { ...baseTicket, status: 'closed', closed_at: '2026-11-06T11:00:00Z' },
      messages: messages.map(message => ({ ...message })),
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="ticket-closed-hint"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="ticket-reply-input"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="ticket-reopen"]').exists()).toBe(true)
  })

  it('counts and clamps replies by Unicode code points', async () => {
    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.get('[data-testid="ticket-reply-input"]')
    // max_body_length = 120：121 个 emoji 超过上限，应被截断到 120 个 code point，
    // 且不能留下半个代理对（旧实现的 slice / 原生 maxlength 会切坏或提前拒绝）。
    await input.setValue('😀'.repeat(121))
    const element = input.element as HTMLTextAreaElement
    expect(Array.from(element.value).length).toBe(120)
    expect(element.value).not.toContain('\ufffd')
    expect(wrapper.get('[data-testid="ticket-reply-counter"]').text()).toBe('120/120')

    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(sendTicketMessage).toHaveBeenCalledWith(7, '😀'.repeat(120))
  })

  it('discards stale responses and clears the draft when the route id changes', async () => {
    let resolveStale: (value: unknown) => void = () => {}
    const stale = new Promise((resolve) => {
      resolveStale = resolve
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="ticket-reply-input"]').setValue('未提交的草稿')

    const stalePayload = {
      ticket: { ...baseTicket, subject: '旧数据标题' },
      messages: messages.map(message => ({ ...message })),
    }
    const nextPayload = {
      ticket: { ...baseTicket, id: 8, ticket_no: 'TK20261106NEW888', subject: '第二张工单', status: 'pending_admin' },
      messages: [
        {
          ...messages[0],
          id: 101,
          ticket_id: 8,
          body: '工单 8 的消息',
        },
      ],
    }

    getTicket.mockImplementation((id: number) => (id === 7 ? stale : Promise.resolve(nextPayload)))

    // 让旧工单（7）发起一个在途请求（页面可见时触发的轮询）。
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()

    // 切到工单 8：旧请求作废、草稿清空、重新拉取。
    testRoute.current!.params.id = '8'
    await flushPromises()

    expect(wrapper.text()).toContain('第二张工单')
    expect((wrapper.get('[data-testid="ticket-reply-input"]').element as HTMLTextAreaElement).value).toBe('')

    // 旧请求晚到：不能把工单 7 的数据回填到工单 8 的页面。
    resolveStale(stalePayload)
    await flushPromises()

    expect(wrapper.text()).toContain('第二张工单')
    expect(wrapper.text()).not.toContain('旧数据标题')
  })
})
