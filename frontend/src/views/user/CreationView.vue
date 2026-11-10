<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-gray-100">{{ t('creation.title') }}</h1>
          <p class="mt-1 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('creation.description') }}</p>
        </div>
        <div v-if="!featureDisabled" class="flex shrink-0 rounded-lg border border-gray-200 bg-white p-1 dark:border-dark-700 dark:bg-dark-800" role="tablist" :aria-label="t('creation.title')">
          <button
            v-for="kind in (['image', 'video'] as CreationKind[])"
            :id="`creation-tab-${kind}`"
            :key="kind"
            type="button"
            role="tab"
            :aria-selected="mode === kind"
            aria-controls="creation-workspace"
            class="rounded-md px-4 py-2 text-sm font-medium transition-colors"
            :class="mode === kind ? 'bg-primary-600 text-white' : 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-dark-700'"
            @click="mode = kind"
          >
            {{ t(`creation.modes.${kind}`) }}
            <span
              v-if="activeCounts[kind]"
              class="ml-1.5 inline-flex min-w-5 items-center justify-center rounded-full px-1.5 text-xs"
              :class="mode === kind ? 'bg-white/20 text-white' : 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-200'"
              :data-testid="`creation-tab-${kind}-active`"
            >{{ activeCounts[kind] }}</span>
          </button>
        </div>
      </div>

      <div v-if="featureDisabled" class="card px-6 py-16 text-center" data-testid="creation-disabled">
        <Icon name="ban" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
        <h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">{{ t('creation.disabled.title') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('creation.disabled.description') }}</p>
      </div>

      <div v-else-if="loadingCatalog && !catalog" class="card space-y-4 p-6" aria-busy="true" role="status">
        <span class="sr-only">{{ t('creation.loading') }}</span>
        <div class="h-5 w-40 animate-pulse rounded bg-gray-100 dark:bg-dark-800" />
        <div class="grid gap-4 xl:grid-cols-[380px_minmax(0,1fr)]"><div class="h-96 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" /><div class="h-96 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" /></div>
      </div>

      <div v-else-if="catalogError" role="alert" class="card border-red-200 p-5 text-sm text-red-700 dark:border-red-900 dark:text-red-300">
        <p>{{ catalogError }}</p>
        <button type="button" class="btn btn-secondary btn-sm mt-3" @click="retryCatalog">{{ t('creation.history.refresh') }}</button>
      </div>

      <div v-else-if="catalog" id="creation-workspace" role="tabpanel" :aria-labelledby="`creation-tab-${mode}`" class="grid items-start gap-5 xl:grid-cols-[380px_minmax(0,1fr)]">
        <div class="min-w-0">
          <CreationComposer
            v-if="availableGroups.length"
            :key="String(userId)"
            :mode="mode"
            :groups="catalog.groups"
            :api-keys="apiKeys"
            :user-id="userId"
            :loading-keys="loadingKeys"
            :creating-key="creatingKey"
            :key-error="keyError"
            :at-capacity="atCapacity"
            :max-active="MAX_ACTIVE_TASKS"
            @submit="submit"
            @create-key="createKey"
            @reload-keys="loadKeys"
          />
          <section v-else class="card px-6 py-12 text-center">
            <Icon name="inbox" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
            <h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-gray-100">{{ t(mode === 'image' ? 'creation.empty.imageTitle' : 'creation.empty.videoTitle') }}</h2>
            <p class="mt-2 text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('creation.empty.description') }}</p>
            <div class="mt-5 flex flex-col justify-center gap-2 sm:flex-row xl:flex-col 2xl:flex-row">
              <router-link to="/available-channels" class="btn btn-secondary">{{ t('creation.empty.channels') }}</router-link>
              <router-link to="/keys" class="btn btn-primary">{{ t('creation.empty.keys') }}</router-link>
            </div>
          </section>
        </div>
        <div class="min-w-0 space-y-5">
          <CreationResult :tasks="modeTasks" @stop="stopTracking" @dismiss="dismissTask" />
          <CreationHistory :tasks="historyTasks" :records="sessionRecords" :loading="loadingHistory" :error="historyError" :downloading-id="downloadingId" @refresh="loadHistory(true)" @download="downloadHistoryTask" />
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, toRaw, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CreationComposer from './creation/CreationComposer.vue'
import CreationResult from './creation/CreationResult.vue'
import CreationHistory from './creation/CreationHistory.vue'
import { creationAPI } from '@/features/creation/api'
import { CreationGatewayError, revokeAssets, submitImage, submitVideo, waitForTask } from '@/features/creation/gateway'
import type { CreationCatalog, CreationKind, CreationTaskSnapshot } from '@/features/creation/types'
import { keysAPI } from '@/api/keys'
import { asyncTasksAPI, type AsyncTask } from '@/api/asyncTasks'
import type { ApiKey } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import {
  filterCreationGroups,
  readCreationHistory,
  safeMediaUrl,
  writeCreationHistory,
  type CreationSessionRecord,
} from './creation/viewModel'
import type { CreationCurrentTask, CreationErrorState, CreationSubmission } from './creation/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const mode = ref<CreationKind>('image')
const catalog = ref<CreationCatalog | null>(null)
const loadingCatalog = ref(false)
const catalogError = ref('')
const catalogNotFound = ref(false)
const apiKeys = ref<ApiKey[]>([])
const loadingKeys = ref(false)
const creatingKey = ref(false)
const keyError = ref('')
const historyTasks = ref<AsyncTask[]>([])
const loadingHistory = ref(false)
const historyError = ref('')
const sessionRecords = ref<CreationSessionRecord[]>([])
const downloadingId = ref('')
/** 本页提交并在后台跟踪的任务，最新的在前。 */
const tasks = ref<CreationCurrentTask[]>([])
/** 同时在后台跟踪的进行中任务上限，避免并发轮询过多、撞上分组并发限制。 */
const MAX_ACTIVE_TASKS = 5
/** 任务列表最多保留的条数，超出时丢弃最早的已结束任务。 */
const MAX_TRACKED_TASKS = 20
/** 历史中仍有处理中的任务时，自动刷新历史的间隔。 */
const HISTORY_POLL_MS = 10_000
const taskControllers = new Map<number, AbortController>()
let catalogController: AbortController | null = null
let keysController: AbortController | null = null
let historyController: AbortController | null = null
let historyTimer: ReturnType<typeof setInterval> | undefined
let catalogRequest = 0
let historyRequest = 0
let taskSequence = 0
let initialized = false
let unmounted = false

