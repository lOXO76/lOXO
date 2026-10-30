<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <section class="card !rounded-3xl !border-0 p-5 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700 sm:p-6">
        <header class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
                <Icon name="chart" size="sm" />
              </span>
              {{ t('degradation.title') }}
            </h1>
            <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('degradation.description') }}
            </p>
            <p v-if="overview?.generated_at" class="mt-2 text-xs text-gray-400 dark:text-gray-500">
              {{ t('degradation.lastUpdated', { time: formatTime(overview.generated_at) }) }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <div class="tabs inline-flex" role="group" :aria-label="t('degradation.windowLabel')">
              <button
                v-for="option in windowOptions"
                :key="option.value"
                type="button"
                class="tab !px-3 !py-1.5 text-xs"
                :class="windowHours === option.value ? 'tab-active' : ''"
                :aria-pressed="windowHours === option.value"
                @click="setWindow(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-icon flex h-9 w-9 items-center justify-center"
              :title="t('common.refresh')"
              :disabled="loading || refreshing"
              @click="loadOverview(true)"
            >
              <Icon name="refresh" size="sm" :class="refreshing ? 'animate-spin' : ''" />
            </button>
          </div>
        </header>
      </section>

      <div v-if="loadError && overview" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300" role="alert">
        {{ t('degradation.loadFailed') }}
      </div>

      <section v-if="loading && !overview" class="card flex min-h-64 items-center justify-center !rounded-3xl text-sm text-gray-400 dark:!bg-dark-800" role="status">
        <LoadingSpinner size="md" class="mr-2" />
        {{ t('common.loading') }}
      </section>

      <section v-else-if="loadError && !overview" class="card !rounded-3xl dark:!bg-dark-800">
        <EmptyState
          data-testid="degradation-error"
          :title="t('degradation.loadFailed')"
          :description="t('degradation.loadFailedDescription')"
          :action-text="t('degradation.retry')"
          @action="loadOverview(true)"
        />
      </section>

      <section v-else-if="!overview?.enabled || groups.length === 0" class="card !rounded-3xl dark:!bg-dark-800">
        <EmptyState
          data-testid="degradation-empty"
          :title="t('degradation.emptyTitle')"
          :description="t('degradation.emptyDescription')"
        />
      </section>

      <div v-else class="grid gap-5 xl:grid-cols-2">
        <article
          v-for="group in groups"
          :key="group.group_id"
          class="card overflow-hidden !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700"
          data-testid="degradation-group"
        >
          <header class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <h2 class="truncate text-base font-bold text-gray-900 dark:text-white">{{ group.group_name }}</h2>
                <span class="badge badge-gray shrink-0">{{ group.platform }}</span>
              </div>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                {{ t('degradation.lastChecked', { time: formatTime(group.last_checked_at) }) }}
              </p>
            </div>
            <span class="badge" :class="isVisualGroup(group) ? 'badge-gray' : verdictBadgeClass(group.verdict)">
              {{ isVisualGroup(group) ? t('degradation.visual.badge') : t(`degradation.verdict.${group.verdict}`) }}
            </span>
          </header>

          <div class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="model in group.models" :key="model.model" class="px-5 py-4">
              <div class="flex flex-wrap items-center justify-between gap-2">
                <strong class="min-w-0 break-all text-sm font-semibold text-gray-900 dark:text-white">{{ model.model }}</strong>
                <span class="badge shrink-0" :class="isPureVisualModel(model) ? 'badge-gray' : verdictBadgeClass(model.verdict)">
                  {{ isPureVisualModel(model) ? t('degradation.visual.badge') : t(`degradation.verdict.${model.verdict}`) }}
                </span>
              </div>
              <template v-if="!isPureVisualModel(model)">
                <div class="mt-3 flex items-center gap-3">
                  <div
                    class="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700"
                    role="progressbar"
                    :aria-label="t('degradation.passRate')"
                    :aria-valuenow="passRatePercent(model.pass_rate)"
                    aria-valuemin="0"
                    aria-valuemax="100"
                  >
                    <div
                      class="h-full rounded-full transition-[width]"
                      :class="verdictBarClass(model.verdict)"
                      :style="{ width: `${passRatePercent(model.pass_rate)}%` }"
                    ></div>
                  </div>
                  <span class="w-14 text-right text-sm font-semibold tabular-nums text-gray-900 dark:text-gray-100">
                    {{ formatPercent(model.pass_rate) }}
                  </span>
                </div>
                <dl class="mt-3 grid grid-cols-2 gap-3 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-3">
                  <div>
                    <dt class="text-gray-400 dark:text-gray-500">{{ t('degradation.samples') }}</dt>
                    <dd class="mt-0.5 font-medium tabular-nums text-gray-700 dark:text-gray-300">{{ model.total }}</dd>
                  </div>
                  <div>
                    <dt class="text-gray-400 dark:text-gray-500">{{ t('degradation.avgLatency') }}</dt>
                    <dd class="mt-0.5 font-medium tabular-nums text-gray-700 dark:text-gray-300">{{ formatLatency(model.avg_latency_ms) }}</dd>
                  </div>
                  <div class="col-span-2 sm:col-span-1">
                    <dt class="text-gray-400 dark:text-gray-500">{{ t('degradation.sampleBreakdown') }}</dt>
                    <dd class="mt-0.5 font-medium tabular-nums text-gray-700 dark:text-gray-300">
                      {{ t('degradation.sampleCounts', { passed: model.passed, failed: model.failed, error: model.error }) }}
                    </dd>
                  </div>
                </dl>
              </template>

              <div v-if="model.showcases.length > 0" class="mt-5 space-y-3" data-testid="visual-showcases">
                <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('degradation.visual.showcases') }}</p>
                <div
                  v-for="showcase in model.showcases"
                  :key="showcase.run_id"
                  class="rounded-2xl bg-gray-50 p-3 dark:bg-dark-900/40"
                  data-testid="visual-showcase"
                >
                  <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
                    <div class="min-w-0">
                      <h3 class="truncate text-sm font-semibold text-gray-800 dark:text-gray-200">{{ showcase.probe_name }}</h3>
                      <p class="mt-0.5 text-xs text-gray-400">{{ showcase.artifact_kind }} · {{ formatLatency(showcase.latency_ms) }}</p>
                    </div>
                    <button type="button" class="btn btn-secondary btn-sm" @click="openHistory(group.group_id, model.model, showcase)">
                      {{ t('degradation.visual.historyCompare') }}
                    </button>
                  </div>
                  <VisualArtifactFrame :run-id="showcase.run_id" scope="user" :title="showcase.probe_name" />
                </div>
              </div>
            </div>
          </div>
        </article>
      </div>
    </div>
    <VisualHistoryDialog
      :show="showHistory"
      :group-id="historyGroupId"
      :model="historyModel"
      :probe-id="historyProbeId"
      :title="historyTitle"
      @close="showHistory = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import VisualArtifactFrame from '@/components/degradation/VisualArtifactFrame.vue'
