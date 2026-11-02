import { describe, expect, it, vi } from 'vitest'
import type {
  CreationGroup,
  CreationImageProtocol,
  CreationKind,
  CreationModel,
  CreationVideoCapability,
  CreationVideoProtocol,
} from '@/features/creation/types'
import {
  clampVideoDuration,
  creationHistoryKey,
  creationPriceEntries,
  creationSelectionKey,
  defaultVideoDuration,
  defaultVideoRatios,
  defaultVideoResolutions,
  filterCreationGroups,
  filterCreationModels,
  formatCreationPrice,
  readCreationHistory,
  restoreSelectedGroupId,
  safeMediaUrl,
  saveSelectedGroupId,
  videoDurationRange,
  writeCreationHistory,
} from '../viewModel'
import type { CreationSessionRecord } from '../viewModel'

function makeGroup(
  id: number,
  image: CreationGroup['image'] = null,
  video: CreationGroup['video'] = null,
): CreationGroup {
  return {
    id,
    name: `Group ${id}`,
    description: '',
    platform: 'composite',
    subscription_type: 'standard',
    rate_multiplier: 1,
    image,
    video,
  }
}

function imageCapability(
  protocols: CreationImageProtocol[] = ['openai'],
  models: CreationModel[] = [],
): NonNullable<CreationGroup['image']> {
  return { protocols, models, async_supported: true, edit_supported: true }
}

function videoCapability(
  protocols: CreationVideoProtocol[] = ['seedance'],
  models: CreationModel[] = [],
): CreationVideoCapability {
  return { protocols, models, resolutions: [], min_duration: null, max_duration: null }
}

function durationCapability(min: number | null, max: number | null): CreationVideoCapability {
  return { ...videoCapability(), min_duration: min, max_duration: max }
}

function createStorage() {
  const data = new Map<string, string>()
  return {
    data,
    getItem(key: string) {
      return data.get(key) ?? null
    },
    setItem(key: string, value: string) {
      data.set(key, String(value))
    },
  }
}

function makeRecord(id: string, kind: CreationKind = 'image'): CreationSessionRecord {
  return {
    id,
    kind,
    model: kind === 'image' ? 'gpt-image-2' : 'seedance-1.0-pro',
    protocol: kind === 'image' ? 'openai' : 'seedance',
    prompt: `Prompt for ${id}`,
    createdAt: 1_700_000_000_000,
    assets: [{ kind, url: `https://cdn.example.com/${id}.media` }],
  }
}

describe('creation catalog view model', () => {
  it('returns an empty group list for a missing catalog', () => {
    expect(filterCreationGroups(null, 'image')).toEqual([])
    expect(filterCreationGroups(undefined, 'video')).toEqual([])
  })

  it('keeps only groups with a non-null capability and at least one protocol', () => {
    const both = makeGroup(1, imageCapability(['openai', 'gemini']), videoCapability())
    const emptyImage = makeGroup(2, imageCapability([]))
    const videoOnly = makeGroup(3, null, videoCapability())
    const emptyVideo = makeGroup(4, null, videoCapability([]))
    const groups = [emptyImage, both, emptyVideo, videoOnly]

    expect(filterCreationGroups(groups, 'image')).toEqual([both])
    expect(filterCreationGroups(groups, 'video')).toEqual([both, videoOnly])
  })

  it('returns no models for a missing group, capability, or unsupported protocol', () => {
    const group = makeGroup(1, imageCapability(['openai'], [{ id: 'wildcard', protocols: [] }]))

    expect(filterCreationModels(null, 'image', 'openai')).toEqual([])
    expect(filterCreationModels(undefined, 'image', 'openai')).toEqual([])
    expect(filterCreationModels(group, 'video', 'seedance')).toEqual([])
    expect(filterCreationModels(group, 'image', 'gemini')).toEqual([])
    expect(filterCreationModels(makeGroup(2, imageCapability([])), 'image', 'openai')).toEqual([])
  })

  it('filters models by protocol, treats an empty model protocol list as wildcard, and deduplicates stably', () => {
    const group = makeGroup(1, imageCapability(['openai', 'gemini'], [
      { id: 'shared', protocols: [] },
      { id: 'openai-only', protocols: ['openai'] },
      { id: 'shared', protocols: ['openai'] },
      { id: 'gemini-only', protocols: ['gemini'] },
      { id: 'openai-only', protocols: ['openai'] },
    ]))

    expect(filterCreationModels(group, 'image', 'openai').map(model => model.id)).toEqual([
      'shared',
      'openai-only',
    ])
  })
})

