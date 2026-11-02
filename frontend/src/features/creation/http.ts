import { buildGatewayUrl } from '@/api/client'

export type JsonObject = Record<string, unknown>

export class CreationGatewayError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId: string
  readonly retryAfterMs?: number

  constructor(message: string, status = 0, code = 'PROTOCOL_ERROR', requestId = '', retryAfterMs?: number) {
    super(message)
    this.name = 'CreationGatewayError'
    this.status = status
    this.code = code
    this.requestId = requestId
    this.retryAfterMs = retryAfterMs
  }
}

export function record(value: unknown): JsonObject {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as JsonObject : {}
}

export function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

export function errorMessage(value: unknown, fallback = 'The gateway could not complete this request.'): string {
  if (typeof value === 'string' && value.trim()) return value.trim()
  const body = record(value)
  const nested = record(body.error)
  return text(nested.message) || text(body.error) || text(body.message) || text(body.status_msg) || text(body.detail) || fallback
}

export function retryAfter(headers: Headers): number | undefined {
  const value = headers.get('Retry-After')?.trim()
  if (!value) return undefined
  const seconds = Number(value)
  const milliseconds = Number.isFinite(seconds) ? seconds * 1000 : Date.parse(value) - Date.now()
  return Number.isFinite(milliseconds) ? Math.max(0, milliseconds) : undefined
}

export function abortError(): CreationGatewayError {
  return new CreationGatewayError('Waiting was cancelled. The task may still be running; check Async Tasks before submitting again.', 0, 'ABORTED')
}

export function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) throw abortError()
}

export function normalizeError(error: unknown, signal?: AbortSignal): CreationGatewayError {
  if (error instanceof CreationGatewayError) return error
  if (signal?.aborted || record(error).name === 'AbortError') return abortError()
  return new CreationGatewayError(errorMessage(error, 'Could not reach the gateway. Check your connection. The task may already have been accepted; check Async Tasks before submitting again.'), 0, 'NETWORK_ERROR')
}

export function gatewayUrl(path: string): string {
  const base = buildGatewayUrl('/')
  const origin = new URL(base, typeof window === 'undefined' ? 'http://localhost' : window.location.origin).origin
  let url: URL
  try {
    url = new URL(path, `${origin}/`)
  } catch {
    throw new CreationGatewayError('The gateway returned an invalid task or download address.', 0, 'UNSAFE_GATEWAY_URL')
  }
  if (!['http:', 'https:'].includes(url.protocol) || url.origin !== origin || url.username || url.password) {
    throw new CreationGatewayError('The gateway returned a task or download address outside this site.', 0, 'UNSAFE_GATEWAY_URL')
  }
  return buildGatewayUrl(`${url.pathname}${url.search}`)
}

export async function gatewayFetch(apiKey: string, path: string, init: RequestInit = {}, google = false): Promise<Response> {
  throwIfAborted(init.signal ?? undefined)
  const headers = new Headers(init.headers)
  if (google) headers.set('x-goog-api-key', apiKey)
  else headers.set('Authorization', `Bearer ${apiKey}`)
  let response: Response
  try {
    response = await fetch(gatewayUrl(path), { ...init, headers, credentials: 'omit', redirect: 'error' })
  } catch (error) {
    throw normalizeError(error, init.signal ?? undefined)
  }
  if (!response.ok) {
    let body: unknown
    try { body = await response.json() } catch { body = {} }
    const details = record(body)
    const nested = record(details.error)
    throw new CreationGatewayError(
      errorMessage(body, response.statusText || `Gateway request failed (HTTP ${response.status}).`),
      response.status,
      String(nested.code || nested.status || nested.type || details.code || 'HTTP_ERROR'),
      response.headers.get('X-Request-Id') || response.headers.get('Request-Id') || '',
      retryAfter(response.headers),
    )
  }
  return response
}

export interface GatewayJSONResult {
  body: JsonObject
  response: Response
  retryAfterMs?: number
}

export async function requestJSON(apiKey: string, path: string, init: RequestInit = {}, google = false): Promise<GatewayJSONResult> {
  const response = await gatewayFetch(apiKey, path, init, google)
  let body: unknown
  try {
    body = await response.json()
  } catch (error) {
    throwIfAborted(init.signal ?? undefined)
    throw new CreationGatewayError(
      error instanceof TypeError ? 'Could not read the gateway response. The task may still be running; check Async Tasks.' : 'The gateway returned an invalid JSON response.',
      response.status,
      error instanceof TypeError ? 'NETWORK_ERROR' : 'INVALID_RESPONSE',
      response.headers.get('X-Request-Id') || response.headers.get('Request-Id') || '',
      retryAfter(response.headers),
    )
  }
  if (!body || typeof body !== 'object' || Array.isArray(body)) {
    throw new CreationGatewayError('The gateway returned an invalid response.', response.status, 'INVALID_RESPONSE', response.headers.get('X-Request-Id') || response.headers.get('Request-Id') || '')
  }
  return { body: body as JsonObject, response, retryAfterMs: retryAfter(response.headers) }
}

export function jsonRequest(body: JsonObject, signal?: AbortSignal): RequestInit {
  return { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal }
}

export function responseError(body: JsonObject, response: Response, fallback: string): CreationGatewayError {
  const nested = record(body.error)
  return new CreationGatewayError(errorMessage(body, fallback), response.status, String(nested.code || nested.status || nested.type || 'PROTOCOL_ERROR'), response.headers.get('X-Request-Id') || response.headers.get('Request-Id') || '', retryAfter(response.headers))
}