const userId = computed(() => authStore.user?.id ?? 'anonymous')
const creationEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.creation))
const featureDisabled = computed(() => !creationEnabled.value || catalogNotFound.value)
const availableGroups = computed(() => filterCreationGroups(catalog.value?.groups, mode.value))
const activeTasks = computed(() => tasks.value.filter(task => task.status === 'processing'))
const activeCounts = computed<Record<CreationKind, number>>(() => ({
  image: activeTasks.value.filter(task => task.kind === 'image').length,
  video: activeTasks.value.filter(task => task.kind === 'video').length,
}))
const atCapacity = computed(() => activeTasks.value.length >= MAX_ACTIVE_TASKS)
const modeTasks = computed(() => tasks.value.filter(task => task.kind === mode.value))

function recordOf(error: unknown): Record<string, unknown> {
  return error && typeof error === 'object' ? error as Record<string, unknown> : {}
}

function errorStatus(error: unknown): number {
  const status = Number(recordOf(error).status)
  return Number.isFinite(status) ? status : 0
}

function errorText(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message
  const message = recordOf(error).message
  return typeof message === 'string' && message.trim() ? message : fallback
}

function isAbortError(error: unknown): boolean {
  const value = recordOf(error)
  return value.name === 'AbortError' || value.code === 'ABORTED' || value.code === 'ERR_CANCELED'
}

async function loadCatalog(): Promise<void> {
  if (!creationEnabled.value) return
  catalogController?.abort()
  const controller = new AbortController()
  catalogController = controller
  const request = ++catalogRequest
  loadingCatalog.value = true
  catalogError.value = ''
  try {
    const result = await creationAPI.getCatalog(controller.signal)
    if (request !== catalogRequest) return
    catalog.value = { groups: Array.isArray(result.groups) ? result.groups : [] }
    catalogNotFound.value = false
  } catch (error) {
    if (request !== catalogRequest || isAbortError(error)) return
    if (errorStatus(error) === 404) catalogNotFound.value = true
    else catalogError.value = errorText(error, t('creation.messages.catalogLoadFailed'))
  } finally {
    if (request === catalogRequest) loadingCatalog.value = false
  }
}

function retryCatalog(): void {
  catalogNotFound.value = false
  void loadCatalog()
}

