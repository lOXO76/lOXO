import { CreationGatewayError, errorMessage, jsonRequest, record, requestJSON, responseError, text, throwIfAborted } from './http'
import type { JsonObject } from './http'
import { inlineImage, readReferenceImage, resolveAsset, revokeAssets, validateReferenceImage } from './protect'
import type { CreationAsset, CreationSubmitResult, CreationTaskHandle, CreationTaskSnapshot, ImageGenerationInput } from './types'

const pollURLs = new WeakMap<CreationTaskHandle, string>()
const geminiFailureReasons: Record<string, string> = {
  SAFETY: 'The prompt or output was blocked by safety filters',
  IMAGE_SAFETY: 'The generated image was blocked by safety filters',
  PROHIBITED_CONTENT: 'The prompt contains prohibited content',
  IMAGE_PROHIBITED_CONTENT: 'The generated image contains prohibited content',
  BLOCKLIST: 'The prompt contains blocked terms',
  RECITATION: 'The response was blocked because it may reproduce protected content',
  MAX_TOKENS: 'The response reached the output limit before an image was generated',
  NO_IMAGE: 'The model did not produce an image',
  IMAGE_OTHER: 'The model could not generate an image for this prompt',
}

export function buildOpenAIImageBody(input: ImageGenerationInput): JsonObject {
  const base = { model: input.model, prompt: input.prompt, n: input.n ?? 1 }
  // xAI Imagine 只认 aspect_ratio 与 resolution（1k / 2k），不发 OpenAI 的 size / quality。
  if (input.platform === 'grok') {
    return {
      ...base,
      ...(input.aspectRatio ? { aspect_ratio: input.aspectRatio } : {}),
      ...(input.imageSize ? { resolution: input.imageSize.toLowerCase() } : {}),
    }
  }
  return {
    ...base,
    ...(input.size ? { size: input.size } : {}),
    ...(input.quality ? { quality: input.quality } : {}),
  }
}

export function buildGeminiImageBody(input: ImageGenerationInput, references: { mimeType: string; data: string }[] = []): JsonObject {
  return {
    contents: [{ role: 'user', parts: [{ text: input.prompt }, ...references.map((image) => ({ inlineData: { mimeType: image.mimeType, data: image.data } }))] }],
    generationConfig: {
      responseModalities: ['TEXT', 'IMAGE'],
      imageConfig: {
        ...(input.aspectRatio ? { aspectRatio: input.aspectRatio } : {}),
        ...(input.imageSize ? { imageSize: input.imageSize } : {}),
      },
    },
  }
}

export interface ImageOutput {
  url?: string
  base64?: string
  mimeType?: string
}

export function parseImageOutputs(body: JsonObject, protocol: 'openai' | 'gemini'): ImageOutput[] {
  const outputs: ImageOutput[] = []
  if (protocol === 'openai') {
    if (Array.isArray(body.data)) {
      for (const value of body.data) {
        const item = record(value)
        if (text(item.url)) outputs.push({ url: text(item.url) })
        else if (text(item.b64_json)) outputs.push({ base64: text(item.b64_json) })
      }
    }
  } else {
    const reasons: string[] = []
    const feedback = record(body.promptFeedback)
    if (text(feedback.blockReason)) reasons.push(text(feedback.blockReason))
    if (Array.isArray(body.candidates)) {
      for (const candidate of body.candidates) {
        const item = record(candidate)
        const parts = record(item.content).parts
        if (Array.isArray(parts)) {
          for (const part of parts) {
            const inline = record(record(part).inlineData ?? record(part).inline_data)
            const mimeType = text(inline.mimeType ?? inline.mime_type)
            if (text(inline.data) && (!mimeType || mimeType.startsWith('image/'))) {
              outputs.push({ base64: text(inline.data), mimeType: mimeType || undefined })
            }
          }
        }
        if (text(item.finishReason) && item.finishReason !== 'STOP') reasons.push(text(item.finishReason))
      }
    }
    if (!outputs.length && reasons.length) {
      const message = reasons.map((reason) => geminiFailureReasons[reason] ? `${geminiFailureReasons[reason]} (${reason})` : reason).join('; ')
      throw new CreationGatewayError(`No image was generated: ${message}.`, 0, reasons[0])
    }
  }
  if (!outputs.length) throw new CreationGatewayError(errorMessage(body, 'The gateway returned no images.'), 0, 'NO_IMAGE_OUTPUT')
  return outputs
}

async function imageAssets(apiKey: string, body: JsonObject, protocol: 'openai' | 'gemini', signal?: AbortSignal): Promise<CreationAsset[]> {
  const assets: CreationAsset[] = []
  try {
    for (const output of parseImageOutputs(body, protocol)) {
      throwIfAborted(signal)
      assets.push(output.url ? await resolveAsset(apiKey, 'image', output.url, signal, output.mimeType) : inlineImage(output.base64!, output.mimeType))
    }
    return assets
  } catch (error) {
    revokeAssets(assets)
    throw error
  }
}

