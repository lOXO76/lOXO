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
          >{{ t(`creation.modes.${kind}`) }}</button>
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
            :busy="running"
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
          <CreationResult :task="currentTask" @cancel="cancelWaiting" />
          <CreationHistory :tasks="historyTasks" :records="sessionRecords" :loading="loadingHistory" :error="historyError" :downloading-id="downloadingId" @refresh="loadHistory" @download="downloadHistoryTask" />
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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
const currentTask = ref<CreationCurrentTask | null>(null)
const running = ref(false)
let catalogController: AbortController | null = null
let keysController: AbortController | null = null
let historyController: AbortController | null = null
let taskController: AbortController | null = null
let catalogRequest = 0
let historyRequest = 0
let taskSequence = 0
let activeApiKey = ''
let initialized = false

const userId = computed(() => authStore.user?.id ?? 'anonymous')
const creationEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.creation))
const featureDisabled = computed(() => !creationEnabled.value || catalogNotFound.value)
const availableGroups = computed(() => filterCreationGroups(catalog.value?.groups, mode.value))

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

async function loadHistory(): Promise<void> {
  if (featureDisabled.value) return
  historyController?.abort()
  const controller = new AbortController()
  historyController = controller
  const request = ++historyRequest
  loadingHistory.value = true
  historyError.value = ''
  try {
    const result = await asyncTasksAPI.list({ page: 1, page_size: 20, kind: mode.value, status: '', search: '' }, controller.signal)
    if (request === historyRequest) historyTasks.value = result.items.map(normalizeHistoryTask)
  } catch (error) {
    if (request === historyRequest && !isAbortError(error)) historyError.value = errorText(error, t('creation.messages.historyLoadFailed'))
  } finally {
    if (request === historyRequest) loadingHistory.value = false
  }
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

function applySnapshot(runId: number, snapshot: CreationTaskSnapshot): void {
  const task = currentTask.value
  if (!task || task.id !== runId) return
  if (task.assets !== snapshot.assets) {
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

async function submit(submission: CreationSubmission): Promise<void> {
  if (running.value) {
    appStore.showError(t('creation.messages.taskInProgress'))
    return
  }
  if (currentTask.value) revokeAssets(currentTask.value.assets)
  const runId = ++taskSequence
  const controller = new AbortController()
  taskController = controller
  activeApiKey = submission.apiKey
  running.value = true
  currentTask.value = {
    id: runId, kind: submission.kind, protocol: submission.input.protocol, model: submission.input.model,
    prompt: submission.input.prompt, groupName: submission.groupName, createdAt: Date.now(),
    status: 'processing', assets: [], error: null, handle: null, submitting: true, cancellationNoticed: false,
  }
  try {
    const result = submission.kind === 'image'
      ? await submitImage({ ...submission.input, signal: controller.signal })
      : await submitVideo({ ...submission.input, signal: controller.signal })
    if (currentTask.value?.id !== runId) {
      revokeAssets(result.snapshot.assets)
      return
    }
    currentTask.value.handle = result.handle
    currentTask.value.submitting = false
    applySnapshot(runId, result.snapshot)
    if (result.handle && result.snapshot.status === 'processing') {
      const snapshot = await waitForTask(activeApiKey, result.handle, {
        signal: controller.signal,
        onUpdate: update => applySnapshot(runId, update),
      })
      applySnapshot(runId, snapshot)
    }
    const task = currentTask.value
    if (task?.id === runId && task.status === 'succeeded') {
      saveSessionRecord(task)
      appStore.showSuccess(t('creation.messages.taskSucceeded'))
    }
  } catch (error) {
    const task = currentTask.value
    if (task?.id === runId) {
      task.submitting = false
      if (isAbortError(error)) {
        task.status = 'cancelled'
        task.cancellationNoticed = true
        task.error = null
      } else {
        task.status = 'failed'
        task.error = taskError(error)
        if (task.error.code === 'TIMEOUT' || task.error.code === 'NETWORK_ERROR') task.cancellationNoticed = true
      }
    }
  } finally {
    if (currentTask.value?.id === runId) {
      running.value = false
      taskController = null
      activeApiKey = ''
      void loadHistory()
    }
  }
}

function cancelWaiting(): void {
  const task = currentTask.value
  if (!task || task.status !== 'processing') return
  task.submitting = false
  task.status = 'cancelled'
  task.cancellationNoticed = true
  taskController?.abort()
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
})

onBeforeUnmount(() => {
  catalogRequest++
  historyRequest++
  taskSequence++
  catalogController?.abort()
  keysController?.abort()
  historyController?.abort()
  taskController?.abort()
  if (currentTask.value) revokeAssets(currentTask.value.assets)
  currentTask.value = null
})
</script>
