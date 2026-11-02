import { buildGatewayUrl } from '@/api/client'
import { CreationGatewayError, gatewayFetch, normalizeError, throwIfAborted } from './http'
import type { CreationAsset, CreationKind } from './types'

export function sniffImageMime(base64: string): string {
  const data = base64.replace(/^data:[^,]*,/, '').replace(/\s/g, '')
  if (data.startsWith('/9j/')) return 'image/jpeg'
  if (data.startsWith('UklGR')) return 'image/webp'
  if (data.startsWith('R0lGOD')) return 'image/gif'
  return 'image/png'
}

export function inlineImage(base64: string, mimeType?: string): CreationAsset {
  const data = base64.replace(/\s/g, '')
  if (!data || !/^[A-Za-z0-9+/]+={0,2}$/.test(data)) {
    throw new CreationGatewayError('The gateway returned invalid image data.', 0, 'INVALID_IMAGE')
  }
  const mime = mimeType?.startsWith('image/') ? mimeType : sniffImageMime(data)
  return { kind: 'image', url: `data:${mime};base64,${data}`, mimeType: mime }
}

export async function resolveAsset(apiKey: string, kind: CreationKind, sourceUrl: string, signal?: AbortSignal, mimeType?: string): Promise<CreationAsset> {
  throwIfAborted(signal)
  const fallbackMime = kind === 'image' ? 'image/png' : 'video/mp4'
  if (/^data:image\/(?:png|jpeg|webp|gif);base64,/i.test(sourceUrl) && kind === 'image') {
    const separator = sourceUrl.indexOf(',')
    return inlineImage(sourceUrl.slice(separator + 1), sourceUrl.slice(5, sourceUrl.indexOf(';')))
  }
  const base = new URL(buildGatewayUrl('/'), typeof window === 'undefined' ? 'http://localhost' : window.location.origin)
  let url: URL
  try { url = new URL(sourceUrl, base) } catch { throw new CreationGatewayError('The gateway returned an invalid media address.', 0, 'INVALID_ASSET_URL') }
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || !sourceUrl.trim()) {
    throw new CreationGatewayError('The gateway returned an unsupported media address.', 0, 'INVALID_ASSET_URL')
  }
  if (url.origin !== base.origin) {
    if (!/^https?:\/\//i.test(sourceUrl)) throw new CreationGatewayError('The gateway returned an invalid media address.', 0, 'INVALID_ASSET_URL')
    return { kind, url: url.href, sourceUrl, ...(mimeType ? { mimeType } : {}) }
  }
  const response = await gatewayFetch(apiKey, `${url.pathname}${url.search}`, { signal })
  let blob: Blob
  try { blob = await response.blob() } catch (error) { throw normalizeError(error, signal) }
  throwIfAborted(signal)
  const contentType = (response.headers.get('Content-Type') || blob.type).split(';')[0]?.trim()
  if (contentType && !contentType.startsWith(`${kind}/`) && contentType !== 'application/octet-stream') {
    throw new CreationGatewayError('The gateway download did not contain the expected media.', response.status, 'INVALID_MEDIA_TYPE')
  }
  const mime = contentType && contentType !== 'application/octet-stream' ? contentType : mimeType || fallbackMime
  if (blob.type !== mime) blob = new Blob([blob], { type: mime })
  return { kind, url: URL.createObjectURL(blob), objectUrl: true, mimeType: mime, sourceUrl }
}

export function revokeAssets(assets: CreationAsset[]): void {
  for (const asset of assets) {
    if (asset.objectUrl) URL.revokeObjectURL(asset.url)
  }
}

export async function resolveAssets(apiKey: string, kind: CreationKind, urls: string[], signal?: AbortSignal): Promise<CreationAsset[]> {
  const assets: CreationAsset[] = []
  try {
    for (const url of urls) assets.push(await resolveAsset(apiKey, kind, url, signal))
    return assets
  } catch (error) {
    revokeAssets(assets)
    throw error
  }
}

export function validateReferenceImage(file: File): void {
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size === 0) {
    throw new CreationGatewayError('Choose a non-empty PNG, JPEG or WebP reference image.', 0, 'INVALID_REFERENCE_IMAGE')
  }
  if (file.size > 20 * 1024 * 1024) {
    throw new CreationGatewayError('Reference images must be no larger than 20 MiB each.', 0, 'INVALID_REFERENCE_IMAGE')
  }
}

export async function readReferenceImage(file: File, signal?: AbortSignal): Promise<{ mimeType: string; data: string; dataUrl: string }> {
  throwIfAborted(signal)
  validateReferenceImage(file)
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    const cleanup = () => signal?.removeEventListener('abort', abort)
    const abort = () => { reader.abort(); cleanup(); reject(normalizeError(new DOMException('Aborted', 'AbortError'), signal)) }
    reader.onerror = () => { cleanup(); reject(new CreationGatewayError('Could not read the reference image.', 0, 'INVALID_REFERENCE_IMAGE')) }
    reader.onload = () => {
      cleanup()
      const dataUrl = String(reader.result)
      resolve({ mimeType: file.type, data: dataUrl.slice(dataUrl.indexOf(',') + 1), dataUrl })
    }
    signal?.addEventListener('abort', abort, { once: true })
    reader.readAsDataURL(file)
  })
}
