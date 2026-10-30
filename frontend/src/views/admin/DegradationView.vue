<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-blue-50 text-blue-500 dark:bg-blue-900/30 dark:text-blue-400">
            <Icon name="chart" size="sm" />
          </span>
          {{ t('admin.degradation.title') }}
        </h1>
        <p class="page-description mt-1.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.degradation.description') }}
        </p>
        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <nav class="tabs inline-flex w-full max-w-xl sm:w-auto" role="tablist" :aria-label="t('admin.degradation.tabs.aria')">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="activeTab === tab.value ? 'tab-active' : ''"
              :aria-selected="activeTab === tab.value"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </nav>
        </div>
      </header>

      <TablePageLayout v-if="activeTab === 'targets'">
        <template #filters>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.degradation.targets.summary', { count: targets.length }) }}</p>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary" :disabled="targetsLoading" @click="loadTargets">
                <Icon name="refresh" size="sm" :class="targetsLoading ? 'animate-spin' : ''" />
                {{ t('common.refresh') }}
              </button>
              <button type="button" class="btn btn-primary" @click="openCreateTarget">
                <Icon name="plus" size="sm" />
                {{ t('admin.degradation.targets.create') }}
              </button>
            </div>
          </div>
        </template>
        <template #table>
          <DataTable :columns="targetColumns" :data="targets" :loading="targetsLoading" row-key="id">
            <template #cell-group_name="{ row }">
              <span class="font-medium text-gray-900 dark:text-white">{{ row.group_name }}</span>
            </template>
            <template #cell-platform="{ value }">
              <span class="badge badge-gray">{{ value }}</span>
            </template>
            <template #cell-model="{ value }">
              <span class="font-mono text-xs text-gray-800 dark:text-gray-200">{{ value }}</span>
            </template>
            <template #cell-interval_seconds="{ value }">
              {{ t('admin.degradation.intervalValue', { seconds: value }) }}
            </template>
            <template #cell-probe_count="{ row }">
              <span>{{ targetProbeCount(row) }}</span>
              <span v-if="row.visual_probe_count > 0" class="badge badge-primary ml-1">{{ t('admin.degradation.targets.visualCount', { count: row.visual_probe_count }) }}</span>
            </template>
            <template #cell-last_run_at="{ value }">
              {{ formatTime(value) }}
            </template>
            <template #cell-verdict="{ row }">
              <span class="badge" :class="verdictBadgeClass(targetVerdict(row))">
                {{ t(`degradation.verdict.${targetVerdict(row)}`) }}
              </span>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-1">
                <button type="button" class="btn btn-ghost btn-sm" :disabled="runningTargetId !== null" @click="handleRunTarget(row)">
                  {{ runningTargetId === row.id ? t('admin.degradation.targets.running') : t('admin.degradation.targets.runNow') }}
                </button>
                <button type="button" class="btn btn-ghost btn-sm" @click="openEditTarget(row)">{{ t('common.edit') }}</button>
                <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="requestDeleteTarget(row)">{{ t('common.delete') }}</button>
              </div>
            </template>
            <template #empty>
              <EmptyState
                :title="targetsLoadFailed ? t('admin.degradation.targets.loadFailed') : t('admin.degradation.targets.empty')"
                :description="targetsLoadFailed ? t('admin.degradation.loadFailedDescription') : t('admin.degradation.targets.emptyDescription')"
                :action-text="targetsLoadFailed ? t('degradation.retry') : t('admin.degradation.targets.create')"
                @action="targetsLoadFailed ? loadTargets() : openCreateTarget()"
              />
            </template>
          </DataTable>
        </template>
      </TablePageLayout>

      <TablePageLayout v-else-if="activeTab === 'probes'">
        <template #filters>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.degradation.probes.summary', { count: probes.length }) }}</p>
            <div class="flex gap-2">
              <button type="button" class="btn btn-secondary" :disabled="probesLoading" @click="loadProbes">
                <Icon name="refresh" size="sm" :class="probesLoading ? 'animate-spin' : ''" />
                {{ t('common.refresh') }}
              </button>
              <button type="button" class="btn btn-primary" @click="openCreateProbe">
                <Icon name="plus" size="sm" />
                {{ t('admin.degradation.probes.create') }}
              </button>
            </div>
          </div>
        </template>
        <template #table>
          <DataTable :columns="probeColumns" :data="probes" :loading="probesLoading" row-key="id">
            <template #cell-name="{ row }">
              <div>
                <div class="flex items-center gap-2">
                  <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
                  <span class="badge shrink-0" :class="row.probe_type === 'visual' ? 'badge-primary' : 'badge-gray'">
                    {{ row.probe_type === 'visual' ? t('admin.degradation.probes.typeVisual') : t('admin.degradation.probes.typeRegex') }}
                  </span>
                </div>
                <p class="mt-0.5 max-w-xs truncate text-xs text-gray-400" :title="row.prompt">{{ row.prompt }}</p>
              </div>
            </template>
            <template #cell-expect_regex="{ value }">
              <code class="block max-w-md truncate rounded bg-gray-50 px-2 py-1 text-xs text-gray-700 dark:bg-dark-900 dark:text-gray-300" :title="value">{{ value }}</code>
            </template>
            <template #cell-regex_flags="{ value }">
              <span class="font-mono text-xs">{{ value || '-' }}</span>
            </template>
            <template #cell-negate="{ value }">
              {{ yesNo(value) }}
            </template>
            <template #cell-enabled="{ value }">
              <span class="badge" :class="value ? 'badge-success' : 'badge-gray'">
                {{ value ? t('admin.degradation.enabled') : t('admin.degradation.disabled') }}
              </span>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-1">
                <button type="button" class="btn btn-ghost btn-sm" @click="openEditProbe(row)">{{ t('common.edit') }}</button>
                <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="requestDeleteProbe(row)">{{ t('common.delete') }}</button>
              </div>
            </template>
            <template #empty>
              <EmptyState
                :title="probesLoadFailed ? t('admin.degradation.probes.loadFailed') : t('admin.degradation.probes.empty')"
                :description="probesLoadFailed ? t('admin.degradation.loadFailedDescription') : t('admin.degradation.probes.emptyDescription')"
                :action-text="probesLoadFailed ? t('degradation.retry') : t('admin.degradation.probes.create')"
                @action="probesLoadFailed ? loadProbes() : openCreateProbe()"
              />
            </template>
          </DataTable>
        </template>
      </TablePageLayout>

      <section v-else class="space-y-5">
        <div class="flex flex-wrap items-center justify-between gap-3">
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
          <button type="button" class="btn btn-secondary" :disabled="overviewLoading" @click="loadOverview">
            <Icon name="refresh" size="sm" :class="overviewLoading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="overviewLoading && !overview" class="card flex min-h-64 items-center justify-center !rounded-3xl text-sm text-gray-400 dark:!bg-dark-800" role="status">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="overviewLoadFailed && resultGroups.length === 0" class="card !rounded-3xl dark:!bg-dark-800">
          <EmptyState
            :title="t('admin.degradation.overview.loadFailed')"
            :description="t('admin.degradation.loadFailedDescription')"
            :action-text="t('degradation.retry')"
            @action="loadOverview"
          />
        </div>
        <div v-else-if="resultGroups.length === 0" class="card !rounded-3xl dark:!bg-dark-800">
          <EmptyState :title="t('degradation.emptyTitle')" :description="t('admin.degradation.overview.emptyDescription')" />
        </div>
        <article v-for="group in resultGroups" v-else :key="group.group_id" class="card overflow-hidden !rounded-3xl !border-0 p-0 shadow-sm ring-1 ring-gray-900/5 dark:!bg-dark-800 dark:ring-dark-700">
          <header class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
            <div>
              <h2 class="text-base font-bold text-gray-900 dark:text-white">{{ group.group_name }}</h2>
              <p class="mt-1 text-xs text-gray-400">{{ group.platform }} · {{ t('degradation.lastChecked', { time: formatTime(group.last_checked_at) }) }}</p>
            </div>
            <span class="badge" :class="verdictBadgeClass(group.verdict)">{{ t(`degradation.verdict.${group.verdict}`) }}</span>
          </header>
          <div class="table-container border-0">
            <table class="table min-w-[760px]">
              <thead>
                <tr>
                  <th>{{ t('admin.degradation.fields.model') }}</th>
                  <th>{{ t('degradation.passRate') }}</th>
                  <th>{{ t('degradation.samples') }}</th>
                  <th>{{ t('degradation.avgLatency') }}</th>
                  <th>{{ t('admin.degradation.overview.lastChecked') }}</th>
                  <th>{{ t('admin.degradation.fields.verdict') }}</th>
                  <th>{{ t('admin.degradation.fields.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="model in group.models" :key="model.model">
                  <td><span class="font-mono text-xs font-semibold text-gray-900 dark:text-white">{{ model.model }}</span></td>
                  <td>
                    <div class="flex min-w-44 items-center gap-2">
                      <span class="h-2 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                        <span class="block h-full rounded-full" :class="verdictBarClass(model.verdict)" :style="{ width: `${passRatePercent(model.pass_rate)}%` }"></span>
                      </span>
                      <span class="text-sm font-medium tabular-nums">{{ formatPercent(model.pass_rate) }}</span>
                    </div>
                  </td>
                  <td class="tabular-nums">{{ model.total }}</td>
                  <td class="tabular-nums">{{ formatLatency(model.avg_latency_ms) }}</td>
                  <td>{{ formatTime(model.last_checked_at) }}</td>
                  <td><span class="badge" :class="verdictBadgeClass(model.verdict)">{{ t(`degradation.verdict.${model.verdict}`) }}</span></td>
                  <td>
                    <button type="button" class="btn btn-ghost btn-sm" @click="openModelRuns(group.group_id, model.model)">
                      {{ t('admin.degradation.overview.viewRuns') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </article>
      </section>
    </div>

    <TargetFormDialog
      :show="showTargetDialog"
      :target="editingTarget"
      :probes="probes"
      @close="closeTargetDialog"
      @saved="handleTargetSaved"
    />
    <ProbeFormDialog
      :show="showProbeDialog"
      :probe="editingProbe"
      @close="closeProbeDialog"
      @saved="handleProbeSaved"
    />
    <TargetRunsDialog
      :show="showRunsDialog"
      :target="runsDialogTarget"
      :runs="runs"
      :loading="runsLoading"
      @close="closeRunsDialog"
    />
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TargetFormDialog from '@/components/admin/degradation/TargetFormDialog.vue'
import ProbeFormDialog from '@/components/admin/degradation/ProbeFormDialog.vue'
import TargetRunsDialog from '@/components/admin/degradation/TargetRunsDialog.vue'
import { adminAPI } from '@/api/admin'
import type { DegradationProbe, DegradationRun, DegradationTarget } from '@/api/admin/degradation'
import type { DegradationModelOverview, DegradationOverview, DegradationVerdict } from '@/api/degradation'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type AdminTab = 'targets' | 'probes' | 'results'
type DeleteKind = 'target' | 'probe'

const { t, locale } = useI18n()
const appStore = useAppStore()
const activeTab = ref<AdminTab>('targets')
const targets = ref<DegradationTarget[]>([])
const probes = ref<DegradationProbe[]>([])
const overview = ref<DegradationOverview | null>(null)
const targetsLoading = ref(false)
const probesLoading = ref(false)
const overviewLoading = ref(false)
const targetsLoadFailed = ref(false)
const probesLoadFailed = ref(false)
const overviewLoadFailed = ref(false)
const windowHours = ref(24)
const runningTargetId = ref<number | null>(null)
let runPollTimer: number | null = null
let runPollSequence = 0
let runPollInFlight = false
const showTargetDialog = ref(false)
const showProbeDialog = ref(false)
const editingTarget = ref<DegradationTarget | null>(null)
const editingProbe = ref<DegradationProbe | null>(null)
const showRunsDialog = ref(false)
const runsDialogTarget = ref<DegradationTarget | null>(null)
const runs = ref<DegradationRun[]>([])
const runsLoading = ref(false)
const showDeleteDialog = ref(false)
const deleteKind = ref<DeleteKind>('target')
const deletingTarget = ref<DegradationTarget | null>(null)
const deletingProbe = ref<DegradationProbe | null>(null)
const deleteLoading = ref(false)
let targetSequence = 0
let probeSequence = 0
let overviewSequence = 0
let runsSequence = 0

const tabs = computed(() => [
  { value: 'targets' as AdminTab, label: t('admin.degradation.tabs.targets') },
  { value: 'probes' as AdminTab, label: t('admin.degradation.tabs.probes') },
  { value: 'results' as AdminTab, label: t('admin.degradation.tabs.results') },
])
const windowOptions = computed(() => [
  { value: 24, label: t('degradation.windows.24h') },
  { value: 72, label: t('degradation.windows.72h') },
  { value: 168, label: t('degradation.windows.7d') },
])
const targetColumns = computed<Column[]>(() => [
  { key: 'group_name', label: t('admin.degradation.fields.group'), sortable: false },
  { key: 'platform', label: t('admin.degradation.fields.platform'), sortable: false },
  { key: 'model', label: t('admin.degradation.fields.model'), sortable: false },
  { key: 'interval_seconds', label: t('admin.degradation.fields.interval'), sortable: false },
  { key: 'probe_count', label: t('admin.degradation.fields.probeCount'), sortable: false },
  { key: 'last_run_at', label: t('admin.degradation.fields.lastRun'), sortable: false },
  { key: 'verdict', label: t('admin.degradation.fields.verdict'), sortable: false },
  { key: 'actions', label: t('admin.degradation.fields.actions'), sortable: false },
])
const probeColumns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.degradation.fields.name'), sortable: false },
  { key: 'expect_regex', label: t('admin.degradation.fields.regex'), sortable: false },
  { key: 'regex_flags', label: t('admin.degradation.fields.flags'), sortable: false },
  { key: 'negate', label: t('admin.degradation.fields.negate'), sortable: false },
  { key: 'enabled', label: t('admin.degradation.fields.enabled'), sortable: false },
  { key: 'sort_order', label: t('admin.degradation.fields.sortOrder'), sortable: false },
  { key: 'actions', label: t('admin.degradation.fields.actions'), sortable: false },
])
const enabledProbeCount = computed(() => probes.value.filter((probe) => probe.enabled).length)
const resultGroups = computed(() => overview.value?.groups ?? [])
const modelOverviewMap = computed(() => {
  const map = new Map<string, DegradationModelOverview>()
  for (const group of resultGroups.value) {
    for (const model of group.models) map.set(`${group.group_id}:${model.model}`, model)
  }
  return map
})
const deleteConfirmMessage = computed(() => {
  if (deleteKind.value === 'target') {
    return t('admin.degradation.targets.deleteConfirm', {
      name: `${deletingTarget.value?.group_name ?? ''} / ${deletingTarget.value?.model ?? ''}`,
    })
  }
  return t('admin.degradation.probes.deleteConfirm', { name: deletingProbe.value?.name ?? '' })
})

async function loadTargets() {
  const sequence = ++targetSequence
  targetsLoading.value = true
  try {
    const result = await adminAPI.degradation.listTargets()
    if (sequence !== targetSequence) return
    targets.value = result.items ?? []
    targetsLoadFailed.value = false
  } catch (error) {
    if (sequence !== targetSequence) return
    targetsLoadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.loadFailed')))
  } finally {
    if (sequence === targetSequence) targetsLoading.value = false
  }
}

async function loadProbes() {
  const sequence = ++probeSequence
  probesLoading.value = true
  try {
    const result = await adminAPI.degradation.listProbes()
    if (sequence !== probeSequence) return
    probes.value = result.items ?? []
    probesLoadFailed.value = false
  } catch (error) {
    if (sequence !== probeSequence) return
    probesLoadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.probes.loadFailed')))
  } finally {
    if (sequence === probeSequence) probesLoading.value = false
  }
}

async function loadOverview() {
  const sequence = ++overviewSequence
  overviewLoading.value = true
  try {
    const result = await adminAPI.degradation.getOverview({ window_hours: windowHours.value })
    if (sequence !== overviewSequence) return
    overview.value = { ...result, groups: result.groups ?? [] }
    overviewLoadFailed.value = false
  } catch (error) {
    if (sequence !== overviewSequence) return
    overviewLoadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.overview.loadFailed')))
  } finally {
    if (sequence === overviewSequence) overviewLoading.value = false
  }
}

function setWindow(hours: number) {
  if (windowHours.value === hours) return
  windowHours.value = hours
  void loadOverview()
}

function targetProbeCount(target: DegradationTarget) {
  if (typeof target.enabled_probe_count === 'number') return target.enabled_probe_count
  if (target.probe_ids.length === 0) return enabledProbeCount.value
  const enabledIds = new Set(probes.value.filter((probe) => probe.enabled).map((probe) => probe.id))
  return target.probe_ids.filter((id) => enabledIds.has(id)).length
}

function targetVerdict(target: DegradationTarget): DegradationVerdict {
  return modelOverviewMap.value.get(`${target.group_id}:${target.model}`)?.verdict ?? 'unknown'
}

function openCreateTarget() {
  editingTarget.value = null
  showTargetDialog.value = true
}

function openEditTarget(target: DegradationTarget) {
  editingTarget.value = target
  showTargetDialog.value = true
}

function closeTargetDialog() {
  showTargetDialog.value = false
  editingTarget.value = null
}

function handleTargetSaved() {
  void Promise.all([loadTargets(), loadOverview()])
}

function openCreateProbe() {
  editingProbe.value = null
  showProbeDialog.value = true
}

function openEditProbe(probe: DegradationProbe) {
  editingProbe.value = probe
  showProbeDialog.value = true
}

function closeProbeDialog() {
  showProbeDialog.value = false
  editingProbe.value = null
}

function handleProbeSaved() {
  void loadProbes()
}

function clearRunPollTimer() {
  if (runPollTimer !== null) {
    window.clearTimeout(runPollTimer)
    runPollTimer = null
  }
}

function stopRunPolling() {
  clearRunPollTimer()
  runPollSequence += 1
  runPollInFlight = false
  runningTargetId.value = null
}

function finishRunPolling(targetId: number, sequence: number, completed: boolean) {
  if (sequence !== runPollSequence || runningTargetId.value !== targetId) return
  clearRunPollTimer()
  runPollSequence += 1
  runPollInFlight = false
  runningTargetId.value = null
  if (completed) appStore.showSuccess(t('admin.degradation.targets.runCompleted'))
  else appStore.showError(t('admin.degradation.targets.runTimeout'))
  void Promise.all([loadTargets(), loadOverview()])
}

function startRunPolling(target: DegradationTarget, result: { probe_count: number; started_at: string }) {
  const sequence = ++runPollSequence
  const startedAt = Date.parse(result.started_at)
  const deadline = Date.now() + 15 * 60 * 1000
  let errorShown = false
  runPollInFlight = false

  const poll = async () => {
    if (sequence !== runPollSequence || runningTargetId.value !== target.id) return
    if (Date.now() >= deadline) {
      finishRunPolling(target.id, sequence, false)
      return
    }
    if (runPollInFlight) {
      runPollTimer = window.setTimeout(() => void poll(), 5000)
      return
    }
    runPollInFlight = true
    try {
      const response = await adminAPI.degradation.listTargetRuns(target.id, { limit: Math.max(50, result.probe_count) })
      if (sequence !== runPollSequence) return
      const completedCount = (response.items ?? []).filter((run) => {
        const createdAt = Date.parse(run.created_at)
        return Number.isFinite(createdAt) && createdAt >= startedAt
      }).length
      if (completedCount >= result.probe_count) {
        finishRunPolling(target.id, sequence, true)
        return
      }
    } catch (error) {
      if (sequence === runPollSequence && !errorShown) {
        errorShown = true
        appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.runPollFailed')))
      }
    } finally {
      if (sequence === runPollSequence) runPollInFlight = false
    }
    if (sequence !== runPollSequence) return
    runPollTimer = window.setTimeout(() => void poll(), 5000)
  }

  if (result.probe_count === 0) {
    finishRunPolling(target.id, sequence, true)
    return
  }
  runPollTimer = window.setTimeout(() => void poll(), 5000)
}

async function handleRunTarget(target: DegradationTarget) {
  if (runningTargetId.value !== null) return
  const sequence = ++runPollSequence
  runningTargetId.value = target.id
  try {
    const result = await adminAPI.degradation.runTarget(target.id)
    if (sequence !== runPollSequence) return
    if (!result.accepted) {
      runningTargetId.value = null
      appStore.showError(t('admin.degradation.targets.runFailed'))
      return
    }
    appStore.showSuccess(t('admin.degradation.targets.runStarted'))
    startRunPolling(target, result)
  } catch (error) {
    if (sequence === runPollSequence) {
      runningTargetId.value = null
      appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.runFailed')))
    }
  }
}

async function openModelRuns(groupId: number, model: string) {
  const target = targets.value.find((item) => item.group_id === groupId && item.model === model)
  if (!target) {
    appStore.showError(t('admin.degradation.overview.targetMissing'))
    return
  }
  const sequence = ++runsSequence
  runsDialogTarget.value = target
  runs.value = []
  runsLoading.value = true
  showRunsDialog.value = true
  try {
    const result = await adminAPI.degradation.listTargetRuns(target.id, { limit: 50 })
    if (sequence === runsSequence) runs.value = result.items ?? []
  } catch (error) {
    if (sequence === runsSequence) {
      appStore.showError(extractApiErrorMessage(error, t('admin.degradation.runs.loadFailed')))
      closeRunsDialog()
    }
  } finally {
    if (sequence === runsSequence) runsLoading.value = false
  }
}

function closeRunsDialog() {
  runsSequence += 1
  showRunsDialog.value = false
  runsDialogTarget.value = null
  runsLoading.value = false
  runs.value = []
}

function requestDeleteTarget(target: DegradationTarget) {
  deleteKind.value = 'target'
  deletingTarget.value = target
  showDeleteDialog.value = true
}

function requestDeleteProbe(probe: DegradationProbe) {
  deleteKind.value = 'probe'
  deletingProbe.value = probe
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (deleteLoading.value) return
  deleteLoading.value = true
  try {
    if (deleteKind.value === 'target' && deletingTarget.value) {
      await adminAPI.degradation.deleteTarget(deletingTarget.value.id)
      appStore.showSuccess(t('admin.degradation.targets.deleteSuccess'))
      void Promise.all([loadTargets(), loadOverview()])
    } else if (deleteKind.value === 'probe' && deletingProbe.value) {
      await adminAPI.degradation.deleteProbe(deletingProbe.value.id)
      appStore.showSuccess(t('admin.degradation.probes.deleteSuccess'))
      void loadProbes()
    }
    showDeleteDialog.value = false
    deletingTarget.value = null
    deletingProbe.value = null
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.deleteFailed')))
  } finally {
    deleteLoading.value = false
  }
}

function yesNo(value: boolean) {
  return value ? t('admin.degradation.yes') : t('admin.degradation.no')
}

function passRatePercent(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, value * 100))
}

function formatPercent(value: number) {
  return `${passRatePercent(value).toFixed(1)}%`
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
  void Promise.all([loadTargets(), loadProbes(), loadOverview()])
})

onBeforeUnmount(() => {
  stopRunPolling()
  targetSequence += 1
  probeSequence += 1
  overviewSequence += 1
  runsSequence += 1
})
</script>
