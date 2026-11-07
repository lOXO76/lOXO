<template>
  <div class="space-y-4" data-testid="ticket-agent-keys-tab">
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative w-full sm:w-64">
        <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="filters.keyword"
          type="text"
          class="input pl-10"
          data-testid="ticket-agent-key-search"
          :placeholder="t('admin.tickets.agentKeys.searchPlaceholder')"
          @input="debounceLoad"
        />
      </div>
      <Select
        v-model="filters.scope"
        :options="scopeOptions"
        class="w-full sm:w-36"
        :aria-label="t('admin.tickets.agentKeys.filterScope')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.status"
        :options="statusOptions"
        class="w-full sm:w-36"
        :aria-label="t('admin.tickets.agentKeys.filterStatus')"
        @change="reloadFromFirstPage"
      />
      <div class="ml-auto flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadKeys">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <p class="rounded-xl border border-gray-100 bg-gray-50/60 px-4 py-3 text-xs leading-5 text-gray-500 dark:border-dark-700 dark:bg-dark-900/40 dark:text-dark-400">
      {{ t('admin.tickets.agentKeys.hint') }}
    </p>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable
        :columns="columns"
        :data="keys"
        :loading="loading"
        row-key="id"
        :actions-count="3"
      >
        <template #cell-owner="{ row }">
          <div class="min-w-0 max-w-[14rem]">
            <p class="truncate text-sm text-gray-900 dark:text-white" :title="row.user_email">{{ row.user_email }}</p>
            <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">#{{ row.user_id }}</p>
          </div>
        </template>
        <template #cell-name="{ row }">
          <div class="min-w-0 max-w-[14rem]">
            <p class="truncate text-sm font-medium text-gray-900 dark:text-white" :title="row.name">{{ row.name }}</p>
            <p class="mt-0.5 truncate font-mono text-xs text-gray-400 dark:text-dark-500">{{ row.key_prefix }}…</p>
          </div>
        </template>
        <template #cell-scope="{ row }">
          <span class="badge" :class="row.scope === 'admin' ? 'badge-danger' : 'badge-gray'">
            {{ t(`admin.tickets.scopes.${row.scope}`) }}
          </span>
        </template>
        <template #cell-qps="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-dark-300">{{ row.qps_limit }}</span>
        </template>
        <template #cell-status="{ row }">
          <span class="badge" :class="row.status === 'active' ? 'badge-success' : 'badge-gray'">
            {{ t(`admin.tickets.keyStatuses.${row.status}`) }}
          </span>
        </template>
        <template #cell-lastUsedAt="{ row }">
          <div class="whitespace-nowrap text-sm text-gray-600 dark:text-dark-300">
            <p>{{ formatTicketDateTime(row.last_used_at) }}</p>
            <p v-if="row.last_used_ip" class="mt-0.5 font-mono text-xs text-gray-400 dark:text-dark-500">{{ row.last_used_ip }}</p>
          </div>
        </template>
        <template #cell-expiresAt="{ row }">
          <span class="whitespace-nowrap text-sm text-gray-600 dark:text-dark-300">
            {{ row.expires_at ? formatTicketDateTime(row.expires_at) : t('admin.tickets.agentKeys.neverExpires') }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm" @click.stop="openQpsDialog(row)">
              {{ t('admin.tickets.agentKeys.editQps') }}
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              @click.stop="toggleStatus(row)"
            >
              {{ row.status === 'active' ? t('admin.tickets.agentKeys.disable') : t('admin.tickets.agentKeys.enable') }}
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
              @click.stop="requestDelete(row)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.tickets.agentKeys.loadFailed') : t('admin.tickets.agentKeys.empty')"
            :description="loadFailed ? t('common.tryAgain') : t('admin.tickets.agentKeys.emptyDescription')"
            :action-text="t('common.refresh')"
            @action="loadKeys"
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

    <BaseDialog
      :show="showQpsDialog"
      :title="t('admin.tickets.agentKeys.editQpsTitle')"
      width="narrow"
      @close="closeQpsDialog"
    >
      <div class="space-y-3">
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ t('admin.tickets.agentKeys.editQpsFor', { name: editingKey?.name ?? '' }) }}
        </p>
        <label class="input-label block text-xs font-medium text-gray-600 dark:text-dark-300">
          {{ t('admin.tickets.agentKeys.qpsLimit') }}
        </label>
        <input
          v-model.number="qpsDraft"
          type="number"
          min="1"
          :max="agentMaxQps"
          class="input w-full"
          data-testid="ticket-agent-key-qps"
        />
        <p v-if="qpsError" class="text-xs text-red-600 dark:text-red-400">{{ qpsError }}</p>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeQpsDialog">{{ t('common.cancel') }}</button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="qpsSaving"
          data-testid="ticket-agent-key-qps-save"
          @click="saveQps"
        >
          {{ t('common.save') }}
        </button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDelete"
      :title="t('admin.tickets.agentKeys.deleteTitle')"
      :message="t('admin.tickets.agentKeys.deleteMessage', { name: deletingKey?.name ?? '' })"
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
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type { AdminTicketAgentKey, TicketAgentKeyScope, TicketAgentKeyStatus } from '@/types/tickets'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatTicketDateTime } from './format'