describe('creation group selection persistence', () => {
  it('builds user- and kind-specific selection keys', () => {
    expect(creationSelectionKey(7, 'image')).toBe('sub2api.creation.selection.7.image')
    expect(creationSelectionKey('user-a', 'video')).toBe('sub2api.creation.selection.user-a.video')
  })

  it('saves and restores a group independently for each user and kind', () => {
    const storage = createStorage()
    const groups = [makeGroup(10), makeGroup(20)]

    saveSelectedGroupId(storage, 7, 'image', 20)
    saveSelectedGroupId(storage, 7, 'video', 10)
    saveSelectedGroupId(storage, 8, 'image', 10)

    expect(restoreSelectedGroupId(storage, 7, 'image', groups)).toBe(20)
    expect(restoreSelectedGroupId(storage, 7, 'video', groups)).toBe(10)
    expect(restoreSelectedGroupId(storage, 8, 'image', groups)).toBe(10)
    expect(storage.data.get('sub2api.creation.selection.7.image')).toBe('20')
  })

  it.each([null, '', 'not-a-number', '2.5', '999'])(
    'does not fall back when the stored selection %j is invalid or absent from the catalog',
    stored => {
      const groups = [makeGroup(1), makeGroup(2)]
      const storage = { getItem: () => stored }

      expect(restoreSelectedGroupId(storage, 7, 'image', groups)).toBeNull()
    },
  )

  it('contains storage read and write failures', () => {
    const storage = {
      getItem: () => { throw new Error('read blocked') },
      setItem: () => { throw new Error('write blocked') },
    }

    expect(restoreSelectedGroupId(storage, 7, 'image', [makeGroup(1)])).toBeNull()
    expect(() => saveSelectedGroupId(storage, 7, 'image', 1)).not.toThrow()
  })
})

