import { describe, expect, it } from 'vitest'
import {
  imageRatiosFor,
  imageSizePixels,
  imageSizePreset,
  resolveImageSize,
} from '../imageSizePresets'
import { creationImageTier } from '@/views/user/creation/viewModel'

/** openai 的比例全集（显示顺序）。 */
const OPENAI_RATIOS = ['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4', '9:16', '16:9', '1:2', '2:1', '9:21', '21:9']

/** 逐格写死的期望尺寸，和映射表一一对应。 */
const OPENAI_SIZES: Record<string, Record<string, string>> = {
  '1K': {
    '1:1': '1024x1024',
    '3:4': '768x1024',
    '4:3': '1024x768',
    '2:3': '672x1008',
    '3:2': '1008x672',
    '4:5': '816x1024',
    '5:4': '1024x816',
  },
  '2K': {
    '1:1': '2048x2048',
    '3:4': '1536x2048',
    '4:3': '2048x1536',
    '2:3': '1344x2016',
    '3:2': '2016x1344',
    '4:5': '1648x2048',
    '5:4': '2048x1648',
    '9:16': '1152x2048',
    '16:9': '2048x1152',
    '1:2': '1024x2048',
    '2:1': '2048x1024',
    '9:21': '864x2016',
    '21:9': '2016x864',
  },
  '4K': {
    '1:1': '2880x2880',
    '3:4': '2448x3264',
    '4:3': '3264x2448',
    '2:3': '2336x3504',
    '3:2': '3504x2336',
    '4:5': '2560x3200',
    '5:4': '3200x2560',
    '9:16': '2160x3840',
    '16:9': '3840x2160',
    '1:2': '1920x3840',
    '2:1': '3840x1920',
    '9:21': '1632x3808',
    '21:9': '3808x1632',
  },
}

/** gpt-image-2 的尺寸约束。 */
const MIN_PIXELS = 655_360
const MAX_PIXELS = 8_294_400
const MAX_EDGE = 3840

function parseSize(size: string): { w: number; h: number } {
  const [w, h] = size.split('x').map(Number)
  return { w: w!, h: h! }
}

function parseRatio(ratio: string): number {
  const [a, b] = ratio.split(':').map(Number)
  return a! / b!
}

