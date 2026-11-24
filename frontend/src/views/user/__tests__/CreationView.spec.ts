import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import CreationView from '../CreationView.vue'
import { creationAPI } from '@/features/creation/api'
import { submitImage, submitVideo, waitForTask } from '@/features/creation/gateway'
import { asyncTasksAPI, type AsyncTask } from '@/api/asyncTasks'
import { keysAPI } from '@/api/keys'
import type { CreationCatalog, CreationTaskHandle, CreationTaskSnapshot, WaitForTaskOptions } from '@/features/creation/types'
import type { CreationSubmission } from '../creation/types'

const appStore = vi.hoisted(() => ({
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))
const authState = vi.hoisted(() => ({
  user: { id: 7, balance: 9.92, enterprise: null as { balance_visible: boolean; balance: number } | null },
  refreshUser: vi.fn(),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('../creation/CreationComposer.vue', () => ({
  default: {
    name: 'CreationComposer',
    props: ['mode', 'groups', 'apiKeys', 'userId', 'loadingKeys', 'creatingKey', 'keyError', 'atCapacity', 'maxActive', 'balance', 'enterpriseBalance'],
    emits: ['submit', 'createKey', 'reloadKeys'],
    template: '<div data-testid="composer" />',
  },
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(authState) }))
vi.mock('@/utils/featureFlags', () => ({ FeatureFlags: { creation: 'creation' }, isFeatureFlagEnabled: () => true }))
vi.mock('@/features/creation/api', () => ({ creationAPI: { getCatalog: vi.fn(), getCosts: vi.fn() } }))
vi.mock('@/features/creation/gateway', () => ({
  CreationGatewayError: class extends Error {},
  revokeAssets: vi.fn(),
  submitImage: vi.fn(),
  submitVideo: vi.fn(),
  waitForTask: vi.fn(),
}))
vi.mock('@/api/keys', () => ({ keysAPI: { list: vi.fn(), create: vi.fn() } }))
vi.mock('@/api/asyncTasks', () => ({ asyncTasksAPI: { list: vi.fn(), refresh: vi.fn(), download: vi.fn() } }))

const catalog: CreationCatalog = {
  groups: [{
    id: 1, name: 'Studio', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: 1,
    image: { protocols: ['openai'], models: [{ id: 'gpt-image-1', protocols: [], pricing: null }], async_supported: true, edit_supported: true },
    video: { protocols: ['seedance'], models: [{ id: 'seedance-1', protocols: [], pricing: null }], resolutions: [], min_duration: null, max_duration: null },
  }],
}

interface PendingWait {
  handle: CreationTaskHandle
  options: WaitForTaskOptions
  resolve: (snapshot: CreationTaskSnapshot) => void
}

let waits: PendingWait[] = []
let wrapper: VueWrapper | null = null

function videoSubmission(prompt: string): CreationSubmission {
  return {
    kind: 'video', groupId: 1, groupName: 'Studio', apiKey: 'sk-test',
    input: { apiKey: 'sk-test', protocol: 'seedance', model: 'seedance-1', prompt },
  }
}

function imageSubmission(prompt: string, preferAsync: boolean): CreationSubmission {
  return {
    kind: 'image', groupId: 1, groupName: 'Studio', apiKey: 'sk-test',
    input: { apiKey: 'sk-test', protocol: 'openai', platform: 'openai', model: 'gpt-image-1', prompt, preferAsync },
  }
}

async function mountView(): Promise<VueWrapper> {
  wrapper = mount(CreationView, { global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  return wrapper
}

function composer(view: VueWrapper) {
  return view.findComponent({ name: 'CreationComposer' })
}

async function showVideoMode(view: VueWrapper): Promise<void> {
  await view.get('#creation-tab-video').trigger('click')
  await flushPromises()
}

describe('CreationView async tasks', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    localStorage.clear()
    reactive(authState).user.id = 7
    reactive(authState).user.balance = 9.92
    reactive(authState).user.enterprise = null
    authState.refreshUser.mockReset().mockResolvedValue(undefined)
    waits = []
    vi.mocked(creationAPI.getCatalog).mockResolvedValue(catalog)
    vi.mocked(creationAPI.getCosts).mockResolvedValue({ items: [] })
    vi.mocked(keysAPI.list).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    let sequence = 0
    vi.mocked(submitVideo).mockImplementation(async input => ({
      handle: { kind: 'video', protocol: input.protocol, taskId: `task-${++sequence}`, model: input.model },
      snapshot: { status: 'processing', assets: [] },
    }))
    vi.mocked(waitForTask).mockImplementation((_apiKey, handle, options = {}) => new Promise((resolve, reject) => {
      waits.push({ handle, options, resolve })
      options.signal?.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), { code: 'ABORTED' })), { once: true })
    }))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    vi.useRealTimers()
  })

  it('returns control right after a task is accepted so several tasks run in the background', async () => {
    const view = await mountView()
    await showVideoMode(view)

    composer(view).vm.$emit('submit', videoSubmission('first'))
    await flushPromises()
    expect(appStore.showSuccess).toHaveBeenCalledWith('creation.messages.taskSubmitted')
    expect(composer(view).props('atCapacity')).toBe(false)

    composer(view).vm.$emit('submit', videoSubmission('second'))
    await flushPromises()

    expect(submitVideo).toHaveBeenCalledTimes(2)
    expect(waits.map(wait => wait.handle.taskId)).toEqual(['task-1', 'task-2'])
    const cards = view.findAll('[data-testid="creation-task"]')
    expect(cards).toHaveLength(2)
    expect(cards[0]?.text()).toContain('second')
    expect(cards[1]?.text()).toContain('first')
    expect(view.get('[data-testid="creation-tab-video-active"]').text()).toBe('2')

    waits[0]?.resolve({ status: 'succeeded', assets: [{ kind: 'video', url: 'https://cdn.example/first.mp4' }] })
    await flushPromises()
    expect(appStore.showSuccess).toHaveBeenCalledWith('creation.messages.taskSucceeded')
    expect(view.findAll('[data-testid="creation-task"]')[1]?.find('video').attributes('src')).toBe('https://cdn.example/first.mp4')
    expect(view.get('[data-testid="creation-tab-video-active"]').text()).toBe('1')
  })

  it('stops tracking one task without touching the others', async () => {
    const view = await mountView()
    await showVideoMode(view)
    composer(view).vm.$emit('submit', videoSubmission('first'))
    composer(view).vm.$emit('submit', videoSubmission('second'))
    await flushPromises()

    const firstCard = view.findAll('[data-testid="creation-task"]')[1]!
    await firstCard.findAll('button').find(button => button.text() === 'creation.result.stopTracking')!.trigger('click')
    await flushPromises()

    expect(waits[0]?.options.signal?.aborted).toBe(true)
    expect(waits[1]?.options.signal?.aborted).toBe(false)
    const cards = view.findAll('[data-testid="creation-task"]')
    expect(cards[1]?.text()).toContain('creation.result.status.cancelled')
    expect(cards[1]?.text()).toContain('creation.result.cancelledNotice')
    expect(cards[0]?.text()).toContain('creation.result.status.processing')
  })

  it('caps the number of tasks tracked at the same time', async () => {
    const view = await mountView()
    await showVideoMode(view)
    for (let index = 0; index < 5; index++) composer(view).vm.$emit('submit', videoSubmission(`task ${index}`))
    await flushPromises()

    expect(composer(view).props('atCapacity')).toBe(true)
    composer(view).vm.$emit('submit', videoSubmission('one too many'))
    await flushPromises()
    expect(submitVideo).toHaveBeenCalledTimes(5)
    expect(appStore.showError).toHaveBeenCalledWith('creation.messages.tooManyTasks')

    waits[0]?.resolve({ status: 'failed', assets: [], error: 'upstream rejected' })
    await flushPromises()
    expect(composer(view).props('atCapacity')).toBe(false)
    expect(appStore.showError).toHaveBeenCalledWith('creation.messages.taskFailed')
  })

  it('submits async-capable images as tasks and keeps sync-only images non-blocking', async () => {
    vi.mocked(submitImage).mockImplementation(async input => input.preferAsync
      ? { handle: { kind: 'image', protocol: 'openai', taskId: 'imgtask_1', model: input.model }, snapshot: { status: 'processing', assets: [] } }
      : new Promise(() => {}))
    const view = await mountView()

    composer(view).vm.$emit('submit', imageSubmission('async image', true))
    composer(view).vm.$emit('submit', imageSubmission('sync image', false))
    await flushPromises()

    expect(waits.map(wait => wait.handle.taskId)).toEqual(['imgtask_1'])
    const cards = view.findAll('[data-testid="creation-task"]')
    expect(cards[0]?.text()).toContain('creation.result.syncRunning')
    expect(cards[0]?.text()).toContain('creation.result.syncNotice')
    expect(cards[1]?.text()).toContain('creation.result.indeterminate')
    expect(cards[1]?.text()).not.toContain('creation.result.syncNotice')
    expect(composer(view).props('atCapacity')).toBe(false)
  })

  describe('prompts of async tasks that were not followed to the end', () => {
    function asyncImageTask(status: AsyncTask['status']): AsyncTask {
      return {
        id: 'imgtask_1', task_id: 'imgtask_1', kind: 'image', protocol: 'openai', model: 'gpt-image-1',
        status, api_key_id: 9, created_at: 100, expires_at: 200,
        ...(status === 'completed' ? { result_url: 'https://cdn.example/edited.png' } : { error: 'upstream rejected' }),
      }
    }

    async function submitAsyncImage(view: VueWrapper): Promise<void> {
      vi.mocked(submitImage).mockResolvedValue({
        handle: { kind: 'image', protocol: 'openai', taskId: 'imgtask_1', model: 'gpt-image-1' },
        snapshot: { status: 'processing', assets: [] },
      })
      composer(view).vm.$emit('submit', imageSubmission('edit the sky', true))
      await flushPromises()
    }

    async function remountWith(task: AsyncTask): Promise<VueWrapper> {
      wrapper?.unmount()
      wrapper = null
      vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [task], total: 1, page: 1, page_size: 20 })
      return mountView()
    }

    it('keeps the prompt when the page is reloaded before the task finishes', async () => {
      const view = await mountView()
      await submitAsyncImage(view)
      const saved = JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')
      expect(saved).toHaveLength(1)
      expect(saved[0]).toMatchObject({ taskId: 'imgtask_1', prompt: 'edit the sky', pending: true, assets: [] })
      expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(0)

      const restored = await remountWith(asyncImageTask('completed'))
      const cards = restored.findAll('[data-testid="creation-history-item"]')
      expect(cards).toHaveLength(1)
      expect(cards[0]?.get('.cs-task-prompt').text()).toBe('edit the sky')
      expect(cards[0]?.get('img').attributes('src')).toBe('https://cdn.example/edited.png')
    })

    it('keeps the prompt of a failed task', async () => {
      const view = await mountView()
      await submitAsyncImage(view)
      waits[0]?.resolve({ status: 'failed', assets: [], error: 'upstream rejected' })
      await flushPromises()
      expect(JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')[0]).toMatchObject({ pending: true })

      const restored = await remountWith(asyncImageTask('failed'))
      const card = restored.get('[data-testid="creation-history-item"]')
      expect(card.get('.cs-task-prompt').text()).toBe('edit the sky')
      expect(card.text()).toContain('upstream rejected')
    })

    it('replaces the pending record with the result once the task succeeds', async () => {
      const view = await mountView()
      await submitAsyncImage(view)
      waits[0]?.resolve({ status: 'succeeded', assets: [{ kind: 'image', url: 'https://cdn.example/edited.png' }] })
      await flushPromises()

      const saved = JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')
      expect(saved).toHaveLength(1)
      expect(saved[0]).toMatchObject({ taskId: 'imgtask_1', assets: [{ kind: 'image', url: 'https://cdn.example/edited.png' }] })
      expect(saved[0]).not.toHaveProperty('pending')
    })

    it('does not show a pending record on its own when the server no longer lists the task', async () => {
      const view = await mountView()
      await submitAsyncImage(view)
      wrapper?.unmount()
      wrapper = null

      const restored = await mountView()
      expect(restored.findAll('[data-testid="creation-history-item"]')).toHaveLength(0)
    })
  })

  it('queries synchronous image charges and saves the amount with the original request ID', async () => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'client-original',
      snapshot: { status: 'succeeded', assets: [{ kind: 'image', url: 'https://cdn.example/sync.png' }] },
    })
    vi.mocked(creationAPI.getCosts).mockResolvedValue({ items: [{ request_id: 'client-original', actual_cost: 0.04 }] })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('sync image', false))
    await flushPromises()

    expect(creationAPI.getCosts).toHaveBeenCalledWith(['client-original'], expect.any(AbortSignal))
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual $0.04')
    const saved = JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')
    expect(saved[0]).toMatchObject({ clientRequestId: 'client-original', actualCost: 0.04 })

    view.unmount()
    wrapper = null
    const restored = await mountView()
    expect(restored.get('[data-testid="creation-history-cost"]').text()).toBe('creation.cost.actual $0.04')
  })

  it.each([false, true])('refreshes the shared balance after a synchronous charge (enterprise: %s)', async enterprise => {
    if (enterprise) reactive(authState).user.enterprise = { balance_visible: true, balance: 9.92 }
    authState.refreshUser.mockImplementation(async () => {
      if (enterprise) reactive(authState).user.enterprise!.balance = 9.84
      else reactive(authState).user.balance = 9.84
    })
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'charged-request', snapshot: { status: 'succeeded', assets: [] },
    })
    vi.mocked(creationAPI.getCosts).mockResolvedValue({ items: [{ request_id: 'charged-request', actual_cost: 0.08 }] })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('charged', false))
    await flushPromises()
    expect(composer(view).props('balance')).toBe(9.92)
    expect(composer(view).props('enterpriseBalance')).toBe(enterprise)
    await vi.advanceTimersByTimeAsync(999)
    expect(authState.refreshUser).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
    expect(authState.refreshUser).toHaveBeenCalledWith({ fresh: true })
    expect(composer(view).props('balance')).toBe(9.84)
  })

  it('debounces several settled charges into one balance refresh', async () => {
    let request = 0
    vi.mocked(submitImage).mockImplementation(async () => ({
      handle: null, clientRequestId: `charged-${++request}`, snapshot: { status: 'succeeded', assets: [] },
    }))
    vi.mocked(creationAPI.getCosts).mockImplementation(async ids => ({ items: [{ request_id: ids[0]!, actual_cost: 0.08 }] }))
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('first', false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(500)
    composer(view).vm.$emit('submit', imageSubmission('second', false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(999)
    expect(authState.refreshUser).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
  })

  it.each(['unmount', 'account change'])('cancels a scheduled balance refresh on %s', async action => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'charged-request', snapshot: { status: 'succeeded', assets: [] },
    })
    vi.mocked(creationAPI.getCosts).mockResolvedValue({ items: [{ request_id: 'charged-request', actual_cost: 0.08 }] })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('charged', false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(500)
    if (action === 'unmount') {
      view.unmount()
      wrapper = null
    } else {
      reactive(authState).user.id = 8
      await flushPromises()
    }
    await vi.advanceTimersByTimeAsync(30_000)
    expect(authState.refreshUser).not.toHaveBeenCalled()
  })

  it('silently preserves the generated result if the balance refresh fails', async () => {
    authState.refreshUser.mockRejectedValue(new Error('temporary network error'))
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'charged-request', snapshot: { status: 'succeeded', assets: [] },
    })
    vi.mocked(creationAPI.getCosts).mockResolvedValue({ items: [{ request_id: 'charged-request', actual_cost: 0.08 }] })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('charged', false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
    expect(appStore.showError).not.toHaveBeenCalled()
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual $0.08')
  })

  it('shows settling while synchronous charges are delayed and retries only a finite number of times', async () => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'delayed-client', snapshot: { status: 'succeeded', assets: [] },
    })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('delayed charge', false))
    await flushPromises()
    expect(view.get('[data-testid="creation-task-cost"]').text()).toContain('creation.cost.settling')
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(2000)
    await vi.advanceTimersByTimeAsync(5000)
    await vi.advanceTimersByTimeAsync(10_000)
    await flushPromises()
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(4)
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual —')
    await vi.advanceTimersByTimeAsync(60_000)
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(4)
  })

  it('retries missing synchronous charges and preserves a free result', async () => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'free-client', snapshot: { status: 'succeeded', assets: [] },
    })
    vi.mocked(creationAPI.getCosts).mockResolvedValueOnce({ items: [] })
      .mockResolvedValue({ items: [{ request_id: 'free-client', actual_cost: 0 }] })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('free', false))
    await flushPromises()
    await vi.advanceTimersByTimeAsync(2000)
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual $0.00')
    await vi.advanceTimersByTimeAsync(30_000)
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(2)
  })

  it('aborts delayed charge lookups when leaving the page', async () => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'pending-client', snapshot: { status: 'succeeded', assets: [] },
    })
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('pending charge', false))
    await flushPromises()
    const signal = vi.mocked(creationAPI.getCosts).mock.calls[0]?.[1]
    view.unmount()
    wrapper = null
    expect(signal?.aborted).toBe(true)
    await vi.advanceTimersByTimeAsync(30_000)
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(1)
  })

  it('resumes unsettled local image charges using the persisted client request ID', async () => {
    localStorage.setItem('sub2api.creation.history.7', JSON.stringify([{
      id: 'saved', kind: 'image', protocol: 'openai', model: 'gpt-image-1', prompt: 'saved image',
      createdAt: 100, assets: [], clientRequestId: 'saved-request',
    }]))
    vi.mocked(creationAPI.getCosts).mockResolvedValueOnce({ items: [] })
      .mockResolvedValue({ items: [{ request_id: 'saved-request', actual_cost: 0.035 }] })
    const view = await mountView()
    expect(view.get('[data-testid="creation-history-cost"]').text()).toContain('creation.cost.settling')
    await vi.advanceTimersByTimeAsync(2000)
    expect(view.get('[data-testid="creation-history-cost"]').text()).toBe('creation.cost.actual $0.035')
    expect(JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')[0]).toMatchObject({ actualCost: 0.035 })
  })

  it('aborts charge requests on account changes and ignores a late response for the previous user', async () => {
    vi.mocked(submitImage).mockResolvedValue({
      handle: null, clientRequestId: 'previous-user', snapshot: { status: 'succeeded', assets: [] },
    })
    let resolveCosts!: (costs: { items: Array<{ request_id: string; actual_cost: number }> }) => void
    vi.mocked(creationAPI.getCosts).mockImplementation(() => new Promise(resolve => { resolveCosts = resolve }))
    const view = await mountView()
    composer(view).vm.$emit('submit', imageSubmission('previous user', false))
    await flushPromises()
    const signal = vi.mocked(creationAPI.getCosts).mock.calls[0]?.[1]
    reactive(authState).user.id = 8
    await flushPromises()
    expect(signal?.aborted).toBe(true)
    resolveCosts({ items: [{ request_id: 'previous-user', actual_cost: 1 }] })
    await flushPromises()
    expect(view.find('[data-testid="creation-task"]').exists()).toBe(false)
    expect(localStorage.getItem('sub2api.creation.history.8')).toBeNull()
    expect(JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')[0]).not.toHaveProperty('actualCost')
    await vi.advanceTimersByTimeAsync(30_000)
    expect(creationAPI.getCosts).toHaveBeenCalledTimes(1)
  })

  it('uses server history charges for finished async tasks and persists them', async () => {
    const view = await mountView()
    composer(view).vm.$emit('submit', videoSubmission('async charged'))
    await flushPromises()
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({
      items: [{
        id: 'vidtask_1', task_id: 'task-1', kind: 'video', protocol: 'seedance', model: 'seedance-1',
        status: 'completed', api_key_id: 9, created_at: 100, expires_at: 200, actual_cost: 0.48,
      }], total: 1, page: 1, page_size: 20,
    })
    waits[0]?.resolve({ status: 'succeeded', assets: [{ kind: 'video', url: 'https://cdn.example/video.mp4' }] })
    await flushPromises()
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual $0.48')
    expect(creationAPI.getCosts).not.toHaveBeenCalled()
    const saved = JSON.parse(localStorage.getItem('sub2api.creation.history.7') || '[]')
    expect(saved[0]).toMatchObject({ taskId: 'task-1', actualCost: 0.48 })
    expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(0)
    await vi.advanceTimersByTimeAsync(1000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
  })

  it('rechecks charges for just-finished async tasks even if the history filter changes', async () => {
    const view = await mountView()
    composer(view).vm.$emit('submit', videoSubmission('async settling'))
    await flushPromises()
    waits[0]?.resolve({ status: 'succeeded', assets: [] })
    await flushPromises()
    expect(view.get('[data-testid="creation-task-cost"]').text()).toContain('creation.cost.settling')
    await view.get('[data-testid="creation-filter-image"]').trigger('click')
    await flushPromises()
    vi.mocked(asyncTasksAPI.list).mockImplementation(async params => ({
      items: params.search ? [{
        id: 'vidtask_1', task_id: 'task-1', kind: 'video', protocol: 'seedance', status: 'completed',
        api_key_id: 9, created_at: 100, expires_at: 200, actual_cost: 0.08,
      }] : [], total: params.search ? 1 : 0, page: 1, page_size: 20,
    }))
    await vi.advanceTimersByTimeAsync(2000)
    expect(asyncTasksAPI.list).toHaveBeenCalledWith(expect.objectContaining({ kind: 'video', search: 'task-1' }), expect.any(AbortSignal))
    await view.get('[data-testid="creation-filter-all"]').trigger('click')
    await flushPromises()
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual $0.08')
    vi.mocked(asyncTasksAPI.list).mockClear()
    await vi.advanceTimersByTimeAsync(30_000)
    expect(asyncTasksAPI.list).not.toHaveBeenCalled()
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
  })

  it('stops checking an async task after the three settlement retries are exhausted', async () => {
    const view = await mountView()
    composer(view).vm.$emit('submit', videoSubmission('missing async charge'))
    await flushPromises()
    waits[0]?.resolve({ status: 'succeeded', assets: [] })
    await flushPromises()
    vi.mocked(asyncTasksAPI.list).mockClear()
    await vi.advanceTimersByTimeAsync(17_000)
    await flushPromises()
    expect(asyncTasksAPI.list).toHaveBeenCalledTimes(3)
    expect(view.get('[data-testid="creation-task-cost"]').text()).toBe('creation.cost.actual —')
    await vi.advanceTimersByTimeAsync(60_000)
    expect(asyncTasksAPI.list).toHaveBeenCalledTimes(3)
  })

  it('settles an untracked history task after it transitions to completed', async () => {
    const task: AsyncTask = {
      id: 'older-video', task_id: 'older-upstream', kind: 'video', protocol: 'seedance',
      status: 'processing', api_key_id: 9, created_at: 100, expires_at: 200,
    }
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [task], total: 1, page: 1, page_size: 20 })
    vi.mocked(asyncTasksAPI.refresh).mockResolvedValue({ ...task, status: 'completed' })
    const view = await mountView()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(view.get('[data-testid="creation-history-cost"]').text()).toContain('creation.cost.settling')
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [{ ...task, status: 'completed', actual_cost: 0.6 }], total: 1, page: 1, page_size: 20 })
    await vi.advanceTimersByTimeAsync(2000)
    expect(view.get('[data-testid="creation-history-cost"]').text()).toBe('creation.cost.actual $0.60')
    await vi.advanceTimersByTimeAsync(1000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
  })

  it.each(['list', 'refresh'])('refreshes balance when a history task completes with a charge via %s', async source => {
    const task: AsyncTask = {
      id: 'older-video', task_id: 'older-upstream', kind: 'video', protocol: 'seedance',
      status: 'processing', api_key_id: 9, created_at: 100, expires_at: 200,
    }
    const completed: AsyncTask = { ...task, status: 'completed', actual_cost: 0.08 }
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [task], total: 1, page: 1, page_size: 20 })
    vi.mocked(asyncTasksAPI.refresh).mockResolvedValue(completed)
    const view = await mountView()
    if (source === 'list') {
      vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [completed], total: 1, page: 1, page_size: 20 })
    }
    await vi.advanceTimersByTimeAsync(10_000)
    expect(view.get('[data-testid="creation-history-cost"]').text()).toBe('creation.cost.actual $0.08')
    expect(authState.refreshUser).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(1000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
    await view.findAll('button').find(button => button.text() === 'creation.history.refresh')!.trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(1000)
    expect(authState.refreshUser).toHaveBeenCalledTimes(1)
  })

  it('auto-refreshes processing history and only asks the server to poll untracked videos', async () => {
    const history = (taskId: string): AsyncTask => ({
      id: `vidtask_${taskId}`, task_id: taskId, kind: 'video', protocol: 'seedance', model: 'seedance-1',
      status: 'processing', api_key_id: 9, created_at: 100, expires_at: 200,
    })
    vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items: [history('task-1'), history('older')], total: 2, page: 1, page_size: 20 })
    vi.mocked(asyncTasksAPI.refresh).mockResolvedValue({ ...history('older'), status: 'completed' })
    const view = await mountView()
    await showVideoMode(view)
    composer(view).vm.$emit('submit', videoSubmission('tracked'))
    await flushPromises()
    vi.mocked(asyncTasksAPI.list).mockClear()

    await vi.advanceTimersByTimeAsync(10_000)
    await flushPromises()

    expect(asyncTasksAPI.list).toHaveBeenCalledTimes(1)
    expect(asyncTasksAPI.refresh).toHaveBeenCalledTimes(1)
    expect(asyncTasksAPI.refresh).toHaveBeenCalledWith('vidtask_older', expect.any(AbortSignal))
    // 本页正在跟踪的任务只显示一张卡片，服务端的同一条记录不会重复出现。
    expect(view.findAll('[data-testid="creation-task"]')).toHaveLength(1)
    expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(1)
  })

  it('stops polling every task when the page is left', async () => {
    const view = await mountView()
    await showVideoMode(view)
    composer(view).vm.$emit('submit', videoSubmission('first'))
    composer(view).vm.$emit('submit', videoSubmission('second'))
    await flushPromises()

    view.unmount()
    wrapper = null
    await flushPromises()

    expect(waits.every(wait => wait.options.signal?.aborted)).toBe(true)
    const calls = vi.mocked(asyncTasksAPI.list).mock.calls.length
    await vi.advanceTimersByTimeAsync(30_000)
    expect(asyncTasksAPI.list).toHaveBeenCalledTimes(calls)
  })

  it('shows finished images and videos inline in one history list, whatever the form mode is', async () => {
    const originalCreate = URL.createObjectURL
    const originalRevoke = URL.revokeObjectURL
    URL.createObjectURL = vi.fn(() => 'blob:protected-video')
    URL.revokeObjectURL = vi.fn()
    // 测试环境默认的 IntersectionObserver 不会回调；这里让元素一被观察就算进入视口。
    vi.stubGlobal('IntersectionObserver', class {
      private readonly notify: IntersectionObserverCallback
      constructor(callback: IntersectionObserverCallback) { this.notify = callback }
      observe(): void { this.notify([{ isIntersecting: true } as IntersectionObserverEntry], this as unknown as IntersectionObserver) }
      disconnect(): void {}
      unobserve(): void {}
    })
    try {
      const base = { api_key_id: 9, created_at: 100, expires_at: 200 }
      const items: AsyncTask[] = [
        { ...base, id: 'imgtask_a', task_id: 'imgtask_a', kind: 'image', protocol: 'openai', model: 'gpt-image-1', status: 'completed', result_url: 'https://cdn.example/a.png', actual_cost: 0.035 },
        { ...base, id: 'vidtask_b', task_id: 'cgt-b', kind: 'video', protocol: 'seedance', model: 'seedance-1', status: 'completed', result_url: 'https://cdn.example/b.mp4' },
        { ...base, id: 'vidtask_c', task_id: 'op-c', kind: 'video', protocol: 'gemini', model: 'veo-3', status: 'completed', result_url: '/api/v1/user/async-tasks/vidtask_c/content' },
        { ...base, id: 'imgtask_d', task_id: 'imgtask_d', kind: 'image', protocol: 'openai', model: 'gpt-image-1', status: 'failed', error: 'blocked by policy' },
      ]
      vi.mocked(asyncTasksAPI.list).mockResolvedValue({ items, total: items.length, page: 1, page_size: 20 })
      vi.mocked(asyncTasksAPI.download).mockResolvedValue(new Blob(['video'], { type: 'video/mp4' }))

      const view = await mountView()
      await flushPromises()

      const cards = view.findAll('[data-testid="creation-history-item"]')
      expect(cards).toHaveLength(4)
      expect(cards[0]?.find('img').attributes('src')).toBe('https://cdn.example/a.png')
      expect(cards[0]?.get('[data-testid="creation-history-cost"]').text()).toBe('creation.cost.actual $0.035')
      expect(cards[1]?.find('video').attributes('src')).toBe('https://cdn.example/b.mp4')
      // 不能直接播放的视频经站内下载接口取回。
      expect(asyncTasksAPI.download).toHaveBeenCalledTimes(1)
      expect(asyncTasksAPI.download).toHaveBeenCalledWith('vidtask_c')
      expect(cards[2]?.find('video').attributes('src')).toBe('blob:protected-video')
      expect(cards[3]?.text()).toContain('blocked by policy')
      expect(cards[3]?.find('[data-testid="creation-history-cost"]').exists()).toBe(false)

      // 图片链接失效时显示占位，而不是裂图。
      await cards[0]!.get('img').trigger('error')
      expect(cards[0]?.find('img').exists()).toBe(false)
      expect(cards[0]?.text()).toContain('creation.history.imageUnavailable')

      // 历史不随左侧的生图 / 生视频模式切换而改变，也不按模式筛选。
      await showVideoMode(view)
      expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(4)
      expect(vi.mocked(asyncTasksAPI.list).mock.calls.every(([params]) => params.kind === '')).toBe(true)

      // 离开页面时释放下载得到的视频地址（jsdom 没有 URL.revokeObjectURL，须在还原前卸载）。
      view.unmount()
      wrapper = null
      expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:protected-video')
    } finally {
      vi.unstubAllGlobals()
      URL.createObjectURL = originalCreate
      URL.revokeObjectURL = originalRevoke
    }
  })

  it('filters history by kind and appends older pages on demand', async () => {
    const all = Array.from({ length: 25 }, (_, index): AsyncTask => ({
      id: `imgtask_${index}`, task_id: `imgtask_${index}`, kind: 'image', protocol: 'openai', model: 'gpt-image-1',
      status: 'failed', api_key_id: 9, created_at: 1000 - index, expires_at: 2000,
    }))
    vi.mocked(asyncTasksAPI.list).mockImplementation(async params => ({
      items: all.slice((params.page - 1) * params.page_size, params.page * params.page_size),
      total: all.length, page: params.page, page_size: params.page_size,
    }))
    const view = await mountView()
    expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(20)

    await view.get('[data-testid="creation-history-more"]').trigger('click')
    await flushPromises()
    expect(view.findAll('[data-testid="creation-history-item"]')).toHaveLength(25)
    expect(view.find('[data-testid="creation-history-more"]').exists()).toBe(false)

    vi.mocked(asyncTasksAPI.list).mockClear()
    await view.get('[data-testid="creation-filter-video"]').trigger('click')
    await flushPromises()
    expect(asyncTasksAPI.list).toHaveBeenCalledTimes(1)
    expect(asyncTasksAPI.list).toHaveBeenCalledWith(expect.objectContaining({ page: 1, kind: 'video' }), expect.any(AbortSignal))
  })
})
