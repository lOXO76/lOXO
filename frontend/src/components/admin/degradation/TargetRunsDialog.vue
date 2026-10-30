<template>
  <BaseDialog
    :show="show"
    :title="target ? t('admin.degradation.runs.title', { group: target.group_name, model: target.model }) : t('admin.degradation.runs.fallbackTitle')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div v-if="loading" class="flex min-h-48 items-center justify-center text-sm text-gray-400" role="status">
      <LoadingSpinner size="md" class="mr-2" />
      {{ t('common.loading') }}
    </div>
    <EmptyState
      v-else-if="runs.length === 0"
      :title="t('admin.degradation.runs.empty')"
      :description="t('admin.degradation.runs.emptyDescription')"
    />
    <div v-else class="space-y-3">
      <article v-for="run in runs" :key="run.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <header class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h4 class="break-words text-sm font-semibold text-gray-900 dark:text-white">{{ run.probe_name }}</h4>
            <p class="mt-1 text-xs text-gray-400">
              {{ formatTime(run.created_at) }} · {{ t('admin.degradation.runs.latency', { value: run.latency_ms }) }} · {{ t('admin.degradation.runs.account', { id: run.account_id }) }}
            </p>
          </div>
          <span class="badge shrink-0" :class="statusBadgeClass(run.status)">{{ t(`admin.degradation.runStatus.${run.status}`) }}</span>
        </header>
        <div v-if="run.probe_type === 'visual'" class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
          <span class="badge badge-primary">{{ t('admin.degradation.probes.typeVisual') }}</span>
          <span>{{ run.artifact_kind || '-' }}</span>
          <span>{{ formatBytes(run.artifact_bytes) }}</span>
        </div>
        <details v-if="run.status === 'rendered'" class="mt-3 rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/50">
          <summary class="cursor-pointer text-sm font-medium text-gray-600 dark:text-gray-300">{{ t('admin.degradation.runs.viewArtifact') }}</summary>
          <VisualArtifactFrame :run-id="run.id" scope="admin" :title="run.probe_name" class="mt-3" />
        </details>
        <p v-if="run.error_message" class="mt-3 whitespace-pre-wrap break-words rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300">
          {{ run.error_message }}
        </p>
        <details v-if="run.answer_text" class="mt-3 rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/50">
          <summary class="cursor-pointer text-sm font-medium text-gray-600 dark:text-gray-300">{{ t('admin.degradation.runs.viewAnswer') }}</summary>
          <pre class="mt-3 max-h-80 overflow-auto whitespace-pre-wrap break-words font-sans text-sm leading-relaxed text-gray-700 dark:text-gray-300">{{ run.answer_text }}</pre>
        </details>
      </article>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import VisualArtifactFrame from '@/components/degradation/VisualArtifactFrame.vue'
import type { DegradationRun } from '@/api/admin/degradation'

defineProps<{
  show: boolean
  target: { group_name: string; model: string } | null
  runs: DegradationRun[]
  loading: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()
const { t, locale } = useI18n()

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(locale.value || undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(date)
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value < 0) return '-'
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`
}

function statusBadgeClass(status: DegradationRun['status']) {
  return {
    passed: 'badge-success',
    failed: 'badge-danger',
    error: 'badge-gray',
    rendered: 'badge-primary',
  }[status]
}
</script>
