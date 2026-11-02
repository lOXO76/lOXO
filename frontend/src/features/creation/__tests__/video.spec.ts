import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { pollVideoTask, submitVideoRequest } from '../video'
import { CreationGatewayError } from '../http'
import { revokeAssets } from '../protect'
import type { CreationImageRole, CreationTaskHandle, CreationVideoProtocol, VideoGenerationInput } from '../types'

vi.mock('@/api/client', () => ({ buildGatewayUrl: (path: string) => `http://localhost:3000${path}` }))

const fetchMock = vi.fn()
const createObjectURL = vi.fn(() => 'blob:creation-video')
const revokeObjectURL = vi.fn()
const input: VideoGenerationInput = { apiKey: 'user-video-key', protocol: 'seedance', model: 'public-video', prompt: 'Waves at sunset' }
const operation = 'models/veo-3.1-generate-preview/operations/operation-1'
const imageData = 'cmVmZXJlbmNl'
const imageURL = `data:image/png;base64,${imageData}`
const resultURL = 'https://cdn.example/video.mp4'
const protectedURL = '/v1/video-tasks/returned-resource/content?index=2'

function json(body: unknown, status = 200, headers: Record<string, string> = {}): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })
}

function reference(role?: CreationImageRole) {
  return { file: new File(['reference'], 'reference.png', { type: 'image/png' }), ...(role ? { role } : {}) }
}

function handle(protocol: CreationVideoProtocol, taskId = 'task-1'): CreationTaskHandle {
  return { kind: 'video', protocol, taskId: protocol === 'gemini' && taskId === 'task-1' ? operation : taskId, model: input.model }
}

function submittedBody(): Record<string, unknown> {
  return JSON.parse(fetchMock.mock.calls[0]![1].body)
}

