<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <header class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">{{ t('rewardTasks.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('rewardTasks.description') }}</p>
        </div>
        <button class="btn btn-secondary shrink-0" type="button" :disabled="refreshing" @click="refreshAll">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': refreshing }" aria-hidden="true" />
          {{ t('rewardTasks.refresh') }}
        </button>
      </header>

      <dl class="grid grid-cols-2 gap-3 sm:gap-5">
        <div class="card p-4 sm:p-6">
          <dt class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('rewardTasks.completedCount') }}</dt>
          <dd class="mt-2 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ data?.summary.completed_count ?? 0 }}</dd>
        </div>
        <div class="card border-emerald-200 bg-emerald-50/40 p-4 dark:border-emerald-900/50 dark:bg-emerald-900/10 sm:p-6">
          <dt class="text-xs font-medium text-gray-500 dark:text-dark-400">{{ t('rewardTasks.totalReward') }}</dt>
          <dd class="mt-2 break-all text-2xl font-semibold tabular-nums text-emerald-700 dark:text-emerald-400" data-testid="total-reward">
            {{ formatReward(data?.summary.total_reward ?? 0) }}
          </dd>
        </div>
      </dl>

      <div v-if="loading" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" role="status" :aria-label="t('rewardTasks.loading')">
        <div v-for="index in 3" :key="index" class="card space-y-5 p-6 motion-safe:animate-pulse">
          <div class="h-10 w-10 rounded-xl bg-gray-100 dark:bg-dark-700"></div>
          <div class="h-5 w-2/3 rounded bg-gray-100 dark:bg-dark-700"></div>
          <div class="h-20 rounded bg-gray-100 dark:bg-dark-700"></div>
        </div>
        <span class="sr-only">{{ t('rewardTasks.loading') }}</span>
      </div>

      <div v-else-if="loadError" class="card px-6 py-16 text-center" role="alert">
        <CircleAlert class="mx-auto h-10 w-10 text-red-500" aria-hidden="true" />
        <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">{{ t('rewardTasks.loadFailed') }}</p>
        <button class="btn btn-secondary mt-4" type="button" data-testid="tasks-retry" @click="refreshAll">{{ t('rewardTasks.retry') }}</button>
      </div>

      <div v-else-if="!data?.enabled" class="card px-6 py-16 text-center" data-testid="tasks-disabled">
        <ListChecks class="mx-auto h-12 w-12 text-gray-400 dark:text-dark-500" aria-hidden="true" />
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('rewardTasks.disabledTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('rewardTasks.disabledDescription') }}</p>
      </div>

      <div v-else-if="!data.tasks.length" class="card px-6 py-16 text-center" data-testid="tasks-empty">
        <ListChecks class="mx-auto h-12 w-12 text-gray-400 dark:text-dark-500" aria-hidden="true" />
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('rewardTasks.emptyTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('rewardTasks.emptyDescription') }}</p>
      </div>

      <section v-else :aria-label="t('rewardTasks.availableTasks')" class="grid items-start gap-4 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="task in data.tasks" :key="task.id" class="card flex min-w-0 flex-col p-5" :data-testid="`task-${task.id}`">
          <div class="flex items-start gap-3">
            <div class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-400">
              <img v-if="task.icon_url && !failedIcons[task.id]" :src="task.icon_url" alt="" class="h-full w-full object-contain" loading="lazy" referrerpolicy="no-referrer" @error="failedIcons[task.id] = true" />
              <component :is="typeIcons[task.task_type]" v-else class="h-5 w-5" aria-hidden="true" />
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ task.title }}</h2>
              <p class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400" :title="targetDomain(task.target_url)">{{ targetDomain(task.target_url) }}</p>
            </div>
          </div>
          <div class="mt-4 flex flex-wrap gap-2 text-xs font-medium">
            <span class="rounded-md bg-primary-50 px-2 py-1 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ t(`rewardTasks.types.${task.task_type}`) }}</span>
            <span class="rounded-md bg-gray-100 px-2 py-1 text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ t(`rewardTasks.periods.${task.period}`) }}</span>
          </div>
          <p class="mt-4 text-2xl font-semibold tabular-nums tracking-tight text-emerald-600 dark:text-emerald-400">+{{ formatReward(task.reward_amount) }}</p>
          <p class="mt-2 text-sm font-medium text-gray-700 dark:text-dark-200">{{ requirement(task) }}</p>
          <details v-if="task.description" class="mt-3 text-sm text-gray-500 dark:text-dark-400">
            <summary class="cursor-pointer rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500">{{ t('rewardTasks.details') }}</summary>
            <p class="mt-2 whitespace-pre-wrap break-words leading-relaxed">{{ task.description }}</p>
          </details>
          <p v-if="task.ends_at" class="mt-3 text-xs text-gray-500 dark:text-dark-400">{{ t('rewardTasks.endsAt', { time: formatDate(task.ends_at) }) }}</p>

          <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
            <template v-if="task.current?.status === 'completed'">
              <div class="flex items-center justify-center gap-2 rounded-xl bg-emerald-50 p-3 text-sm font-medium text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300">
                <CircleCheck class="h-4 w-4 shrink-0" aria-hidden="true" />
                {{ t('rewardTasks.completed', { amount: formatReward(task.current.reward_amount) }) }}
              </div>
              <p v-if="task.period !== 'once' && task.next_reset_at" class="mt-3 text-xs text-gray-500 dark:text-dark-400">{{ t('rewardTasks.nextReset', { time: formatDate(task.next_reset_at) }) }}</p>
            </template>
            <template v-else-if="!active(task)">
              <p v-if="task.current" class="mb-3 text-xs text-amber-700 dark:text-amber-400">{{ t('rewardTasks.expired') }}</p>
              <button class="btn btn-primary w-full" type="button" :disabled="busy[task.id] || task.quota_exhausted" :data-testid="`start-${task.id}`" @click="start(task)">
                {{ task.quota_exhausted ? t('rewardTasks.actions.exhausted') : busy[task.id] ? t('rewardTasks.actions.working') : t('rewardTasks.actions.start') }}
              </button>
            </template>
            <template v-else-if="task.verify_mode === 'timer'">
              <button class="btn btn-primary w-full" type="button" :disabled="busy[task.id] || !task.current?.claimable_at || remaining(task) > 0" :data-testid="`claim-${task.id}`" @click="claim(task)">
                {{ busy[task.id] ? t('rewardTasks.actions.working') : remaining(task) > 0 ? t('rewardTasks.actions.countdown', { seconds: remaining(task) }) : t('rewardTasks.actions.claim') }}
              </button>
              <button class="btn btn-ghost mt-2 w-full" type="button" :disabled="busy[task.id]" :data-testid="`start-${task.id}`" @click="start(task)">
                <ExternalLink class="h-3.5 w-3.5" aria-hidden="true" />{{ t('rewardTasks.actions.reopen') }}
              </button>
            </template>
            <template v-else>
              <template v-if="task.verify_mode === 'sdk'">
                <p class="text-sm tabular-nums text-gray-600 dark:text-dark-300">{{ t('rewardTasks.sdkProgress', { current: task.current?.progress_seconds ?? 0, total: task.browse_seconds }) }}</p>
                <div role="progressbar" :aria-label="t('rewardTasks.requirements.browse', { seconds: task.browse_seconds })" :aria-valuenow="Math.min(task.current?.progress_seconds ?? 0, task.browse_seconds)" :aria-valuemin="0" :aria-valuemax="task.browse_seconds" class="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-primary-500 transition-[width] motion-reduce:transition-none" :style="{ width: `${Math.min(100, ((task.current?.progress_seconds ?? 0) / Math.max(1, task.browse_seconds)) * 100)}%` }"></div>
                </div>
              </template>
              <p v-else-if="task.task_type === 'recharge'" class="text-sm font-medium tabular-nums text-gray-700 dark:text-dark-200">{{ t('rewardTasks.rechargeProgress', { current: formatYuan(task.current?.recharge_total ?? 0), total: formatYuan(task.min_recharge_amount) }) }}</p>
              <p class="mt-2 text-xs leading-relaxed text-gray-500 dark:text-dark-400">{{ automaticTaskHint(task) }}</p>
              <button class="btn btn-primary mt-4 w-full" type="button" :disabled="busy[task.id]" :data-testid="`start-${task.id}`" @click="start(task)">
                <ExternalLink class="h-4 w-4" aria-hidden="true" />
                {{ busy[task.id] ? t('rewardTasks.actions.working') : task.verify_mode === 'sdk' ? t('rewardTasks.actions.continueBrowsing') : t('rewardTasks.actions.continue') }}
              </button>
            </template>
            <a v-if="blockedLinks[task.id] && task.current?.status !== 'completed'" :href="blockedLinks[task.id]" target="_blank" rel="noopener noreferrer" class="mt-3 block break-words text-center text-sm font-medium text-primary-600 underline dark:text-primary-400" :data-testid="`blocked-link-${task.id}`">{{ t('rewardTasks.popupLink') }}</a>
          </div>
        </article>
      </section>

      <section class="card overflow-hidden" data-testid="task-records" :aria-busy="recordsLoading">
        <header class="px-5 py-5">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('rewardTasks.records.title') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('rewardTasks.records.description') }}</p>
        </header>
        <div v-if="recordsError" class="px-5 pb-6 text-center text-sm" role="alert">
          <p class="text-red-600 dark:text-red-400">{{ t('rewardTasks.records.loadFailed') }}</p>
          <button class="btn btn-secondary mt-3" type="button" @click="loadRecords()">{{ t('rewardTasks.retry') }}</button>
        </div>
        <p v-else-if="recordsLoading" class="px-5 pb-6 text-sm text-gray-500 dark:text-dark-400" role="status">{{ t('rewardTasks.records.loading') }}</p>
        <p v-else-if="!records?.items.length" class="px-5 pb-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('rewardTasks.records.empty') }}</p>
        <div v-else class="overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-5 py-3 font-medium" scope="col">{{ t('rewardTasks.records.task') }}</th>
                <th class="px-5 py-3 font-medium" scope="col">{{ t('rewardTasks.records.type') }}</th>
                <th class="px-5 py-3 font-medium" scope="col">{{ t('rewardTasks.records.period') }}</th>
                <th class="px-5 py-3 font-medium" scope="col">{{ t('rewardTasks.records.reward') }}</th>
                <th class="px-5 py-3 font-medium" scope="col">{{ t('rewardTasks.records.completedAt') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="record in records.items" :key="record.id" class="text-gray-700 dark:text-dark-200">
                <td class="min-w-40 max-w-xs break-words px-5 py-4 font-medium">{{ record.task_title }}</td>
                <td class="whitespace-nowrap px-5 py-4">{{ t(`rewardTasks.types.${record.task_type}`) }}</td>
                <td class="whitespace-nowrap px-5 py-4">{{ record.period_key === 'once' ? t('rewardTasks.periods.once') : record.period_key }}</td>
                <td class="whitespace-nowrap px-5 py-4 font-medium tabular-nums text-emerald-600 dark:text-emerald-400">+{{ formatReward(record.reward_amount) }}</td>
                <td class="whitespace-nowrap px-5 py-4 text-xs">{{ formatDate(record.completed_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination v-if="records && records.total > 0" :total="records.total" :page="recordsPage" :page-size="pageSize" :show-page-size-selector="false" @update:page="changeRecordsPage" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleAlert, CircleCheck, ExternalLink, Globe, ListChecks, RefreshCw, UserPlus, Wallet } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { claimRewardTask, listMyRewardTaskRecords, listRewardTasks, startRewardTask, type UserRewardTask, type UserRewardTaskList, type UserRewardTaskRecord } from '@/api/rewardTasks'
import type { PaginatedResponse } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorCode, extractApiErrorMetadata } from '@/utils/apiError'

const { t, te, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const typeIcons = { browse: Globe, register: UserPlus, recharge: Wallet }
const data = ref<UserRewardTaskList | null>(null)
const loading = ref(true)
const refreshing = ref(false)
const loadError = ref(false)
const records = ref<PaginatedResponse<UserRewardTaskRecord> | null>(null)
const recordsPage = ref(1)
const pageSize = 20
const recordsLoading = ref(false)
const recordsError = ref(false)
const busy = ref<Record<number, boolean>>({})
const blockedLinks = ref<Record<number, string>>({})
const failedIcons = ref<Record<number, boolean>>({})
const now = ref(Date.now())
let clockTimer: ReturnType<typeof setInterval> | undefined
let pollTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let listVersion = 0
let recordsVersion = 0
let listPending: Promise<void> | null = null

const preciseRewardFormatter = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 4 })
const compactRewardFormatter = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 })
const yuanFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 })

