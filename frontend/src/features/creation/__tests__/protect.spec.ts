import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { gatewayUrl } from '../http'
import { readReferenceImage, resolveAsset, revokeAssets } from '../protect'

vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `http://localhost:3000${path}` }))

const fetchMock = vi.fn()
const createObjectURL = vi.fn(() => 'blob:creation-download')
const revokeObjectURL = vi.fn()

describe('creation protected assets', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = createObjectURL
      static revokeObjectURL = revokeObjectURL
    })
  })
  afterEach(() => vi.unstubAllGlobals())

  it('downloads same-gateway absolute paths with the API key and MIME from headers', async () => {
    fetchMock.mockResolvedValueOnce(new Response('video', { headers: { 'Content-Type': 'video/webm; charset=binary' } }))
    const asset = await resolveAsset('selected-group-key', 'video', 'http://localhost:3000/v1/video-tasks/owned/content?index=1')
    expect(asset).toMatchObject({ url: 'blob:creation-download', objectUrl: true, mimeType: 'video/webm' })
    expect(fetchMock.mock.calls[0]?.[1].headers.get('Authorization')).toBe('Bearer selected-group-key')
    revokeAssets([asset, { kind: 'image', url: 'https://cdn.example/image.png' }])
    expect(revokeObjectURL).toHaveBeenCalledTimes(1)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:creation-download')
  })

  it('uses the fallback MIME for binary downloads and rejects HTML error pages', async () => {
    fetchMock.mockResolvedValueOnce(new Response('image', { headers: { 'Content-Type': 'application/octet-stream' } }))
    expect(await resolveAsset('key', 'image', '/v1/image-tasks/owned/content')).toMatchObject({ mimeType: 'image/png' })
    fetchMock.mockResolvedValueOnce(new Response('<html>error</html>', { headers: { 'Content-Type': 'text/html' } }))
    await expect(resolveAsset('key', 'image', '/v1/image-tasks/owned/content')).rejects.toMatchObject({ code: 'INVALID_MEDIA_TYPE' })
    expect(createObjectURL).toHaveBeenCalledTimes(1)
  })

  it('keeps public and inline media local without sending API keys to other origins', async () => {
    expect(await resolveAsset('key', 'image', 'https://cdn.example/image.png')).toMatchObject({ url: 'https://cdn.example/image.png' })
    expect(await resolveAsset('key', 'image', 'data:image/webp;base64,UklGRg==')).toMatchObject({ url: 'data:image/webp;base64,UklGRg==', mimeType: 'image/webp' })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each(['javascript:alert(1)', '//other.example/content', 'https://user:password@example.com/image', 'data:text/html;base64,PGgxPg==', ''])('rejects unsafe asset address %s', async (url) => {
    await expect(resolveAsset('key', 'image', url)).rejects.toMatchObject({ code: 'INVALID_ASSET_URL' })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each(['http://[', 'https://other.example/task', '//other.example/task'])('rejects invalid or external polling address %s', (url) => {
    expect(() => gatewayUrl(url)).toThrow()
  })

  it('validates reference file size before reading and respects cancellation', async () => {
    const file = new File(['image'], 'input.png', { type: 'image/png' })
    Object.defineProperty(file, 'size', { value: 20 * 1024 * 1024 + 1 })
    await expect(readReferenceImage(file)).rejects.toThrow('20 MiB')
    const controller = new AbortController()
    controller.abort()
    await expect(readReferenceImage(file, controller.signal)).rejects.toMatchObject({ code: 'ABORTED' })
  })
})