export async function submitImageRequest(input: ImageGenerationInput): Promise<CreationSubmitResult> {
  if (!['openai', 'gemini'].includes(input.protocol)) throw new CreationGatewayError('This image protocol is not supported.', 0, 'INVALID_INPUT')
  if (!Number.isInteger(input.n ?? 1) || (input.n ?? 1) < 1) throw new CreationGatewayError('The image count must be a positive integer.', 0, 'INVALID_INPUT')
  if (input.protocol === 'gemini') {
    if ((input.n ?? 1) !== 1) throw new CreationGatewayError('Gemini generates one image request at a time. Set the image count to 1.', 0, 'INVALID_INPUT')
    const references = []
    for (const reference of input.referenceImages ?? []) references.push(await readReferenceImage(reference.file, input.signal))
    const { body, response } = await requestJSON(input.apiKey, `/v1beta/models/${encodeURIComponent(input.model.replace(/^models\//, ''))}:generateContent`, jsonRequest(buildGeminiImageBody(input, references), input.signal), true)
    if (body.error) throw responseError(body, response, 'Image generation failed.')
    return { handle: null, snapshot: { status: 'succeeded', assets: await imageAssets(input.apiKey, body, 'gemini', input.signal) } }
  }
  const editing = Boolean(input.referenceImages?.length)
  const path = `/v1/images/${editing ? 'edits' : 'generations'}${input.preferAsync ? '/async' : ''}`
  const payload = buildOpenAIImageBody(input)
  let init: RequestInit
  if (editing) {
    const form = new FormData()
    for (const [key, value] of Object.entries(payload)) form.append(key, String(value))
    if (input.platform === 'grok' && (input.referenceImages?.length ?? 0) > 3) {
      throw new CreationGatewayError('Grok image editing supports at most three reference images.', 0, 'INVALID_REFERENCE_IMAGE')
    }
    for (const reference of input.referenceImages ?? []) {
      validateReferenceImage(reference.file)
      form.append('image[]', reference.file, reference.file.name)
    }
    init = { method: 'POST', body: form, signal: input.signal }
  } else init = jsonRequest(payload, input.signal)
  const { body, response, retryAfterMs } = await requestJSON(input.apiKey, path, init)
  if (body.error) throw responseError(body, response, 'Image generation failed.')
  if (input.preferAsync || response.status === 202) {
    const taskId = text(body.task_id) || text(body.id)
    if (!taskId) throw responseError(body, response, 'The gateway accepted the request without returning a task ID. Check Async Tasks before submitting again.')
    const handle: CreationTaskHandle = { kind: 'image', protocol: 'openai', taskId, model: input.model }
    const pollUrl = text(body.poll_url) || response.headers.get('Location')
    if (pollUrl) pollURLs.set(handle, pollUrl)
    return { handle, snapshot: { status: 'processing', assets: [], retryAfterMs: retryAfterMs ?? 3000 } }
  }
  return { handle: null, snapshot: { status: 'succeeded', assets: await imageAssets(input.apiKey, body, 'openai', input.signal) } }
}

export async function pollImageTask(apiKey: string, handle: CreationTaskHandle, signal?: AbortSignal): Promise<CreationTaskSnapshot> {
  if (handle.protocol !== 'openai') throw new CreationGatewayError('This image protocol does not support task polling.', 0, 'INVALID_INPUT')
  const { body, response, retryAfterMs } = await requestJSON(apiKey, pollURLs.get(handle) || `/v1/images/tasks/${encodeURIComponent(handle.taskId)}`, { signal })
  const status = text(body.status)
  if (status === 'completed' || status === 'succeeded') {
    const result = record(body.result)
    const payload = Array.isArray(result.data) ? result : { data: [{ url: text(body.image_url) }] }
    return { status: 'succeeded', assets: await imageAssets(apiKey, payload, 'openai', signal) }
  }
  if (status === 'failed' || status === 'cancelled') return { status, assets: [], error: errorMessage(body, status === 'failed' ? 'Image generation failed.' : 'Image generation was cancelled.') }
  if (!['processing', 'queued', 'running', 'pending'].includes(status)) {
    if (body.error) throw responseError(body, response, 'Could not query the image task.')
    throw new CreationGatewayError('The gateway returned an unknown image task status.', response.status, 'INVALID_TASK_STATUS', response.headers.get('X-Request-Id') || '')
  }
  const progress = typeof body.progress === 'number' && Number.isFinite(body.progress) ? Math.max(0, Math.min(100, body.progress)) : undefined
  return { status: 'processing', assets: [], retryAfterMs: retryAfterMs ?? 3000, ...(progress !== undefined ? { progress } : {}) }
}