function formatReward(amount: number): string {
  return Math.abs(amount) >= 1 ? compactRewardFormatter.format(amount) : preciseRewardFormatter.format(amount)
}

function formatYuan(amount: number): string {
  return yuanFormatter.format(amount)
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '-' : new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function targetDomain(url: string): string {
  try { return new URL(url).hostname } catch { return '' }
}

function requirement(task: UserRewardTask): string {
  if (task.task_type === 'browse') return t('rewardTasks.requirements.browse', { seconds: task.browse_seconds })
  if (task.task_type === 'recharge') return t('rewardTasks.requirements.recharge', { amount: formatYuan(task.min_recharge_amount) })
  return t('rewardTasks.requirements.register')
}

function automaticTaskHint(task: UserRewardTask): string {
  if (task.verify_mode === 'sdk') {
    return task.idle_timeout_seconds === 0
      ? t('rewardTasks.sdkHintNoIdle')
      : t('rewardTasks.sdkHint', { seconds: task.idle_timeout_seconds })
  }
  return t(task.task_type === 'register' ? 'rewardTasks.registerHint' : 'rewardTasks.rechargeHint')
}

function active(task: UserRewardTask): boolean {
  return task.current?.status === 'in_progress' && Date.parse(task.current.ticket_expires_at) > now.value
}

function remaining(task: UserRewardTask): number {
  return task.current?.claimable_at ? Math.max(0, Math.ceil((Date.parse(task.current.claimable_at) - now.value) / 1000)) : 0
}

const activeTasks = computed(() => data.value?.enabled ? data.value.tasks.filter(active) : [])
const needsPolling = computed(() => activeTasks.value.some(task => task.verify_mode !== 'timer'))

function clearTimers(): void {
  clearInterval(clockTimer)
  clearTimeout(pollTimer)
  clockTimer = undefined
  pollTimer = undefined
}

function configureTimers(): void {
  clearTimers()
  if (disposed || document.visibilityState !== 'visible') return
  if (activeTasks.value.length) {
    clockTimer = setInterval(() => {
      now.value = Date.now()
      if (!activeTasks.value.length) clearTimers()
      else if (!needsPolling.value) {
        clearTimeout(pollTimer)
        pollTimer = undefined
      }
    }, 1000)
  }
  if (needsPolling.value) pollTimer = setTimeout(() => { void loadTasks(true) }, 10_000)
}

function errorMessage(error: unknown): string {
  const code = extractApiErrorCode(error)
  const key = `rewardTasks.errors.${code}`
  return code && te(key) ? t(key, { remaining_seconds: 0, ...extractApiErrorMetadata(error) }) : t('rewardTasks.errors.fallback')
}

async function refreshBalance(): Promise<void> {
  try { await authStore.refreshUser({ fresh: true }) } catch (error) {
    console.error('Failed to refresh user after reward task:', error)
  }
}

async function loadRecords(page = recordsPage.value): Promise<void> {
  const version = ++recordsVersion
  recordsLoading.value = true
  recordsError.value = false
  try {
    const result = await listMyRewardTaskRecords(page, pageSize)
    if (disposed || version !== recordsVersion) return
    records.value = result
    recordsPage.value = page
  } catch {
    if (!disposed && version === recordsVersion) recordsError.value = true
  } finally {
    if (!disposed && version === recordsVersion) recordsLoading.value = false
  }
}

function changeRecordsPage(page: number): void {
  void loadRecords(page)
}

function loadTasks(silent = false, detectCompletion = true): Promise<void> {
  if (listPending) return listPending
  const version = ++listVersion
  if (!silent && !data.value) loading.value = true
  listPending = (async () => {
    try {
      const result = await listRewardTasks()
      if (disposed || version !== listVersion) return
      const completed = detectCompletion && data.value && result.summary.completed_count > data.value.summary.completed_count
      data.value = result
      now.value = Date.now()
      loadError.value = false
      if (completed) {
        appStore.showSuccess(t('rewardTasks.autoCompleted'))
        await Promise.all([refreshBalance(), loadRecords(1)])
      }
    } catch {
      if (disposed || version !== listVersion) return
      if (!data.value) loadError.value = true
      else if (!silent) appStore.showError(t('rewardTasks.refreshFailed'))
    } finally {
      listPending = null
      if (!disposed) {
        loading.value = false
        configureTimers()
      }
    }
  })()
  return listPending
}

async function refreshAll(): Promise<void> {
  if (refreshing.value) return
  refreshing.value = true
  try { await Promise.all([loadTasks(), loadRecords()]) } finally { refreshing.value = false }
}

async function handleActionError(error: unknown): Promise<void> {
  appStore.showError(errorMessage(error))
  if (extractApiErrorCode(error) === 'REWARD_TASKS_DISABLED' && data.value) {
    data.value.enabled = false
    data.value.tasks = []
    configureTimers()
  } else await loadTasks(true)
}

async function start(task: UserRewardTask): Promise<void> {
  if (busy.value[task.id]) return
  // 必须在点击事件内同步开窗，等待接口返回后再导航，避免浏览器拦截异步弹窗。
  let win: Window | null = null
  try { win = window.open('about:blank', '_blank') } catch { /* 拦截时使用卡片中的链接。 */ }
  busy.value[task.id] = true
  delete blockedLinks.value[task.id]
  try {
    const result = await startRewardTask(task.id)
    if (disposed) { win?.close(); return }
    const url = new URL(result.jump_url)
    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      win?.close()
      appStore.showError(t('rewardTasks.errors.invalidJumpUrl'))
      return
    }
    ++listVersion
    const displayedTask = data.value?.tasks.find(item => item.id === task.id)
    if (displayedTask) displayedTask.current = result.current
    now.value = Date.now()
    if (win && !win.closed) {
      win.location.href = result.jump_url
      win.opener = null
    } else {
      blockedLinks.value[task.id] = result.jump_url
      appStore.showInfo(t('rewardTasks.popupBlocked'))
    }
  } catch (error) {
    win?.close()
    if (!disposed) await handleActionError(error)
  } finally {
    busy.value[task.id] = false
    configureTimers()
  }
}

async function claim(task: UserRewardTask): Promise<void> {
  if (busy.value[task.id]) return
  busy.value[task.id] = true
  try {
    const result = await claimRewardTask(task.id)
    if (disposed) return
    ++listVersion
    const displayedTask = data.value?.tasks.find(item => item.id === task.id)
    if (displayedTask) displayedTask.current = result.current
    appStore.showSuccess(t('rewardTasks.claimSuccess', { amount: formatReward(result.reward_amount) }))
    if (listPending) await listPending
    if (!disposed) await Promise.all([loadTasks(true, false), loadRecords(1), refreshBalance()])
  } catch (error) {
    if (!disposed) await handleActionError(error)
  } finally {
    busy.value[task.id] = false
    configureTimers()
  }
}

function onVisibilityChange(): void {
  now.value = Date.now()
  configureTimers()
  if (document.visibilityState === 'visible') void loadTasks(true)
}

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  void refreshAll()
})

onBeforeUnmount(() => {
  disposed = true
  ++listVersion
  ++recordsVersion
  clearTimers()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>