describe('image size presets', () => {
  it('lists resolutions and ratios per vendor in display order', () => {
    expect(imageSizePreset('openai').resolutions).toEqual(['1K', '2K', '4K', 'auto'])
    expect(imageSizePreset('openai').ratios).toEqual(OPENAI_RATIOS)

    expect(imageSizePreset('gemini').resolutions).toEqual(['1K', '2K', '4K'])
    expect(imageSizePreset('gemini').ratios).toEqual([
      '1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4', '9:16', '16:9', '21:9', 'auto',
    ])

    expect(imageSizePreset('grok').resolutions).toEqual(['1K', '2K'])
    expect(imageSizePreset('grok').ratios).toEqual([
      '1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3', '2:1', '1:2', '19.5:9', '9:19.5', '20:9', '9:20',
    ])
  })

  it('hands out copies so callers cannot mutate the built-in table', () => {
    const preset = imageSizePreset('gemini')
    preset.resolutions.push('8K')
    preset.ratios.length = 0
    expect(imageSizePreset('gemini').resolutions).toEqual(['1K', '2K', '4K'])
    expect(imageSizePreset('gemini').ratios).toHaveLength(11)

    const ratios = imageRatiosFor('grok', '1K')
    ratios.push('42:1')
    expect(imageRatiosFor('grok', '1K')).not.toContain('42:1')
  })

  it('filters OpenAI ratios by resolution and hides them for auto', () => {
    // 1K 没有宽幅比例（见映射表注释），其余两档是全集。
    expect(imageRatiosFor('openai', '1K')).toEqual(['1:1', '3:4', '4:3', '2:3', '3:2', '4:5', '5:4'])
    expect(imageRatiosFor('openai', '2K')).toEqual(OPENAI_RATIOS)
    expect(imageRatiosFor('openai', '4K')).toEqual(OPENAI_RATIOS)
    expect(imageRatiosFor('openai', 'auto')).toEqual([])
    expect(imageRatiosFor('openai', '8K')).toEqual([])
  })

  it('keeps the whole ratio set for gemini and grok, but only for known resolutions', () => {
    expect(imageRatiosFor('gemini', '1K')).toEqual(imageSizePreset('gemini').ratios)
    expect(imageRatiosFor('gemini', '4K')).toEqual(imageSizePreset('gemini').ratios)
    expect(imageRatiosFor('gemini', 'auto')).toEqual([])
    expect(imageRatiosFor('grok', '2K')).toEqual(imageSizePreset('grok').ratios)
    expect(imageRatiosFor('grok', '4K')).toEqual([])
  })

  it('maps every OpenAI combination to the hard-coded pixel size', () => {
    for (const [resolution, cells] of Object.entries(OPENAI_SIZES)) {
      for (const [ratio, size] of Object.entries(cells)) {
        expect(resolveImageSize('openai', resolution, ratio), `${resolution} ${ratio}`).toEqual({ kind: 'size', size })
      }
    }
    // 表里一共 7 + 13 + 13 格，且尺寸互不重复。
    const all = Object.values(OPENAI_SIZES).flatMap(cells => Object.values(cells))
    expect(all).toHaveLength(33)
    expect(new Set(all).size).toBe(33)
  })

  it('keeps every OpenAI size inside the gpt-image-2 constraints and its billing tier', () => {
    for (const [resolution, cells] of Object.entries(OPENAI_SIZES)) {
      for (const [ratio, expected] of Object.entries(cells)) {
        const { w, h } = parseSize(expected)
        const label = `${resolution} ${ratio} → ${expected}`

        // 宽高都是 16 的倍数
        expect(w % 16, label).toBe(0)
        expect(h % 16, label).toBe(0)
        // 总像素在 655,360 – 8,294,400 之间
        expect(w * h, label).toBeGreaterThanOrEqual(MIN_PIXELS)
        expect(w * h, label).toBeLessThanOrEqual(MAX_PIXELS)
        // 长边 ≤ 3840，宽高比 ≤ 3:1
        expect(Math.max(w, h), label).toBeLessThanOrEqual(MAX_EDGE)
        expect(Math.max(w / h, h / w), label).toBeLessThanOrEqual(3)
        // 比例误差 ≤ 1%
        const target = parseRatio(ratio)
        expect(Math.abs(w / h - target) / target, label).toBeLessThanOrEqual(0.01)
        // 按长边算出的计费档就是所选的分辨率档
        expect(creationImageTier(expected), label).toBe(resolution)
      }
    }
  })

  it('lets OpenAI auto ignore the ratio and rejects unsupported combinations', () => {
    expect(resolveImageSize('openai', 'auto', '1:1')).toEqual({ kind: 'size', size: 'auto' })
    expect(resolveImageSize('openai', 'auto', '9:16')).toEqual({ kind: 'size', size: 'auto' })

    // 1K 的宽幅比例凑不够最小像素（或会跳到 2K 计费档），所以不存在。
    expect(resolveImageSize('openai', '1K', '16:9')).toBeNull()
    expect(resolveImageSize('openai', '1K', '9:16')).toBeNull()
    expect(resolveImageSize('openai', '1K', '21:9')).toBeNull()
    expect(resolveImageSize('openai', '1K', '1:2')).toBeNull()

    expect(resolveImageSize('openai', '8K', '1:1')).toBeNull()
    expect(resolveImageSize('openai', '2K', '5:7')).toBeNull()
  })

  it('sends aspect ratio plus resolution tier for gemini and grok', () => {
    expect(resolveImageSize('gemini', '4K', '3:4')).toEqual({ kind: 'aspect', aspectRatio: '3:4', imageSize: '4K' })
    expect(resolveImageSize('gemini', '1K', '4:5')).toEqual({ kind: 'aspect', aspectRatio: '4:5', imageSize: '1K' })
    expect(resolveImageSize('gemini', '2K', '21:9')).toEqual({ kind: 'aspect', aspectRatio: '21:9', imageSize: '2K' })
    expect(resolveImageSize('gemini', '1K', 'auto')).toEqual({ kind: 'aspect', aspectRatio: 'auto', imageSize: '1K' })
    expect(resolveImageSize('grok', '2K', '20:9')).toEqual({ kind: 'aspect', aspectRatio: '20:9', imageSize: '2K' })

    expect(resolveImageSize('gemini', 'auto', '1:1')).toBeNull()
    expect(resolveImageSize('gemini', '2K', '9:21')).toBeNull()
    expect(resolveImageSize('grok', '4K', '1:1')).toBeNull()
    expect(resolveImageSize('grok', '1K', '4:3')).not.toBeNull()
  })

  it('formats pixel sizes for the output hint and skips non-pixel values', () => {
    expect(imageSizePixels('2048x1152')).toBe('2048×1152')
    expect(imageSizePixels(' 1024 x 1024 ')).toBe('1024×1024')
    expect(imageSizePixels('auto')).toBe('')
    expect(imageSizePixels('')).toBe('')
  })
})
