import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CreationComposer from '../CreationComposer.vue'
import type { ApiKey } from '@/types'
import type { CreationGroup, ImageGenerationInput } from '@/features/creation/types'
import type { CreationSubmission } from '../types'

// 带参数的 t 会拼上参数，方便断言「输出尺寸」这类插值文案。
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => params ? `${key}|${JSON.stringify(params)}` : key,
    locale: { value: 'en' },
  }),
}))

const group: CreationGroup = {
  id: 1, name: 'Video', description: '', platform: 'video', subscription_type: 'standard', rate_multiplier: 1,
  image: { protocols: ['openai', 'gemini'], models: [{ id: 'gpt-image-1', protocols: ['openai'], pricing: null }], async_supported: true, edit_supported: false },
  video: {
    protocols: ['minimax_v1', 'minimax_v2', 'seedance'], resolutions: [], min_duration: null, max_duration: null,
    models: [
      { id: 'MiniMax-H3', protocols: ['minimax_v1', 'minimax_v2'], pricing: null },
      { id: 'MiniMax-Hailuo-02', protocols: ['minimax_v1', 'minimax_v2'], pricing: null },
      { id: 'doubao-seedance-1-0-pro', protocols: ['seedance'], pricing: null },
    ],
  },
}
const apiKey = { id: 5, key: 'sk-test-key-000000', name: 'Key', group_id: 1, status: 'active' } as ApiKey

/** 生图专用分组：一个模型同时支持 openai / gemini，才切得动协议。 */
const imageGroup: CreationGroup = {
  id: 2, name: 'Images', description: '', platform: 'composite', subscription_type: 'standard', rate_multiplier: 1,
  image: {
    protocols: ['openai', 'gemini'], async_supported: false, edit_supported: false,
    models: [{ id: 'gpt-image-2', protocols: [], pricing: null }],
  },
  video: null,
}
const imageKey = { id: 6, key: 'sk-image-key-000000', name: 'Image key', group_id: 2, status: 'active' } as ApiKey

/** Grok 分组：走 openai 协议，但只认比例 + 分辨率档。 */
const grokGroup: CreationGroup = {
  id: 3, name: 'Grok', description: '', platform: 'grok', subscription_type: 'standard', rate_multiplier: 1,
  image: {
    protocols: ['openai'], async_supported: true, edit_supported: false,
    models: [{ id: 'grok-2-image', protocols: ['openai'], pricing: null }],
  },
  video: null,
}
const grokKey = { id: 7, key: 'sk-grok-key-000000', name: 'Grok key', group_id: 3, status: 'active' } as ApiKey

function mountComposer(
  mode: 'image' | 'video',
  options: { groups?: CreationGroup[]; apiKeys?: ApiKey[] } = {},
): VueWrapper {
  return mount(CreationComposer, {
    props: {
      mode, groups: options.groups ?? [group], apiKeys: options.apiKeys ?? [apiKey], userId: 7,
      loadingKeys: false, creatingKey: false, keyError: '', atCapacity: false, maxActive: 5,
    },
  })
}

async function pickModel(wrapper: VueWrapper, model: string): Promise<void> {
  // 下拉依次是：分组、API Key、模型。
  wrapper.findAllComponents({ name: 'Select' })[2]!.vm.$emit('update:modelValue', model)
  await wrapper.vm.$nextTick()
}

async function submitted(wrapper: VueWrapper): Promise<CreationSubmission> {
  await wrapper.get('#creation-prompt').setValue('a quiet harbour at dawn')
  const before = wrapper.emitted('submit')?.length ?? 0
  await wrapper.get('form').trigger('submit')
  const events = wrapper.emitted('submit') ?? []
  expect(events).toHaveLength(before + 1)
  // 提交后有 1 秒防重复点击的冷却，过了冷却才能再次提交。
  vi.advanceTimersByTime(1000)
  await wrapper.vm.$nextTick()
  return events[events.length - 1]![0] as CreationSubmission
}