describe('creation video submission', () => {
  beforeEach(() => { fetchMock.mockReset(); vi.stubGlobal('fetch', fetchMock) })
  afterEach(() => vi.unstubAllGlobals())

  it.each([
    ['seedance', '/api/v3/contents/generations/tasks', { id: 'task-1' }],
    ['gemini', '/v1beta/models/public-video:predictLongRunning', { name: operation }],
    ['minimax_v1', '/v1/video_generation', { task_id: 'task-1', base_resp: { status_code: 0 } }],
    ['minimax_v2', '/v2/video_generation', { task_id: 'task-1' }],
    ['grok', '/v1/videos', { request_id: 'task-1' }],
  ] as const)('uses the %s endpoint and only the selected API key', async (protocol, path, response) => {
    const signal = new AbortController().signal
    fetchMock.mockResolvedValueOnce(json(response, 202, { 'Retry-After': '4' }))
    const result = await submitVideoRequest({ ...input, protocol, signal })
    const [url, init] = fetchMock.mock.calls[0]!
    expect(url).toBe(`http://localhost:3000${path}`)
    expect(init).toMatchObject({ method: 'POST', credentials: 'omit', redirect: 'error', signal })
    expect(init.headers.get('Authorization')).toBe('Bearer user-video-key')
    expect(init.headers.get('x-goog-api-key')).toBeNull()
    expect(init.headers.get('Content-Type')).toBe('application/json')
    expect(result).toEqual({ handle: handle(protocol), snapshot: { status: 'processing', assets: [], retryAfterMs: 4000 } })
  })

  it('submits Seedance text, frame roles and inline images without quantity aliases', async () => {
    fetchMock.mockResolvedValueOnce(json({ id: 'seedance-1' }))
    await submitVideoRequest({ ...input, duration: -1, resolution: '720P', ratio: '9:16', referenceImages: [reference(), reference('last_frame'), reference('reference_image')] })
    expect(submittedBody()).toEqual({
      model: input.model,
      content: [{ type: 'text', text: input.prompt }, ...(['first_frame', 'last_frame', 'reference_image'] as const).map(role => ({ type: 'image_url', image_url: { url: imageURL }, role }))],
      duration: -1, resolution: '720p', ratio: '9:16',
    })
  })

  it('leaves unspecified duration and resolution to backend capability defaults', async () => {
    fetchMock.mockResolvedValueOnce(json({ id: 'task-1' }))
    expect((await submitVideoRequest(input)).snapshot.retryAfterMs).toBe(3000)
    expect(submittedBody()).toEqual({ model: input.model, content: [{ type: 'text', text: input.prompt }] })
  })

  it('uses official Veo bytesBase64Encoded images, frame fields and URL model', async () => {
    fetchMock.mockResolvedValueOnce(json({ name: operation }))
    await submitVideoRequest({ ...input, protocol: 'gemini', model: 'models/veo-3.1-generate-preview', duration: 8, resolution: '1080P', ratio: '16:9', referenceImages: [reference(), reference('last_frame')] })
    expect(fetchMock.mock.calls[0]![0]).toBe('http://localhost:3000/v1beta/models/veo-3.1-generate-preview:predictLongRunning')
    expect(submittedBody()).toEqual({
      instances: [{ prompt: input.prompt, image: { bytesBase64Encoded: imageData, mimeType: 'image/png' }, lastFrame: { bytesBase64Encoded: imageData, mimeType: 'image/png' } }],
      parameters: { sampleCount: 1, durationSeconds: 8, resolution: '1080p', aspectRatio: '16:9' },
    })
  })

  it('sends three Veo references and lets the backend resolve aliases and duration overrides', async () => {
    fetchMock.mockResolvedValueOnce(json({ name: operation }))
    await submitVideoRequest({ ...input, protocol: 'gemini', duration: 6, referenceImages: Array.from({ length: 3 }, () => reference('reference_image')) })
    expect(submittedBody()).toEqual({
      instances: [{ prompt: input.prompt, referenceImages: Array.from({ length: 3 }, () => ({ image: { bytesBase64Encoded: imageData, mimeType: 'image/png' }, referenceType: 'asset' })) }],
      parameters: { sampleCount: 1, durationSeconds: 6 },
    })
  })

  it('uses native MiniMax v1 frame and subject fields', async () => {
    fetchMock.mockResolvedValueOnce(json({ task_id: 'task-1' }))
    await submitVideoRequest({ ...input, protocol: 'minimax_v1', duration: 10, resolution: '768p', referenceImages: [reference(), reference('last_frame'), reference('reference_image')] })
    expect(submittedBody()).toEqual({ model: input.model, prompt: input.prompt, duration: 10, resolution: '768P', first_frame_image: imageURL, last_frame_image: imageURL, subject_reference: [{ type: 'character', image: [imageURL] }] })
  })

  it('uses MiniMax v2 content and supplies valid ratios for text and frame input', async () => {
    fetchMock.mockResolvedValueOnce(json({ task_id: 'task-1' }))
    await submitVideoRequest({ ...input, protocol: 'minimax_v2', resolution: '2k' })
    expect(submittedBody()).toEqual({ model: input.model, content: [{ type: 'text', text: input.prompt }], resolution: '2K', ratio: '16:9' })
    fetchMock.mockClear().mockResolvedValueOnce(json({ task_id: 'task-2' }))
    await submitVideoRequest({ ...input, protocol: 'minimax_v2', referenceImages: [reference()] })
    expect(submittedBody()).toMatchObject({ ratio: 'adaptive', content: [{ type: 'text', text: input.prompt }, { type: 'image_url', image_url: { url: imageURL }, role: 'first_frame' }] })
  })

  it('supports nine MiniMax v2 references using their explicit role', async () => {
    fetchMock.mockResolvedValueOnce(json({ task_id: 'task-1' }))
    await submitVideoRequest({ ...input, protocol: 'minimax_v2', referenceImages: Array.from({ length: 9 }, () => reference('reference_image')) })
    expect(submittedBody().content).toHaveLength(10)
    expect(submittedBody().ratio).toBe('adaptive')
  })

  it('uses Grok image objects, last_frame, reference_images and top-level controls', async () => {
    fetchMock.mockResolvedValueOnce(json({ data: { request_id: 'grok-1' } }))
    const result = await submitVideoRequest({ ...input, protocol: 'grok', duration: 12, resolution: '480P', ratio: '1:1', referenceImages: [reference(), reference('last_frame'), reference('reference_image')] })
    expect(result.handle?.taskId).toBe('grok-1')
    expect(submittedBody()).toEqual({ model: input.model, prompt: input.prompt, duration: 12, resolution: '480p', aspect_ratio: '1:1', image: { url: imageURL, type: 'image_url' }, last_frame: { url: imageURL, type: 'image_url' }, reference_images: [{ url: imageURL, type: 'image_url' }] })
  })

  it.each([['gemini', 4], ['minimax_v2', 10]] as const)('rejects excessive %s reference images before submitting', async (protocol, count) => {
    await expect(submitVideoRequest({ ...input, protocol, referenceImages: Array.from({ length: count }, () => reference('reference_image')) })).rejects.toMatchObject({ code: 'INVALID_REFERENCE_IMAGE' })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rejects MiniMax v2 mixed reference media and duplicated first frames', async () => {
    await expect(submitVideoRequest({ ...input, protocol: 'minimax_v2', referenceImages: [reference(), reference('reference_image')] })).rejects.toThrow('cannot be mixed')
    await expect(submitVideoRequest({ ...input, referenceImages: [reference(), reference()] })).rejects.toThrow('Only one first frame')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('enforces shared size and MIME restrictions before network I/O', async () => {
    const oversized = reference()
    Object.defineProperty(oversized.file, 'size', { value: 20 * 1024 * 1024 + 1 })
    await expect(submitVideoRequest({ ...input, referenceImages: [oversized] })).rejects.toThrow('20 MiB')
    for (const file of [new File([], 'empty.png', { type: 'image/png' }), new File(['svg'], 'image.svg', { type: 'image/svg+xml' })]) {
      await expect(submitVideoRequest({ ...input, referenceImages: [{ file }] })).rejects.toMatchObject({ code: 'INVALID_REFERENCE_IMAGE' })
    }
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('accounts for base64 expansion against the default 256 MiB request limit', async () => {
    const references = Array.from({ length: 10 }, () => reference('reference_image'))
    for (const item of references) Object.defineProperty(item.file, 'size', { value: 20 * 1024 * 1024 })
    await expect(submitVideoRequest({ ...input, referenceImages: references })).rejects.toThrow('256 MiB')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it.each([0, -2, 1.5, Number.NaN, Number.POSITIVE_INFINITY])('rejects invalid duration %s', async duration => {
    await expect(submitVideoRequest({ ...input, duration })).rejects.toMatchObject({ code: 'INVALID_INPUT' })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rejects HTTP 200 business errors and preserves request metadata', async () => {
    fetchMock.mockResolvedValueOnce(json({ base_resp: { status_code: 1008, status_msg: 'Insufficient balance' } }, 200, { 'Request-Id': 'request-1', 'Retry-After': '7' }))
    await expect(submitVideoRequest({ ...input, protocol: 'minimax_v1' })).rejects.toMatchObject({ message: 'Insufficient balance', status: 200, code: '1008', requestId: 'request-1', retryAfterMs: 7000 })
    fetchMock.mockResolvedValueOnce(json({ error: { code: 'SAFETY', message: 'Blocked prompt' } }))
    await expect(submitVideoRequest(input)).rejects.toMatchObject({ code: 'SAFETY', message: 'Blocked prompt' })
    fetchMock.mockResolvedValueOnce(json({ error: 'Plain rejection' }))
    await expect(submitVideoRequest(input)).rejects.toThrow('Plain rejection')
  })

  it('rejects missing or unsafe task IDs without guessing one', async () => {
    for (const response of [{}, { id: '../other' }, { id: 'https://other.example/task' }]) {
      fetchMock.mockResolvedValueOnce(json(response))
      await expect(submitVideoRequest(input)).rejects.toBeInstanceOf(CreationGatewayError)
    }
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })
})

describe('creation video polling and protected output', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    createObjectURL.mockClear()
    revokeObjectURL.mockClear()
    vi.stubGlobal('fetch', fetchMock)
    vi.stubGlobal('URL', class extends URL {
      static createObjectURL = createObjectURL
      static revokeObjectURL = revokeObjectURL
    })
  })
  afterEach(() => vi.unstubAllGlobals())

  it.each([
    ['seedance', '/api/v3/contents/generations/tasks/task-1', { status: 'running', progress: 37 }],
    ['gemini', `/v1beta/${operation}`, { name: operation, done: false, metadata: { progress: 37 } }],
    ['minimax_v1', '/v1/query/video_generation?task_id=task-1', { status: 'Queueing', progress: 37, base_resp: { status_code: 0 } }],
    ['minimax_v2', '/v2/query/video_generation/task-1', { task: { id: 'task-1', status: 'running', progress: 37 } }],
    ['grok', '/v1/videos/task-1', { status: 'pending', progress: 37 }],
  ] as const)('polls %s with native paths, user-key authentication and Retry-After', async (protocol, path, body) => {
    const signal = new AbortController().signal
    fetchMock.mockResolvedValueOnce(json(body, 200, { 'Retry-After': '6' }))
    expect(await pollVideoTask(input.apiKey, handle(protocol), signal)).toEqual({ status: 'processing', assets: [], progress: 37, retryAfterMs: 6000 })
    expect(fetchMock.mock.calls[0]![0]).toBe(`http://localhost:3000${path}`)
    expect(fetchMock.mock.calls[0]![1]).toMatchObject({ signal, credentials: 'omit', redirect: 'error' })
    expect(fetchMock.mock.calls[0]![1].headers.get('Authorization')).toBe('Bearer user-video-key')
  })

  it.each(['Preparing', 'Queueing', 'Processing'])('keeps MiniMax v1 %s pending without requesting files', async status => {
    fetchMock.mockResolvedValueOnce(json({ task_id: 'task-1', status, base_resp: { status_code: 0 } }))
    expect(await pollVideoTask(input.apiKey, handle('minimax_v1'))).toEqual({ status: 'processing', assets: [], retryAfterMs: 3000 })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['seedance', { status: 'succeeded', content: { video_url: resultURL } }],
    ['minimax_v2', { task: { id: 'task-1', status: 'succeeded', content: { url: resultURL } } }],
  ] as const)('reads %s output without sending credentials externally', async (protocol, body) => {
    fetchMock.mockResolvedValueOnce(json(body))
    expect(await pollVideoTask(input.apiKey, handle(protocol))).toEqual({ status: 'succeeded', assets: [{ kind: 'video', url: resultURL, sourceUrl: resultURL }] })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('retrieves the MiniMax v1 file ID before resolving its protected download', async () => {
    fetchMock.mockResolvedValueOnce(json({ task_id: 'task-1', status: 'Success', file_id: 'file-9', base_resp: { status_code: 0 } }))
    fetchMock.mockResolvedValueOnce(json({ file: { file_id: 'file-9', download_url: protectedURL }, base_resp: { status_code: 0 } }))
    fetchMock.mockResolvedValueOnce(new Response('video', { headers: { 'Content-Type': 'video/mp4' } }))
    const snapshot = await pollVideoTask(input.apiKey, handle('minimax_v1'))
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(['http://localhost:3000/v1/query/video_generation?task_id=task-1', 'http://localhost:3000/v1/files/retrieve?file_id=file-9', `http://localhost:3000${protectedURL}`])
    expect(fetchMock.mock.calls.every(([, init]) => init.headers.get('Authorization') === 'Bearer user-video-key')).toBe(true)
    expect(snapshot).toEqual({ status: 'succeeded', assets: [{ kind: 'video', url: 'blob:creation-video', objectUrl: true, sourceUrl: protectedURL, mimeType: 'video/mp4' }] })
    revokeAssets(snapshot.assets)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:creation-video')
  })

  it('reports MiniMax file errors and permits later polling to recover', async () => {
    fetchMock.mockResolvedValueOnce(json({ status: 'Success', file_id: 'file-9' }))
    fetchMock.mockResolvedValueOnce(json({ base_resp: { status_code: 1002, status_msg: 'File temporarily unavailable' } }))
    await expect(pollVideoTask(input.apiKey, handle('minimax_v1'))).rejects.toMatchObject({ code: '1002', message: 'File temporarily unavailable' })
    fetchMock.mockResolvedValueOnce(json({ status: 'Success', file_id: 'file-9' }))
    fetchMock.mockResolvedValueOnce(json({ file: { file_id: 'file-9', download_url: resultURL } }))
    expect((await pollVideoTask(input.apiKey, handle('minimax_v1'))).status).toBe('succeeded')
  })

  it('resolves all Gemini samples using exact returned resources and indexes', async () => {
    fetchMock.mockResolvedValueOnce(json({ done: true, response: { generateVideoResponse: { generatedSamples: [{ video: { uri: protectedURL } }, { video: { uri: resultURL } }] } } }))
    fetchMock.mockResolvedValueOnce(new Response('video', { headers: { 'Content-Type': 'video/webm' } }))
    const snapshot = await pollVideoTask(input.apiKey, handle('gemini'))
    expect(snapshot.assets).toEqual([{ kind: 'video', url: 'blob:creation-video', objectUrl: true, sourceUrl: protectedURL, mimeType: 'video/webm' }, { kind: 'video', url: resultURL, sourceUrl: resultURL }])
    expect(fetchMock.mock.calls[1]![0]).toBe(`http://localhost:3000${protectedURL}`)
    expect(fetchMock.mock.calls[1]![1].headers.get('Authorization')).toBe('Bearer user-video-key')
  })

  it('supports generatedVideos and omitted done on pending operations', async () => {
    fetchMock.mockResolvedValueOnce(json({ name: operation }))
    expect((await pollVideoTask(input.apiKey, handle('gemini'))).status).toBe('processing')
    fetchMock.mockResolvedValueOnce(json({ done: true, response: { generatedVideos: [{ video: { uri: resultURL } }] } }))
    expect((await pollVideoTask(input.apiKey, handle('gemini'))).assets[0]?.url).toBe(resultURL)
  })

  it('normalizes Gemini operation paths and blocks external poll URLs', async () => {
    for (const taskId of [operation, `/v1beta/${operation}`, `http://localhost:3000/v1beta/${operation}`]) {
      fetchMock.mockResolvedValueOnce(json({ done: false }))
      await pollVideoTask(input.apiKey, handle('gemini', taskId))
    }
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual(Array(3).fill(`http://localhost:3000/v1beta/${operation}`))
    await expect(pollVideoTask(input.apiKey, handle('gemini', `https://external.example/v1beta/${operation}`))).rejects.toMatchObject({ code: 'UNSAFE_GATEWAY_URL' })
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('downloads the exact authenticated Grok content path', async () => {
    const path = '/v1/videos/task-1/content'
    fetchMock.mockResolvedValueOnce(json({ request_id: 'task-1', status: 'done', video: { url: path } }))
    fetchMock.mockResolvedValueOnce(new Response('video', { headers: { 'Content-Type': 'video/mp4' } }))
    expect((await pollVideoTask(input.apiKey, handle('grok'))).assets[0]).toMatchObject({ objectUrl: true, sourceUrl: path })
    expect(fetchMock.mock.calls[1]![0]).toBe(`http://localhost:3000${path}`)
    expect(fetchMock.mock.calls[1]![1].headers.get('Authorization')).toBe('Bearer user-video-key')
  })

  it.each([
    ['seedance', { status: 'failed', error: { message: 'Seedance rejected input' } }, 'failed', 'Seedance rejected input'],
    ['seedance', { status: 'cancelled' }, 'cancelled', 'cancelled'],
    ['gemini', { done: true, error: { code: 3, status: 'INVALID_ARGUMENT', message: 'Veo rejected input' } }, 'failed', 'Veo rejected input'],
    ['minimax_v1', { status: 'Fail', base_resp: { status_code: 1004, status_msg: 'MiniMax rejected input' } }, 'failed', 'MiniMax rejected input'],
    ['minimax_v2', { task: { id: 'task-1', status: 'failed', error: { message: 'H3 rejected input' } } }, 'failed', 'H3 rejected input'],
    ['minimax_v2', { task: { id: 'task-1', status: 'cancelled' } }, 'cancelled', 'cancelled'],
    ['grok', { status: 'expired' }, 'failed', 'expired'],
    ['grok', { status: 'failed', error: 'Grok rejected input' }, 'failed', 'Grok rejected input'],
  ] as const)('normalizes %s terminal failures and cancellation', async (protocol, body, status, error) => {
    fetchMock.mockResolvedValueOnce(json(body))
    expect(await pollVideoTask(input.apiKey, handle(protocol))).toMatchObject({ status, error: expect.stringContaining(error), assets: [] })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['seedance', { status: 'succeeded' }], ['gemini', { done: true, response: {} }],
    ['minimax_v2', { task: { status: 'succeeded', content: {} } }], ['grok', { status: 'done' }],
  ] as const)('does not mark %s empty output as successful', async (protocol, body) => {
    fetchMock.mockResolvedValueOnce(json(body))
    expect(await pollVideoTask(input.apiKey, handle(protocol))).toEqual({ status: 'failed', assets: [], error: 'Video generation returned no output.' })
    expect(createObjectURL).not.toHaveBeenCalled()
  })

  it.each(['seedance', 'minimax_v1', 'minimax_v2', 'grok'] as const)('rejects unknown %s statuses', async protocol => {
    fetchMock.mockResolvedValueOnce(json(protocol === 'minimax_v2' ? { task: { status: 'mystery' } } : { status: 'mystery' }))
    await expect(pollVideoTask(input.apiKey, handle(protocol))).rejects.toMatchObject({ code: 'INVALID_TASK_STATUS' })
  })

  it('rejects mismatched task and file identities before downloading', async () => {
    fetchMock.mockResolvedValueOnce(json({ task: { id: 'another-task', status: 'succeeded', content: { url: protectedURL } } }))
    await expect(pollVideoTask(input.apiKey, handle('minimax_v2'))).rejects.toMatchObject({ code: 'INVALID_TASK_RESPONSE' })
    fetchMock.mockResolvedValueOnce(json({ status: 'Success', file_id: 'file-9' }))
    fetchMock.mockResolvedValueOnce(json({ file: { file_id: 'another-file', download_url: protectedURL } }))
    await expect(pollVideoTask(input.apiKey, handle('minimax_v1'))).rejects.toMatchObject({ code: 'INVALID_TASK_RESPONSE' })
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('cleans up earlier object URLs if a later protected sample download fails', async () => {
    fetchMock.mockResolvedValueOnce(json({ done: true, response: { generateVideoResponse: { generatedSamples: [{ video: { uri: protectedURL } }, { video: { uri: '/v1/video-tasks/other/content?index=1' } }] } } }))
    fetchMock.mockResolvedValueOnce(new Response('video', { headers: { 'Content-Type': 'video/mp4' } }))
    fetchMock.mockResolvedValueOnce(json({ error: { message: 'Not owned by this key' } }, 404))
    await expect(pollVideoTask(input.apiKey, handle('gemini'))).rejects.toMatchObject({ status: 404, message: 'Not owned by this key' })
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:creation-video')
  })

  it('rejects unsafe result schemes', async () => {
    fetchMock.mockResolvedValueOnce(json({ status: 'succeeded', content: { video_url: 'javascript:alert(1)' } }))
    await expect(pollVideoTask(input.apiKey, handle('seedance'))).rejects.toMatchObject({ code: 'INVALID_ASSET_URL' })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('preserves HTTP errors and rejects malformed JSON and network failures', async () => {
    fetchMock.mockResolvedValueOnce(json({ error: { code: 'authentication_error', message: 'Invalid API key' } }, 401, { 'X-Request-Id': 'auth-request' }))
    await expect(pollVideoTask(input.apiKey, handle('grok'))).rejects.toMatchObject({ status: 401, code: 'authentication_error', requestId: 'auth-request', message: 'Invalid API key' })
    fetchMock.mockResolvedValueOnce(new Response('not JSON'))
    await expect(pollVideoTask(input.apiKey, handle('grok'))).rejects.toMatchObject({ code: 'INVALID_RESPONSE' })
    fetchMock.mockRejectedValueOnce(new TypeError('offline'))
    await expect(pollVideoTask(input.apiKey, handle('grok'))).rejects.toMatchObject({ code: 'NETWORK_ERROR' })
  })

  it('aborts before reading references or sending requests without cancelling upstream', async () => {
    const controller = new AbortController()
    controller.abort()
    await expect(submitVideoRequest({ ...input, signal: controller.signal, referenceImages: [reference()] })).rejects.toMatchObject({ code: 'ABORTED' })
    await expect(pollVideoTask(input.apiKey, handle('grok'), controller.signal)).rejects.toMatchObject({ code: 'ABORTED' })
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rejects unsafe operation names before issuing a request', async () => {
    for (const taskId of ['../task', 'models/../operations/one', 'models/veo/operations/one?secret=true', '//external.example/task']) {
      await expect(pollVideoTask(input.apiKey, handle('gemini', taskId))).rejects.toMatchObject({ code: 'INVALID_TASK_HANDLE' })
    }
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
