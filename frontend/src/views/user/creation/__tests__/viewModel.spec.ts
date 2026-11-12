import { describe, expect, it, vi } from 'vitest'
import type { AsyncTask } from '@/api/asyncTasks'
import type {
  CreationGroup,
  CreationImageProtocol,
  CreationKind,
  CreationModel,
  CreationVideoCapability,
  CreationVideoProtocol,
} from '@/features/creation/types'
import {
  buildCreationHistory,
  clampVideoDuration,
  creationHistoryKey,
  creationImageTier,
  creationPreviewShape,
  creationPriceEntries,
  creationPriceRange,
  creationSelectionKey,
  creationVideoModels,
  defaultVideoDuration,
  defaultVideoRatios,
  defaultVideoResolutions,
  estimateCreationCost,
  filterCreationGroups,
  filterCreationModels,
  formatCreationElapsed,
  formatCreationPrice,
  parseCreationAspect,
  readCreationHistory,
  resolveCreationVideoProtocol,
  restoreSelectedGroupId,
  safeMediaUrl,
  saveSelectedGroupId,
  videoDurationRange,
  writeCreationHistory,
} from '../viewModel'
import type { CreationSessionRecord } from '../viewModel'
import type { CreationSubmission } from '../types'

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

describe('creation task card helpers', () => {
  function imageSubmission(input: Partial<Extract<CreationSubmission, { kind: 'image' }>['input']>): CreationSubmission {
    return {
      kind: 'image', groupId: 1, groupName: 'Studio', apiKey: 'sk-test',
      input: { apiKey: 'sk-test', protocol: 'openai', platform: 'openai', model: 'gpt-image-1', prompt: 'cat', ...input },
    }
  }

  function videoSubmission(ratio?: string): CreationSubmission {
    return {
      kind: 'video', groupId: 1, groupName: 'Studio', apiKey: 'sk-test',
      input: { apiKey: 'sk-test', protocol: 'seedance', model: 'seedance-1', prompt: 'cat', ratio },
    }
  }

  it('parses ratios and pixel sizes, and rejects values without a fixed aspect', () => {
    expect(parseCreationAspect('16:9')).toEqual({ w: 16, h: 9 })
    expect(parseCreationAspect('19.5:9')).toEqual({ w: 19.5, h: 9 })
    expect(parseCreationAspect('1536x1024')).toEqual({ w: 1536, h: 1024 })
    expect(parseCreationAspect(' 9 : 16 ')).toEqual({ w: 9, h: 16 })
    for (const value of ['auto', 'adaptive', '', '0:9', '16:0', '1K', '16:9:1', undefined, null, 169]) {
      expect(parseCreationAspect(value)).toBeNull()
    }
  })

  it('shapes image placeholders from the requested count and size', () => {
    expect(creationPreviewShape(imageSubmission({ n: 4, size: '1536x1024' }))).toEqual({ previewCount: 4, aspectW: 1536, aspectH: 1024 })
    expect(creationPreviewShape(imageSubmission({ aspectRatio: '9:16' }))).toEqual({ previewCount: 1, aspectW: 9, aspectH: 16 })
    expect(creationPreviewShape(imageSubmission({ n: 2, size: 'auto' }))).toEqual({ previewCount: 2, aspectW: 1, aspectH: 1 })
  })

  it('keeps the placeholder count within one to eight tiles', () => {
    expect(creationPreviewShape(imageSubmission({ n: 50 })).previewCount).toBe(8)
    expect(creationPreviewShape(imageSubmission({ n: 0 })).previewCount).toBe(1)
    expect(creationPreviewShape(imageSubmission({ n: Number.NaN })).previewCount).toBe(1)
  })

  it('shapes video placeholders from the ratio and defaults to 16:9', () => {
    expect(creationPreviewShape(videoSubmission('9:16'))).toEqual({ previewCount: 1, aspectW: 9, aspectH: 16 })
    expect(creationPreviewShape(videoSubmission('adaptive'))).toEqual({ previewCount: 1, aspectW: 16, aspectH: 9 })
    expect(creationPreviewShape(videoSubmission())).toEqual({ previewCount: 1, aspectW: 16, aspectH: 9 })
  })

  it('formats elapsed time as mm:ss and switches to h:mm:ss after an hour', () => {
    expect(formatCreationElapsed(0)).toBe('00:00')
    expect(formatCreationElapsed(42_900)).toBe('00:42')
    expect(formatCreationElapsed(125_000)).toBe('02:05')
    expect(formatCreationElapsed(3_725_000)).toBe('1:02:05')
    expect(formatCreationElapsed(-5_000)).toBe('00:00')
    expect(formatCreationElapsed(Number.NaN)).toBe('00:00')
  })

  it('summarises model pricing as a single price or a range', () => {
    expect(creationPriceRange(null)).toBe('')
    expect(creationPriceRange({ unit: 'image', tiers: [] })).toBe('')
    expect(creationPriceRange({ unit: 'image', price: 0.04, tiers: [] })).toBe('$0.04')
    expect(creationPriceRange({ unit: 'second', tiers: [{ label: '720p', price: 0.12 }, { label: '480p', price: 0.05 }] })).toBe('$0.05 – $0.12')
    expect(creationPriceRange({ unit: 'image', tiers: [{ label: '1K', price: 0.04 }, { label: '2K', price: 0.04 }] })).toBe('$0.04')
  })
})

