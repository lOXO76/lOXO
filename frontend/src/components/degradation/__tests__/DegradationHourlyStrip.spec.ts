import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import DegradationHourlyStrip from '../DegradationHourlyStrip.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key),
      locale: { value: 'en-US' },
    }),
  }
})

const HOUR = 3_600_000
const bucketWindow = { startMs: Date.parse('2026-10-30T08:00:00Z'), count: 3, stepMs: HOUR }

describe('DegradationHourlyStrip', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('renders one bar per hour with level-encoded heights', () => {
    const wrapper = mount(DegradationHourlyStrip, {
      props: {
        buckets: [{ start: '2026-10-30T10:00:00Z', passed: 2, failed: 1, error: 0, pass_rate: 2 / 3, avg_latency_ms: 4200 }],
        bucketWindow,
        threshold: 0.8,
      },
    })
    const bars = wrapper.findAll('[data-testid="hourly-bar"]')
    expect(bars.map((bar) => bar.attributes('data-level'))).toEqual(['empty', 'empty', 'degraded'])
    expect(bars[2].attributes('style')).toContain('height: 35%')
    expect(wrapper.text()).toContain('degradation.hourly.now')
    wrapper.unmount()
  })

  it('shows a floating tooltip with the hour summary on hover', async () => {
    const wrapper = mount(DegradationHourlyStrip, {
      attachTo: document.body,
      props: {
        buckets: [{ start: '2026-10-30T08:00:00Z', passed: 4, failed: 1, error: 1, pass_rate: 0.8, avg_latency_ms: 4200 }],
        bucketWindow,
        threshold: 0.8,
      },
    })
    const slots = wrapper.findAll('.hourly-slot')
    await slots[0].trigger('mouseenter', { clientX: 100, clientY: 100 })

    const tooltip = document.body.querySelector('[role="tooltip"]')
    expect(tooltip).not.toBeNull()
    expect(tooltip?.textContent).toContain('degradation.hourly.levels.warning')
    expect(tooltip?.textContent).toContain('80.0%')
    expect(tooltip?.textContent).toContain('4.2s')
    expect(tooltip?.textContent).toContain('"passed":4')

    await slots[1].trigger('mouseenter', { clientX: 100, clientY: 100 })
    expect(document.body.querySelector('[role="tooltip"]')?.textContent).toContain('degradation.hourly.noData')

    await wrapper.find('[role="img"]').trigger('mouseleave')
    expect(document.body.querySelector('[role="tooltip"]')).toBeNull()
    wrapper.unmount()
  })
})
