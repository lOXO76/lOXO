import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CreationGatewayError, pollTask, submitImage, waitForTask } from '../gateway'
import { retryAfter } from '../http'
import { resolveAssets, revokeAssets } from '../protect'
import type { CreationTaskHandle } from '../types'

vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `http://localhost:3000${path}` }))

const fetchMock = vi.fn()
const createObjectURL = vi.fn(() => 'blob:creation-result')
const revokeObjectURL = vi.fn()
const handle: CreationTaskHandle = { kind: 'image', protocol: 'openai', taskId: 'imgtask_1', model: 'gpt-image-2' }
const input = { apiKey: 'user-key', protocol: 'openai' as const, platform: 'openai', model: 'gpt-image-2', prompt: 'A lighthouse' }

function json(body: unknown, status = 200, headers: HeadersInit = {}) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })
}

describe('creation gateway', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = createObjectURL
      static revokeObjectURL = revokeObjectURL
    })
  })

  afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

  it('submits with the user API key and parses both base64 and URL output', async () => {
    fetchMock.mockResolvedValueOnce(json({ data: [{ b64_json: '/9j/AA==' }, { url: 'https://cdn.example/image.png' }] }))
    const result = await submitImage(input)
    expect(result.handle).toBeNull()
    expect(result.snapshot.status).toBe('succeeded')
    expect(result.snapshot.assets[0]).toMatchObject({ url: 'data:image/jpeg;base64,/9j/AA==', mimeType: 'image/jpeg' })
    expect(result.snapshot.assets[1]?.url).toBe('https://cdn.example/image.png')
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toBe('http://localhost:3000/v1/images/generations')
    expect(init.headers.get('Authorization')).toBe('Bearer user-key')
    expect(JSON.parse(init.body)).toEqual({ model: input.model, prompt: input.prompt, n: 1 })
    expect(init.credentials).toBe('omit')
  })

  it('submits multipart image edits without manually setting the boundary', async () => {
    fetchMock.mockResolvedValueOnce(json({ data: [{ b64_json: 'iVBORw0KGgo=' }] }))
    const file = new File(['image'], 'reference.png', { type: 'image/png' })
    await submitImage({ ...input, referenceImages: [{ file }], size: '1024x1024', quality: 'high' })
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toMatch(/\/v1\/images\/edits$/)
    expect(init.body).toBeInstanceOf(FormData)
    expect(init.body.get('image[]').name).toBe(file.name)
    expect(init.body.get('quality')).toBe('high')
    expect(init.headers.has('Content-Type')).toBe(false)
  })

  it('honors the returned async poll URL and Retry-After', async () => {
    fetchMock.mockResolvedValueOnce(json({ id: 'imgtask_1', poll_url: '/v1/images/tasks/imgtask_1?poll=1' }, 202, { 'Retry-After': '7' }))
    fetchMock.mockResolvedValueOnce(json({ status: 'completed', result: { data: [{ url: '/v1/image-tasks/owned/content' }] } }))
    fetchMock.mockResolvedValueOnce(new Response('image', { headers: { 'Content-Type': 'image/webp' } }))
    const result = await submitImage({ ...input, preferAsync: true })
    expect(result.snapshot.retryAfterMs).toBe(7000)
    const snapshot = await pollTask(input.apiKey, result.handle!)
    expect(fetchMock.mock.calls[1]?.[0]).toMatch(/\/v1\/images\/tasks\/imgtask_1\?poll=1$/)
    expect(fetchMock.mock.calls[2]?.[1].headers.get('Authorization')).toBe('Bearer user-key')
    expect(snapshot.assets[0]).toMatchObject({ url: 'blob:creation-result', objectUrl: true, mimeType: 'image/webp' })
    revokeAssets(snapshot.assets)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:creation-result')
  })

  it('never forwards API keys to an external poll URL or redirect', async () => {
    fetchMock.mockResolvedValueOnce(json({ id: 'imgtask_1', poll_url: 'https://other.example/poll' }, 202))
    const result = await submitImage({ ...input, preferAsync: true })
    await expect(pollTask(input.apiKey, result.handle!)).rejects.toMatchObject({ code: 'UNSAFE_GATEWAY_URL' })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]?.[1].redirect).toBe('error')
  })

  it('normalizes HTTP errors including code, request ID and readable message', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: { code: 'quota_exceeded', message: 'Not enough balance.' } }, 402, { 'X-Request-Id': 'req-test' }))
    await expect(submitImage(input)).rejects.toMatchObject({ message: 'Not enough balance.', status: 402, code: 'quota_exceeded', requestId: 'req-test' })
    fetchMock.mockResolvedValueOnce(new Response('<html>failed</html>', { status: 502, statusText: 'Bad Gateway' }))
    await expect(submitImage(input)).rejects.toMatchObject({ message: 'Bad Gateway', status: 502 })
    fetchMock.mockRejectedValueOnce(new TypeError('Connection lost'))
    await expect(submitImage(input)).rejects.toBeInstanceOf(CreationGatewayError)
  })

  it('reports task failure snapshots and rejects malformed JSON/status', async () => {
    fetchMock.mockResolvedValueOnce(json({ status: 'failed', error: { message: 'Content blocked.' } }))
    expect(await pollTask(input.apiKey, handle)).toMatchObject({ status: 'failed', error: 'Content blocked.' })
    fetchMock.mockResolvedValueOnce(new Response('not json'))
    await expect(pollTask(input.apiKey, handle)).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
    fetchMock.mockResolvedValueOnce(json({ status: 'unknown' }))
    await expect(pollTask(input.apiKey, handle)).rejects.toMatchObject({ code: 'INVALID_TASK_STATUS' })
  })

  it('cleans up earlier object URLs if a later protected download fails', async () => {
    fetchMock.mockResolvedValueOnce(new Response('image', { headers: { 'Content-Type': 'image/png' } }))
    fetchMock.mockResolvedValueOnce(json({ error: { message: 'Not found' } }, 404))
    await expect(resolveAssets('user-key', 'image', ['/one/content', '/two/content'])).rejects.toMatchObject({ status: 404 })
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:creation-result')
  })

  it('polls using Retry-After, emits every snapshot and tolerates transient failures', async () => {
    vi.useFakeTimers()
    fetchMock.mockRejectedValueOnce(new TypeError('temporary'))
    fetchMock.mockResolvedValueOnce(json({ status: 'processing' }, 200, { 'Retry-After': '2' }))
    fetchMock.mockResolvedValueOnce(json({ status: 'completed', result: { data: [{ b64_json: 'iVBORw0KGgo=' }] } }))
    const onUpdate = vi.fn()
    const completion = waitForTask('user-key', handle, { intervalMs: 1000, onUpdate })
    await vi.advanceTimersByTimeAsync(999)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(2000)
    expect((await completion).status).toBe('succeeded')
    expect(onUpdate.mock.calls.map(([snapshot]) => snapshot.status)).toEqual(['processing', 'succeeded'])
  })

  it('stops after three tolerated consecutive errors and does not retry permission failures', async () => {
    vi.useFakeTimers()
    fetchMock.mockRejectedValue(new TypeError('offline'))
    const completion = waitForTask('user-key', handle, { intervalMs: 100 })
    const assertion = expect(completion).rejects.toMatchObject({ code: 'NETWORK_ERROR' })
    await vi.advanceTimersByTimeAsync(300)
    await assertion
    expect(fetchMock).toHaveBeenCalledTimes(4)
    fetchMock.mockReset().mockResolvedValue(json({ error: { message: 'Permission denied' } }, 403))
    await expect(waitForTask('user-key', handle)).rejects.toMatchObject({ status: 403 })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('cancels waiting without cancelling the upstream task', async () => {
    vi.useFakeTimers()
    fetchMock.mockResolvedValue(json({ status: 'processing' }))
    const controller = new AbortController()
    const completion = waitForTask('user-key', handle, { signal: controller.signal })
    const assertion = expect(completion).rejects.toMatchObject({ code: 'ABORTED', message: expect.stringContaining('may still be running') })
    await vi.advanceTimersByTimeAsync(0)
    controller.abort()
    await assertion
    expect(fetchMock.mock.calls.every(([, init]) => !init.method || init.method === 'GET')).toBe(true)
  })

  it('enforces timeout even while a network request is pending', async () => {
    vi.useFakeTimers()
    fetchMock.mockImplementation((_url, init) => new Promise((_resolve, reject) => {
      init.signal.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')))
    }))
    const completion = waitForTask('user-key', handle, { timeoutMs: 500 })
    const assertion = expect(completion).rejects.toMatchObject({ code: 'TIMEOUT', message: expect.stringContaining('may still be running') })
    await vi.advanceTimersByTimeAsync(500)
    await assertion
  })

  it('parses numeric and HTTP-date Retry-After headers', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-31T00:00:00Z'))
    expect(retryAfter(new Headers({ 'Retry-After': '3' }))).toBe(3000)
    expect(retryAfter(new Headers({ 'Retry-After': 'Sat, 31 Oct 2026 00:00:05 GMT' }))).toBe(5000)
    expect(retryAfter(new Headers({ 'Retry-After': 'nonsense' }))).toBeUndefined()
  })

  it('returns readable terminal task failures and progress without retrying generation', async () => {
    fetchMock.mockResolvedValueOnce(json({ status: 'running', progress: 42 }))
    expect(await pollTask('user-key', handle)).toMatchObject({ status: 'processing', progress: 42 })
    fetchMock.mockResolvedValueOnce(json({ status: 'failed', error: { message: 'Insufficient balance.' } }))
    const onUpdate = vi.fn()
    expect(await waitForTask('user-key', handle, { onUpdate })).toMatchObject({ status: 'failed', assets: [], error: 'Insufficient balance.' })
    expect(onUpdate).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls.every(([, init]) => !init.method || init.method === 'GET')).toBe(true)
  })

  it('keeps HTTP-200 protocol errors readable and rejects invalid task statuses', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: { code: 'TASK_MISSING', message: 'The image task expired.' } }, 200, { 'Request-Id': 'req-protocol' }))
    await expect(pollTask('user-key', handle)).rejects.toMatchObject({ message: 'The image task expired.', code: 'TASK_MISSING', requestId: 'req-protocol' })
    fetchMock.mockResolvedValueOnce(json({ status: 'unexpected' }))
    await expect(pollTask('user-key', handle)).rejects.toMatchObject({ code: 'INVALID_TASK_STATUS' })
  })

  it('does not reinterpret invalid JSON as a successful generation', async () => {
    fetchMock.mockResolvedValueOnce(new Response('not JSON', { headers: { 'X-Request-Id': 'req-json' } }))
    await expect(submitImage(input)).rejects.toMatchObject({ code: 'INVALID_RESPONSE', requestId: 'req-json' })
  })

  it('retries when a polling response body is interrupted after headers arrive', async () => {
    vi.useFakeTimers()
    const interrupted = json({ status: 'processing' })
    vi.spyOn(interrupted, 'json').mockRejectedValueOnce(new TypeError('Connection interrupted'))
    fetchMock.mockResolvedValueOnce(interrupted)
      .mockResolvedValueOnce(json({ status: 'completed', result: { data: [{ b64_json: 'iVBORw0KGgo=' }] } }))
    const completion = waitForTask('user-key', handle, { intervalMs: 100 })
    await vi.advanceTimersByTimeAsync(100)
    expect((await completion).status).toBe('succeeded')
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('resets transient error tolerance after each successful poll', async () => {
    vi.useFakeTimers()
    fetchMock.mockRejectedValueOnce(new TypeError('temporary'))
      .mockResolvedValueOnce(json({ status: 'processing' }))
      .mockRejectedValueOnce(new TypeError('temporary'))
      .mockRejectedValueOnce(new TypeError('temporary'))
      .mockRejectedValueOnce(new TypeError('temporary'))
      .mockResolvedValueOnce(json({ status: 'completed', result: { data: [{ b64_json: 'iVBORw0KGgo=' }] } }))
    const completion = waitForTask('user-key', handle, { intervalMs: 100 })
    await vi.advanceTimersByTimeAsync(3400)
    expect((await completion).status).toBe('succeeded')
    expect(fetchMock).toHaveBeenCalledTimes(6)
  })

  it('uses throttling Retry-After and stops already-aborted or invalid waits', async () => {
    vi.useFakeTimers()
    fetchMock.mockResolvedValueOnce(json({ error: { message: 'Too many requests' } }, 429, { 'Retry-After': '2' }))
      .mockResolvedValueOnce(json({ status: 'cancelled', error: 'Cancelled upstream.' }))
    const completion = waitForTask('user-key', handle, { intervalMs: 100 })
    await vi.advanceTimersByTimeAsync(1999)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(await completion).toMatchObject({ status: 'cancelled', error: 'Cancelled upstream.' })
    const controller = new AbortController()
    controller.abort()
    await expect(waitForTask('user-key', handle, { signal: controller.signal })).rejects.toMatchObject({ code: 'ABORTED' })
    await expect(waitForTask('user-key', handle, { timeoutMs: 0 })).rejects.toMatchObject({ code: 'INVALID_INPUT' })
    await expect(pollTask('', handle)).rejects.toMatchObject({ code: 'INVALID_INPUT' })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