describe('creation history feed', () => {
  const nothingHidden = { taskIds: new Set<string>(), recordIds: new Set<string>() }

  function serverTask(overrides: Partial<AsyncTask> = {}): AsyncTask {
    return {
      id: 'imgtask_1', task_id: 'imgtask_1', kind: 'image', protocol: 'openai', model: 'gpt-image-2',
      status: 'completed', api_key_id: 9, created_at: 1_700_000_100, expires_at: 1_700_090_000,
      result_url: 'https://cdn.example.com/first.png', ...overrides,
    }
  }

  it('turns a completed image task into a card with its result', () => {
    expect(buildCreationHistory([serverTask()], [], nothingHidden)).toEqual([{
      key: 'server:imgtask_1', source: 'server', kind: 'image', protocol: 'openai', model: 'gpt-image-2',
      status: 'completed', prompt: '', taskId: 'imgtask_1', createdAt: 1_700_000_100_000,
      images: ['https://cdn.example.com/first.png'], videoUrl: '', downloadId: '', error: '',
    }])
  })

  it('restores the prompt and every image from the record saved on this device', () => {
    const record: CreationSessionRecord = {
      ...makeRecord('local'), taskId: 'imgtask_1', prompt: 'a lighthouse',
      assets: [{ kind: 'image', url: 'https://cdn.example.com/1.png' }, { kind: 'image', url: 'https://cdn.example.com/2.png' }],
    }

    const items = buildCreationHistory([serverTask()], [record], nothingHidden)

    expect(items).toHaveLength(1)
    expect(items[0]).toMatchObject({
      source: 'server', prompt: 'a lighthouse',
      images: ['https://cdn.example.com/1.png', 'https://cdn.example.com/2.png'],
    })
  })

  it('plays Seedance links directly and sends other videos through the download endpoint', () => {
    const [seedance, protectedLink, veo] = buildCreationHistory([
      serverTask({ id: 'vidtask_a', task_id: 'cgt-a', kind: 'video', protocol: 'seedance', result_url: 'https://cdn.example.com/a.mp4' }),
      serverTask({ id: 'vidtask_b', task_id: 'cgt-b', kind: 'video', protocol: 'seedance', result_url: '/api/v1/user/async-tasks/vidtask_b/content' }),
      serverTask({ id: 'vidtask_c', task_id: 'op-c', kind: 'video', protocol: 'gemini', result_url: 'https://generativelanguage.googleapis.com/v1beta/files/c' }),
    ], [], nothingHidden)

    expect(seedance).toMatchObject({ videoUrl: 'https://cdn.example.com/a.mp4', downloadId: '', images: [] })
    expect(protectedLink).toMatchObject({ videoUrl: '', downloadId: 'vidtask_b' })
    expect(veo).toMatchObject({ videoUrl: '', downloadId: 'vidtask_c' })
  })

  it('shows no media for unfinished tasks, missing results, or unsafe links', () => {
    const items = buildCreationHistory([
      serverTask({ id: 'a', task_id: 'a', status: 'processing', result_url: undefined }),
      serverTask({ id: 'b', task_id: 'b', status: 'failed', error: 'blocked' }),
      serverTask({ id: 'c', task_id: 'c', result_url: 'javascript:alert(1)' }),
      serverTask({ id: 'd', task_id: 'd', kind: 'video', protocol: 'seedance', result_url: undefined }),
    ], [], nothingHidden)

    expect(items).toHaveLength(4)
    for (const item of items) expect(item).toMatchObject({ images: [], videoUrl: '', downloadId: '' })
    expect(items[1]?.error).toBe('blocked')
  })

  it('keeps local-only records and orders everything newest first', () => {
    const older = { ...makeRecord('older'), createdAt: 1_700_000_000_000 }
    const newer = { ...makeRecord('newer', 'video'), createdAt: 1_700_000_200_000 }

    const items = buildCreationHistory([serverTask()], [older, newer], nothingHidden)

    expect(items.map(item => item.key)).toEqual(['local:newer', 'server:imgtask_1', 'local:older'])
    expect(items[0]).toMatchObject({ source: 'local', status: 'completed', videoUrl: 'https://cdn.example.com/newer.media', images: [] })
    expect(items[2]).toMatchObject({ images: ['https://cdn.example.com/older.media'], taskId: '' })
  })

  it('leaves out tasks and records already shown as live cards', () => {
    const tracked = { ...makeRecord('tracked'), taskId: 'imgtask_2' }

    const items = buildCreationHistory(
      [serverTask(), serverTask({ id: 'imgtask_2', task_id: 'imgtask_2' })],
      [makeRecord('live-record'), tracked, makeRecord('kept')],
      { taskIds: new Set(['imgtask_1', 'imgtask_2']), recordIds: new Set(['live-record']) },
    )

    expect(items.map(item => item.key)).toEqual(['local:kept'])
  })

  it('round-trips the task id of a saved record', () => {
    const storage = createStorage()

    writeCreationHistory(storage, 7, [{ ...makeRecord('with-task'), taskId: 'imgtask_9' }, makeRecord('without-task')])
    const [withTask, withoutTask] = readCreationHistory(storage, 7, 'image')

    expect(withTask?.taskId).toBe('imgtask_9')
    expect(withoutTask).not.toHaveProperty('taskId')
  })
})

