<template>
  <BaseDialog
    :show="show"
    :title="t('degradation.visual.historyTitle', { name: title })"
    width="full"
    @close="emit('close')"
  >
    <div v-if="loading" class="flex min-h-48 items-center justify-center text-sm text-gray-400" role="status">
      <LoadingSpinner size="md" class="mr-2" />
      {{ t('common.loading') }}
    </div>
    <EmptyState
      v-else-if="loadError"
      :title="loadError"
      :description="t('degradation.visual.historyLoadFailedDescription')"
      :action-text="t('degradation.retry')"
      @action="loadHistory"
    />
    <EmptyState
      v-else-if="items.length === 0"
      :title="t('degradation.visual.historyEmpty')"
      :description="t('degradation.visual.historyEmptyDescription')"
    />
    <div v-else class="space-y-5">
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('degradation.visual.historyHint') }}</p>
      <div class="max-h-56 space-y-2 overflow-y-auto rounded-xl border border-gray-200 p-3 dark:border-dark-700">
        <label
          v-for="item in items"
          :key="item.run_id"
          class="flex items-center gap-3 rounded-lg px-2 py-2 text-sm"
          :class="selectedIds.includes(item.run_id) ? 'bg-primary-50 dark:bg-primary-900/20' : 'hover:bg-gray-50 dark:hover:bg-dark-800'"
        >
          <input
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
            :checked="selectedIds.includes(item.run_id)"
            :disabled="selectedIds.length >= 2 && !selectedIds.includes(item.run_id)"
            @change="toggleSelection(item.run_id, $event)"
          />
          <span class="min-w-0 flex-1">
            <span class="block font-medium text-gray-800 dark:text-gray-200">#{{ item.run_id }} · {{ item.artifact_kind }}</span>
            <span class="mt-0.5 block text-xs text-gray-400">{{ formatTime(item.created_at) }} · {{ formatLatency(item.latency_ms) }}</span>
          </span>
        </label>
      </div>

      <div v-if="selectedItems.length > 0" class="grid gap-4" :class="selectedItems.length === 2 ? 'lg:grid-cols-2' : ''">
        <VisualArtifactFrame
          v-for="item in selectedItems"
          :key="item.run_id"
          :run-id="item.run_id"
          scope="user"
          :title="t('degradation.visual.historyArtifactTitle', { id: item.run_id, kind: item.artifact_kind })"
        />
      </div>
      <EmptyState
        v-else
        :title="t('degradation.visual.selectToCompare')"
        :description="t('degradation.visual.selectToCompareDescription')"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import VisualArtifactFrame from '@/components/degradation/VisualArtifactFrame.vue'
import { degradationAPI } from '@/api/degradation'
import type { DegradationShowcase } from '@/api/degradation'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  groupId: number
  model: string
  probeId: number
  title: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()
const { t, locale } = useI18n()
const items = ref<DegradationShowcase[]>([])
const selectedIds = ref<number[]>([])
const loading = ref(false)
const loadError = ref('')
let requestSequence = 0

const selectedItems = computed(() => items.value.filter((item) => selectedIds.value.includes(item.run_id)))

async function loadHistory() {
  const sequence = ++requestSequence
  loading.value = true
  loadError.value = ''
  try {
    const result = await degradationAPI.listVisualRuns({
      group_id: props.groupId,
      model: props.model,
      probe_id: props.probeId,
      limit: 50,
    })
    if (sequence !== requestSequence) return
    items.value = result.items ?? []
    selectedIds.value = items.value.slice(0, 2).map((item) => item.run_id)
  } catch (error) {
    if (sequence === requestSequence) {
      items.value = []
      selectedIds.value = []
      loadError.value = extractApiErrorMessage(error, t('degradation.visual.historyLoadFailed'))
    }
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

function toggleSelection(runId: number, event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  if (checked) {
    if (selectedIds.value.length < 2 && !selectedIds.value.includes(runId)) {
      selectedIds.value = [...selectedIds.value, runId]
    }
    return
  }
  selectedIds.value = selectedIds.value.filter((id) => id !== runId)
}

function formatTime(value: string) {
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

function formatLatency(value: number) {
  return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`
}

watch(
  () => [props.show, props.groupId, props.model, props.probeId] as const,
  ([show]) => {
    if (show) void loadHistory()
    else requestSequence += 1
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  requestSequence += 1
})
</script>