function imageInput(submission: CreationSubmission): ImageGenerationInput {
  if (submission.kind !== 'image') throw new Error(`expected an image submission, got ${submission.kind}`)
  return submission.input
}

/** 某个胶囊组的全部取值（顺序即渲染顺序）。 */
function chipValues(wrapper: VueWrapper, name: string): string[] {
  return wrapper.findAll(`input[name="${name}"]`).map(input => input.attributes('value') ?? '')
}

/** 某个胶囊组当前选中的取值。 */
function checkedChips(wrapper: VueWrapper, name: string): string[] {
  return wrapper.findAll(`input[name="${name}"]`)
    .filter(input => (input.element as HTMLInputElement).checked)
    .map(input => input.attributes('value') ?? '')
}

async function pickChip(wrapper: VueWrapper, name: string, value: string): Promise<void> {
  const input = wrapper.findAll(`input[name="${name}"]`).find(chip => chip.attributes('value') === value)
  if (!input) throw new Error(`no chip ${name}=${value}`)
  await input.setValue(true)
  await wrapper.vm.$nextTick()
}

describe('CreationComposer video protocol', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
  })
  afterEach(() => vi.useRealTimers())

  it('offers no protocol choice for video and lists every model of the group', () => {
    const wrapper = mountComposer('video')

    expect(wrapper.find('input[name="creation-protocol"]').exists()).toBe(false)
    expect(wrapper.findAllComponents({ name: 'Select' })[2]!.props('options')).toEqual([
      expect.objectContaining({ value: 'MiniMax-H3' }),
      expect.objectContaining({ value: 'MiniMax-Hailuo-02' }),
      expect.objectContaining({ value: 'doubao-seedance-1-0-pro' }),
    ])
  })

  it('submits with the protocol that belongs to the chosen model', async () => {
    const wrapper = mountComposer('video')

    expect((await submitted(wrapper)).input).toMatchObject({ model: 'MiniMax-H3', protocol: 'minimax_v2' })

    await pickModel(wrapper, 'MiniMax-Hailuo-02')
    expect((await submitted(wrapper)).input).toMatchObject({ model: 'MiniMax-Hailuo-02', protocol: 'minimax_v1' })

    await pickModel(wrapper, 'doubao-seedance-1-0-pro')
    expect((await submitted(wrapper)).input).toMatchObject({ model: 'doubao-seedance-1-0-pro', protocol: 'seedance' })
  })

  it('switches the parameters to the new protocol when the model changes interface', async () => {
    const wrapper = mountComposer('video')
    const resolutions = () => wrapper.findAll('input[name="creation-video-resolution"]').map(input => input.attributes('value'))

    expect(resolutions()).toEqual(['', '480p', '768p', '1080p', '2k'])
    await pickModel(wrapper, 'doubao-seedance-1-0-pro')
    expect(resolutions()).toEqual(['', '480p', '720p', '1080p'])
  })

  it('still lets the user choose between image protocols', () => {
    const wrapper = mountComposer('image')

    expect(wrapper.findAll('input[name="creation-protocol"]').map(input => input.attributes('value'))).toEqual(['openai', 'gemini'])
  })
})