import VisualHistoryDialog from '@/components/degradation/VisualHistoryDialog.vue'
import { degradationAPI } from '@/api/degradation'
import type {
  DegradationGroupOverview,
  DegradationModelOverview,
  DegradationOverview,
  DegradationShowcase,
  DegradationVerdict,
} from '@/api/degradation'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()
const windowHours = ref(24)
const overview = ref<DegradationOverview | null>(null)
const loading = ref(false)
const refreshing = ref(false)
const loadError = ref(false)
const showHistory = ref(false)
const historyGroupId = ref(0)
const historyModel = ref('')
const historyProbeId = ref(0)
const historyTitle = ref('')
let requestSequence = 0
let refreshTimer: number | null = null

const windowOptions = computed(() => [
  { value: 24, label: t('degradation.windows.24h') },
  { value: 72, label: t('degradation.windows.72h') },
  { value: 168, label: t('degradation.windows.7d') },
])
const groups = computed(() => overview.value?.groups ?? [])

function isPureVisualModel(model: DegradationModelOverview) {
  return model.total === 0 && model.showcases.length > 0
}

function isVisualGroup(group: DegradationGroupOverview) {
  return group.models.length > 0 && group.models.every(isPureVisualModel)
}

function openHistory(groupId: number, model: string, showcase: DegradationShowcase) {
  historyGroupId.value = groupId
  historyModel.value = model
  historyProbeId.value = showcase.probe_id
  historyTitle.value = showcase.probe_name
  showHistory.value = true
}

async function loadOverview(showLoading = false) {
  const sequence = ++requestSequence
  if (showLoading) loading.value = true
  else refreshing.value = true
  try {
    const result = await degradationAPI.getOverview({ window_hours: windowHours.value })
    if (sequence !== requestSequence) return
    overview.value = {
      ...result,
      groups: (result.groups ?? []).map((group) => ({
        ...group,
        models: (group.models ?? []).map((model) => ({ ...model, showcases: model.showcases ?? [] })),
      })),
    }
    loadError.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadError.value = true
    appStore.showError(extractApiErrorMessage(error, t('degradation.loadFailed')))
  } finally {
    if (sequence === requestSequence) {
      loading.value = false
      refreshing.value = false
    }
  }
}

function setWindow(hours: number) {
  if (windowHours.value === hours) return
  windowHours.value = hours
  void loadOverview(true)
}

function stopAutoRefresh() {
  if (refreshTimer !== null) {
    window.clearInterval(refreshTimer)
    refreshTimer = null
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  if (document.visibilityState === 'hidden') return
  refreshTimer = window.setInterval(() => {
    if (!loading.value && !refreshing.value) void loadOverview(false)
  }, 60_000)
}

function handleVisibilityChange() {
  if (document.visibilityState === 'hidden') {
    stopAutoRefresh()
    return
  }
  void loadOverview(false)
  startAutoRefresh()
}

function passRatePercent(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value * 100))
}

function formatPercent(value: number) {
  return `${(passRatePercent(value)).toFixed(1)}%`
}

function formatLatency(value: number) {
  if (!Number.isFinite(value) || value < 0) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`
}

function formatTime(value: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(locale.value || undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function verdictBadgeClass(verdict: DegradationVerdict) {
  return {
    healthy: 'badge-success',
    degraded: 'badge-danger',
    unknown: 'badge-gray',
  }[verdict]
}

function verdictBarClass(verdict: DegradationVerdict) {
  return {
    healthy: 'bg-green-500 dark:bg-green-400',
    degraded: 'bg-red-500 dark:bg-red-400',
    unknown: 'bg-gray-400 dark:bg-gray-500',
  }[verdict]
}

onMounted(() => {
  void loadOverview(true)
  startAutoRefresh()
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  requestSequence += 1
  stopAutoRefresh()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>
