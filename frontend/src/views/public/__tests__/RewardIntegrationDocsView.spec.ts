import { flushPromises, mount } from '@vue/test-utils'
import { webcrypto } from 'node:crypto'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

const appMock = vi.hoisted(() => ({
  cachedPublicSettings: { site_name: 'Sub2API', site_logo: '' },
  fetchPublicSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appMock,
}))

// 用真实 en 文案解析 key：既验证渲染文案，也能发现拼错的动态 key
vi.mock('vue-i18n', async () => {
  const { ref } = await vi.importActual<typeof import('vue')>('vue')
  const en = (await import('@/i18n/locales/en')).default as Record<string, unknown>
  const zh = (await import('@/i18n/locales/zh')).default as Record<string, unknown>
  const locale = ref<'en' | 'zh'>('en')

  function lookup(messages: Record<string, unknown>, key: string): string {
    const value = key.split('.').reduce<unknown>((current, segment) => {
      if (current && typeof current === 'object') {
        return (current as Record<string, unknown>)[segment]
      }
      return undefined
    }, messages)
    return typeof value === 'string' ? value : key
  }

  function translate(key: string, params?: Record<string, unknown>): string {
    const messages = locale.value === 'zh' ? zh : en
    let text = lookup(messages, key)
    for (const [name, value] of Object.entries(params ?? {})) {
      text = text.split(`{${name}}`).join(String(value))
    }
    return text
  }

  return { useI18n: () => ({ t: translate, locale }) }
})

vi.mock('@/i18n', () => ({ i18n: { global: { t: (key: string) => key } } }))

import en from '../../../i18n/locales/en'
import zh from '../../../i18n/locales/zh'
import { REWARD_OPEN_ERROR_CODES } from '../../../components/rewardIntegration/errorCodes'
import { buildRewardCodeExamples } from '../../../components/rewardIntegration/codeExamples'
import { REWARD_DOCS_SECTIONS, rewardDocsAnchor } from '../../../components/rewardIntegration/sections'
import RewardIntegrationDocsView from '../RewardIntegrationDocsView.vue'

// 期望值由 Node crypto.createHmac 预先算出：
// createHmac('sha256', secret).update(timestamp + '\n' + nonce + '\n' + body, 'utf8').digest('hex')
const FIXED_SECRET = 'rs_local_test_secret_1234567890abcdef'
const FIXED_TIMESTAMP = '1730790000'
const FIXED_NONCE = 'nonce_abc123456789'
const FIXED_BODY = '{"event_id":"evt_test_1","event_type":"register","ticket":"rt_test","external_user_id":"u_1"}'
const EXPECTED_SIGNATURE = '2284ee2b31d1083b49b1be0e308482abab1af4b821944e2d80452bb3a12b1bf6'

function render() {
  return mount(RewardIntegrationDocsView, {
    attachTo: document.body,
    global: {
      stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } },
    },
  })
}

function hasLocaleKey(messages: unknown, key: string): boolean {
  const value = key.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object') {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
  return typeof value === 'string' && value.trim() !== ''
}

beforeAll(() => {
  // jsdom 不自带 SubtleCrypto，用 Node 的 WebCrypto 补上
  if (!globalThis.crypto?.subtle) {
    Object.defineProperty(globalThis, 'crypto', { configurable: true, value: webcrypto })
  }
})

beforeEach(() => {
  vi.clearAllMocks()
  appMock.fetchPublicSettings.mockResolvedValue({ site_name: 'Sub2API', site_logo: '' })
})

