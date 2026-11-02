import { CreationGatewayError, errorMessage, jsonRequest, record, requestJSON, responseError, text, throwIfAborted } from './http'
import type { JsonObject } from './http'
import { readReferenceImage, resolveAssets, validateReferenceImage } from './protect'
import type { CreationImageRole, CreationSubmitResult, CreationTaskHandle, CreationTaskSnapshot, CreationVideoProtocol, VideoGenerationInput } from './types'

const maxRequestBytes = 256 * 1024 * 1024
const defaultRetryAfterMs = 3000

interface VideoReference {
  role: CreationImageRole
  mimeType: string
  data: string
  dataUrl: string
}

function invalidReferences(message: string): never {
  throw new CreationGatewayError(message, 0, 'INVALID_REFERENCE_IMAGE')
}

function validateReferences(input: VideoGenerationInput): void {
  const references = input.referenceImages ?? []
  let encodedBytes = 0
  for (const reference of references) {
    validateReferenceImage(reference.file)
    encodedBytes += 4 * Math.ceil(reference.file.size / 3)
    if (reference.role && !['first_frame', 'last_frame', 'reference_image'].includes(reference.role)) {
      invalidReferences('Choose a first frame, last frame or reference image role.')
    }
  }
  if (encodedBytes > maxRequestBytes) invalidReferences('The inline reference images exceed the default 256 MiB gateway request limit. Use fewer or smaller images.')
  for (const role of ['first_frame', 'last_frame']) {
    if (references.filter(reference => (reference.role ?? 'first_frame') === role).length > 1) {
      invalidReferences('Only one first frame and one last frame can be submitted.')
    }
  }
  const assets = references.filter(reference => reference.role === 'reference_image')
  if (input.protocol === 'gemini' || input.protocol === 'minimax_v2') {
    const maximum = input.protocol === 'gemini' ? 3 : 9
    if (assets.length > maximum) invalidReferences(`This video protocol accepts at most ${maximum} reference images.`)
    if (input.protocol === 'minimax_v2' && assets.length && assets.length !== references.length) invalidReferences('MiniMax v2 reference images cannot be mixed with first or last frames.')
  }
}

function videoBody(input: VideoGenerationInput, references: VideoReference[]): JsonObject {
  const options = {
    ...(input.duration !== undefined ? { duration: input.duration } : {}),
    ...(input.resolution ? { resolution: input.protocol.startsWith('minimax_') ? input.resolution.toUpperCase() : input.resolution.toLowerCase() } : {}),
  }
  if (input.protocol === 'gemini') {
    const instance: JsonObject = { prompt: input.prompt }
    const referenceImages = []
    for (const reference of references) {
      const image = { bytesBase64Encoded: reference.data, mimeType: reference.mimeType }
      if (reference.role === 'reference_image') referenceImages.push({ image, referenceType: 'asset' })
      else instance[reference.role === 'last_frame' ? 'lastFrame' : 'image'] = image
    }
    if (referenceImages.length) instance.referenceImages = referenceImages
    return {
      instances: [instance],
      parameters: {
        sampleCount: 1,
        ...(input.duration !== undefined ? { durationSeconds: input.duration } : {}),
        ...(input.resolution ? { resolution: input.resolution.toLowerCase() } : {}),
        ...(input.ratio ? { aspectRatio: input.ratio } : {}),
      },
    }
  }
  if (input.protocol === 'seedance' || input.protocol === 'minimax_v2') {
    return {
      model: input.model,
      content: [
        { type: 'text', text: input.prompt },
        ...references.map(reference => ({ type: 'image_url', image_url: { url: reference.dataUrl }, role: reference.role })),
      ],
      ...options,
      ...(input.protocol === 'minimax_v2' ? { ratio: input.ratio || (references.length ? 'adaptive' : '16:9') } : input.ratio ? { ratio: input.ratio } : {}),
    }
  }
  const body: JsonObject = { model: input.model, prompt: input.prompt, ...options }
  if (input.protocol === 'minimax_v1') {
    const images: string[] = []
    for (const reference of references) {
      if (reference.role === 'reference_image') images.push(reference.dataUrl)
      else body[`${reference.role}_image`] = reference.dataUrl
    }
    if (images.length) body.subject_reference = [{ type: 'character', image: images }]
  } else {
    if (input.ratio) body.aspect_ratio = input.ratio
    const images = []
    for (const reference of references) {
      const image = { url: reference.dataUrl, type: 'image_url' }
      if (reference.role === 'reference_image') images.push(image)
      else body[reference.role === 'last_frame' ? 'last_frame' : 'image'] = image
    }
    if (images.length) body.reference_images = images
  }
  return body
}

