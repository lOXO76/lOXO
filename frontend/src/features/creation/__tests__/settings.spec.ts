import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { SystemSettings, UpdateSettingsRequest } from '@/api/admin/settings'
import type { PublicSettings } from '@/types'
import { FeatureFlags, isFeatureFlagEnabled, makeSidebarFlag, resolveFeatureFlag } from '@/utils/featureFlags'
import SettingsView from '@/views/admin/SettingsView.vue'

const { getSettings, updateSettings, appStore, adminSettingsFetch } = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  appStore: {
    cachedPublicSettings: null as Partial<PublicSettings> | null,
    fetchPublicSettings: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn(),
    showInfo: vi.fn(),
  },
  adminSettingsFetch: vi.fn(),
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ fetch: adminSettingsFetch }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }),
}))
vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getSettings,
      updateSettings,
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      updateWebSearchEmulationConfig: vi.fn().mockResolvedValue({}),
      getAdminApiKey: vi.fn().mockResolvedValue({ exists: false, masked_key: '' }),
      getOverloadCooldownSettings: vi.fn().mockResolvedValue({}),
      getRateLimit429CooldownSettings: vi.fn().mockResolvedValue({}),
      getPanelRateLimitSettings: vi.fn().mockResolvedValue({}),
      getStreamTimeoutSettings: vi.fn().mockResolvedValue({}),
      getRectifierSettings: vi.fn().mockResolvedValue({}),
      getBetaPolicySettings: vi.fn().mockResolvedValue({ rules: [] }),
    },
    accounts: {
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({}),
      getOllamaCloudUsageSettings: vi.fn().mockResolvedValue({}),
      getOpenCodeGoUsageSettings: vi.fn().mockResolvedValue({}),
    },
    groups: { getAll: vi.fn().mockResolvedValue([]) },
    proxies: { list: vi.fn().mockResolvedValue({ items: [] }) },
    payment: { getProviders: vi.fn().mockResolvedValue({ data: [] }) },
  },
}))

function mountSettings() {
  return shallowMount(SettingsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Toggle: false,
        RouterLink: true,
      },
    },
  })
}

async function creationToggle(wrapper: ReturnType<typeof mountSettings>) {
  await wrapper.get('#settings-tab-features').trigger('click')
  const card = wrapper.findAll('.card').find((node) =>
    node.find('h2').text() === 'admin.settings.features.creation.title',
  )
  expect(card).toBeDefined()
  return card!.get('[role="switch"]')
}

beforeEach(() => {
  vi.clearAllMocks()
  appStore.cachedPublicSettings = null
  getSettings.mockResolvedValue({ backend_mode_enabled: false })
  updateSettings.mockImplementation(async (payload: UpdateSettingsRequest) => payload)
})

describe('creation feature flag', () => {
  it('registers Creation Studio as opt-out', () => {
    expect(FeatureFlags.creation).toEqual({
      key: 'creation_enabled',
      mode: 'opt-out',
      label: 'Creation Studio',
    })
  })

  it.each([undefined, null, {}, { creation_enabled: true }, { creation_enabled: false }])(
    'resolves public settings %j and keeps the sidebar getter in sync', (settings) => {
      const enabled = settings?.creation_enabled !== false
      const sidebarFlag = makeSidebarFlag(FeatureFlags.creation)
      appStore.cachedPublicSettings = settings ?? null

      expect(resolveFeatureFlag(settings, FeatureFlags.creation)).toBe(enabled)
      expect(isFeatureFlagEnabled(FeatureFlags.creation)).toBe(enabled)
      expect(sidebarFlag()).toBe(enabled)

      appStore.cachedPublicSettings = { creation_enabled: !enabled }
      expect(sidebarFlag()).toBe(!enabled)
    },
  )
})

describe('creation admin settings', () => {
  it.each([
    [{}, true],
    [{ creation_enabled: true }, true],
    [{ creation_enabled: false }, false],
  ] satisfies [Partial<SystemSettings>, boolean][])(
    'loads and saves %j as %s', async (settings, enabled) => {
      getSettings.mockResolvedValue({ backend_mode_enabled: false, ...settings })
      const wrapper = mountSettings()
      try {
        await flushPromises()
        const toggle = await creationToggle(wrapper)
        expect(toggle.attributes('aria-checked')).toBe(String(enabled))

        await wrapper.get('form').trigger('submit.prevent')
        await flushPromises()
        expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({ creation_enabled: enabled }))

        await toggle.trigger('click')
        await wrapper.get('form').trigger('submit.prevent')
        await flushPromises()
        expect(updateSettings).toHaveBeenLastCalledWith(expect.objectContaining({ creation_enabled: !enabled }))
        expect(appStore.fetchPublicSettings).toHaveBeenLastCalledWith(true)
        expect(adminSettingsFetch).toHaveBeenLastCalledWith(true)
        expect(appStore.showError).not.toHaveBeenCalled()
      } finally {
        wrapper.unmount()
      }
    },
  )
})