describe('video duration rules', () => {
  it.each([
    ['seedance', 'doubao-seedance-1-0-pro-250528', { min: 2, max: 12 }],
    ['seedance', 'seedance-1.5-pro', { min: 4, max: 12 }],
    ['seedance', 'seedance-2.0', { min: 4, max: 15 }],
    ['seedance', 'seedance-2.5', { min: 4, max: 30 }],
    ['gemini', 'veo-2.0-generate-001', { min: 5, max: 8 }],
    ['gemini', 'veo-3.0-generate-001', { min: 8, max: 8 }],
    ['gemini', 'veo-3.1-generate-preview', { min: 4, max: 8 }],
    ['minimax_v1', 'video-01', { min: 6, max: 10 }],
    ['minimax_v2', 'MiniMax-H3', { min: 4, max: 15 }],
    ['minimax_v2', 'MiniMax-H3-Max', { min: 5, max: 15 }],
    ['grok', 'grok-imagine-video', { min: null, max: null }],
    ['seedance', 'unrecognized-mapped-model', { min: null, max: null }],
  ] satisfies [CreationVideoProtocol, string, { min: number | null; max: number | null }][])(
    'uses the protocol defaults for %s %s',
    (protocol, model, expected) => {
      expect(videoDurationRange(protocol, model)).toEqual(expected)
    },
  )

  it('prefers each configured catalog bound and falls back independently for null bounds', () => {
    expect(videoDurationRange('seedance', 'seedance-2.0', durationCapability(6, null))).toEqual({ min: 6, max: 15 })
    expect(videoDurationRange('seedance', 'seedance-2.0', durationCapability(null, 10))).toEqual({ min: 4, max: 10 })
    expect(videoDurationRange('grok', 'grok-video', durationCapability(3, 9))).toEqual({ min: 3, max: 9 })
  })

  it.each([
    ['seedance', 'seedance-1.0-pro', 5],
    ['seedance', 'seedance-2.0', 5],
    ['seedance', 'seedance-2.5', undefined],
    ['gemini', 'veo-2.0-generate-001', 8],
    ['gemini', 'veo-3.0-generate-001', 8],
    ['gemini', 'veo-3.1-generate-preview', 8],
    ['minimax_v1', 'video-01', 6],
    ['minimax_v2', 'MiniMax-H3', 6],
    ['grok', 'grok-video', undefined],
  ] satisfies [CreationVideoProtocol, string, number | undefined][])(
    'uses the default duration for %s %s',
    (protocol, model, expected) => {
      expect(defaultVideoDuration(protocol, model)).toBe(expected)
    },
  )

  it('keeps configured defaults within the effective range and discrete Veo choices', () => {
    expect(defaultVideoDuration('seedance', 'seedance-1.0-pro', durationCapability(8, 10))).toBe(8)
    expect(defaultVideoDuration('gemini', 'veo-2.0', durationCapability(5, 6))).toBe(6)
    expect(defaultVideoDuration('gemini', 'veo-3.1', durationCapability(5, 7))).toBe(6)
    expect(defaultVideoDuration('minimax_v1', 'video-01', durationCapability(8, 12))).toBe(8)
  })

  it.each([null, undefined, '', '   '])('allows an empty duration %j to remain empty', value => {
    expect(clampVideoDuration(value, 'seedance', 'seedance-1.0-pro')).toBeUndefined()
  })

  it('accepts numeric strings, rounds numbers, and clamps them to the effective range', () => {
    expect(clampVideoDuration('7.6', 'seedance', 'seedance-1.0-pro')).toBe(8)
    expect(clampVideoDuration(1, 'seedance', 'seedance-1.0-pro')).toBe(2)
    expect(clampVideoDuration(99, 'seedance', 'seedance-1.0-pro')).toBe(12)
    expect(clampVideoDuration(4.4, 'grok', 'grok-video')).toBe(4)
  })

  it('falls back to the default for invalid duration input', () => {
    for (const value of ['not-a-number', Number.NaN, Number.POSITIVE_INFINITY, false, {}, []]) {
      expect(clampVideoDuration(value, 'seedance', 'seedance-1.0-pro')).toBe(5)
    }
    expect(clampVideoDuration('not-a-number', 'seedance', 'seedance-2.5')).toBeUndefined()
  })

  it.each([
    [3, 4],
    [4.4, 4],
    [4.6, 6],
    [5, 6],
    [6.4, 6],
    [7, 8],
    [100, 8],
  ])('clamps Veo 3.1 duration %s to the nearest allowed value %s', (value, expected) => {
    expect(clampVideoDuration(value, 'gemini', 'veo-3.1-generate-preview')).toBe(expected)
  })

  it('restricts Veo 3.0 to eight seconds', () => {
    expect(clampVideoDuration(4, 'gemini', 'veo-3.0-generate-001')).toBe(8)
    expect(clampVideoDuration(12, 'gemini', 'veo-3.0-generate-001')).toBe(8)
  })

  it('uses only six or ten seconds for an unconfigured MiniMax v1 catalog', () => {
    expect(clampVideoDuration(5, 'minimax_v1', 'video-01')).toBe(6)
    expect(clampVideoDuration(7, 'minimax_v1', 'video-01')).toBe(6)
    expect(clampVideoDuration(8, 'minimax_v1', 'video-01')).toBe(10)
    expect(clampVideoDuration(11, 'minimax_v1', 'video-01')).toBe(10)
  })

  it('uses the configured continuous MiniMax v1 range instead of discrete defaults', () => {
    const capability = durationCapability(5, 12)

    expect(clampVideoDuration(7, 'minimax_v1', 'video-01', capability)).toBe(7)
    expect(clampVideoDuration(20, 'minimax_v1', 'video-01', capability)).toBe(12)
  })
})

describe('video resolution and ratio defaults', () => {
  it.each([
    ['seedance', ['480p', '720p', '1080p']],
    ['gemini', ['720p', '1080p']],
    ['minimax_v1', ['480p', '768p', '1080p', '2k']],
    ['minimax_v2', ['480p', '768p', '1080p', '2k']],
    ['grok', ['480p', '720p', '1080p']],
  ] satisfies [CreationVideoProtocol, string[]][])('uses resolution defaults for %s', (protocol, expected) => {
    expect(defaultVideoResolutions(protocol)).toEqual(expected)
  })

  it('prefers non-empty catalog resolutions, deduplicates them, and keeps their order', () => {
    const capability = { ...videoCapability(), resolutions: ['1080p', '720p', '1080p'] }

    expect(defaultVideoResolutions('seedance', capability)).toEqual(['1080p', '720p'])
    expect(defaultVideoResolutions('seedance', videoCapability())).toEqual(['480p', '720p', '1080p'])
  })

  it.each([
    ['gemini', ['16:9', '9:16']],
    ['minimax_v1', ['16:9', '9:16', '1:1', 'adaptive']],
    ['minimax_v2', ['16:9', '9:16', '1:1', 'adaptive']],
    ['seedance', ['16:9', '9:16', '1:1', 'adaptive']],
    ['grok', ['16:9', '9:16', '1:1', 'adaptive']],
  ] satisfies [CreationVideoProtocol, string[]][])('uses ratio defaults for %s', (protocol, expected) => {
    expect(defaultVideoRatios(protocol)).toEqual(expected)
  })
})