describe('RewardIntegrationDocsView', () => {
  it('渲染九个章节锚点与目录项', () => {
    const wrapper = render()

    expect(REWARD_DOCS_SECTIONS).toHaveLength(9)

    for (const section of REWARD_DOCS_SECTIONS) {
      const anchor = rewardDocsAnchor(section.id)
      expect(wrapper.find(`#${anchor}`).exists(), `missing section #${anchor}`).toBe(true)
      expect(wrapper.find(`[data-test="reward-docs-toc-${section.id}"]`).exists()).toBe(true)
      expect(wrapper.find(`a[href="#${anchor}"]`).exists()).toBe(true)
    }

    expect(wrapper.findAll('[data-test^="reward-docs-section-"]')).toHaveLength(9)
  })

  it('按契约展示关键请求头、路径与错误码', () => {
    const wrapper = render()
    const text = wrapper.text()

    expect(text).toContain('X-Reward-App-Key')
    expect(text).toContain('X-Reward-Timestamp')
    expect(text).toContain('X-Reward-Nonce')
    expect(text).toContain('X-Reward-Signature')
    expect(text).toContain('/api/v1/reward-open/ping')
    expect(text).toContain('/api/v1/reward-open/tickets/query')
    expect(text).toContain('/api/v1/reward-open/events')
    expect(text).toContain('/api/v1/reward-open/sdk/heartbeat')
    expect(text).toContain('REWARD_SIGNATURE_INVALID')
    expect(text).toContain('REWARD_EXTERNAL_USER_USED')
    expect(text).toContain('task_reward')
    expect(text).toContain('heartbeat_interval')
    expect(text).toContain('idle_timeout_seconds')
  })

  it('签名计算器对固定输入算出正确的小写 hex', async () => {
    const wrapper = render()

    await wrapper.get('#reward-calc-secret').setValue(FIXED_SECRET)
    await wrapper.get('#reward-calc-timestamp').setValue(FIXED_TIMESTAMP)
    await wrapper.get('#reward-calc-nonce').setValue(FIXED_NONCE)
    await wrapper.get('#reward-calc-body').setValue(FIXED_BODY)
    await flushPromises()

    await vi.waitFor(() => {
      expect(wrapper.get('[data-test="signature-hex"]').text()).toBe(EXPECTED_SIGNATURE)
    })

    expect(wrapper.get('[data-test="signing-string"]').text()).toBe(
      [FIXED_TIMESTAMP, FIXED_NONCE, FIXED_BODY].join('\n'),
    )
  })

  it('六种语言示例都包含当前站点 origin', async () => {
    const wrapper = render()
    const origin = window.location.origin

    for (const example of buildRewardCodeExamples(origin)) {
      await wrapper.get(`[data-test="code-tab-${example.id}"]`).trigger('click')
      const code = wrapper.get(`[data-test="code-${example.id}"]`).text()
      expect(code, `${example.id} example should embed ${origin}`).toContain(origin)
      expect(code).toContain('/api/v1/reward-open/events')
      expect(code).toContain('X-Reward-Signature')
    }
  })

  it('按 R1 展示每秒心跳、心跳响应字段与挂机检测', () => {
    const wrapper = render()

    // 心跳响应示例：每秒心跳 + 挂机阈值
    const heartbeat = wrapper.get('[data-test="heartbeat-response"]').text()
    expect(heartbeat).toContain('"heartbeat_interval": 1')
    expect(heartbeat).toContain('"idle_timeout_seconds": 15')

    const text = wrapper.text()
    expect(text).toContain('idle_timeout_seconds')
    expect(text).toContain('paused_reason')
    expect(text).toContain('hidden')
    expect(text).toContain('isTrusted')
    expect(text).toContain('defaults to 1')
    // 旧的「每 5 秒」说法不应再出现
    expect(text).not.toContain('every 5 seconds')

    // SDK 章节的挂机检测小节（阈值、暂停、恢复、能力边界、自绘示例）
    const idle = wrapper.get('[data-test="sdk-idle"]')
    expect(idle.text()).toContain('idle')
    expect(idle.text()).toContain('15 seconds')

    const snippet = wrapper.get('[data-test="sdk-custom-ui"]').text()
    expect(snippet).toContain('data-badge="false"')
    expect(snippet).toContain('paused_reason')
    expect(snippet).toContain('onChange')
    expect(snippet).toContain(window.location.origin)
  })

  it('页面用到的动态 i18n key 在中英文里都存在', () => {
    const keys = new Set<string>()

    for (const section of REWARD_DOCS_SECTIONS) keys.add(section.titleKey)
    for (const item of REWARD_OPEN_ERROR_CODES) {
      keys.add(item.scenarioKey)
      keys.add(item.actionKey)
    }
    for (const example of buildRewardCodeExamples('http://localhost')) {
      keys.add(example.labelKey)
      keys.add(example.runKey)
    }
    keys.add('rewardIntegration.originHint')

    for (const key of keys) {
      expect(hasLocaleKey(en, key), `en missing ${key}`).toBe(true)
      expect(hasLocaleKey(zh, key), `zh missing ${key}`).toBe(true)
    }
  })
})
