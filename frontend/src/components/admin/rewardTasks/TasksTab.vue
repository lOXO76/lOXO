<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative w-full sm:w-64">
        <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="filters.search"
          type="text"
          class="input pl-10"
          data-testid="task-search"
          :placeholder="t('admin.rewardTasks.tasks.searchPlaceholder')"
          @input="debounceLoad"
        />
      </div>
      <Select
        v-model="filters.task_type"
        :options="typeOptions"
        class="w-full sm:w-40"
        :aria-label="t('admin.rewardTasks.tasks.filterType')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.enabled"
        :options="stateOptions"
        class="w-full sm:w-40"
        :aria-label="t('admin.rewardTasks.tasks.filterEnabled')"
        @change="reloadFromFirstPage"
      />
      <div class="ml-auto flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadTasks">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('admin.rewardTasks.refresh') }}
        </button>
        <button type="button" class="btn btn-primary" data-testid="task-create" @click="openCreate">
          <Icon name="plus" size="sm" />
          {{ t('admin.rewardTasks.tasks.create') }}
        </button>
      </div>
    </div>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable
        :columns="columns"
        :data="tasks"
        :loading="loading"
        row-key="id"
        :actions-count="2"
      >
        <template #cell-title="{ row }">
          <div class="min-w-0 max-w-xs">
            <p class="truncate font-medium text-gray-900 dark:text-white" :title="row.title">{{ row.title }}</p>
            <p v-if="row.description" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-500" :title="row.description">
              {{ row.description }}
            </p>
          </div>
        </template>
        <template #cell-type="{ row }">
          <span class="badge badge-gray">{{ t(`admin.rewardTasks.taskTypes.${row.task_type}`) }}</span>
        </template>
        <template #cell-verifyMode="{ row }">
          <span class="badge" :class="row.verify_mode === 'timer' ? 'badge-warning' : 'badge-primary'">
            {{ t(`admin.rewardTasks.verifyModes.${row.verify_mode}`) }}
          </span>
        </template>
        <template #cell-period="{ row }">
          <span class="text-sm text-gray-600 dark:text-dark-300">{{ t(`admin.rewardTasks.periods.${row.period}`) }}</span>
        </template>
        <template #cell-reward="{ row }">
          <span class="tabular-nums text-sm font-medium text-gray-900 dark:text-white">+{{ formatCredit(row.reward_amount) }}</span>
        </template>
        <template #cell-quota="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">
            {{ row.max_completions > 0
              ? t('admin.rewardTasks.tasks.quotaValue', { done: row.completed_count, max: row.max_completions })
              : t('admin.rewardTasks.tasks.quotaUnlimited') }}
          </span>
        </template>
        <template #cell-inProgress="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">{{ row.in_progress_count }}</span>
        </template>
        <template #cell-totalReward="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">{{ formatCredit(row.total_reward) }}</span>
        </template>
        <template #cell-enabled="{ row }">
          <Toggle :model-value="row.enabled" @update:model-value="(value: boolean) => toggleEnabled(row, value)" />
        </template>
        <template #cell-sortOrder="{ row }">
          <span class="tabular-nums text-sm text-gray-500 dark:text-dark-400">{{ row.sort_order }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm" @click="openEdit(row)">{{ t('common.edit') }}</button>
            <button
              type="button"
              class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
              @click="requestDelete(row)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.rewardTasks.tasks.loadFailed') : t('admin.rewardTasks.tasks.empty')"
            :description="loadFailed ? t('admin.rewardTasks.loadFailed') : t('admin.rewardTasks.tasks.emptyDescription')"
            :action-text="loadFailed ? t('common.tryAgain') : t('admin.rewardTasks.tasks.create')"
            @action="loadFailed ? loadTasks() : openCreate()"
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

    <TaskFormDialog
      :show="showForm"
      :task="editingTask"
      :partners="partners"
      @close="closeForm"
      @saved="handleSaved"
    />

    <ConfirmDialog
      :show="showDelete"
      :title="t('common.delete')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDelete = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import TaskFormDialog from './TaskFormDialog.vue'
import { formatCredit } from './format'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type {
  AdminRewardTask,
  AdminRewardTaskPartner,
  AdminRewardTaskPayload,
  RewardTaskType,
} from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const emit = defineEmits<{
  // 任务数据变化后通知页面刷新概览
  (e: 'changed'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const tasks = ref<AdminRewardTask[]>([])
const partners = ref<AdminRewardTaskPartner[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const filters = reactive<{ task_type: RewardTaskType | ''; enabled: 'true' | 'false' | ''; search: string }>({
  task_type: '',
  enabled: '',
  search: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const showForm = ref(false)
const editingTask = ref<AdminRewardTask | null>(null)
const showDelete = ref(false)
const deletingTask = ref<AdminRewardTask | null>(null)
const deleteLoading = ref(false)
let requestSequence = 0
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const typeOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.tasks.allTypes') },
  { value: 'browse', label: t('admin.rewardTasks.taskTypes.browse') },
  { value: 'register', label: t('admin.rewardTasks.taskTypes.register') },
  { value: 'recharge', label: t('admin.rewardTasks.taskTypes.recharge') },
])
const stateOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.tasks.allStates') },
  { value: 'true', label: t('admin.rewardTasks.tasks.onlyEnabled') },
  { value: 'false', label: t('admin.rewardTasks.tasks.onlyDisabled') },
])
const columns = computed<Column[]>(() => [
  { key: 'title', label: t('admin.rewardTasks.tasks.columns.title'), sortable: false },
  { key: 'type', label: t('admin.rewardTasks.tasks.columns.type'), sortable: false },
  { key: 'verifyMode', label: t('admin.rewardTasks.tasks.columns.verifyMode'), sortable: false },
  { key: 'period', label: t('admin.rewardTasks.tasks.columns.period'), sortable: false },
  { key: 'reward', label: t('admin.rewardTasks.tasks.columns.reward'), sortable: false },
  { key: 'quota', label: t('admin.rewardTasks.tasks.columns.quota'), sortable: false },
  { key: 'inProgress', label: t('admin.rewardTasks.tasks.columns.inProgress'), sortable: false },
  { key: 'totalReward', label: t('admin.rewardTasks.tasks.columns.totalReward'), sortable: false },
  { key: 'enabled', label: t('admin.rewardTasks.tasks.columns.enabled'), sortable: false },
  { key: 'sortOrder', label: t('admin.rewardTasks.tasks.columns.sortOrder'), sortable: false },
  { key: 'actions', label: t('admin.rewardTasks.tasks.columns.actions'), sortable: false },
])
const deleteMessage = computed(() =>
  t('admin.rewardTasks.tasks.deleteConfirm', { title: deletingTask.value?.title ?? '' }),
)

async function loadTasks() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.rewardTasks.listTasks({
      page: pagination.page,
      page_size: pagination.page_size,
      task_type: filters.task_type || undefined,
      enabled: filters.enabled || undefined,
      search: filters.search.trim() || undefined,
    })
    if (sequence !== requestSequence) return
    tasks.value = result.items ?? []
    pagination.total = result.total ?? 0
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.tasks.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadPartners() {
  try {
    partners.value = await adminAPI.rewardTasks.listPartners()
  } catch {
    // 站点列表仅用于表单下拉，失败时保留空列表，不额外打断任务列表
    partners.value = []
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadTasks()
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(reloadFromFirstPage, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadTasks()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  reloadFromFirstPage()
}

function openCreate() {
  editingTask.value = null
  showForm.value = true
  void loadPartners()
}

function openEdit(task: AdminRewardTask) {
  editingTask.value = task
  showForm.value = true
  void loadPartners()
}

function closeForm() {
  showForm.value = false
  editingTask.value = null
}

function handleSaved() {
  void loadTasks()
  emit('changed')
}

// 行内启用开关：整对象回写（task_type 不可变，其余字段保持原值）
function toPayload(task: AdminRewardTask): AdminRewardTaskPayload {
  return {
    title: task.title,
    description: task.description,
    task_type: task.task_type,
    verify_mode: task.verify_mode,
    partner_id: task.partner_id,
    target_url: task.target_url,
    icon_url: task.icon_url,
    reward_amount: task.reward_amount,
    period: task.period,
    browse_seconds: task.browse_seconds,
    idle_timeout_seconds: task.idle_timeout_seconds,
    min_recharge_amount: task.min_recharge_amount,
    max_completions: task.max_completions,
    starts_at: task.starts_at,
    ends_at: task.ends_at,
    enabled: task.enabled,
    sort_order: task.sort_order,
  }
}

async function toggleEnabled(task: AdminRewardTask, enabled: boolean) {
  const previous = task.enabled
  task.enabled = enabled
  try {
    await adminAPI.rewardTasks.updateTask(task.id, { ...toPayload(task), enabled })
    appStore.showSuccess(t('admin.rewardTasks.tasks.updateSuccess'))
    emit('changed')
  } catch (error) {
    task.enabled = previous
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.tasks.saveFailed')))
  }
}

function requestDelete(task: AdminRewardTask) {
  deletingTask.value = task
  showDelete.value = true
}

async function confirmDelete() {
  if (deleteLoading.value || !deletingTask.value) return
  deleteLoading.value = true
  try {
    await adminAPI.rewardTasks.deleteTask(deletingTask.value.id)
    appStore.showSuccess(t('admin.rewardTasks.tasks.deleteSuccess'))
    showDelete.value = false
    deletingTask.value = null
    // 删除最后一页的唯一一条时回退一页
    if (tasks.value.length === 1 && pagination.page > 1) pagination.page -= 1
    void loadTasks()
    emit('changed')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.tasks.deleteFailed')))
  } finally {
    deleteLoading.value = false
  }
}

onMounted(() => {
  void loadTasks()
})

onBeforeUnmount(() => {
  requestSequence += 1
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