async function loadKeys(): Promise<void> {
  keysController?.abort()
  const controller = new AbortController()
  keysController = controller
  loadingKeys.value = true
  keyError.value = ''
  try {
    const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' }, { signal: controller.signal })
    if (!controller.signal.aborted) apiKeys.value = response.items
  } catch (error) {
    if (!controller.signal.aborted) keyError.value = errorText(error, t('creation.messages.keysLoadFailed'))
  } finally {
    if (keysController === controller) loadingKeys.value = false
  }
}

async function createKey(group: { id: number; name: string }): Promise<void> {
  if (creatingKey.value) return
  creatingKey.value = true
  keyError.value = ''
  try {
    await keysAPI.create(t('creation.composer.defaultKeyName', { group: group.name }), group.id)
    await loadKeys()
    appStore.showSuccess(t('creation.messages.keyCreated'))
  } catch (error) {
    keyError.value = errorText(error, t('creation.messages.keyCreateFailed'))
  } finally {
    creatingKey.value = false
  }
}

function historyResultUrl(value: unknown): string | undefined {
  const direct = safeMediaUrl(value)
  if (direct) return direct
  return typeof value === 'string' && value.startsWith('/api/v1/user/async-tasks/') ? value : undefined
}

function normalizeHistoryTask(task: AsyncTask): AsyncTask {
  return {
    id: task.id,
    task_id: task.task_id,
    kind: task.kind,
    protocol: task.protocol,
    model: task.model?.trim() || '',
    status: task.status,
    api_key_id: task.api_key_id,
    group_id: task.group_id,
    result_url: historyResultUrl(task.result_url),
    error: task.error,
    created_at: task.created_at,
    completed_at: task.completed_at,
    expires_at: task.expires_at,
  }
}

/**
 * 拉取最近任务；refreshUpstream 时再让后端刷新「不在本页跟踪中」的处理中视频任务
 * （本页跟踪的任务已由浏览器轮询网关，后端记录会随之更新，无需重复刷新）。
 */
async function loadHistory(refreshUpstream = false): Promise<void> {
  if (featureDisabled.value) return
  historyController?.abort()
  const controller = new AbortController()
  historyController = controller
  const request = ++historyRequest
  loadingHistory.value = true
  historyError.value = ''
  try {
    const result = await asyncTasksAPI.list({ page: 1, page_size: 20, kind: mode.value, status: '', search: '' }, controller.signal)
    if (request !== historyRequest) return
    historyTasks.value = result.items.map(normalizeHistoryTask)
    if (refreshUpstream) await refreshUntrackedVideos(request, controller.signal)
  } catch (error) {
    if (request === historyRequest && !isAbortError(error)) historyError.value = errorText(error, t('creation.messages.historyLoadFailed'))
  } finally {
    if (request === historyRequest) loadingHistory.value = false
  }
}

async function refreshUntrackedVideos(request: number, signal: AbortSignal): Promise<void> {
  const tracked = new Set(activeTasks.value.flatMap(task => task.handle ? [task.handle.taskId] : []))
  const pending = historyTasks.value.filter(task => task.kind === 'video' && task.status === 'processing' && !tracked.has(task.task_id))
  // 与异步任务页一致：每次最多并发 3 个上游刷新。
  for (let index = 0; index < pending.length && request === historyRequest; index += 3) {
    await Promise.all(pending.slice(index, index + 3).map(async task => {
      try {
        const updated = normalizeHistoryTask(await asyncTasksAPI.refresh(task.id, signal))
        if (request !== historyRequest) return
        const position = historyTasks.value.findIndex(item => item.id === task.id)
        if (position >= 0) historyTasks.value[position] = updated
      } catch {
        // 单个任务刷新失败不影响列表，下一轮自动刷新会重试。
      }
    }))
  }
}

function pollHistory(): void {
  if (document.hidden || loadingHistory.value || featureDisabled.value || !catalog.value) return
  if (historyTasks.value.some(task => task.status === 'processing')) void loadHistory(true)
}

function loadSessionHistory(): void {
  sessionRecords.value = readCreationHistory(localStorage, userId.value, mode.value)
}