describe('safeMediaUrl', () => {
  it.each([
    'http://example.com/video.mp4',
    'https://cdn.example.com/image.png?size=large',
    'HTTPS://CDN.EXAMPLE.COM/IMAGE.PNG',
    'blob:https://example.com/4b48b1ca-0a6c-4b76-9f4d-6bb48f865dc4',
    'data:image/png;base64,iVBORw0KGgo=',
    'data:image/jpeg;base64,/9j/4AAQSkZJRg==',
    'data:image/webp;base64,UklGRg==',
    'data:image/gif;base64,R0lGOD==',
  ])('accepts supported absolute media URL %s', value => {
    expect(safeMediaUrl(value)).toBe(value)
  })

  it('trims surrounding whitespace from a supported URL', () => {
    expect(safeMediaUrl('  https://cdn.example.com/image.png  ')).toBe('https://cdn.example.com/image.png')
  })

  it.each([
    null,
    undefined,
    42,
    '',
    '/v1/video-tasks/resource/content',
    '//cdn.example.com/image.png',
    'javascript:alert(1)',
    'ftp://example.com/video.mp4',
    'data:image/svg+xml;base64,PHN2Zz4=',
    'data:text/html;base64,PGgxPg==',
    'https://',
    'http://[',
  ])('rejects unsafe, relative, or malformed media URL %j', value => {
    expect(safeMediaUrl(value)).toBe('')
  })
})

