import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import TicketDetailView from '../TicketDetailView.vue'
import type { AdminTicketDetail, TicketMessage } from '@/types/tickets'

const mocks = vi.hoisted(() => ({
  getTicket: vi.fn(),
  getSettings: vi.fn(),
  listAssignees: vi.fn(),
  addMessage: vi.fn(),
  updateTicket: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  push: vi.fn(),
  // 由 vue-router mock 替换成 reactive，测试里改 params.id 模拟 SPA 内跳工单
  route: { params: { id: '1' } } as { params: { id: string } },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tickets: {
      getTicket: mocks.getTicket,
      getSettings: mocks.getSettings,
      listAssignees: mocks.listAssignees,
      addMessage: mocks.addMessage,
      updateTicket: mocks.updateTicket,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string | null) => `dt:${value ?? ''}`,
  formatRelativeTime: (value: string | null) => `rel:${value ?? ''}`,
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  const { reactive } = await import('vue')
  // 同一个 reactive 路由对象：测试里改 route.params.id 会触发组件内的 watch
  const route = reactive(mocks.route)
  mocks.route = route
  return {
    ...actual,
    useRoute: () => route,
    useRouter: () => ({ push: mocks.push }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}(${Object.values(params).join(',')})` : key,
    }),
  }
})

function makeMessage(overrides: Partial<TicketMessage> = {}): TicketMessage {
  return {
    id: 1,
    ticket_id: 1,
    author_type: 'user',
    author_id: 42,
    author_name: 'alice',
    via: 'web',
    internal: false,
    body: 'My upgrade keeps failing.',
    created_at: '2026-11-06T10:21:00Z',
    ...overrides,
  }
}

function makeDetail(messages: TicketMessage[], id = 1): AdminTicketDetail {
  return {
    ticket: {
      id,
      ticket_no: `TK20261106A1B2C${id}`,
      subject: 'Cannot upgrade my plan',
      category: 'billing',
      priority: 'high',
      status: 'pending_admin',
      user_id: 42,
      assignee_id: null,
      unread: true,
      message_count: messages.length,
      last_message_at: '2026-11-06T10:21:00Z',
      last_message_by: 'user',
      created_at: '2026-11-06T10:20:00Z',
      updated_at: '2026-11-06T10:21:00Z',
      closed_at: null,
      user_email: 'alice@example.com',
      user_username: 'alice',
      assignee_name: null,
      user_unread: false,
      admin_unread: true,
    },
    messages,
  }
}

function mountView() {
  return mount(TicketDetailView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        EmptyState: { props: ['title'], template: '<div class="empty-stub">{{ title }}</div>' },
        Select: {
          props: ['modelValue', 'options', 'ariaLabel', 'disabled'],
          emits: ['update:modelValue', 'change'],
          template: `<select class="select-stub" :aria-label="ariaLabel" :value="modelValue"
            @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)">
            <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
          </select>`,
        },
      },
    },
  })
}

describe('TicketDetailView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.route.params.id = '1'
    mocks.getSettings.mockResolvedValue({ max_body_length: 5000 })
    mocks.listAssignees.mockResolvedValue([{ id: 7, email: 'support@example.com', username: 'Support' }])
    mocks.addMessage.mockResolvedValue(makeMessage({ id: 9, author_type: 'admin', author_name: 'Support' }))
    mocks.updateTicket.mockResolvedValue(makeDetail([]).ticket)
  })

  afterEach(() => {
    vi.clearAllMocks()
    mocks.route.params.id = '1'
  })

  it('renders the message stream as plain text and marks internal notes and system messages', async () => {
    mocks.getTicket.mockResolvedValue(
      makeDetail([
        makeMessage({ id: 1, body: 'My upgrade keeps failing.\nPlease check.' }),
        makeMessage({
          id: 2,
          author_type: 'admin',
          author_id: 1,
          author_name: 'Support',
          via: 'agent',
          body: '<img src=x onerror=alert(1)>',
        }),
        makeMessage({
          id: 3,
          author_type: 'admin',
          author_id: 1,
          author_name: 'Support',
          internal: true,
          body: 'internal note',
        }),
        makeMessage({ id: 4, author_type: 'system', author_id: null, author_name: 'system', body: 'status_changed:resolved' }),
      ]),
    )

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="ticket-detail-subject"]').text()).toBe('Cannot upgrade my plan')
    expect(wrapper.find('[data-testid="ticket-detail-user-email"]').text()).toBe('alice@example.com')

    const bodies = wrapper.findAll('[data-testid="ticket-message-body"]')
    expect(bodies).toHaveLength(3)
    expect(bodies[0].text()).toContain('My upgrade keeps failing.')
    // 纯文本渲染：HTML 标签必须被转义成文本，不能进入 DOM
    expect(bodies[1].text()).toBe('<img src=x onerror=alert(1)>')
    expect(bodies[1].html()).not.toContain('<img')

    // 内部备注徽章 + via=agent 标签
    expect(wrapper.find('[data-testid="ticket-internal-badge"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.tickets.detail.viaAgent')

    // 系统消息按 i18n key 渲染
    expect(wrapper.text()).toContain('admin.tickets.system.statusChanged(admin.tickets.statuses.resolved)')

    wrapper.unmount()
  })

  it('sends an internal note with the internal flag and patches the status', async () => {
    mocks.getTicket.mockResolvedValue(makeDetail([makeMessage()]))
    const wrapper = mountView()
    await flushPromises()

    // 计数器按 Unicode code point 计：emoji 算 1 个，而不是 2 个
    await wrapper.find('[data-testid="ticket-reply-body"]').setValue('a😀')
    await flushPromises()
    expect(wrapper.find('[data-testid="ticket-reply-counter"]').text()).toBe('2 / 5000')

    await wrapper.find('[data-testid="ticket-reply-body"]').setValue('Checking this now')
    await wrapper.find('[data-testid="ticket-reply-internal"]').setValue(true)
    await flushPromises()
    await wrapper.find('[data-testid="ticket-reply-send"]').trigger('click')
    await flushPromises()

    expect(mocks.addMessage).toHaveBeenCalledWith(1, { body: 'Checking this now', internal: true })
    expect(mocks.showSuccess).toHaveBeenCalled()

    // 状态下拉即时保存
    await wrapper.find('[data-testid="ticket-status-select"]').setValue('resolved')
    await flushPromises()
    expect(mocks.updateTicket).toHaveBeenCalledWith(1, { status: 'resolved' })

    wrapper.unmount()
  })

  it('targets the new ticket after the route id changes inside the SPA', async () => {
    mocks.getTicket.mockImplementation((id: number) =>
      Promise.resolve(makeDetail([makeMessage({ id: id * 10, ticket_id: id })], id)),
    )
    const wrapper = mountView()
    await flushPromises()
    expect(mocks.getTicket).toHaveBeenCalledWith(1)

    // 组件被复用：路由从 /admin/tickets/1 变成 /admin/tickets/2
    mocks.route.params.id = '2'
    await nextTick()
    await flushPromises()
    expect(mocks.getTicket).toHaveBeenCalledWith(2)

    await wrapper.find('[data-testid="ticket-reply-body"]').setValue('reply to ticket two')
    await wrapper.find('[data-testid="ticket-reply-send"]').trigger('click')
    await flushPromises()

    // 写操作必须打到新工单
    expect(mocks.addMessage).toHaveBeenCalledTimes(1)
    expect(mocks.addMessage).toHaveBeenCalledWith(2, { body: 'reply to ticket two', internal: false })

    wrapper.unmount()
  })

  it('shows the not-found state when the ticket cannot be loaded', async () => {
    mocks.getTicket.mockRejectedValue({ message: 'not found', reason: 'TICKET_NOT_FOUND' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('admin.tickets.detail.notFound')

    wrapper.unmount()
  })
})
