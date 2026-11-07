import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const { copyToClipboard } = vi.hoisted(() => ({ copyToClipboard: vi.fn().mockResolvedValue(true) }))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: { value: false }, copyToClipboard }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } }) }
})

import AgentKeyCreatedDialog from '../AgentKeyCreatedDialog.vue'

const createdKey = {
  id: 3,
  name: 'my-claude',
  key_prefix: 'lxt_AbCdEfGh',
  scope: 'user' as const,
  qps_limit: 2,
  status: 'active' as const,
  last_used_at: null,
  last_used_ip: null,
  expires_at: null,
  created_at: '2026-11-06T09:00:00Z',
  secret: 'lxt_verySecretPlaintextOnlyOnce1234567890',
}

function mountDialog(props: Record<string, unknown>) {
  return mount(AgentKeyCreatedDialog, {
    props,
    global: {
      stubs: {
        // BaseDialog 用 Teleport 到 body，测试里改成内联渲染便于断言。
        BaseDialog: {
          props: ['show', 'title'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
      },
    },
  })
}

describe('AgentKeyCreatedDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the full secret exactly once and clears it after closing', async () => {
    const wrapper = mountDialog({ show: true, createdKey })

    const input = wrapper.get('[data-testid="agent-secret-value"]')
    expect((input.element as HTMLInputElement).value).toBe(createdKey.secret)

    await wrapper.get('[data-testid="agent-secret-done"]').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
    // 关闭后局部明文立即清空，避免再次打开时残留
    expect((input.element as HTMLInputElement).value).toBe('')
  })

  it('copies the plaintext through the clipboard composable', async () => {
    const wrapper = mountDialog({ show: true, createdKey })

    await wrapper.get('[data-testid="agent-secret-copy"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(createdKey.secret, 'tickets.agent.createdDialog.copied')
  })

  it('renders nothing when the dialog is closed', () => {
    const wrapper = mountDialog({ show: false, createdKey: null })
    expect(wrapper.find('[data-testid="agent-secret-value"]').exists()).toBe(false)
  })
})
