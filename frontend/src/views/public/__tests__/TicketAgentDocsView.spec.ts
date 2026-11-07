import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import TicketAgentDocsView from '../TicketAgentDocsView.vue'

const mocks = vi.hoisted(() => ({
  fetchPublicSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: undefined,
    fetchPublicSettings: mocks.fetchPublicSettings,
    showSuccess: mocks.showSuccess,
    showError: mocks.showError,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}(${Object.values(params).join(',')})` : key,
      locale: { value: 'zh' },
    }),
  }
})

function mountView() {
  return mount(TicketAgentDocsView, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
        Icon: true,
      },
    },
  })
}

describe('TicketAgentDocsView', () => {
  it('renders every documented section with the endpoint reference and error codes', async () => {
    mocks.fetchPublicSettings.mockResolvedValue(undefined)
    const wrapper = mountView()
    await flushPromises()

    // 10 个章节 + 目录
    const sections = wrapper.findAll('[data-test^="ticket-agent-docs-section-"]')
    expect(sections).toHaveLength(10)
    expect(wrapper.findAll('[data-test^="ticket-agent-docs-toc-"]')).toHaveLength(10)

    // 全部 Agent 接口都出现在接口参考里
    for (const endpointId of [
      'me',
      'listTickets',
      'createTicket',
      'getTicket',
      'sendMessage',
      'closeTicket',
      'reopenTicket',
      'patchTicket',
      'updates',
      'docsMd',
      'openapi',
    ]) {
      expect(wrapper.find(`[data-test="ticket-agent-endpoint-${endpointId}"]`).exists()).toBe(true)
    }
    expect(wrapper.text()).toContain('/api/v1/agent/updates?cursor=0&limit=50')
    expect(wrapper.text()).toContain('/api/v1/agent/docs.md')
    expect(wrapper.text()).toContain('/api/v1/agent/openapi.json')

    // 错误码表覆盖 SPEC 6.4 的关键码
    expect(wrapper.text()).toContain('AGENT_KEY_INVALID')
    expect(wrapper.text()).toContain('TICKET_NOT_FOUND')
    expect(wrapper.text()).toContain('AGENT_RATE_LIMITED')
    expect(wrapper.text()).toContain('AGENT_AUTH_BLOCKED')
    expect(wrapper.text()).toContain('AGENT_RATE_LIMIT_UNAVAILABLE')

    // 鉴权与提示词模板
    expect(wrapper.text()).toContain('Authorization: Bearer')
    expect(wrapper.text()).toContain('Idempotency-Key')
    const prompt = wrapper.find('[data-testid="ticket-agent-docs-prompt"]').text()
    expect(prompt).toContain('{{BASE_URL}}')
    expect(prompt).toContain('{{AGENT_KEY}}')
    expect(prompt).toContain('/api/v1/agent/updates')

    expect(mocks.fetchPublicSettings).toHaveBeenCalled()
  })

  it('links to the raw markdown and openapi documents', async () => {
    mocks.fetchPublicSettings.mockResolvedValue(undefined)
    const wrapper = mountView()
    await flushPromises()

    const mdLink = wrapper.find('[data-testid="ticket-agent-docs-md-link"]')
    const openapiLink = wrapper.find('[data-testid="ticket-agent-openapi-link"]')
    expect(mdLink.attributes('href')).toContain('/api/v1/agent/docs.md')
    expect(openapiLink.attributes('href')).toContain('/api/v1/agent/openapi.json')
    expect(mdLink.attributes('target')).toBe('_blank')
  })
})
