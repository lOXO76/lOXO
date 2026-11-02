import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ApiKey } from '@/types'
import type { CreationGroup } from '../types'

const get = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', () => ({ apiClient: { get } }))
import { creationAPI, creationPlatformLabel, getCreationCatalog, groupSupports, pickKeysForGroup, resolveModels } from '../api'

const group: CreationGroup = {
  id: 3, name: 'Images', description: '', platform: 'composite', subscription_type: 'standard', rate_multiplier: 1,
  image: {
    protocols: ['openai', 'gemini'], async_supported: true, edit_supported: true,
    models: [{ id: 'all', protocols: [] }, { id: 'gpt-image-2', protocols: ['openai'] }, { id: 'gemini-image', protocols: ['gemini'] }],
  },
  video: null,
}

describe('creation catalog', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the JWT client and forwards cancellation', async () => {
    const catalog = { groups: [group] }
    const signal = new AbortController().signal
    get.mockResolvedValue({ data: catalog })
    expect(await getCreationCatalog(signal)).toBe(catalog)
    expect(get).toHaveBeenCalledWith('/user/creation/catalog', { signal })
    expect(creationAPI.getCatalog).toBe(getCreationCatalog)
  })

  it('filters models by capability and includes models without protocol restrictions', () => {
    expect(groupSupports(group, 'image')).toBe(true)
    expect(groupSupports(group, 'video')).toBe(false)
    expect(resolveModels(group, 'image', 'openai').map((model) => model.id)).toEqual(['all', 'gpt-image-2'])
    expect(resolveModels(group, 'image', 'gemini').map((model) => model.id)).toEqual(['all', 'gemini-image'])
    expect(resolveModels(group, 'image', 'grok')).toEqual([])
    expect(resolveModels(group, 'video', 'seedance')).toEqual([])
    expect(creationPlatformLabel('minimax_v2')).toBe('minimax_v2')
  })

  it('only selects active API keys assigned to the selected group', () => {
    const keys = [
      { id: 1, group_id: 3, status: 'active' }, { id: 2, group_id: 3, status: 'inactive' },
      { id: 3, group_id: 4, status: 'active' }, { id: 4, group_id: null, status: 'active' },
      { id: 5, group_id: 3, status: 'expired' }, { id: 6, group_id: 3, status: 'quota_exhausted' },
    ] as ApiKey[]
    expect(pickKeysForGroup(keys, 3).map((key) => key.id)).toEqual([1])
    expect(keys).toHaveLength(6)
  })
})
