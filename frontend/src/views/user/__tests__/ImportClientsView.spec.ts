import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportClientsView from '../ImportClientsView.vue'

const { listKeys, showError, showInfo, copyToClipboard } = vi.hoisted(() => ({
  listKeys: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  copyToClipboard: vi.fn(async () => true)
}))

const appStore = {
  cachedPublicSettings: {
    api_base_url: 'https://api.example.com',
    site_name: 'Sub2API'
  },
  showError,
  showInfo
}

vi.mock('@/api', () => ({
  keysAPI: { list: listKeys }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { name: 'AppLayout', template: '<div><slot /></div>' }
}))
vi.mock('@/components/icons/Icon.vue', () => ({
  default: { name: 'Icon', props: ['name', 'size'], template: '<i />' }
}))

const baseGroup = {
  id: 1,
  name: 'Claude Only',
  description: null,
  platform: 'anthropic',
  rate_multiplier: 1,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  claude_code_only: true,
  allow_messages_dispatch: false
}

const baseKey = {
  id: 11,
  user_id: 1,
  key: 'sk-test-key',
  name: 'my-key',
  group_id: 1,
  status: 'active',
  group: baseGroup
}

const mountView = async () => {
  const wrapper = mount(ImportClientsView, {
    global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } }
  })
  await flushPromises()
  return wrapper
}

const selectKey = async (wrapper: Awaited<ReturnType<typeof mountView>>, keyId = '11') => {
  await wrapper.get('[data-test="client-import-key"]').setValue(keyId)
  await flushPromises()
}

describe('ImportClientsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listKeys.mockResolvedValue({ items: [baseKey], total: 1, page: 1, page_size: 100, pages: 1 })
    ;(globalThis as { fetch?: unknown }).fetch = vi.fn(async () => ({
      ok: true,
      json: async () => ({ data: [{ id: 'claude-opus-4' }] })
    }))
  })

  it('claude_code_only 分组下只放行 Claude Code，其余客户端置灰并给出原因', async () => {
    const wrapper = await mountView()
    await selectKey(wrapper)

    const claudeCode = wrapper.get('[data-test="client-card-claude-code"]')
    expect(claudeCode.attributes('disabled')).toBeUndefined()

    for (const id of ['codex', 'chatbox', 'magpie', 'cherry-studio', 'pi']) {
      const card = wrapper.get(`[data-test="client-card-${id}"]`)
      expect(card.attributes('disabled')).toBeDefined()
      expect(wrapper.get(`[data-test="client-blocked-${id}"]`).text())
        .toBe('clientImport.disabled.claudeCodeOnly')
    }
    wrapper.unmount()
  })

  it('选择 Claude Code 后渲染带密钥的配置文件预览', async () => {
    const wrapper = await mountView()
    await selectKey(wrapper)
    await wrapper.get('[data-test="client-card-claude-code"]').trigger('click')
    await flushPromises()

    const panel = wrapper.get('[data-test="client-import-panel"]')
    expect(panel.text()).toContain('~/.claude/settings.json')
    expect(panel.text()).toContain('sk-test-key')
    expect(panel.text()).toContain('https://api.example.com')
    wrapper.unmount()
  })

  it('换密钥会重置客户端选择，因为分组可能随之改变', async () => {
    listKeys.mockResolvedValue({
      items: [
        baseKey,
        {
          ...baseKey,
          id: 12,
          name: 'open-key',
          group_id: 2,
          group: { ...baseGroup, id: 2, name: 'Open', claude_code_only: false, platform: 'openai' }
        }
      ],
      total: 2,
      page: 1,
      page_size: 200,
      pages: 1
    })
    const wrapper = await mountView()
    await selectKey(wrapper)
    await wrapper.get('[data-test="client-card-claude-code"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="client-import-panel"]').exists()).toBe(true)

    await selectKey(wrapper, '12')
    expect(wrapper.find('[data-test="client-import-panel"]').exists()).toBe(false)
    // 新分组是 openai 且未开 messages dispatch，Claude Code 应被禁用
    expect(wrapper.get('[data-test="client-card-claude-code"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="client-blocked-claude-code"]').text())
      .toBe('clientImport.disabled.messagesDispatchOff')
    wrapper.unmount()
  })

  it('密钥未绑定分组时给出提示且不展示客户端', async () => {
    listKeys.mockResolvedValue({
      items: [{ ...baseKey, group_id: null, group: undefined }],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1
    })
    const wrapper = await mountView()
    // 未分组的密钥会被过滤掉，列表为空时提示先去创建
    expect(wrapper.text()).toContain('clientImport.keyMissing')
    wrapper.unmount()
  })

  it('CC Switch 的目标应用按分组筛选，claude_code_only 分组只有 Claude Code', async () => {
    const wrapper = await mountView()
    await selectKey(wrapper)
    await wrapper.get('[data-test="client-card-cc-switch"]').trigger('click')
    await flushPromises()

    const apps = wrapper.findAll('[data-test^="cc-switch-app-"]').map((node) => node.attributes('data-test'))
    expect(apps).toEqual(['cc-switch-app-claude'])

    const link = wrapper.get('[data-test="client-import-deeplink"]').attributes('href')!
    const params = new URL(link).searchParams
    expect(params.get('app')).toBe('claude')
    expect(params.get('apiKey')).toBe('sk-test-key')
    wrapper.unmount()
  })

  it('切换 CC Switch 目标应用会改写深链的 app 与协议', async () => {
    listKeys.mockResolvedValue({
      items: [{
        ...baseKey,
        group: { ...baseGroup, platform: 'kimi', claude_code_only: false }
      }],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1
    })
    const wrapper = await mountView()
    await selectKey(wrapper)
    await wrapper.get('[data-test="client-card-cc-switch"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="cc-switch-app-opencode"]').trigger('click')
    await flushPromises()
    const params = new URL(wrapper.get('[data-test="client-import-deeplink"]').attributes('href')!).searchParams
    expect(params.get('app')).toBe('opencode')
    expect(params.get('apiFormat')).toBe('openai_chat')
    expect(params.get('endpoint')).toBe('https://api.example.com/v1')
    wrapper.unmount()
  })

  it('管理端开启「隐藏 CCS 导入按钮」时不展示 CC Switch 卡片', async () => {
    const settings = appStore.cachedPublicSettings as Record<string, unknown>
    settings.hide_ccs_import_button = true
    try {
      const wrapper = await mountView()
      await selectKey(wrapper)
      expect(wrapper.find('[data-test="client-card-cc-switch"]').exists()).toBe(false)
      expect(wrapper.find('[data-test="client-card-claude-code"]').exists()).toBe(true)
      wrapper.unmount()
    } finally {
      delete settings.hide_ccs_import_button
    }
  })

  it('Claude Code 展示一键命令，可在 bash 与 PowerShell 之间切换并复制', async () => {
    const wrapper = await mountView()
    await selectKey(wrapper)
    await wrapper.get('[data-test="client-card-claude-code"]').trigger('click')
    await flushPromises()

    const block = wrapper.get('[data-test="client-import-commands"]')
    expect(block.text()).toContain('command -v node')

    await wrapper.get('[data-test="client-import-shell-powershell"]').trigger('click')
    expect(wrapper.get('[data-test="client-import-commands"]').text()).toContain('ConvertTo-Json')

    await wrapper.get('[data-test="client-import-copy-command"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(
      expect.stringContaining('ConvertTo-Json'),
      'clientImport.result.copied'
    )
    wrapper.unmount()
  })
})
