import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { HeaderLink } from '@/types'

const mocks = vi.hoisted(() => ({
  auth: { user: { role: 'user', email: 'user@example.com', balance: 0 }, isAdmin: false },
  app: { contactInfo: '', docUrl: '', cachedPublicSettings: { header_links: [] as HeaderLink[] }, toggleMobileSidebar: vi.fn() },
  admin: { headerLinks: [] as HeaderLink[], customMenuItems: [] },
}))
vi.mock('@/stores', () => ({ useAuthStore: () => mocks.auth, useAppStore: () => mocks.app, useOnboardingStore: () => ({ replay: vi.fn() }) }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => mocks.admin }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }), useRoute: () => ({ name: 'Dashboard', meta: {}, params: {} }) }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/featureFlags', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/utils/featureFlags')>(),
  FeatureFlags: { subscription: {}, modelPlaza: {} },
  isFeatureFlagEnabled: () => false,
  resolveFeatureFlag: () => false,
}))

import AppHeader from '../AppHeader.vue'

function link(id: string, overrides: Partial<HeaderLink> = {}): HeaderLink {
  return { id, label: id, url: `https://example.com/${id}`, icon_svg: '', visibility: 'user', sort_order: 0, open_in_new_tab: true, ...overrides }
}

const wrappers: ReturnType<typeof mount>[] = []
function render() {
  const wrapper = mount(AppHeader, {
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        LocaleSwitcher: true, SubscriptionProgressMini: true, AnnouncementBell: true,
        RouterLink: { props: ['to'], template: '<a :href="to" data-router-link><slot /></a>' },
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

beforeEach(() => {
  mocks.auth.isAdmin = false
  mocks.app.cachedPublicSettings.header_links = []
  mocks.admin.headerLinks = []
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

describe('AppHeader configured links', () => {
  it('only displays public user links, sorted without mutating settings', () => {
    const publicLinks = [link('later', { sort_order: 5 }), link('leaked-admin', { visibility: 'admin' }), link('first', { sort_order: 1 })]
    mocks.app.cachedPublicSettings.header_links = publicLinks
    mocks.admin.headerLinks = [link('private', { visibility: 'admin' })]
    const wrapper = render()
    expect(wrapper.findAll('[data-header-link]').map(node => node.attributes('data-header-link'))).toEqual(['first', 'later'])
    expect(publicLinks.map(item => item.id)).toEqual(['later', 'leaked-admin', 'first'])
  })

  it('adds admin-only links from admin settings and sorts both sources together', () => {
    mocks.auth.isAdmin = true
    mocks.app.cachedPublicSettings.header_links = [link('public', { sort_order: 2 })]
    mocks.admin.headerLinks = [link('public', { sort_order: 2 }), link('private', { visibility: 'admin', sort_order: 1 })]
    expect(render().findAll('[data-header-link]').map(node => node.attributes('data-header-link'))).toEqual(['private', 'public'])
  })

  it('uses anchors for new tabs and external same-page links, and router-link for internal same-page links', () => {
    mocks.app.cachedPublicSettings.header_links = [
      link('external-new'), link('internal-new', { url: '/docs/api' }),
      link('internal-same', { url: '/keys?view=all', open_in_new_tab: false }), link('external-same', { open_in_new_tab: false }),
    ]
    const wrapper = render()
    for (const id of ['external-new', 'internal-new']) {
      const node = wrapper.get(`[data-header-link="${id}"]`)
      expect(node.element.tagName).toBe('A')
      expect(node.attributes()).toMatchObject({ target: '_blank', rel: 'noopener noreferrer', 'aria-label': id })
      expect(node.attributes('data-router-link')).toBeUndefined()
    }
    const internal = wrapper.get('[data-header-link="internal-same"]')
    expect(internal.attributes('data-router-link')).toBeDefined()
    expect(internal.attributes('href')).toBe('/keys?view=all')
    for (const id of ['internal-same', 'external-same']) {
      expect(wrapper.get(`[data-header-link="${id}"]`).attributes('target')).toBeUndefined()
      expect(wrapper.get(`[data-header-link="${id}"]`).attributes('rel')).toBeUndefined()
    }
    expect(wrapper.get('[data-header-link="external-same"]').attributes('data-router-link')).toBeUndefined()
  })

  it('sanitizes uploaded SVG, supplies a fallback icon and keeps accessible labels on small screens', () => {
    mocks.app.cachedPublicSettings.header_links = [link('svg', { icon_svg: '<svg onload="alert(1)"><script>alert(1)</script><path fill="#123456" d="M0 0"/></svg>' }), link('fallback')]
    const wrapper = render()
    const svgLink = wrapper.get('[data-header-link="svg"]')
    expect(svgLink.find('script').exists()).toBe(false)
    expect(svgLink.get('svg').attributes('onload')).toBeUndefined()
    expect(svgLink.get('path').attributes('fill')).toBe('#123456')
    expect(wrapper.get('[data-header-link="fallback"] svg').exists()).toBe(true)
    expect(svgLink.classes()).toContain('flex')
    expect(svgLink.find('span.hidden').classes()).toContain('xl:inline')
  })

  it('ignores unsafe destinations and handles missing header_links from older responses', () => {
    mocks.app.cachedPublicSettings.header_links = ['javascript:alert(1)', '//other.example', '/\\other.example', '/bad path', 'data:text/html,test'].map((url, index) => link(String(index), { url }))
    expect(render().findAll('[data-header-link]')).toHaveLength(0)
    mocks.app.cachedPublicSettings.header_links = undefined as unknown as HeaderLink[]
    expect(render().findAll('[data-header-link]')).toHaveLength(0)
  })
})