describe('CreationComposer pinned footer', () => {
  let rectTop = 241

  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  /** 面板顶部固定返回 top；matchMedia 默认按桌面视口（matches = true）。 */
  function mountAtViewport(top: number, matches = true): { wrapper: VueWrapper; media: Record<string, unknown> } {
    rectTop = top
    const media = {
      matches, media: '(min-width: 768px)', onchange: null,
      addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    }
    const matchMedia = vi.fn(() => media)
    vi.stubGlobal('matchMedia', matchMedia)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(() => ({
      top: rectTop, bottom: 0, left: 0, right: 0, width: 0, height: 0, x: 0, y: 0, toJSON: () => ({}),
    }) as DOMRect)
    return { wrapper: mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] }), media }
  }

  function panelTop(wrapper: VueWrapper): string {
    return (wrapper.get('form').element as HTMLElement).style.getPropertyValue('--cs-panel-top')
  }

  async function flushFrame(): Promise<void> {
    vi.advanceTimersByTime(32)
    await Promise.resolve()
  }

  it('writes the panel top to --cs-panel-top so the footer stays inside the viewport', async () => {
    const { wrapper } = mountAtViewport(241)

    expect(window.matchMedia).toHaveBeenCalledWith('(min-width: 768px)')
    // 未滚动：面板在页面标题下方，高度按 241px 收窄。
    expect(panelTop(wrapper)).toBe('241px')

    // 页面滚动后面板吸顶，顶部变成 96px（= --cs-sticky-top），高度恢复整屏。
    rectTop = 96
    window.dispatchEvent(new Event('scroll'))
    await flushFrame()
    expect(panelTop(wrapper)).toBe('96px')

    // 快速连续滚动只在下一帧写一次（rAF 节流）。
    rectTop = 88
    window.dispatchEvent(new Event('scroll'))
    window.dispatchEvent(new Event('scroll'))
    expect(panelTop(wrapper)).toBe('96px')
    await flushFrame()
    expect(panelTop(wrapper)).toBe('88px')
  })

  it('re-measures when the viewport resizes', async () => {
    const { wrapper } = mountAtViewport(300)
    expect(panelTop(wrapper)).toBe('300px')

    rectTop = 120
    window.dispatchEvent(new Event('resize'))
    await flushFrame()
    expect(panelTop(wrapper)).toBe('120px')
  })

  it('leaves the variable empty below md so the mobile sticky footer still applies', async () => {
    const { wrapper } = mountAtViewport(241, false)
    expect(panelTop(wrapper)).toBe('')

    // 从桌面缩到窄屏：下一次测量会清掉变量。
    const form = wrapper.get('form').element as HTMLElement
    form.style.setProperty('--cs-panel-top', '241px')
    window.dispatchEvent(new Event('scroll'))
    await flushFrame()
    expect(panelTop(wrapper)).toBe('')
  })

  it('cleans up the variable and the listeners on unmount', async () => {
    const { wrapper } = mountAtViewport(241)
    const form = wrapper.get('form').element as HTMLElement
    expect(form.style.getPropertyValue('--cs-panel-top')).toBe('241px')

    wrapper.unmount()
    expect(form.style.getPropertyValue('--cs-panel-top')).toBe('')

    // 卸载后再滚动 / 缩放不应报错，也不会再写变量。
    rectTop = 96
    window.dispatchEvent(new Event('scroll'))
    window.dispatchEvent(new Event('resize'))
    await flushFrame()
    expect(form.style.getPropertyValue('--cs-panel-top')).toBe('')
  })
})

