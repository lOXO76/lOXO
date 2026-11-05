<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative w-full sm:w-64">
        <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="filters.search"
          type="text"
          class="input pl-10"
          data-testid="record-search"
          :placeholder="t('admin.rewardTasks.records.searchPlaceholder')"
          @input="debounceLoad"
        />
      </div>
      <Select
        v-model="filters.task_id"
        :options="taskOptions"
        class="w-full sm:w-56"
        :aria-label="t('admin.rewardTasks.records.filterTask')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.status"
        :options="statusOptions"
        class="w-full sm:w-40"
        :aria-label="t('admin.rewardTasks.records.filterStatus')"
        @change="reloadFromFirstPage"
      />
      <button type="button" class="btn btn-secondary ml-auto" :disabled="loading" @click="loadRecords">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        {{ t('admin.rewardTasks.refresh') }}
      </button>
    </div>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable :columns="columns" :data="records" :loading="loading" row-key="id">
        <template #cell-user="{ row }">
          <div class="min-w-0">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
              {{ row.user_email || t('admin.rewardTasks.records.deletedUser') }}
            </p>
            <p class="text-xs text-gray-500 dark:text-dark-400">
              #{{ row.user_id }}<template v-if="row.username"> · {{ row.username }}</template>
            </p>
          </div>
        </template>
        <template #cell-task="{ row }">
          <div class="min-w-0 max-w-xs">
            <p class="truncate text-sm text-gray-900 dark:text-white" :title="row.task_title">{{ row.task_title }}</p>
            <span class="badge badge-gray mt-0.5">{{ t(`admin.rewardTasks.taskTypes.${row.task_type}`) }}</span>
          </div>
        </template>
        <template #cell-periodKey="{ row }">
          <span class="font-mono text-xs text-gray-600 dark:text-dark-300">{{ row.period_key }}</span>
        </template>
        <template #cell-status="{ row }">
          <span class="badge" :class="row.status === 'completed' ? 'badge-success' : 'badge-primary'">
            {{ t(`admin.rewardTasks.recordStatus.${row.status}`) }}
          </span>
        </template>
        <template #cell-progress="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">{{ progressText(row) }}</span>
        </template>
        <template #cell-externalUser="{ row }">
          <span class="block max-w-[10rem] truncate font-mono text-xs text-gray-500 dark:text-dark-400" :title="row.external_user_id">
            {{ row.external_user_id || t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-startedAt="{ row }">
          <span class="text-xs text-gray-600 dark:text-dark-300">{{ formatDateTime(row.started_at) }}</span>
        </template>
        <template #cell-completedAt="{ row }">
          <span class="text-xs text-gray-600 dark:text-dark-300">
            {{ row.completed_at ? formatDateTime(row.completed_at) : t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-reward="{ row }">
          <span class="tabular-nums text-sm font-medium text-gray-900 dark:text-white">
            {{ row.reward_amount > 0 ? `+${formatCredit(row.reward_amount)}` : t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.rewardTasks.records.loadFailed') : t('admin.rewardTasks.records.empty')"
            :description="loadFailed ? t('admin.rewardTasks.loadFailed') : t('admin.rewardTasks.records.empty')"
            :action-text="loadFailed ? t('common.tryAgain') : undefined"
            @action="loadRecords"
          />
        </template>
      </DataTable>

      <div v-if="pagination.total > 0" class="border-t border-gray-100 px-5 py-3 dark:border-dark-700">
        <Pagination
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCredit, formatYuan } from './format'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type {
  AdminRewardTask,
  AdminRewardTaskRecord,
  RewardTaskRecordStatus,
} from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const records = ref<AdminRewardTaskRecord[]>([])
const tasks = ref<AdminRewardTask[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const filters = reactive<{ search: string; task_id: string; status: RewardTaskRecordStatus | '' }>({
  search: '',
  task_id: '',
  status: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let requestSequence = 0
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const taskOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.records.allTasks') },
  ...tasks.value.map((task) => ({ value: String(task.id), label: task.title })),
])
const statusOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.records.allStatuses') },
  { value: 'in_progress', label: t('admin.rewardTasks.recordStatus.in_progress') },
  { value: 'completed', label: t('admin.rewardTasks.recordStatus.completed') },
])
const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.rewardTasks.records.columns.user'), sortable: false },
  { key: 'task', label: t('admin.rewardTasks.records.columns.task'), sortable: false },
  { key: 'periodKey', label: t('admin.rewardTasks.records.columns.periodKey'), sortable: false },
  { key: 'status', label: t('admin.rewardTasks.records.columns.status'), sortable: false },
  { key: 'progress', label: t('admin.rewardTasks.records.columns.progress'), sortable: false },
  { key: 'externalUser', label: t('admin.rewardTasks.records.columns.externalUser'), sortable: false },
  { key: 'startedAt', label: t('admin.rewardTasks.records.columns.startedAt'), sortable: false },
  { key: 'completedAt', label: t('admin.rewardTasks.records.columns.completedAt'), sortable: false },
  { key: 'reward', label: t('admin.rewardTasks.records.columns.reward'), sortable: false },
])

// browse 看累计秒数，recharge 看累计金额，register 无进度
function progressText(record: AdminRewardTaskRecord): string {
  if (record.task_type === 'browse') {
    return t('admin.rewardTasks.records.progressSeconds', { seconds: record.progress_seconds })
  }
  if (record.task_type === 'recharge') {
    return t('admin.rewardTasks.records.rechargeValue', { amount: formatYuan(record.recharge_total) })
  }
  return t('admin.rewardTasks.emptyValue')
}

async function loadRecords() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.rewardTasks.listRecords({
      page: pagination.page,
      page_size: pagination.page_size,
      task_id: filters.task_id ? Number(filters.task_id) : undefined,
      status: filters.status || undefined,
      search: filters.search.trim() || undefined,
    })
    if (sequence !== requestSequence) return
    records.value = result.items ?? []
    pagination.total = result.total ?? 0
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.records.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadTaskOptions() {
  try {
    const result = await adminAPI.rewardTasks.listTasks({ page: 1, page_size: 100 })
    tasks.value = result.items ?? []
  } catch {
    // 任务下拉仅用于筛选，失败时退化为「全部任务」
    tasks.value = []
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadRecords()
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(reloadFromFirstPage, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  reloadFromFirstPage()
}

onMounted(() => {
  void loadRecords()
  void loadTaskOptions()
})

onBeforeUnmount(() => {
  requestSequence += 1
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
