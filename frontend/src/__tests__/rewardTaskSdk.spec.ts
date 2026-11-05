import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { JSDOM } from 'jsdom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

interface SDKState {
  status: 'idle' | 'in_progress' | 'paused' | 'completed' | 'error'
  paused_reason: 'idle' | 'hidden' | null
  progress_seconds: number
  browse_seconds: number
  completed: boolean
  error: string | null
}

interface SDK {
  getState(): SDKState
  onChange(callback: (state: SDKState) => void): () => void
}

interface BootOptions {
  url?: string
  storedTicket?: string
  hidden?: boolean
  fallbackScript?: boolean
  scriptSrc?: string
  appKey?: string
  badge?: string
  lang?: string
  navigatorLanguage?: string
  blockedStorage?: boolean
  noBody?: boolean
}

const ticket = `rt_${'a'.repeat(40)}`
const storedTicket = `rt_${'b'.repeat(40)}`
const appKey = `rk_${'c'.repeat(24)}`
const ticketStorageKey = 'reward_task_ticket'
const sdkSourcePath = resolve('public/reward-task-sdk.js')
const activityEvents = ['mousemove', 'mousedown', 'pointerdown', 'keydown', 'wheel', 'touchstart', 'touchmove', 'scroll']
const terminalReasons = [
  'REWARD_TICKET_NOT_FOUND',
  'REWARD_TICKET_EXPIRED',
  'REWARD_TICKET_PARTNER_MISMATCH',
  'REWARD_EVENT_TYPE_MISMATCH',
  'REWARD_ORIGIN_NOT_ALLOWED',
  'REWARD_PARTNER_INVALID',
  'REWARD_TASKS_DISABLED',
  'REWARD_TASK_UNAVAILABLE',
  'REWARD_TASK_ALREADY_COMPLETED',
  'REWARD_TASK_QUOTA_EXHAUSTED'
]

function success(overrides: Record<string, unknown> = {}): Response {
  return new Response(JSON.stringify({
    code: 0,
    message: 'success',
    data: {
      status: 'in_progress',
      progress_seconds: 5,
      browse_seconds: 30,
      completed: false,
      reward_amount: 0.5,
      heartbeat_interval: 1,
      idle_timeout_seconds: 15,
      ...overrides
    }
  }))
}

function failure(reason: string, status = 400): Response {
  return new Response(JSON.stringify({ code: status, message: 'Request failed', reason }), { status })
}