describe('creation cost estimate', () => {
  const imageTiers = { unit: 'image' as const, tiers: [{ label: '1K', price: 0.042 }, { label: '2K', price: 0.063 }, { label: '4K', price: 0.167 }] }
  const videoTiers = { unit: 'second' as const, tiers: [{ label: '480p', price: 0.05 }, { label: '720p', price: 0.08 }, { label: '768p', price: 0.09 }, { label: '1080p', price: 0.12 }] }

  it('classifies image sizes into billing tiers by the longest edge', () => {
    expect(creationImageTier('1024x1024')).toBe('1K')
    expect(creationImageTier('1536x1024')).toBe('2K')
    expect(creationImageTier('1024x1536')).toBe('2K')
    expect(creationImageTier('3840x2160')).toBe('4K')
    expect(creationImageTier('2K')).toBe('2K')
    expect(creationImageTier(' 4k ')).toBe('4K')
    for (const size of ['auto', '', '0x0', '1024', 'wide']) expect(creationImageTier(size)).toBeNull()
  })

  it('multiplies the tier price by the image count', () => {
    const estimate = estimateCreationCost(imageTiers, { kind: 'image', size: '1536x1024', count: 2 })

    expect(estimate).toMatchObject({ unit: 'image', unitPrice: 0.063, quantity: 2, tier: '2K', assumed: false })
    expect(estimate?.amount).toBeCloseTo(0.126)
    expect(estimateCreationCost(imageTiers, { kind: 'image', size: '4K', count: 1 })?.amount).toBeCloseTo(0.167)
  })

  it('falls back to the 2K tier for an automatic size and marks the estimate as approximate', () => {
    expect(estimateCreationCost(imageTiers, { kind: 'image', size: 'auto', count: 1 }))
      .toMatchObject({ unitPrice: 0.063, tier: '2K', assumed: true })
  })

  it('uses a flat image price whatever the size is', () => {
    const estimate = estimateCreationCost({ unit: 'image', price: 0.04, tiers: [] }, { kind: 'image', size: 'auto', count: 3 })

    expect(estimate).toMatchObject({ unitPrice: 0.04, quantity: 3, tier: '', assumed: false })
    expect(estimate?.amount).toBeCloseTo(0.12)
  })

  it('normalises the image count to a positive integer', () => {
    expect(estimateCreationCost(imageTiers, { kind: 'image', size: '1K', count: 2.9 })?.quantity).toBe(2)
    expect(estimateCreationCost(imageTiers, { kind: 'image', size: '1K', count: 0 })?.quantity).toBe(1)
    expect(estimateCreationCost(imageTiers, { kind: 'image', size: '1K', count: Number.NaN })?.quantity).toBe(1)
  })

  it('multiplies the resolution price by the video duration', () => {
    const estimate = estimateCreationCost(videoTiers, { kind: 'video', protocol: 'seedance', resolution: '1080p', duration: 8 })

    expect(estimate).toMatchObject({ unit: 'second', unitPrice: 0.12, quantity: 8, tier: '1080p', assumed: false })
    expect(estimate?.amount).toBeCloseTo(0.96)
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'minimax_v2', resolution: '768P', duration: 6 }))
      .toMatchObject({ unitPrice: 0.09, tier: '768p', assumed: false })
  })

  it('uses each protocol’s billing defaults for an unspecified resolution or duration', () => {
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'seedance', resolution: '' }))
      .toMatchObject({ unitPrice: 0.08, quantity: 5, tier: '720p', assumed: true })
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'gemini', resolution: '' }))
      .toMatchObject({ unitPrice: 0.08, quantity: 8, tier: '720p', assumed: true })
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'minimax_v1', resolution: '' }))
      .toMatchObject({ unitPrice: 0.09, quantity: 6, tier: '768p', assumed: true })
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'grok', resolution: '', duration: 10 }))
      .toMatchObject({ unitPrice: 0.05, quantity: 10, tier: '480p', assumed: true })
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'seedance', resolution: '720p', duration: -1 }))
      .toMatchObject({ quantity: 5, assumed: true })
  })

  it('charges a fixed amount for per-request pricing', () => {
    const perRequest = { unit: 'request' as const, price: 0.6, tiers: [] }

    expect(estimateCreationCost(perRequest, { kind: 'video', protocol: 'minimax_v2', resolution: '', duration: 10 }))
      .toEqual({ amount: 0.6, unit: 'request', unitPrice: 0.6, quantity: 1, tier: '', assumed: false })
    expect(estimateCreationCost({ unit: 'request', tiers: [] }, { kind: 'video', protocol: 'grok', resolution: '' })).toBeNull()
  })

  it('returns no estimate when the price is missing, invalid, or for another kind', () => {
    expect(estimateCreationCost(null, { kind: 'image', size: '1K', count: 1 })).toBeNull()
    expect(estimateCreationCost({ unit: 'image', tiers: [{ label: '1K', price: 0.04 }] }, { kind: 'image', size: '2K', count: 1 })).toBeNull()
    expect(estimateCreationCost({ unit: 'image', tiers: [] }, { kind: 'image', size: '1K', count: 1 })).toBeNull()
    expect(estimateCreationCost({ unit: 'image', price: -1, tiers: [] }, { kind: 'image', size: '1K', count: 1 })).toBeNull()
    expect(estimateCreationCost(videoTiers, { kind: 'image', size: '1K', count: 1 })).toBeNull()
    expect(estimateCreationCost(imageTiers, { kind: 'video', protocol: 'seedance', resolution: '720p', duration: 5 })).toBeNull()
    expect(estimateCreationCost(videoTiers, { kind: 'video', protocol: 'seedance', resolution: '2k', duration: 5 })).toBeNull()
  })

  it('treats a zero price as free rather than missing', () => {
    expect(estimateCreationCost({ unit: 'image', price: 0, tiers: [] }, { kind: 'image', size: '1K', count: 4 })?.amount).toBe(0)
  })
})

