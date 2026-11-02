import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { buildGeminiImageBody, buildOpenAIImageBody, parseImageOutputs } from '../image'
import { submitImage } from '../gateway'
import { inlineImage, sniffImageMime } from '../protect'
import type { ImageGenerationInput } from '../types'

vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `http://localhost:3000${path}` }))
const fetchMock = vi.fn()
const input: ImageGenerationInput = { apiKey: 'key', protocol: 'openai', platform: 'openai', model: 'gpt-image-2', prompt: 'A cat' }

describe('creation image adapters', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.stubGlobal('fetch', fetchMock) })
  afterEach(() => vi.unstubAllGlobals())

  it('leaves response_format at the gateway default and sends Grok-native geometry', () => {
    expect(buildOpenAIImageBody(input)).toEqual({ model: 'gpt-image-2', prompt: 'A cat', n: 1 })
    expect(buildOpenAIImageBody({ ...input, platform: 'grok', aspectRatio: '16:9', imageSize: '2K', n: 2, quality: 'high', size: '1536x1024' })).toEqual({ model: 'gpt-image-2', prompt: 'A cat', n: 2, aspect_ratio: '16:9', resolution: '2k' })
    expect(buildOpenAIImageBody({ ...input, platform: 'openai', aspectRatio: '16:9', imageSize: '2K', quality: 'high', size: '1536x1024' })).toEqual({ model: 'gpt-image-2', prompt: 'A cat', n: 1, quality: 'high', size: '1536x1024' })
  })

  it('builds Gemini multimodal input using the documented field names', () => {
    expect(buildGeminiImageBody({ ...input, protocol: 'gemini', aspectRatio: '9:16', imageSize: '2K' }, [{ mimeType: 'image/png', data: 'aW1hZ2U=' }])).toEqual({
      contents: [{ role: 'user', parts: [{ text: 'A cat' }, { inlineData: { mimeType: 'image/png', data: 'aW1hZ2U=' } }] }],
      generationConfig: { responseModalities: ['TEXT', 'IMAGE'], imageConfig: { aspectRatio: '9:16', imageSize: '2K' } },
    })
  })

  it('extracts every Gemini image and translates blocked output to readable errors', () => {
    expect(parseImageOutputs({ candidates: [{ content: { parts: [{ text: 'Generated:' }, { inlineData: { mimeType: 'image/webp', data: 'UklGRg==' } }] }, finishReason: 'STOP' }] }, 'gemini')).toEqual([{ base64: 'UklGRg==', mimeType: 'image/webp' }])
    expect(() => parseImageOutputs({ promptFeedback: { blockReason: 'SAFETY' } }, 'gemini')).toThrow('SAFETY')
    expect(() => parseImageOutputs({ candidates: [{ finishReason: 'IMAGE_SAFETY' }] }, 'gemini')).toThrow('IMAGE_SAFETY')
    expect(() => parseImageOutputs({ candidates: [] }, 'gemini')).toThrow('no images')
  })

  it('uses x-goog-api-key for Gemini and reads local reference image data', async () => {
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ candidates: [{ content: { parts: [{ inlineData: { mimeType: 'image/png', data: 'iVBORw0KGgo=' } }] } }] })))
    const file = new File(['reference'], 'input.webp', { type: 'image/webp' })
    const result = await submitImage({ ...input, protocol: 'gemini', model: 'gemini-2.5-flash-image', referenceImages: [{ file }] })
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toMatch(/\/v1beta\/models\/gemini-2\.5-flash-image:generateContent$/)
    expect(init.headers.get('x-goog-api-key')).toBe('key')
    expect(init.headers.has('Authorization')).toBe(false)
    expect(JSON.parse(init.body).contents[0].parts[1]).toEqual({ inlineData: { mimeType: 'image/webp', data: 'cmVmZXJlbmNl' } })
    expect(result.snapshot.assets[0]?.mimeType).toBe('image/png')
  })

  it('sniffs PNG/JPEG/WebP and rejects non-image output data', () => {
    expect(sniffImageMime('iVBORw0KGgo=')).toBe('image/png')
    expect(sniffImageMime('/9j/AA==')).toBe('image/jpeg')
    expect(sniffImageMime('UklGRg==')).toBe('image/webp')
    expect(() => inlineImage('broken!')).toThrow('invalid image data')
  })

  it('ignores non-image inline parts instead of rendering them as PNG', () => {
    expect(() => parseImageOutputs({ candidates: [{ content: { parts: [{ inlineData: { mimeType: 'audio/wav', data: 'aW1hZ2U=' } }] } }] }, 'gemini')).toThrow('no images')
  })

  it('rejects invalid reference images and excessive Grok edits before submission', async () => {
    await expect(submitImage({ ...input, referenceImages: [{ file: new File(['text'], 'input.txt', { type: 'text/plain' }) }] })).rejects.toMatchObject({ code: 'INVALID_REFERENCE_IMAGE' })
    const reference = { file: new File(['image'], 'input.png', { type: 'image/png' }) }
    await expect(submitImage({ ...input, platform: 'grok', referenceImages: Array.from({ length: 4 }, () => reference) })).rejects.toThrow('at most three')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rejects unsupported protocols and Gemini image counts before submission', async () => {
    await expect(submitImage({ ...input, protocol: 'unknown' as ImageGenerationInput['protocol'] })).rejects.toMatchObject({ code: 'INVALID_INPUT' })
    await expect(submitImage({ ...input, protocol: 'gemini', n: 2 })).rejects.toThrow('image count to 1')
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