const { t } = useI18n()
const appStore = useAppStore()

const keys = ref<AdminTicketAgentKey[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const filters = reactive<{ keyword: string; scope: TicketAgentKeyScope | ''; status: TicketAgentKeyStatus | '' }>({
  keyword: '',
  scope: '',
  status: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const showQpsDialog = ref(false)
const editingKey = ref<AdminTicketAgentKey | null>(null)
const qpsDraft = ref<number | string>(1)
const qpsError = ref('')
const qpsSaving = ref(false)
// 单 key QPS 上限来自全局设置（agent_max_qps）；取不到时回退到 1000。
const agentMaxQps = ref(1000)
const showDelete = ref(false)
const deletingKey = ref<AdminTicketAgentKey | null>(null)
const deleteSaving = ref(false)
let requestSequence = 0
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'owner', label: t('admin.tickets.agentKeys.columns.owner') },
  { key: 'name', label: t('admin.tickets.agentKeys.columns.name') },
  { key: 'scope', label: t('admin.tickets.agentKeys.columns.scope') },
  { key: 'qps', label: t('admin.tickets.agentKeys.columns.qps') },
  { key: 'status', label: t('admin.tickets.agentKeys.columns.status') },
  { key: 'lastUsedAt', label: t('admin.tickets.agentKeys.columns.lastUsed') },
  { key: 'expiresAt', label: t('admin.tickets.agentKeys.columns.expiresAt') },
  { key: 'actions', label: t('admin.tickets.columns.actions') },
])

const scopeOptions = computed(() => [
  { value: '', label: t('admin.tickets.agentKeys.allScopes') },
  { value: 'user', label: t('admin.tickets.scopes.user') },
  { value: 'admin', label: t('admin.tickets.scopes.admin') },
])
const statusOptions = computed(() => [
  { value: '', label: t('admin.tickets.agentKeys.allStatuses') },
  { value: 'active', label: t('admin.tickets.keyStatuses.active') },
  { value: 'disabled', label: t('admin.tickets.keyStatuses.disabled') },
])

async function loadKeys() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.tickets.listAgentKeys({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: filters.keyword.trim(),
      scope: filters.scope,
      status: filters.status,
    })
    if (sequence !== requestSequence) return
    keys.value = result.items ?? []
    pagination.total = result.total ?? 0
    pagination.page = result.page ?? pagination.page
    pagination.page_size = result.page_size ?? pagination.page_size
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    keys.value = []
    pagination.total = 0
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.agentKeys.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadKeys()
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    debounceTimer = null
    reloadFromFirstPage()
  }, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadKeys()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadKeys()
}

function openQpsDialog(row: AdminTicketAgentKey) {
  editingKey.value = row
  qpsDraft.value = row.qps_limit
  qpsError.value = ''
  showQpsDialog.value = true
}

function closeQpsDialog() {
  showQpsDialog.value = false
  editingKey.value = null
  qpsError.value = ''
}

async function saveQps() {
  const key = editingKey.value
  if (!key || qpsSaving.value) return
  const value = Number(qpsDraft.value)
  if (!Number.isInteger(value) || value < 1 || value > agentMaxQps.value) {
    qpsError.value = t('admin.tickets.settings.fields.invalid', { min: 1, max: agentMaxQps.value })
    return
  }
  qpsSaving.value = true
  try {
    await adminAPI.tickets.updateAgentKey(key.id, { qps_limit: value })
    appStore.showSuccess(t('admin.tickets.agentKeys.qpsSaved'))
    closeQpsDialog()
    void loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.agentKeys.updateFailed')))
  } finally {
    qpsSaving.value = false
  }
}

async function toggleStatus(row: AdminTicketAgentKey) {
  const next: TicketAgentKeyStatus = row.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.tickets.updateAgentKey(row.id, { status: next })
    appStore.showSuccess(
      next === 'active' ? t('admin.tickets.agentKeys.enabledToast') : t('admin.tickets.agentKeys.disabledToast'),
    )
    void loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.agentKeys.updateFailed')))
  }
}

function requestDelete(row: AdminTicketAgentKey) {
  deletingKey.value = row
  showDelete.value = true
}

async function confirmDelete() {
  const key = deletingKey.value
  if (!key || deleteSaving.value) return
  deleteSaving.value = true
  try {
    await adminAPI.tickets.deleteAgentKey(key.id)
    appStore.showSuccess(t('admin.tickets.agentKeys.deletedToast'))
    showDelete.value = false
    deletingKey.value = null
    void loadKeys()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.agentKeys.deleteFailed')))
  } finally {
    deleteSaving.value = false
  }
}

async function loadAgentMaxQps() {
  try {
    const settings = await adminAPI.tickets.getSettings()
    const max = Number(settings?.agent_max_qps)
    if (Number.isInteger(max) && max > 0) agentMaxQps.value = max
  } catch {
    // 取不到全局上限时保留宽松回退值，不影响改 QPS
  }
}

onMounted(() => {
  void loadKeys()
  void loadAgentMaxQps()
})

onBeforeUnmount(() => {
  requestSequence += 1
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