describe('video protocol follows the model', () => {
  const group = makeGroup(1, null, videoCapability(['minimax_v1', 'minimax_v2', 'seedance'], [
    { id: 'doubao-seedance-1-0-pro', protocols: ['seedance'], pricing: null },
    { id: 'MiniMax-H3', protocols: ['minimax_v1', 'minimax_v2'], pricing: null },
    { id: 'MiniMax-Hailuo-02', protocols: ['minimax_v1', 'minimax_v2'], pricing: null },
    { id: 'studio-alias', protocols: [], pricing: null },
    { id: 'veo-3.1', protocols: ['gemini'], pricing: null },
    { id: 'doubao-seedance-1-0-pro', protocols: ['seedance'], pricing: null },
  ]))

  it('lists every video model of the group once, whatever protocol it uses', () => {
    expect(creationVideoModels(group).map(model => model.id)).toEqual([
      'doubao-seedance-1-0-pro', 'MiniMax-H3', 'MiniMax-Hailuo-02', 'studio-alias',
    ])
    expect(creationVideoModels(null)).toEqual([])
    expect(creationVideoModels(makeGroup(2, imageCapability()))).toEqual([])
  })

  it('leaves out models whose protocols the group does not support', () => {
    expect(creationVideoModels(group).some(model => model.id === 'veo-3.1')).toBe(false)
    expect(resolveCreationVideoProtocol(group, 'veo-3.1')).toBeNull()
  })

  it('uses the only protocol a model declares', () => {
    expect(resolveCreationVideoProtocol(group, 'doubao-seedance-1-0-pro')).toBe('seedance')
  })

  it('sends H3 models to the v2 endpoint and other MiniMax models to v1', () => {
    expect(resolveCreationVideoProtocol(group, 'MiniMax-H3')).toBe('minimax_v2')
    expect(resolveCreationVideoProtocol(group, 'MiniMax-Hailuo-02')).toBe('minimax_v1')
    const v1Only = makeGroup(3, null, videoCapability(['minimax_v1'], [{ id: 'MiniMax-Hailuo-02', protocols: [], pricing: null }]))
    expect(resolveCreationVideoProtocol(v1Only, 'MiniMax-Hailuo-02')).toBe('minimax_v1')
  })

  it('matches a model without declared protocols by its name, then falls back to a fixed order', () => {
    const mixed = makeGroup(4, null, videoCapability(['gemini', 'grok', 'minimax_v2', 'seedance'], [
      { id: 'veo-3.1-generate-preview', protocols: [], pricing: null },
      { id: 'xai/grok-imagine-video', protocols: [], pricing: null },
      { id: 'my-seedance-alias', protocols: [], pricing: null },
      { id: 'custom-video', protocols: [], pricing: null },
    ]))

    expect(resolveCreationVideoProtocol(mixed, 'veo-3.1-generate-preview')).toBe('gemini')
    expect(resolveCreationVideoProtocol(mixed, 'xai/grok-imagine-video')).toBe('grok')
    expect(resolveCreationVideoProtocol(mixed, 'my-seedance-alias')).toBe('seedance')
    expect(resolveCreationVideoProtocol(mixed, 'custom-video')).toBe('seedance')
    expect(resolveCreationVideoProtocol(group, 'studio-alias')).toBe('seedance')
  })

  it('returns no protocol for an unknown model or a group without video', () => {
    expect(resolveCreationVideoProtocol(group, 'missing')).toBeNull()
    expect(resolveCreationVideoProtocol(null, 'MiniMax-H3')).toBeNull()
    expect(resolveCreationVideoProtocol(makeGroup(5, imageCapability()), 'MiniMax-H3')).toBeNull()
  })
})
