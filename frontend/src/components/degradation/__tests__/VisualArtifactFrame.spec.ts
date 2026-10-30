import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import VisualArtifactFrame from '../VisualArtifactFrame.vue'

const { createVisualViewToken, createRunViewToken } = vi.hoisted(() => ({
  createVisualViewToken: vi.fn(),
  createRunViewToken: vi.fn(),
}))

vi.mock('@/api/degradation', async () => {
  const actual = await vi.importActual<typeof import('@/api/degradation')>('@/api/degradation')
  return {
    ...actual,
    degradationAPI: { createVisualViewToken },
  }
})
vi.mock('@/api/admin/degradation', () => ({
  degradationAdminAPI: { createRunViewToken },
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountFrame(scope: 'user' | 'admin' = 'user') {
  return mount(VisualArtifactFrame, {
    props: { runId: 1, scope, title: 'Generated artifact' },
    global: { stubs: { LoadingSpinner: true } },
  })
}

describe('VisualArtifactFrame', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createVisualViewToken.mockResolvedValue({ token: 'user-token', expires_at: '2026-10-30T10:15:00Z' })
    createRunViewToken.mockResolvedValue({ token: 'admin-token', expires_at: '2026-10-30T10:15:00Z' })
  })

  it('loads on click and uses exactly the isolated sandbox token set', async () => {
    const wrapper = mountFrame()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(createVisualViewToken).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="visual-load"]').trigger('click')
    await flushPromises()

    const iframe = wrapper.get('iframe')
    expect(iframe.attributes('sandbox')).toBe('allow-scripts')
    expect(iframe.attributes('referrerpolicy')).toBe('no-referrer')
    expect(iframe.attributes('src')).toBe('/api/v1/degradation/visual-render/user-token')
    expect(createVisualViewToken).toHaveBeenCalledWith(1)

    await wrapper.setProps({ title: 'Updated title' })
    expect(wrapper.find('iframe').exists()).toBe(true)
    expect(createVisualViewToken).toHaveBeenCalledTimes(1)
  })

  it('returns to the click placeholder when the run id changes', async () => {
    const wrapper = mountFrame()
    await wrapper.get('[data-testid="visual-load"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(true)

    await wrapper.setProps({ runId: 2 })
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.find('[data-testid="visual-load"]').exists()).toBe(true)
    expect(createVisualViewToken).toHaveBeenCalledTimes(1)
  })

  it('uses the admin token endpoint in admin scope', async () => {
    const wrapper = mountFrame('admin')
    await wrapper.get('[data-testid="visual-load"]').trigger('click')
    await flushPromises()

    expect(createRunViewToken).toHaveBeenCalledWith(1)
    expect(createVisualViewToken).not.toHaveBeenCalled()
    expect(wrapper.get('iframe').attributes('sandbox')).toBe('allow-scripts')
  })
})