describe('creation history persistence', () => {
  it('builds a user-specific history key', () => {
    expect(creationHistoryKey(7)).toBe('sub2api.creation.history.7')
    expect(creationHistoryKey('user-a')).toBe('sub2api.creation.history.user-a')
  })

  it.each([null, '', '{', '{}', 'null', '"text"'])('returns no history for missing or malformed storage value %j', raw => {
    expect(readCreationHistory({ getItem: () => raw }, 7, 'image')).toEqual([])
  })

  it('returns no history when storage reads fail', () => {
    const storage = { getItem: () => { throw new Error('read blocked') } }

    expect(readCreationHistory(storage, 7, 'image')).toEqual([])
  })

  it('validates every record field and discards malformed records', () => {
    const valid = makeRecord('valid')
    const storage = {
      getItem: () => JSON.stringify([
        null,
        [],
        {},
        { ...valid, id: 7 },
        { ...valid, kind: 'audio' },
        { ...valid, model: null },
        { ...valid, protocol: 'unknown' },
        { ...valid, prompt: 42 },
        { ...valid, createdAt: 'yesterday' },
        { ...valid, assets: 'not-an-array' },
        valid,
      ]),
    }

    expect(readCreationHistory(storage, 7, 'image')).toEqual([valid])
  })

  it('filters by kind, removes invalid and blob asset URLs, and returns only whitelisted fields', () => {
    const storage = {
      getItem: () => JSON.stringify([
        {
          ...makeRecord('image-record'),
          apiKey: 'must-not-be-restored',
          assets: [
            { kind: 'image', url: 'https://cdn.example.com/image.png', mimeType: 'image/png' },
            { kind: 'image', url: 'data:image/webp;base64,UklGRg==' },
            { kind: 'image', url: 'blob:https://example.com/session-only' },
            { kind: 'image', url: '/relative/image.png' },
            { kind: 'image', url: 'javascript:alert(1)' },
            { kind: 'audio', url: 'https://cdn.example.com/audio.mp3' },
            { kind: 'image', url: 42 },
            { kind: 'image', url: 'https://cdn.example.com/image.png', mimeType: 42 },
          ],
        },
        makeRecord('video-record', 'video'),
      ]),
    }

    expect(readCreationHistory(storage, 7, 'image')).toEqual([{
      id: 'image-record',
      kind: 'image',
      model: 'gpt-image-2',
      protocol: 'openai',
      prompt: 'Prompt for image-record',
      createdAt: 1_700_000_000_000,
      assets: [
        { kind: 'image', url: 'https://cdn.example.com/image.png', mimeType: 'image/png' },
        { kind: 'image', url: 'data:image/webp;base64,UklGRg==' },
      ],
    }])
  })

  it('limits restored history to the first twenty valid records of the requested kind', () => {
    const records = Array.from({ length: 25 }, (_, index) => makeRecord(`record-${index}`))
    const storage = { getItem: () => JSON.stringify(records) }

    const restored = readCreationHistory(storage, 7, 'image')

    expect(restored).toHaveLength(20)
    expect(restored.map(record => record.id)).toEqual(records.slice(0, 20).map(record => record.id))
  })

  it('writes only whitelisted fields and drops secrets, invalid URLs, and blob URLs', () => {
    const storage = createStorage()
    const record = {
      id: 'sensitive-record',
      kind: 'image' as const,
      model: 'gpt-image-2',
      protocol: 'openai' as const,
      prompt: 'A safe prompt',
      createdAt: 123,
      apiKey: 'root-api-key',
      key: 'root-key',
      token: 'root-token',
      assets: [
        {
          kind: 'image' as const,
          url: 'https://cdn.example.com/image.png',
          mimeType: 'image/png',
          apiKey: 'asset-api-key',
          sourceUrl: 'https://private.example.com/source',
          objectUrl: false,
        },
        { kind: 'image' as const, url: 'blob:https://example.com/session-only', token: 'asset-token' },
        { kind: 'image' as const, url: '/relative/image.png' },
        { kind: 'image' as const, url: 'data:image/gif;base64,R0lGOD==' },
      ],
    }

    writeCreationHistory(storage, 7, [record])

    const raw = storage.data.get('sub2api.creation.history.7')
    expect(raw).toBeDefined()
    expect(JSON.parse(raw ?? '[]')).toEqual([{
      id: 'sensitive-record',
      kind: 'image',
      model: 'gpt-image-2',
      protocol: 'openai',
      prompt: 'A safe prompt',
      createdAt: 123,
      assets: [
        { kind: 'image', url: 'https://cdn.example.com/image.png', mimeType: 'image/png' },
        { kind: 'image', url: 'data:image/gif;base64,R0lGOD==' },
      ],
    }])
    expect(raw).not.toContain('root-api-key')
    expect(raw).not.toContain('asset-api-key')
    expect(raw).not.toContain('root-token')
    expect(raw).not.toContain('asset-token')
  })

  it('limits written history to twenty records', () => {
    const storage = createStorage()
    const records = Array.from({ length: 25 }, (_, index) => makeRecord(`record-${index}`))

    writeCreationHistory(storage, 7, records)

    const raw = storage.data.get('sub2api.creation.history.7')
    const written: unknown = JSON.parse(raw ?? '[]')
    expect(Array.isArray(written)).toBe(true)
    expect(written).toHaveLength(20)
    expect(readCreationHistory(storage, 7, 'image').map(record => record.id)).toEqual(
      records.slice(0, 20).map(record => record.id),
    )
  })

  it('contains storage write failures', () => {
    const setItem = vi.fn(() => { throw new Error('write blocked') })

    expect(() => writeCreationHistory({ setItem }, 7, [makeRecord('record')])).not.toThrow()
  })
})

describe('creation model pricing display', () => {
  it('keeps sub-cent precision without trailing noise', () => {
    expect(formatCreationPrice(0.04)).toBe('$0.04')
    expect(formatCreationPrice(0.035)).toBe('$0.035')
    expect(formatCreationPrice(0.04 * 1.5)).toBe('$0.06')
    expect(formatCreationPrice(0.0003)).toBe('$0.0003')
    expect(formatCreationPrice(1)).toBe('$1.00')
  })

  it('expands tiers or a flat price', () => {
    expect(creationPriceEntries(null)).toEqual([])
    expect(creationPriceEntries({ unit: 'image', tiers: [{ label: '1K', price: 0.04 }, { label: '2K', price: 0.08 }] }))
      .toEqual([{ label: '1K', price: '$0.04' }, { label: '2K', price: '$0.08' }])
    expect(creationPriceEntries({ unit: 'request', price: 2, tiers: [] })).toEqual([{ label: '', price: '$2.00' }])
    expect(creationPriceEntries({ unit: 'image', tiers: [] })).toEqual([])
  })
})
