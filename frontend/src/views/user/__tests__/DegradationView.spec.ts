import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import DegradationView from '../DegradationView.vue'

const { getOverview, showError } = vi.hoisted(() => ({
  getOverview: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/degradation', () => ({
  degradationAPI: { getOverview, listRuns: vi.fn() },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } }),
  }
})

const emptyOverview = {
  enabled: true,
  window_hours: 24,
  threshold: 0.8,
  generated_at: '2026-10-30T10:00:00Z',
  groups: [],
}

function mountView() {
  return mount(DegradationView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
}

describe('DegradationView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOverview.mockResolvedValue(emptyOverview)
  })

  it('renders the empty state when no targets are configured', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="degradation-empty"]').text()).toContain('degradation.emptyTitle')
    expect(wrapper.find('[data-testid="degradation-group"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders group and model overview data', async () => {
    getOverview.mockResolvedValue({
      ...emptyOverview,
      groups: [
        {
          group_id: 12,
          group_name: 'Claude Official',
          platform: 'anthropic',
          verdict: 'degraded',
          last_checked_at: '2026-10-30T09:58:00Z',
          models: [
            {
              model: 'claude-sonnet-4-6',
              verdict: 'degraded',
              pass_rate: 0.625,
              total: 8,
              passed: 5,
              failed: 3,
              error: 0,
              avg_latency_ms: 4210,
              last_checked_at: '2026-10-30T09:58:00Z',
              last_run_status: 'passed',
            },
          ],
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="degradation-group"]').text()).toContain('Claude Official')
    expect(wrapper.text()).toContain('claude-sonnet-4-6')
    expect(wrapper.text()).toContain('62.5%')
    // 旧版后端没有 bucket_* / hourly 字段时，按 generated_at 回推并画出空小时条
    expect(wrapper.findAll('[data-testid="hourly-bar"]')).toHaveLength(24)
    expect(getOverview).toHaveBeenCalledWith({ window_hours: 24 })
    wrapper.unmount()
  })

  it('renders an hourly strip per model aligned to the backend buckets', async () => {
    getOverview.mockResolvedValue({
      ...emptyOverview,
      bucket_start: '2026-10-29T11:00:00Z',
      bucket_count: 24,
      bucket_seconds: 3600,
      groups: [
        {
          group_id: 12,
          group_name: 'Claude Official',
          platform: 'anthropic',
          verdict: 'healthy',
          last_checked_at: '2026-10-30T09:58:00Z',
          models: [
            {
              model: 'claude-sonnet-4-6',
              verdict: 'healthy',
              pass_rate: 0.9,
              total: 10,
              passed: 9,
              failed: 1,
              error: 0,
              avg_latency_ms: 4210,
              last_checked_at: '2026-10-30T09:58:00Z',
              last_run_status: 'passed',
              hourly: [
                { start: '2026-10-29T11:00:00Z', passed: 1, failed: 1, error: 0, pass_rate: 0.5, avg_latency_ms: 4000 },
                { start: '2026-10-30T10:00:00Z', passed: 8, failed: 0, error: 0, pass_rate: 1, avg_latency_ms: 4200 },
              ],
            },
          ],
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="degradation-legend"]').exists()).toBe(true)
    const bars = wrapper.findAll('[data-testid="hourly-bar"]')
    expect(bars).toHaveLength(24)
    expect(bars[0].attributes('data-level')).toBe('degraded')
    expect(bars[23].attributes('data-level')).toBe('healthy')
    expect(bars.slice(1, 23).every((bar) => bar.attributes('data-level') === 'empty')).toBe(true)
    expect(wrapper.find('[role="progressbar"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('90.0%')
    wrapper.unmount()
  })

  it('uses the neutral visual branch without pass-rate or sample statistics', async () => {
    getOverview.mockResolvedValue({
      ...emptyOverview,
      groups: [
        {
          group_id: 12,
          group_name: 'Visual Models',
          platform: 'anthropic',
          verdict: 'unknown',
          last_checked_at: '2026-10-30T09:58:00Z',
          models: [
            {
              model: 'claude-sonnet-4-6',
              verdict: 'unknown',
              pass_rate: 0,
              total: 0,
              passed: 0,
              failed: 0,
              error: 0,
              avg_latency_ms: 0,
              last_checked_at: '2026-10-30T09:58:00Z',
              last_run_status: '',
              showcases: [
                {
                  run_id: 101,
                  probe_id: 7,
                  probe_name: 'Pelican bicycle',
                  artifact_kind: 'html',
                  latency_ms: 81234,
                  created_at: '2026-10-30T09:58:00Z',
                },
              ],
            },
          ],
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    const group = wrapper.get('[data-testid="degradation-group"]')
    expect(group.text()).toContain('degradation.visual.badge')
    expect(group.text()).not.toContain('degradation.verdict.unknown')
    expect(group.find('[role="progressbar"]').exists()).toBe(false)
    expect(group.find('[data-testid="degradation-hourly"]').exists()).toBe(false)
    expect(group.text()).not.toContain('degradation.samples')
    expect(group.get('[data-testid="visual-showcase"]').text()).toContain('Pelican bicycle')
    expect(group.find('iframe').exists()).toBe(false)
    wrapper.unmount()
  })
})
