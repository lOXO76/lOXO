import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CreationComposer from '../CreationComposer.vue'
import type { ApiKey } from '@/types'
import type { CreationGroup } from '@/features/creation/types'
import type { CreationSubmission } from '../types'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))

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

function mountComposer(mode: 'image' | 'video'): VueWrapper {
  return mount(CreationComposer, {
    props: {
      mode, groups: [group], apiKeys: [apiKey], userId: 7,
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