function saveSessionRecord(task: CreationCurrentTask): void {
  const assets = task.assets.flatMap(asset => {
    const candidate = !asset.url.startsWith('data:') && !asset.url.startsWith('blob:') ? asset.url : asset.sourceUrl
    const url = safeMediaUrl(candidate)
    return url && !url.startsWith('blob:') ? [{ kind: asset.kind, url, ...(asset.mimeType ? { mimeType: asset.mimeType } : {}) }] : []
  })
  const record: CreationSessionRecord = {
    id: `${task.createdAt}-${task.id}`, kind: task.kind, model: task.model, protocol: task.protocol,
    prompt: task.prompt, createdAt: task.createdAt, assets,
  }
  const previous = [
    ...readCreationHistory(localStorage, userId.value, 'image'),
    ...readCreationHistory(localStorage, userId.value, 'video'),
  ]
  const records = [record, ...previous.filter(item => item.id !== record.id)]
    .sort((left, right) => right.createdAt - left.createdAt)
    .slice(0, 20)
  writeCreationHistory(localStorage, userId.value, records)
  loadSessionHistory()
}

function taskError(error: unknown): CreationErrorState {
  if (error instanceof CreationGatewayError) {
    return { message: error.message, code: error.code, requestId: error.requestId, status: error.status }
  }
  const value = recordOf(error)
  return {
    message: errorText(error, t('creation.messages.submitFailed')),
    code: typeof value.code === 'string' ? value.code : 'REQUEST_FAILED',
    requestId: typeof value.requestId === 'string' ? value.requestId : '',
    status: errorStatus(error),
  }
}

function findTask(id: number): CreationCurrentTask | undefined {
  return tasks.value.find(task => task.id === id)
}

/** 超出保留上限时，从最早的已结束任务开始丢弃（进行中的任务不丢）。 */
function trimTasks(): void {
  for (let index = tasks.value.length - 1; index >= 0 && tasks.value.length > MAX_TRACKED_TASKS; index--) {
    const task = tasks.value[index]
    if (!task || task.status === 'processing') continue
    revokeAssets(task.assets)
    tasks.value.splice(index, 1)
  }
}

function applySnapshot(id: number, snapshot: CreationTaskSnapshot): void {
  const task = findTask(id)
  // 任务已移除或已停止跟踪：丢弃迟到的快照，并释放其中的 blob URL。
  if (!task || taskControllers.get(id)?.signal.aborted) {
    if (!task || toRaw(task.assets) !== snapshot.assets) revokeAssets(snapshot.assets)
    return
  }
  // 同一个终态快照会经 onUpdate 与返回值各应用一次；task.assets 是响应式代理，需比较原始数组。
  if (toRaw(task.assets) !== snapshot.assets) {
    revokeAssets(task.assets)
    task.assets = snapshot.assets
  }
  task.status = snapshot.status
  task.progress = snapshot.progress
  if (snapshot.status === 'failed') {
    task.error = { message: snapshot.error || t('creation.messages.submitFailed'), code: 'TASK_FAILED', requestId: '', status: 0 }
  } else if (snapshot.status === 'cancelled') {
    task.cancellationNoticed = true
    task.error = null
  } else {
    task.error = null
  }
}

function notifyFinished(id: number): void {
  const task = findTask(id)
  if (task?.status === 'succeeded') {
    saveSessionRecord(task)
    appStore.showSuccess(t('creation.messages.taskSucceeded'))
  } else if (task?.status === 'failed') {
    appStore.showError(t('creation.messages.taskFailed'))
  }
}

/**
 * 提交即返回：任务进入右侧列表并在后台轮询，表单立刻可以继续提交下一条。
 * 异步任务（视频、开启异步的 OpenAI/Grok 生图）离开本页后仍在服务端运行，可在历史/异步任务中查看；
 * 网关只支持同步的协议（如 Gemini 生图）同样在后台进行，但需保持页面打开才能拿到结果。
 */