describe('reward task browser SDK', () => {
  const windows: JSDOM[] = []
  let fetchMock = vi.fn<typeof fetch>()

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-05T10:00:00Z'))
    fetchMock = vi.fn<typeof fetch>().mockImplementation(async () => success())
  })

  afterEach(async () => {
    await vi.advanceTimersByTimeAsync(0)
    for (const dom of windows.splice(0)) dom.window.close()
    vi.clearAllTimers()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  function boot(options: BootOptions = {}) {
    const dom = new JSDOM('<!doctype html><html><head></head><body></body></html>', {
      url: options.url ?? `https://partner.example/visit?reward_ticket=${ticket}`,
      runScripts: 'outside-only'
    })
    windows.push(dom)
    const sdkWindow = dom.window as unknown as Window & typeof globalThis & { RewardTaskSDK: SDK }
    const doc = sdkWindow.document
    sdkWindow.history.replaceState({ hostPage: 'state' }, '')
    let hidden = options.hidden ?? false
    Object.defineProperty(doc, 'visibilityState', { configurable: true, get: () => hidden ? 'hidden' : 'visible' })
    Object.defineProperty(sdkWindow.navigator, 'language', {
      configurable: true,
      value: options.navigatorLanguage ?? 'zh-CN'
    })
    sdkWindow.fetch = fetchMock
    sdkWindow.Date = window.Date
    sdkWindow.setTimeout = window.setTimeout.bind(window)
    sdkWindow.clearTimeout = window.clearTimeout.bind(window)
    if (options.storedTicket) sdkWindow.sessionStorage.setItem(ticketStorageKey, options.storedTicket)
    if (options.blockedStorage) {
      Object.defineProperty(sdkWindow, 'sessionStorage', {
        configurable: true,
        get: () => { throw new DOMException('Storage unavailable', 'SecurityError') }
      })
    }

    const script = doc.createElement('script')
    script.src = options.scriptSrc ?? 'https://rewards.example/reward-task-sdk.js?v=1'
    script.setAttribute('data-app-key', options.appKey ?? appKey)
    if (options.badge !== undefined) script.setAttribute('data-badge', options.badge)
    if (options.lang) script.setAttribute('data-lang', options.lang)
    doc.head.appendChild(script)
    Object.defineProperty(doc, 'currentScript', {
      configurable: true,
      value: options.fallbackScript ? null : script
    })
    if (options.noBody) doc.body.remove()
    const addListener = vi.spyOn(doc, 'addEventListener')
    const removeListener = vi.spyOn(doc, 'removeEventListener')
    sdkWindow.eval(readFileSync(sdkSourcePath, 'utf8'))

    return {
      window: sdkWindow,
      document: doc,
      sdk: sdkWindow.RewardTaskSDK,
      addListener,
      removeListener,
      activity(type = 'mousemove', trusted = true) {
        // 直接调用捕获的监听回调，不伪造浏览器事件的只读 isTrusted 属性。
        for (const [name, callback] of addListener.mock.calls) {
          if (name !== type || !callback) continue
          const event = { isTrusted: trusted } as Event
          if (typeof callback === 'function') callback.call(doc, event)
          else callback.handleEvent(event)
        }
      },
      setHidden(value: boolean) {
        hidden = value
        doc.dispatchEvent(new sdkWindow.Event('visibilitychange'))
      }
    }
  }

  async function settle() {
    await vi.advanceTimersByTimeAsync(0)
  }

  it('uses the URL ticket, saves it, removes every ticket query, and preserves history state and other URL parts', async () => {
    const url = `https://partner.example/visit?from=campaign&reward_ticket=${ticket}&reward_ticket=duplicate#details`
    const runtime = boot({ url, storedTicket })
    await settle()

    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBe(ticket)
    expect(runtime.window.location.href).toBe('https://partner.example/visit?from=campaign#details')
    expect(runtime.window.history.state).toEqual({ hostPage: 'state' })
    expect(fetchMock).toHaveBeenCalledWith('https://rewards.example/api/v1/reward-open/sdk/heartbeat', {
      method: 'POST',
      headers: { 'Content-Type': 'text/plain' },
      body: JSON.stringify({ app_key: appKey, ticket }),
      credentials: 'omit',
      keepalive: true
    })
    expect(runtime.sdk.getState()).toEqual({
      status: 'in_progress', paused_reason: null, progress_seconds: 5, browse_seconds: 30, completed: false, error: null
    })
  })

  it('continues with the sessionStorage ticket when another page has no query ticket', async () => {
    const runtime = boot({ url: 'https://partner.example/another-page?foo=bar', storedTicket })
    await settle()

    expect(JSON.parse(fetchMock.mock.calls[0][1]?.body as string)).toEqual({ app_key: appKey, ticket: storedTicket })
    expect(runtime.window.location.search).toBe('?foo=bar')
  })

  it('does nothing without a ticket and exposes only the documented SDK methods', async () => {
    const runtime = boot({ url: 'https://partner.example/visit' })
    await vi.advanceTimersByTimeAsync(60_000)

    expect(fetchMock).not.toHaveBeenCalled()
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
    expect(Object.keys(runtime.sdk).sort()).toEqual(['getState', 'onChange'])
    expect(runtime.sdk.getState().status).toBe('idle')
  })

  it('does not send requests when the app key is missing', async () => {
    boot({ appKey: '' })
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('finds the script when currentScript is unavailable and uses its origin including the port', async () => {
    boot({ fallbackScript: true, scriptSrc: 'https://rewards.example:8443/assets/reward-task-sdk.js?cache=1' })
    await settle()

    expect(fetchMock.mock.calls[0][0]).toBe('https://rewards.example:8443/api/v1/reward-open/sdk/heartbeat')
  })

  it('keeps a fixed one-second cadence even if an old server returns another heartbeat interval', async () => {
    fetchMock.mockResolvedValueOnce(success({ heartbeat_interval: 7 }))
    boot()
    await settle()
    await vi.advanceTimersByTimeAsync(999)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('pauses all heartbeats while hidden and restores state immediately before the next visible beat', async () => {
    const runtime = boot()
    await settle()
    runtime.setHidden(true)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'hidden' })
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    runtime.setHidden(false)
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    expect(fetchMock).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('waits for visibility when the SDK first loads in a background page', async () => {
    const runtime = boot({ hidden: true })
    await vi.advanceTimersByTimeAsync(30_000)
    expect(fetchMock).not.toHaveBeenCalled()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'hidden' })

    runtime.setHidden(false)
    await settle()
    expect(fetchMock).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('does not overlap in-flight requests when visibility changes', async () => {
    let resolveRequest!: (response: Response) => void
    fetchMock.mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveRequest = resolve }))
    const runtime = boot()
    runtime.setHidden(true)
    runtime.setHidden(false)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    runtime.setHidden(true)
    resolveRequest(success())
    await settle()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    runtime.setHidden(false)
    await settle()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('stops permanently, clears the saved ticket, notifies, and shows completion after the server awards the reward', async () => {
    fetchMock.mockResolvedValueOnce(success({ status: 'completed', completed: true, progress_seconds: 30 }))
    const runtime = boot()
    const listener = vi.fn()
    runtime.sdk.onChange(listener)
    await settle()

    expect(runtime.sdk.getState()).toEqual({
      status: 'completed', paused_reason: null, progress_seconds: 30, browse_seconds: 30, completed: true, error: null
    })
    expect(listener).toHaveBeenLastCalledWith(runtime.sdk.getState())
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBeNull()
    expect(runtime.document.body.textContent).toContain('任务完成，奖励已发放到你的账户')
    runtime.setHidden(true)
    runtime.setHidden(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it.each(terminalReasons)('clears the ticket and stops on terminal reason %s', async (reason) => {
    fetchMock.mockResolvedValueOnce(failure(reason, 403))
    const runtime = boot()
    await settle()

    expect(runtime.sdk.getState()).toMatchObject({ status: 'error', error: reason, completed: false })
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBeNull()
    expect(runtime.document.body.textContent).not.toContain('浏览任务进行中')
    runtime.setHidden(true)
    runtime.setHidden(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('retries network/5xx failures at 2, 4, 8, 16, then at most 30 seconds and resets after success', async () => {
    fetchMock
      .mockResolvedValueOnce(success({ idle_timeout_seconds: 0 }))
      .mockRejectedValueOnce(new TypeError('Network unavailable'))
      .mockResolvedValueOnce(failure('INTERNAL_ERROR', 500))
      .mockResolvedValueOnce(new Response('Gateway unavailable', { status: 503 }))
      .mockResolvedValueOnce(failure('INTERNAL_ERROR', 502))
      .mockRejectedValueOnce(new TypeError('Network unavailable'))
      .mockResolvedValueOnce(failure('INTERNAL_ERROR', 500))
      .mockResolvedValueOnce(success({ idle_timeout_seconds: 0 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState().error).toBe('NETWORK_ERROR')

    for (const [index, delay] of [2_000, 4_000, 8_000, 16_000, 30_000, 30_000].entries()) {
      await vi.advanceTimersByTimeAsync(delay - 1)
      expect(fetchMock).toHaveBeenCalledTimes(index + 2)
      await vi.advanceTimersByTimeAsync(1)
      expect(fetchMock).toHaveBeenCalledTimes(index + 3)
    }
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBe(ticket)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', error: null })
    fetchMock.mockRejectedValueOnce(new TypeError('Network unavailable'))
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(9)
    await vi.advanceTimersByTimeAsync(1_999)
    expect(fetchMock).toHaveBeenCalledTimes(9)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(10)
  })

  it('keeps 429 rate limits retryable without losing the ticket', async () => {
    fetchMock.mockResolvedValueOnce(failure('RATE_LIMITED', 429))
    const runtime = boot()
    await settle()
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBe(ticket)
    await vi.advanceTimersByTimeAsync(2_000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(runtime.sdk.getState().error).toBeNull()
  })

  it('rejects malformed success data without trusting or displaying arbitrary response content', async () => {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ code: 0, data: { progress_seconds: '<img src=x>' } })))
    const runtime = boot()
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'error', error: 'INVALID_RESPONSE', progress_seconds: 0 })
    expect(runtime.document.querySelector('img')).toBeNull()
    await vi.advanceTimersByTimeAsync(2_000)
    expect(runtime.sdk.getState().status).toBe('in_progress')
  })

  it('keeps public state immutable and isolates callback failures, mutation, and unsubscription', async () => {
    const runtime = boot()
    runtime.sdk.onChange(() => { throw new Error('Host callback failed') })
    runtime.sdk.onChange((state) => { state.progress_seconds = 999 })
    const listener = vi.fn()
    const unsubscribe = runtime.sdk.onChange(listener)
    await settle()

    expect(listener).toHaveBeenLastCalledWith(expect.objectContaining({ progress_seconds: 5 }))
    const snapshot = runtime.sdk.getState()
    snapshot.progress_seconds = 999
    expect(runtime.sdk.getState().progress_seconds).toBe(5)
    unsubscribe()
    fetchMock.mockResolvedValueOnce(success({ progress_seconds: 10 }))
    await vi.advanceTimersByTimeAsync(1_000)
    expect(listener).toHaveBeenCalledTimes(1)
    expect(runtime.sdk.getState().progress_seconds).toBe(10)
  })

  it('uses inline styles and allows closing the badge without stopping heartbeats', async () => {
    const runtime = boot()
    await settle()
    const badge = runtime.document.querySelector<HTMLElement>('[data-reward-task-badge]')!
    expect(badge.style.position).toBe('fixed')
    expect(badge.style.zIndex).toBe('2147483000')
    expect(badge.textContent).toContain('5 / 30')
    expect(runtime.document.querySelector('link[rel=stylesheet]')).toBeNull()
    badge.querySelector('button')!.click()
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
  })

  it('honors data-badge=false', async () => {
    const runtime = boot({ badge: 'false' })
    await settle()
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it.each([
    { lang: 'en', navigatorLanguage: 'zh-CN' },
    { navigatorLanguage: 'en-US' }
  ])('uses explicit or navigator English locale: %j', async (options) => {
    const runtime = boot(options)
    await settle()
    expect(runtime.document.body.textContent).toContain('Browse task in progress')
    expect(runtime.document.querySelector('button')?.getAttribute('aria-label')).toBe('Close task progress')
  })

  it('continues with a URL ticket when sessionStorage is blocked', async () => {
    const runtime = boot({ blockedStorage: true })
    await settle()
    expect(runtime.window.location.search).toBe('')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    fetchMock.mockResolvedValueOnce(success({ status: 'completed', completed: true, progress_seconds: 30 }))
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState().status).toBe('completed')
  })

  it('can load in the document head before the body exists', async () => {
    const runtime = boot({ noBody: true })
    await settle()
    runtime.document.documentElement.appendChild(runtime.document.createElement('body'))
    runtime.document.dispatchEvent(new runtime.window.Event('DOMContentLoaded'))
    expect(runtime.document.querySelector('[data-reward-task-badge]')?.textContent).toContain('5 / 30')
  })

  it('does not create duplicate SDK instances when the script is included twice', async () => {
    const runtime = boot()
    const original = runtime.sdk
    runtime.window.eval(readFileSync(sdkSourcePath, 'utf8'))
    await settle()
    expect(runtime.window.RewardTaskSDK).toBe(original)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(runtime.document.querySelectorAll('[data-reward-task-badge]')).toHaveLength(1)
  })

  it('sends on planned one-second boundaries without adding a 350ms response RTT', async () => {
    const startedAt = Date.now()
    const requestTimes: number[] = []
    fetchMock.mockImplementation(() => {
      requestTimes.push(Date.now() - startedAt)
      return new Promise<Response>((resolve) => window.setTimeout(() => resolve(success({
        progress_seconds: requestTimes.length - 1
      })), 350))
    })
    const runtime = boot()
    const listener = vi.fn()
    runtime.sdk.onChange(listener)
    await vi.advanceTimersByTimeAsync(3_500)

    expect(requestTimes).toEqual([0, 1_000, 2_000, 3_000])
    expect(runtime.sdk.getState().progress_seconds).toBe(3)
    expect(listener.mock.calls.map(([state]) => state.progress_seconds)).toEqual([0, 1, 2, 3])
    expect(runtime.document.querySelector('[data-reward-task-badge]')?.textContent).toContain('3 / 30')
  })

  it('skips in-flight beats without overlap or an immediate catch-up request when the response arrives', async () => {
    const startedAt = Date.now()
    const requestTimes: number[] = []
    fetchMock.mockImplementation(() => {
      requestTimes.push(Date.now() - startedAt)
      return new Promise<Response>((resolve) => window.setTimeout(() => resolve(success()), 1_400))
    })
    boot()
    await vi.advanceTimersByTimeAsync(4_500)
    expect(requestTimes).toEqual([0, 2_000, 4_000])
  })

  it('registers every real activity signal in document capture phase with passive listeners', () => {
    const runtime = boot()
    for (const event of activityEvents) {
      expect(runtime.addListener).toHaveBeenCalledWith(event, expect.any(Function), { capture: true, passive: true })
    }
  })

  it('defaults to 15 seconds before the first response and cannot lose an idle pause to a late response', async () => {
    let resolveRequest!: (response: Response) => void
    fetchMock.mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveRequest = resolve }))
    const runtime = boot()
    const listener = vi.fn()
    runtime.sdk.onChange(listener)
    await vi.advanceTimersByTimeAsync(15_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    expect(listener).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'paused', paused_reason: 'idle' }))

    resolveRequest(success({ progress_seconds: 12 }))
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle', progress_seconds: 12 })
    await vi.advanceTimersByTimeAsync(4_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('uses the server idle threshold and pauses only after the exact threshold has elapsed', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(5_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    expect(fetchMock).toHaveBeenCalledTimes(6)
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    await vi.advanceTimersByTimeAsync(10_000)
    expect(fetchMock).toHaveBeenCalledTimes(6)
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBe(ticket)
  })

  it.each(activityEvents)('immediately restores state after trusted %s and sends on the next planned beat', async (event) => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(6_350)
    expect(runtime.sdk.getState().paused_reason).toBe('idle')
    runtime.activity(event)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null, error: null })
    expect(fetchMock).toHaveBeenCalledTimes(6)
    await vi.advanceTimersByTimeAsync(649)
    expect(fetchMock).toHaveBeenCalledTimes(6)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(7)
  })

  it('does not count untrusted callbacks or synthetic DOM events as activity', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(4_000)
    for (const event of activityEvents) {
      runtime.activity(event, false)
      runtime.document.dispatchEvent(new runtime.window.Event(event))
    }
    await vi.advanceTimersByTimeAsync(2_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    for (const event of activityEvents) runtime.activity(event, false)
    expect(runtime.sdk.getState().status).toBe('paused')
    await vi.advanceTimersByTimeAsync(2_000)
    expect(fetchMock).toHaveBeenCalledTimes(6)
  })

  it('keeps browsing active beyond the idle threshold when trusted activity continues', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    for (let index = 0; index < 5; index += 1) {
      await vi.advanceTimersByTimeAsync(4_000)
      runtime.activity('scroll')
      expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    }
    expect(fetchMock).toHaveBeenCalledTimes(21)
  })

  it('disables idle detection for server threshold zero, but never bypasses visibility pauses', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 0 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).toHaveBeenCalledTimes(61)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    runtime.setHidden(true)
    runtime.activity()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'hidden' })
    expect(fetchMock).toHaveBeenCalledTimes(61)
  })

  it('treats a hidden-to-visible transition as fresh activity even after a long idle pause', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(6_100)
    runtime.setHidden(true)
    await vi.advanceTimersByTimeAsync(20_000)
    expect(runtime.sdk.getState().paused_reason).toBe('hidden')
    runtime.setHidden(false)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    await vi.advanceTimersByTimeAsync(900)
    expect(fetchMock).toHaveBeenCalledTimes(7)
  })

  it.each(['idle', 'hidden'] as const)('preserves a %s pause when an in-flight success response arrives', async (reason) => {
    let resolveRequest!: (response: Response) => void
    fetchMock
      .mockResolvedValueOnce(success({ idle_timeout_seconds: 5 }))
      .mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveRequest = resolve }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(1_000)
    if (reason === 'hidden') runtime.setHidden(true)
    await vi.advanceTimersByTimeAsync(5_100)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: reason })
    resolveRequest(success({ idle_timeout_seconds: 5, progress_seconds: 12 }))
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: reason, progress_seconds: 12 })
    await vi.advanceTimersByTimeAsync(2_000)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('does not let a late network rejection replace the idle warning with an error state', async () => {
    let rejectRequest!: (reason: Error) => void
    fetchMock.mockImplementationOnce(() => new Promise<Response>((_resolve, reject) => { rejectRequest = reject }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(16_100)
    rejectRequest(new TypeError('Network unavailable'))
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    expect(runtime.document.body.textContent).toContain('检测到你没有在浏览，计时已暂停')
    await vi.advanceTimersByTimeAsync(3_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('checks idleness on every beat even while network retries are in a long backoff', async () => {
    fetchMock.mockRejectedValue(new TypeError('Network unavailable'))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(15_000)
    expect(fetchMock).toHaveBeenCalledTimes(4)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'error', paused_reason: null })
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    await vi.advanceTimersByTimeAsync(4_350)
    runtime.activity('keydown')
    expect(runtime.sdk.getState()).toMatchObject({ status: 'in_progress', paused_reason: null })
    await vi.advanceTimersByTimeAsync(3_650)
    runtime.activity('mousemove')
    await vi.advanceTimersByTimeAsync(4_000)
    runtime.activity('mousemove')
    await vi.advanceTimersByTimeAsync(1_999)
    expect(fetchMock).toHaveBeenCalledTimes(4)
    await vi.advanceTimersByTimeAsync(1)
    expect(fetchMock).toHaveBeenCalledTimes(5)
  })

  it('uses the latest server threshold without resetting the last real activity time', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 0 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(9_000)
    fetchMock.mockResolvedValueOnce(success({ idle_timeout_seconds: 5 }))
    await vi.advanceTimersByTimeAsync(1_000)
    expect(runtime.sdk.getState()).toMatchObject({ status: 'paused', paused_reason: 'idle' })
    expect(fetchMock).toHaveBeenCalledTimes(11)
    await vi.advanceTimersByTimeAsync(1_000)
    expect(fetchMock).toHaveBeenCalledTimes(11)
  })

  it('reopens a dismissed badge on each new idle pause with the exact amber warning and restores normal styling on activity', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5, progress_seconds: 12 }))
    const runtime = boot()
    await settle()
    const original = runtime.document.querySelector<HTMLElement>('[data-reward-task-badge]')!
    const normalBackground = original.style.backgroundColor
    original.querySelector('button')!.click()
    await vi.advanceTimersByTimeAsync(6_000)

    const badge = runtime.document.querySelector<HTMLElement>('[data-reward-task-badge]')!
    expect(badge.textContent).toContain('检测到你没有在浏览，计时已暂停（12 / 30 秒）。滚动页面或移动鼠标即可继续。')
    expect(badge.style.backgroundColor).toBe('rgb(255, 251, 235)')
    expect(badge.style.color).toBe('rgb(146, 64, 14)')
    expect(badge.style.borderColor).toBe('rgb(245, 158, 11)')
    runtime.activity()
    expect(badge.textContent).toContain('浏览任务进行中 12 / 30 秒')
    expect(badge.style.backgroundColor).toBe(normalBackground)
    badge.querySelector('button')!.click()
    await vi.advanceTimersByTimeAsync(6_000)
    expect(runtime.document.querySelector('[data-reward-task-badge]')?.textContent).toContain('检测到你没有在浏览')
  })

  it('does not reopen a badge dismissed during the same idle episode until a new pause occurs', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(6_000)
    runtime.document.querySelector('[data-reward-task-badge] button')!.dispatchEvent(new runtime.window.MouseEvent('click'))
    await vi.advanceTimersByTimeAsync(5_000)
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
  })

  it('never renders an idle warning when data-badge=false but still publishes paused state changes', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot({ badge: 'false' })
    const listener = vi.fn()
    runtime.sdk.onChange(listener)
    await vi.advanceTimersByTimeAsync(6_000)
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
    expect(listener).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'paused', paused_reason: 'idle' }))
    runtime.activity('wheel')
    expect(listener).toHaveBeenLastCalledWith(expect.objectContaining({ status: 'in_progress', paused_reason: null }))
    expect(runtime.document.querySelector('[data-reward-task-badge]')).toBeNull()
  })

  it('uses an English idle warning for the English badge', async () => {
    fetchMock.mockImplementation(async () => success({ idle_timeout_seconds: 5 }))
    const runtime = boot({ lang: 'en' })
    await vi.advanceTimersByTimeAsync(6_000)
    expect(runtime.document.body.textContent).toContain('No browsing activity detected. Timing paused (5 / 30 seconds). Scroll or move your mouse to continue.')
  })

  it.each(['completed', 'terminal_error'])('removes activity listeners and the scheduler after %s even when the final response arrives paused', async (result) => {
    let resolveRequest!: (response: Response) => void
    fetchMock.mockImplementationOnce(() => new Promise<Response>((resolve) => { resolveRequest = resolve }))
    const runtime = boot()
    await vi.advanceTimersByTimeAsync(16_000)
    resolveRequest(result === 'completed'
      ? success({ status: 'completed', completed: true, progress_seconds: 30 })
      : failure('REWARD_TICKET_EXPIRED'))
    await settle()
    expect(runtime.sdk.getState()).toMatchObject({ status: result === 'completed' ? 'completed' : 'error', paused_reason: null })
    expect(runtime.window.sessionStorage.getItem(ticketStorageKey)).toBeNull()
    for (const event of activityEvents) {
      expect(runtime.removeListener).toHaveBeenCalledWith(event, expect.any(Function), true)
    }
    runtime.activity()
    runtime.setHidden(true)
    runtime.setHidden(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
