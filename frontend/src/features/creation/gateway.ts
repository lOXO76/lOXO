import { abortError, CreationGatewayError, normalizeError, throwIfAborted } from './http'
import { pollImageTask, submitImageRequest } from './image'
import { pollVideoTask, submitVideoRequest } from './video'
import type { CreationSubmitResult, CreationTaskHandle, CreationTaskSnapshot, ImageGenerationInput, VideoGenerationInput, WaitForTaskOptions } from './types'

export { CreationGatewayError } from './http'
export { revokeAssets } from './protect'

function validateInput(input: ImageGenerationInput | VideoGenerationInput): void {
  throwIfAborted(input.signal)
  if (!input.apiKey.trim() || !input.model.trim() || !input.prompt.trim()) {
    throw new CreationGatewayError('Choose an API key and model, and enter a prompt.', 0, 'INVALID_INPUT')
  }
}

export async function submitImage(input: ImageGenerationInput): Promise<CreationSubmitResult> {
  try {
    validateInput(input)
    return await submitImageRequest(input)
  } catch (error) { throw normalizeError(error, input.signal) }
}

export async function submitVideo(input: VideoGenerationInput): Promise<CreationSubmitResult> {
  try {
    validateInput(input)
    return await submitVideoRequest(input)
  } catch (error) { throw normalizeError(error, input.signal) }
}

export async function pollTask(apiKey: string, handle: CreationTaskHandle, signal?: AbortSignal): Promise<CreationTaskSnapshot> {
  try {
    throwIfAborted(signal)
    if (!apiKey.trim() || !handle.taskId.trim() || !['image', 'video'].includes(handle.kind)) {
      throw new CreationGatewayError('Choose an API key and a valid creation task.', 0, 'INVALID_INPUT')
    }
    return handle.kind === 'image' ? await pollImageTask(apiKey, handle, signal) : await pollVideoTask(apiKey, handle, signal)
  } catch (error) { throw normalizeError(error, signal) }
}

function delay(milliseconds: number, signal: AbortSignal): Promise<void> {
  throwIfAborted(signal)
  return new Promise((resolve, reject) => {
    const abort = () => { clearTimeout(timer); signal.removeEventListener('abort', abort); reject(abortError()) }
    const timer = setTimeout(() => { signal.removeEventListener('abort', abort); resolve() }, milliseconds)
    signal.addEventListener('abort', abort, { once: true })
  })
}

export async function waitForTask(apiKey: string, handle: CreationTaskHandle, options: WaitForTaskOptions = {}): Promise<CreationTaskSnapshot> {
  const timeoutMs = options.timeoutMs ?? 30 * 60 * 1000
  const intervalMs = options.intervalMs ?? 3000
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0 || !Number.isFinite(intervalMs) || intervalMs < 0) {
    throw new CreationGatewayError('The wait timeout must be positive and the polling interval cannot be negative.', 0, 'INVALID_INPUT')
  }
  throwIfAborted(options.signal)
  const controller = new AbortController()
  let timedOut = false
  const abort = () => controller.abort()
  options.signal?.addEventListener('abort', abort, { once: true })
  const timer = setTimeout(() => { timedOut = true; controller.abort() }, timeoutMs)
  let consecutiveErrors = 0
  try {
    while (true) {
      let snapshot: CreationTaskSnapshot
      try {
        snapshot = await pollTask(apiKey, handle, controller.signal)
      } catch (error) {
        const failure = normalizeError(error, controller.signal)
        const retryable = failure.code === 'NETWORK_ERROR' || [408, 429].includes(failure.status) || failure.status >= 500
        consecutiveErrors += 1
        if (!retryable || consecutiveErrors > 3 || controller.signal.aborted) throw failure
        await delay(Math.max(100, failure.retryAfterMs ?? intervalMs), controller.signal)
        continue
      }
      consecutiveErrors = 0
      options.onUpdate?.(snapshot)
      if (snapshot.status !== 'processing') return snapshot
      await delay(Math.max(100, snapshot.retryAfterMs ?? intervalMs), controller.signal)
    }
  } catch (error) {
    if (timedOut) throw new CreationGatewayError('Waiting timed out. The task may still be running; check Async Tasks before submitting again.', 0, 'TIMEOUT')
    throw normalizeError(error, controller.signal)
  } finally {
    clearTimeout(timer)
    options.signal?.removeEventListener('abort', abort)
  }
}
