import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import TicketsView from '../TicketsView.vue'
import type { AdminTicketAgentKey, AdminTicketSummary, TicketSettings } from '@/types/tickets'

const mocks = vi.hoisted(() => ({
  listTickets: vi.fn(),
  getStats: vi.fn(),
  getTicket: vi.fn(),
  addMessage: vi.fn(),
  updateTicket: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  testNotification: vi.fn(),
  listAgentKeys: vi.fn(),
  updateAgentKey: vi.fn(),
  deleteAgentKey: vi.fn(),
  listAssignees: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  replace: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tickets: {
      listTickets: mocks.listTickets,
      getStats: mocks.getStats,
      getTicket: mocks.getTicket,
      addMessage: mocks.addMessage,
      updateTicket: mocks.updateTicket,
      getSettings: mocks.getSettings,
      updateSettings: mocks.updateSettings,
      testNotification: mocks.testNotification,
      listAgentKeys: mocks.listAgentKeys,
      updateAgentKey: mocks.updateAgentKey,
      deleteAgentKey: mocks.deleteAgentKey,
      listAssignees: mocks.listAssignees,
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
  return {
    ...actual,
    useRoute: () => ({ query: {}, params: {} }),
    useRouter: () => ({ replace: mocks.replace, push: mocks.replace }),
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

function makeTicket(overrides: Partial<AdminTicketSummary> = {}): AdminTicketSummary {
  return {
    id: 1,
    ticket_no: 'TK20261106A1B2C3',
    subject: 'Cannot upgrade my plan',
    category: 'billing',
    priority: 'high',
    status: 'pending_admin',
    user_id: 42,
    assignee_id: null,
    unread: true,
    message_count: 3,
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
    ...overrides,
  }
}

function makeAgentKey(overrides: Partial<AdminTicketAgentKey> = {}): AdminTicketAgentKey {
  return {
    id: 7,
    name: 'my-claude',
    key_prefix: 'lxt_AbCdEfGh',
    scope: 'user',
    qps_limit: 2,
    status: 'active',
    last_used_at: null,
    last_used_ip: null,
    expires_at: null,
    created_at: '2026-11-01T08:00:00Z',
    user_id: 42,
    user_email: 'alice@example.com',
    ...overrides,
  }
}

const settings: TicketSettings = {
  enabled: true,
  admin_email_enabled: true,
  admin_notify_emails: ['ops@example.com'],
  admin_feishu_enabled: false,
  admin_feishu_webhook_configured: true,
  admin_feishu_webhook_url_masked: 'https://open.feishu.cn/open-apis/bot/v2/hook/abcd****wxyz',
  admin_feishu_secret_configured: false,
  notify_throttle_seconds: 60,
  max_open_tickets_per_user: 5,
  create_per_hour_per_user: 10,
  messages_per_minute_per_user: 20,
  max_subject_length: 200,
  max_body_length: 5000,
  agent_enabled: true,
  agent_default_qps: 2,
  agent_max_qps: 10,
  agent_user_total_qps: 5,
  agent_admin_total_qps: 20,
  agent_max_keys_per_user: 5,
  agent_failed_auth_per_minute_per_ip: 20,
  agent_ip_requests_per_minute: 600,
}

function pageOf<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20, pages: 1 }
}

function mountView() {
  return mount(TicketsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        RouterLink: RouterLinkStub,
        Pagination: true,
        StatCard: {
          props: ['title', 'value'],
          template: '<div class="stat-card-stub">{{ title }}={{ value }}</div>',
        },
        Select: {
          props: ['modelValue', 'options', 'ariaLabel'],
          emits: ['update:modelValue', 'change'],
          template: '<select class="select-stub" :aria-label="ariaLabel" />',
        },
        DataTable: {
          props: ['data', 'columns'],
          template: `<div class="table-stub">
            <span v-for="column in columns" :key="column.key" class="column-label">{{ column.label }}</span>
            <div v-if="!data || data.length === 0" class="table-empty"><slot name="empty" /></div>
            <div v-for="row in data" :key="row.id" class="data-row">
              <slot name="cell-subject" :row="row" />
              <slot name="cell-user" :row="row" />
              <slot name="cell-status" :row="row" />
              <slot name="cell-priority" :row="row" />
              <slot name="cell-category" :row="row" />
              <slot name="cell-assignee" :row="row" />
              <slot name="cell-messages" :row="row" />
              <slot name="cell-lastMessageAt" :row="row" />
              <slot name="cell-owner" :row="row" />
              <slot name="cell-name" :row="row" />
              <slot name="cell-scope" :row="row" />
              <slot name="cell-qps" :row="row" />
              <slot name="cell-lastUsedAt" :row="row" />
              <slot name="cell-expiresAt" :row="row" />
              <slot name="cell-actions" :row="row" />
            </div>
          </div>`,
        },
      },
    },
  })
}