describe('CreationComposer image parameters', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    localStorage.clear()
  })
  afterEach(() => vi.useRealTimers())

  it('shows resolution first, then the aspect ratios that resolution supports', async () => {
    const wrapper = mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] })

    expect(chipValues(wrapper, 'creation-image-resolution')).toEqual(['1K', '2K', '4K', 'auto'])
    expect(checkedChips(wrapper, 'creation-image-resolution')).toEqual(['1K'])
    // 1K 只有长边 1024 能成立的七种比例（宽幅比例会低于最小像素或跳到 2K 计费档）。
    expect(chipValues(wrapper, 'creation-image-ratio')).toEqual(['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4'])
    expect(checkedChips(wrapper, 'creation-image-ratio')).toEqual(['1:1'])
    // 旧的「尺寸」胶囊（1024x1024 …）已经不存在。
    expect(wrapper.find('input[name="creation-image-size"]').exists()).toBe(false)

    await pickChip(wrapper, 'creation-image-resolution', '2K')
    expect(chipValues(wrapper, 'creation-image-ratio')).toHaveLength(13)
    expect(checkedChips(wrapper, 'creation-image-ratio')).toEqual(['1:1'])
  })

  it('submits the pixel size resolved from resolution + ratio for OpenAI', async () => {
    const wrapper = mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] })

    await pickChip(wrapper, 'creation-image-resolution', '2K')
    await pickChip(wrapper, 'creation-image-ratio', '16:9')
    expect(wrapper.get('[data-testid="creation-output-size"]').text()).toContain('2048×1152')

    expect(imageInput(await submitted(wrapper))).toMatchObject({ protocol: 'openai', size: '2048x1152', quality: 'auto' })
  })

  it('falls back to the first ratio the new resolution offers', async () => {
    const wrapper = mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] })

    await pickChip(wrapper, 'creation-image-resolution', '2K')
    await pickChip(wrapper, 'creation-image-ratio', '16:9')
    expect(checkedChips(wrapper, 'creation-image-ratio')).toEqual(['16:9'])
    expect(imageInput(await submitted(wrapper))).toMatchObject({ size: '2048x1152' })

    // 1K 没有 16:9 → 自动落到该分辨率下的第一个比例 1:1。
    await pickChip(wrapper, 'creation-image-resolution', '1K')
    expect(chipValues(wrapper, 'creation-image-ratio')).toEqual(['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4'])
    expect(checkedChips(wrapper, 'creation-image-ratio')).toEqual(['1:1'])

    // 4K 同样支持 1:1，切过去后不会被重置；再手选 3:4 提交，尺寸是 4K 档的 2448x3264。
    await pickChip(wrapper, 'creation-image-resolution', '4K')
    expect(chipValues(wrapper, 'creation-image-ratio')).toHaveLength(13)
    expect(checkedChips(wrapper, 'creation-image-ratio')).toEqual(['1:1'])
    await pickChip(wrapper, 'creation-image-ratio', '3:4')
    expect(imageInput(await submitted(wrapper))).toMatchObject({ size: '2448x3264' })
  })

  it('hides the ratio group for the OpenAI auto size and submits size=auto', async () => {
    const wrapper = mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] })

    await pickChip(wrapper, 'creation-image-resolution', 'auto')
    expect(wrapper.find('input[name="creation-image-ratio"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="creation-output-size"]').exists()).toBe(false)

    expect(imageInput(await submitted(wrapper))).toMatchObject({ size: 'auto' })
  })

  it('sends aspectRatio + imageSize for the Gemini protocol', async () => {
    const wrapper = mountComposer('image', { groups: [imageGroup], apiKeys: [imageKey] })

    await pickChip(wrapper, 'creation-protocol', 'gemini')
    expect(chipValues(wrapper, 'creation-image-resolution')).toEqual(['1K', '2K', '4K'])
    expect(chipValues(wrapper, 'creation-image-ratio')).toHaveLength(11)

    await pickChip(wrapper, 'creation-image-resolution', '4K')
    await pickChip(wrapper, 'creation-image-ratio', '3:4')
    // 只有 openai 才显示输出像素尺寸。
    expect(wrapper.find('[data-testid="creation-output-size"]').exists()).toBe(false)

    const input = imageInput(await submitted(wrapper))
    expect(input).toMatchObject({ protocol: 'gemini', aspectRatio: '3:4', imageSize: '4K' })
    expect(input.size).toBeUndefined()
  })

  it('sends aspectRatio + imageSize for Grok groups without a quality row', async () => {
    const wrapper = mountComposer('image', { groups: [grokGroup], apiKeys: [grokKey] })

    expect(chipValues(wrapper, 'creation-image-resolution')).toEqual(['1K', '2K'])
    expect(chipValues(wrapper, 'creation-image-ratio')).toHaveLength(13)
    expect(wrapper.find('input[name="creation-image-quality"]').exists()).toBe(false)

    await pickChip(wrapper, 'creation-image-resolution', '2K')
    await pickChip(wrapper, 'creation-image-ratio', '19.5:9')

    const input = imageInput(await submitted(wrapper))
    expect(input).toMatchObject({ protocol: 'openai', aspectRatio: '19.5:9', imageSize: '2K', preferAsync: true })
    expect(input.size).toBeUndefined()
  })
})