function taskSegment(taskId: string, protocol: CreationVideoProtocol): string {
  const valid = protocol.startsWith('minimax_') ? /^[A-Za-z0-9_-]{1,128}$/ : /^[A-Za-z0-9_.-]{1,128}$/
  if (!valid.test(taskId) || /^\.+$/.test(taskId)) {
    throw new CreationGatewayError('The gateway returned an invalid video task ID. Check Async Tasks before submitting again.', 0, 'INVALID_TASK_HANDLE')
  }
  return encodeURIComponent(taskId)
}

function videoPollPath(handle: CreationTaskHandle): string {
  if (handle.kind !== 'video') throw new CreationGatewayError('A video task handle is required.', 0, 'INVALID_TASK_HANDLE')
  if (handle.protocol === 'gemini') {
    const absolute = /^https?:\/\//i.test(handle.taskId)
    let name = handle.taskId
    if (absolute) {
      try { name = new URL(name).pathname } catch { throw new CreationGatewayError('The gateway returned an invalid video operation name.', 0, 'INVALID_TASK_HANDLE') }
    }
    name = name.replace(/^\/?v1beta\//, '').replace(/^\//, '')
    const segments = name.split('/')
    if (segments.length !== 4 || segments[0] !== 'models' || segments[2] !== 'operations') {
      throw new CreationGatewayError('The gateway returned an invalid video operation name.', 0, 'INVALID_TASK_HANDLE')
    }
    for (const segment of segments) taskSegment(segment, 'gemini')
    return absolute ? handle.taskId : `/v1beta/${name}`
  }
  if (!['seedance', 'minimax_v1', 'minimax_v2', 'grok'].includes(handle.protocol)) {
    throw new CreationGatewayError('This video protocol is not supported.', 0, 'INVALID_TASK_HANDLE')
  }
  const taskId = taskSegment(handle.taskId, handle.protocol as CreationVideoProtocol)
  switch (handle.protocol) {
    case 'seedance': return `/api/v3/contents/generations/tasks/${taskId}`
    case 'minimax_v1': return `/v1/query/video_generation?task_id=${taskId}`
    case 'minimax_v2': return `/v2/query/video_generation/${taskId}`
    default: return `/v1/videos/${taskId}`
  }
}

function checkVideoResponse(body: JsonObject, response: Response): void {
  if (body.error) throw responseError(body, response, 'Video generation failed.')
  const business = record(body.base_resp)
  if (business.status_code !== undefined && business.status_code !== 0) {
    throw responseError({ ...body, error: { code: business.status_code, message: errorMessage(business, 'MiniMax rejected the video request.') } }, response, 'Video generation failed.')
  }
}

function checkTaskIdentity(returnedId: unknown, expectedId: string): void {
  if (text(returnedId) && text(returnedId) !== expectedId) {
    throw new CreationGatewayError('The gateway returned a different video task ID.', 0, 'INVALID_TASK_RESPONSE')
  }
}

function taskProgress(body: JsonObject): Pick<CreationTaskSnapshot, 'progress'> {
  const progress = body.progress ?? record(body.metadata).progress
  return typeof progress === 'number' && Number.isFinite(progress) ? { progress: Math.min(100, Math.max(0, progress)) } : {}
}

async function completedVideo(apiKey: string, urls: string[], signal?: AbortSignal): Promise<CreationTaskSnapshot> {
  if (!urls.length) return { status: 'failed', assets: [], error: 'Video generation returned no output.' }
  return { status: 'succeeded', assets: await resolveAssets(apiKey, 'video', urls, signal) }
}

export async function submitVideoRequest(input: VideoGenerationInput): Promise<CreationSubmitResult> {
  throwIfAborted(input.signal)
  const paths: Record<CreationVideoProtocol, string> = {
    seedance: '/api/v3/contents/generations/tasks',
    gemini: `/v1beta/models/${encodeURIComponent(input.model.replace(/^models\//, ''))}:predictLongRunning`,
    minimax_v1: '/v1/video_generation',
    minimax_v2: '/v2/video_generation',
    grok: '/v1/videos',
  }
  const path = paths[input.protocol]
  if (!path) throw new CreationGatewayError('This video protocol is not supported.', 0, 'INVALID_INPUT')
  if (input.duration !== undefined && (!Number.isInteger(input.duration) || input.duration <= 0 && !(input.protocol === 'seedance' && input.duration === -1))) {
    throw new CreationGatewayError('Video duration must be a positive integer, or -1 for Seedance adaptive duration.', 0, 'INVALID_INPUT')
  }
  validateReferences(input)
  const references: VideoReference[] = []
  for (const reference of input.referenceImages ?? []) {
    references.push({ ...await readReferenceImage(reference.file, input.signal), role: reference.role ?? 'first_frame' })
  }
  const init = jsonRequest(videoBody(input, references), input.signal)
  if (new Blob([init.body as string]).size > maxRequestBytes) invalidReferences('The video request exceeds the default 256 MiB gateway request limit. Use fewer or smaller images.')
  const { body, response, retryAfterMs } = await requestJSON(input.apiKey, path, init)
  checkVideoResponse(body, response)
  const grokTaskId = [body, record(body.data), record(body.video)].map(source => text(source.request_id) || text(source.id) || text(source.task_id)).find(Boolean)
  const taskId = input.protocol === 'gemini' ? text(body.name)
    : input.protocol.startsWith('minimax_') ? text(body.task_id)
      : input.protocol === 'seedance' ? text(body.id)
        : grokTaskId
  if (!taskId) throw responseError(body, response, 'The gateway accepted the request without returning a video task ID. Check Async Tasks before submitting again.')
  const handle: CreationTaskHandle = { kind: 'video', protocol: input.protocol, taskId, model: input.model }
  videoPollPath(handle)
  return { handle, snapshot: { status: 'processing', assets: [], retryAfterMs: retryAfterMs ?? defaultRetryAfterMs } }
}

export async function pollVideoTask(apiKey: string, handle: CreationTaskHandle, signal?: AbortSignal): Promise<CreationTaskSnapshot> {
  throwIfAborted(signal)
  const { body, response, retryAfterMs } = await requestJSON(apiKey, videoPollPath(handle), { signal })
  const task = handle.protocol === 'minimax_v2' ? record(body.task) : body
  const progress = taskProgress(task)
  if (handle.protocol === 'gemini') {
    if (body.error) return { status: 'failed', assets: [], error: errorMessage(body, 'Video generation failed.'), ...progress }
    if (body.done === true) {
      const result = record(body.response)
      const samples = record(result.generateVideoResponse).generatedSamples ?? result.generatedVideos
      const urls = Array.isArray(samples) ? samples.map(sample => text(record(record(sample).video).uri)).filter(Boolean) : []
      return { ...await completedVideo(apiKey, urls, signal), ...progress }
    }
    if (body.done !== undefined && body.done !== false) throw new CreationGatewayError('The gateway returned an invalid video operation status.', 0, 'INVALID_TASK_STATUS')
    return { status: 'processing', assets: [], retryAfterMs: retryAfterMs ?? defaultRetryAfterMs, ...progress }
  }
  checkTaskIdentity(handle.protocol === 'minimax_v1' ? body.task_id : handle.protocol === 'grok' ? body.request_id : task.id, handle.taskId)
  const status = text(task.status).toLowerCase()
  if (['failed', 'fail', 'expired', 'cancelled', 'canceled'].includes(status)) {
    const cancelled = status === 'cancelled' || status === 'canceled'
    const fallback = cancelled ? 'Video generation was cancelled.' : status === 'expired' ? 'The video task expired.' : 'Video generation failed.'
    const error = handle.protocol === 'minimax_v1' ? errorMessage(body, errorMessage(record(body.base_resp), fallback)) : errorMessage(task, fallback)
    return { status: cancelled ? 'cancelled' : 'failed', assets: [], error, ...progress }
  }
  checkVideoResponse(body, response)
  if (handle.protocol === 'minimax_v1' && status === 'success') {
    const fileId = text(body.file_id)
    if (!fileId) throw responseError(body, response, 'The completed MiniMax task did not return a file ID. Retry the query or check Async Tasks.')
    const fileResult = await requestJSON(apiKey, `/v1/files/retrieve?file_id=${taskSegment(fileId, 'minimax_v1')}`, { signal })
    checkVideoResponse(fileResult.body, fileResult.response)
    const file = record(fileResult.body.file)
    checkTaskIdentity(file.file_id, fileId)
    const url = text(file.download_url)
    if (!url) throw responseError(fileResult.body, fileResult.response, 'MiniMax did not return a video download URL. Retry the query or check Async Tasks.')
    return { ...await completedVideo(apiKey, [url], signal), ...progress }
  }
  const successful = handle.protocol === 'grok' ? ['done', 'completed', 'succeeded'].includes(status) : status === 'succeeded'
  if (successful) {
    const url = handle.protocol === 'minimax_v2' ? text(record(task.content).url) : text(record(task.content).video_url) || text(record(task.video).url)
    return { ...await completedVideo(apiKey, url ? [url] : [], signal), ...progress }
  }
  const pending = handle.protocol === 'minimax_v1' ? ['preparing', 'queueing', 'processing']
    : handle.protocol === 'grok' ? ['pending', 'queued', 'running', 'processing'] : ['queued', 'running']
  if (!pending.includes(status)) throw new CreationGatewayError('The gateway returned an unknown video task status.', response.status, 'INVALID_TASK_STATUS')
  return { status: 'processing', assets: [], retryAfterMs: retryAfterMs ?? defaultRetryAfterMs, ...progress }
}
