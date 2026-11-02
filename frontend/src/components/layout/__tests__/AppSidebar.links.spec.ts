import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { CustomMenuItem } from '@/types'

const mocks = vi.hoisted(() => ({
  auth: { user: { enterprise: null }, isAdmin: false, isSimpleMode: false },
  app: { sidebarCollapsed: false, mobileOpen: false, sidebarScrollTop: 0, siteName: 'Test', siteLogo: '', siteVersion: '', publicSettingsLoaded: true, cachedPublicSettings: { custom_menu_items: [] as CustomMenuItem[] }, toggleSidebar: vi.fn(), setMobileOpen: vi.fn() },
  admin: { customMenuItems: [] as CustomMenuItem[], fetch: vi.fn(), paymentEnabled: true, opsMonitoringEnabled: true },
  push: vi.fn(),
}))
vi.mock('@/stores', () => ({ useAuthStore: () => mocks.auth, useAppStore: () => mocks.app, useAdminSettingsStore: () => mocks.admin, useOnboardingStore: () => ({ isCurrentStep: () => false }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ path: '/dashboard' }), useRouter: () => ({ push: mocks.push }) }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: {}, makeSidebarFlag: () => () => true, resolveFeatureFlag: () => true }))
vi.mock('@/composables/useBatchImageAccess', () => ({ useBatchImageAccess: () => ({ canUseBatchImage: { value: true }, refreshBatchImageAccess: vi.fn() }) }))
vi.mock('@/stores/enterprise', () => ({ useEnterpriseStore: () => ({ tabs: [], ensureLoaded: vi.fn() }) }))

import AppSidebar from '../AppSidebar.vue'

function item(id: string, overrides: Partial<CustomMenuItem> = {}): CustomMenuItem {
  return { id, label: id, url: `https://example.com/${id}`, icon_svg: '', visibility: 'user', sort_order: 0, ...overrides }
}
const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(AppSidebar, { global: { stubs: { VersionBadge: true, RouterLink: { props: ['to'], template: '<a :href="to" data-router-link><slot /></a>' } } } })
  wrappers.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.auth.isAdmin = false
  mocks.auth.isSimpleMode = false
  mocks.app.sidebarCollapsed = false
  mocks.app.mobileOpen = false
  mocks.app.cachedPublicSettings.custom_menu_items = []
  mocks.admin.customMenuItems = []
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })

describe('AppSidebar custom menu new tabs', () => {
  it.each([
    [false, false], [true, false], [true, true],
  ])('renders HTTP and Markdown new-tab entries for admin=%s simple=%s', (admin, simple) => {
    mocks.auth.isAdmin = admin
    mocks.auth.isSimpleMode = simple
    const items = [item('external', { open_in_new_tab: true }), item('markdown', { url: 'md:help', open_in_new_tab: true }), item('embedded'), item('explicit-same', { open_in_new_tab: false })]
    if (admin && simple) mocks.admin.customMenuItems = items.map(menu => ({ ...menu, visibility: 'admin' }))
    else mocks.app.cachedPublicSettings.custom_menu_items = items
    const wrapper = render()
    for (const href of ['https://example.com/external', '/custom/markdown']) {
      const node = wrapper.get(`a[href="${href}"]`)
      expect(node.attributes()).toMatchObject({ target: '_blank', rel: 'noopener noreferrer' })
      expect(node.attributes('data-router-link')).toBeUndefined()
    }
    for (const id of ['embedded', 'explicit-same']) {
      const node = wrapper.get(`a[href="/custom/${id}"]`)
      expect(node.attributes('data-router-link')).toBeDefined()
      expect(node.attributes('target')).toBeUndefined()
    }
  })

  it('renders admin-only custom links in standard mode', () => {
    mocks.auth.isAdmin = true
    mocks.admin.customMenuItems = [item('private', { visibility: 'admin', open_in_new_tab: true })]
    expect(render().get('a[href="https://example.com/private"]').attributes('target')).toBe('_blank')
  })

  it('keeps new-tab links usable when collapsed and closes mobile navigation after clicking', async () => {
    vi.useFakeTimers()
    mocks.app.sidebarCollapsed = true
    mocks.app.mobileOpen = true
    mocks.app.cachedPublicSettings.custom_menu_items = [item('help', { open_in_new_tab: true })]
    const wrapper = render()
    const node = wrapper.get('a[href="https://example.com/help"]')
    expect(node.attributes('title')).toBe('help')
    expect(node.classes()).toContain('sidebar-link-collapsed')
    await node.trigger('click')
    await vi.advanceTimersByTimeAsync(150)
    expect(mocks.app.setMobileOpen).toHaveBeenCalledWith(false)
    expect(mocks.push).not.toHaveBeenCalled()
  })

  it('does not create a new-tab link for an unsafe URL', () => {
    mocks.app.cachedPublicSettings.custom_menu_items = [item('unsafe', { url: 'javascript:alert(1)', open_in_new_tab: true })]
    expect(render().get('a[href="/custom/unsafe"]').attributes('target')).toBeUndefined()
  })
})