async function submit(submission: CreationSubmission): Promise<void> {
  if (atCapacity.value) {
    appStore.showError(t('creation.messages.tooManyTasks', { count: MAX_ACTIVE_TASKS }))
    return
  }
  const id = ++taskSequence
  const controller = new AbortController()
  taskControllers.set(id, controller)
  const apiKey = submission.apiKey
  tasks.value.unshift({
    id, kind: submission.kind, protocol: submission.input.protocol, model: submission.input.model,
    prompt: submission.input.prompt, groupName: submission.groupName, createdAt: Date.now(),
    status: 'processing', assets: [], error: null, handle: null, submitting: true,
    sync: submission.kind === 'image' && !submission.input.preferAsync, cancellationNoticed: false,
  })
  trimTasks()
  try {
    const result = submission.kind === 'image'
      ? await submitImage({ ...submission.input, signal: controller.signal })
      : await submitVideo({ ...submission.input, signal: controller.signal })
    const task = findTask(id)
    if (!task || controller.signal.aborted) {
      revokeAssets(result.snapshot.assets)
      return
    }
    task.handle = result.handle
    task.submitting = false
    applySnapshot(id, result.snapshot)
    if (result.handle && result.snapshot.status === 'processing') {
      appStore.showSuccess(t('creation.messages.taskSubmitted'))
      void loadHistory()
      const snapshot = await waitForTask(apiKey, result.handle, {
        signal: controller.signal,
        onUpdate: update => applySnapshot(id, update),
      })
      applySnapshot(id, snapshot)
    }
    notifyFinished(id)
  } catch (error) {
    const task = findTask(id)
    if (task) {
      task.submitting = false
      if (isAbortError(error)) {
        task.status = 'cancelled'
        task.cancellationNoticed = true
        task.error = null
      } else {
        task.status = 'failed'
        task.error = taskError(error)
        if (task.error.code === 'TIMEOUT' || task.error.code === 'NETWORK_ERROR') task.cancellationNoticed = true
        // 跟踪超时不代表任务失败（上游可能仍在跑），卡片里已提示去异步任务确认，不再弹失败提示。
        if (task.error.code !== 'TIMEOUT') appStore.showError(t('creation.messages.taskFailed'))
      }
    }
  } finally {
    taskControllers.delete(id)
    if (!unmounted) void loadHistory()
  }
}

/** 只停止浏览器侧的轮询，上游任务仍会继续运行并计费。 */
function stopTracking(id: number): void {
  const task = findTask(id)
  if (!task || task.status !== 'processing') return
  task.submitting = false
  task.status = 'cancelled'
  task.cancellationNoticed = true
  task.error = null
  taskControllers.get(id)?.abort()
}

function dismissTask(id: number): void {
  const index = tasks.value.findIndex(task => task.id === id)
  const task = tasks.value[index]
  if (!task || task.status === 'processing') return
  revokeAssets(task.assets)
  tasks.value.splice(index, 1)
}

function clearTasks(): void {
  for (const controller of taskControllers.values()) controller.abort()
  taskControllers.clear()
  for (const task of tasks.value) revokeAssets(task.assets)
  tasks.value = []
}

async function downloadHistoryTask(task: AsyncTask): Promise<void> {
  if (downloadingId.value) return
  downloadingId.value = task.id
  try {
    const blob = await asyncTasksAPI.download(task.id)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    const id = task.task_id.replace(/[^a-zA-Z0-9-_]/g, '-') || task.id
    const imageExtension = blob.type.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
    link.href = url
    link.download = `${task.protocol || task.kind}-${id}.${task.kind === 'video' ? 'mp4' : imageExtension}`
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch {
    appStore.showError(t('creation.messages.downloadFailed'))
  } finally {
    downloadingId.value = ''
  }
}

async function startRequests(): Promise<void> {
  if (!creationEnabled.value) return
  await loadCatalog()
  if (featureDisabled.value) return
  await Promise.all([loadKeys(), loadHistory()])
}

watch(mode, () => {
  historyTasks.value = []
  loadSessionHistory()
  if (initialized && creationEnabled.value) void loadHistory()
})
watch(userId, () => {
  // 换了账号：上一位用户的任务与密钥不能继续在本页轮询或展示。
  clearTasks()
  loadSessionHistory()
  if (initialized && creationEnabled.value) {
    void loadKeys()
    void loadHistory()
  }
})
watch(creationEnabled, enabled => {
  if (!initialized) return
  if (enabled) {
    catalogNotFound.value = false
    void startRequests()
  } else {
    catalogNotFound.value = true
    catalogController?.abort()
    keysController?.abort()
    historyController?.abort()
  }
})

onMounted(async () => {
  loadSessionHistory()
  if (!appStore.publicSettingsLoaded) {
    try { await appStore.fetchPublicSettings() } catch { /* 目录接口继续提供最终开关状态。 */ }
  }
  initialized = true
  void startRequests()
  historyTimer = setInterval(pollHistory, HISTORY_POLL_MS)
})

onBeforeUnmount(() => {
  unmounted = true
  catalogRequest++
  historyRequest++
  if (historyTimer) clearInterval(historyTimer)
  catalogController?.abort()
  keysController?.abort()
  historyController?.abort()
  // 离开页面只停止浏览器侧轮询；已提交的异步任务继续在服务端运行。
  clearTasks()
})
</script>