describe('TicketsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listTickets.mockResolvedValue(pageOf([makeTicket()]))
    mocks.getStats.mockResolvedValue({ pending_admin: 3, pending_user: 5, resolved: 10, closed: 40, unread: 2 })
    mocks.listAssignees.mockResolvedValue([{ id: 7, email: 'support@example.com', username: 'Support' }])
    mocks.getSettings.mockResolvedValue(settings)
    mocks.listAgentKeys.mockResolvedValue(pageOf([makeAgentKey()]))
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders stats and the ticket list, then loads settings and agent keys tabs', async () => {
    const wrapper = mountView()
    await flushPromises()

    // 概览卡片
    expect(wrapper.find('[data-testid="tickets-stats"]').text()).toContain('admin.tickets.stats.pendingAdmin=3')
    expect(wrapper.find('[data-testid="tickets-stats"]').text()).toContain('admin.tickets.stats.unread=2')

    // 列表：标题、工单号、用户邮箱、状态/优先级徽章文案
    expect(mocks.listTickets).toHaveBeenCalledWith({
      page: 1,
      page_size: 20,
      keyword: '',
      status: '',
      priority: '',
      category: '',
      assignee_id: undefined,
      unread_only: undefined,
    })
    expect(wrapper.text()).toContain('Cannot upgrade my plan')
    expect(wrapper.text()).toContain('TK20261106A1B2C3')
    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('admin.tickets.statuses.pending_admin')
    expect(wrapper.text()).toContain('admin.tickets.priorities.high')
    expect(wrapper.text()).toContain('admin.tickets.unassigned')

    // 设置 Tab
    await wrapper.find('[data-testid="tickets-tab-settings"]').trigger('click')
    await flushPromises()
    expect(mocks.getSettings).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="ticket-system-state"]').text()).toBe('admin.tickets.settings.general.enabled')
    expect(wrapper.find('[data-testid="ticket-setting-notify_throttle_seconds"]').element).toHaveProperty('value', '60')
    expect(wrapper.text()).toContain('ops@example.com')
    expect(wrapper.text()).toContain('https://open.feishu.cn/open-apis/bot/v2/hook/abcd****wxyz')

    // Agent 密钥 Tab
    await wrapper.find('[data-testid="tickets-tab-agentKeys"]').trigger('click')
    await flushPromises()
    expect(mocks.listAgentKeys).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('my-claude')
    expect(wrapper.text()).toContain('lxt_AbCdEfGh…')
    expect(wrapper.text()).toContain('alice@example.com')
  })

  it('saves settings without sending untouched write-only feishu fields', async () => {
    mocks.updateSettings.mockResolvedValue(settings)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="tickets-tab-settings"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-testid="ticket-settings-save"]').trigger('click')
    await flushPromises()

    expect(mocks.updateSettings).toHaveBeenCalledTimes(1)
    const payload = mocks.updateSettings.mock.calls[0][0] as Record<string, unknown>
    expect(payload.enabled).toBe(true)
    expect(payload.notify_throttle_seconds).toBe(60)
    expect(payload.admin_notify_emails).toEqual(['ops@example.com'])
    // 写入专用字段未改动时不应出现在 payload 里
    expect(payload).not.toHaveProperty('admin_feishu_webhook_url')
    expect(payload).not.toHaveProperty('admin_feishu_secret')
    expect(mocks.showSuccess).toHaveBeenCalled()
  })

  it('rejects out-of-range numeric settings before calling the API', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="tickets-tab-settings"]').trigger('click')
    await flushPromises()

    await wrapper.find('[data-testid="ticket-setting-agent_max_qps"]').setValue('0')
    await flushPromises()

    expect(wrapper.text()).toContain('admin.tickets.settings.fields.invalid(1,1000)')
    const saveButton = wrapper.find('[data-testid="ticket-settings-save"]')
    expect(saveButton.attributes('disabled')).toBeDefined()

    await saveButton.trigger('click')
    await flushPromises()
    expect(mocks.updateSettings).not.toHaveBeenCalled()
  })

  it('accepts every value the backend accepts and can still save other fields', async () => {
    // 与 backend/internal/service/ticket_settings.go 的 validateTicketSettings 边界对齐
    const boundary: TicketSettings = {
      ...settings,
      max_open_tickets_per_user: 1,
      create_per_hour_per_user: 1,
      messages_per_minute_per_user: 10000,
      max_subject_length: 1,
      max_body_length: 1,
      agent_default_qps: 1,
      agent_max_qps: 1,
      agent_max_keys_per_user: 1000,
      agent_failed_auth_per_minute_per_ip: 1,
      agent_ip_requests_per_minute: 0,
    }
    mocks.getSettings.mockResolvedValue(boundary)
    mocks.updateSettings.mockResolvedValue(boundary)

    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('[data-testid="tickets-tab-settings"]').trigger('click')
    await flushPromises()

    // 后端合法的边界值不应被前端标错，也不应阻塞保存
    expect(wrapper.text()).not.toContain('admin.tickets.settings.fields.invalid')
    const saveButton = wrapper.find('[data-testid="ticket-settings-save"]')
    expect(saveButton.attributes('disabled')).toBeUndefined()

    await wrapper.find('[data-testid="ticket-setting-notify_throttle_seconds"]').setValue('120')
    await saveButton.trigger('click')
    await flushPromises()

    expect(mocks.updateSettings).toHaveBeenCalledTimes(1)
    const payload = mocks.updateSettings.mock.calls[0][0] as Record<string, unknown>
    expect(payload.notify_throttle_seconds).toBe(120)
    expect(payload.max_open_tickets_per_user).toBe(1)
    expect(payload.create_per_hour_per_user).toBe(1)
    expect(payload.messages_per_minute_per_user).toBe(10000)
    expect(payload.max_subject_length).toBe(1)
    expect(payload.max_body_length).toBe(1)
    expect(payload.agent_max_keys_per_user).toBe(1000)
  })
})
