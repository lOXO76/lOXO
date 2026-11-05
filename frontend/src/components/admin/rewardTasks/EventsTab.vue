<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <Select
        v-model="filters.partner_id"
        :options="partnerOptions"
        class="w-full sm:w-56"
        :aria-label="t('admin.rewardTasks.events.filterPartner')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.result"
        :options="resultOptions"
        class="w-full sm:w-40"
        :aria-label="t('admin.rewardTasks.events.filterResult')"
        @change="reloadFromFirstPage"
      />
      <button type="button" class="btn btn-secondary ml-auto" :disabled="loading" @click="loadEvents">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        {{ t('admin.rewardTasks.refresh') }}
      </button>
    </div>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable :columns="columns" :data="events" :loading="loading" row-key="id">
        <template #cell-partner="{ row }">
          <span class="text-sm text-gray-900 dark:text-white">{{ row.partner_name || `#${row.partner_id}` }}</span>
        </template>
        <template #cell-eventId="{ row }">
          <span class="block max-w-[12rem] truncate font-mono text-xs text-gray-600 dark:text-dark-300" :title="row.event_id">
            {{ row.event_id }}
          </span>
        </template>
        <template #cell-eventType="{ row }">
          <span class="text-sm text-gray-600 dark:text-dark-300">
            {{ row.event_type || t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-ticket="{ row }">
          <span class="block max-w-[10rem] truncate font-mono text-xs text-gray-500 dark:text-dark-400" :title="row.ticket">
            {{ row.ticket || t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-amount="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">
            {{ row.amount > 0 ? t('admin.rewardTasks.events.amountValue', { amount: formatYuan(row.amount) }) : t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-result="{ row }">
          <span class="badge" :class="resultBadgeClass(row.result)">{{ t(`admin.rewardTasks.eventResults.${row.result}`) }}</span>
        </template>
        <template #cell-errorCode="{ row }">
          <span class="font-mono text-xs" :class="row.error_code ? 'text-red-600 dark:text-red-400' : 'text-gray-400 dark:text-dark-500'">
            {{ row.error_code || t('admin.rewardTasks.emptyValue') }}
          </span>
        </template>
        <template #cell-remoteIp="{ row }">
          <span class="font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.remote_ip || t('admin.rewardTasks.emptyValue') }}</span>
        </template>
        <template #cell-createdAt="{ row }">
          <span class="text-xs text-gray-600 dark:text-dark-300">{{ formatDateTime(row.created_at) }}</span>
        </template>
        <template #cell-payload="{ row }">
          <button type="button" class="btn btn-ghost btn-sm" @click="openPayload(row)">
            {{ t('admin.rewardTasks.events.viewPayload') }}
          </button>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.rewardTasks.events.loadFailed') : t('admin.rewardTasks.events.empty')"
            :description="loadFailed ? t('admin.rewardTasks.loadFailed') : t('admin.rewardTasks.events.empty')"
            :action-text="loadFailed ? t('common.tryAgain') : undefined"
            @action="loadEvents"
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

    <EventPayloadDialog :show="showPayload" :payload="activePayload" @close="showPayload = false" />
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
import EventPayloadDialog from './EventPayloadDialog.vue'
import { formatYuan } from './format'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type {
  AdminRewardTaskEvent,
  AdminRewardTaskPartner,
  RewardTaskEventResult,
} from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const events = ref<AdminRewardTaskEvent[]>([])
const partners = ref<AdminRewardTaskPartner[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const filters = reactive<{ partner_id: string; result: RewardTaskEventResult | '' }>({ partner_id: '', result: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const showPayload = ref(false)
const activePayload = ref<unknown>({})
let requestSequence = 0

const partnerOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.events.allPartners') },
  ...partners.value.map((partner) => ({ value: String(partner.id), label: partner.name })),
])
const resultOptions = computed(() => [
  { value: '', label: t('admin.rewardTasks.events.allResults') },
  { value: 'accepted', label: t('admin.rewardTasks.eventResults.accepted') },
  { value: 'completed', label: t('admin.rewardTasks.eventResults.completed') },
  { value: 'ignored', label: t('admin.rewardTasks.eventResults.ignored') },
  { value: 'rejected', label: t('admin.rewardTasks.eventResults.rejected') },
])
const columns = computed<Column[]>(() => [
  { key: 'partner', label: t('admin.rewardTasks.events.columns.partner'), sortable: false },
  { key: 'eventId', label: t('admin.rewardTasks.events.columns.eventId'), sortable: false },
  { key: 'eventType', label: t('admin.rewardTasks.events.columns.eventType'), sortable: false },
  { key: 'ticket', label: t('admin.rewardTasks.events.columns.ticket'), sortable: false },
  { key: 'amount', label: t('admin.rewardTasks.events.columns.amount'), sortable: false },
  { key: 'result', label: t('admin.rewardTasks.events.columns.result'), sortable: false },
  { key: 'errorCode', label: t('admin.rewardTasks.events.columns.errorCode'), sortable: false },
  { key: 'remoteIp', label: t('admin.rewardTasks.events.columns.remoteIp'), sortable: false },
  { key: 'createdAt', label: t('admin.rewardTasks.events.columns.createdAt'), sortable: false },
  { key: 'payload', label: t('admin.rewardTasks.events.columns.payload'), sortable: false },
])

function resultBadgeClass(result: RewardTaskEventResult): string {
  return {
    accepted: 'badge-primary',
    completed: 'badge-success',
    ignored: 'badge-gray',
    rejected: 'badge-danger',
  }[result] ?? 'badge-gray'
}

async function loadEvents() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.rewardTasks.listEvents({
      page: pagination.page,
      page_size: pagination.page_size,
      partner_id: filters.partner_id ? Number(filters.partner_id) : undefined,
      result: filters.result || undefined,
    })
    if (sequence !== requestSequence) return
    events.value = result.items ?? []
    pagination.total = result.total ?? 0
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.events.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadPartnerOptions() {
  try {
    partners.value = await adminAPI.rewardTasks.listPartners()
  } catch {
    // 站点下拉仅用于筛选，失败时退化为「全部站点」
    partners.value = []
  }
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadEvents()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadEvents()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  reloadFromFirstPage()
}

function openPayload(event: AdminRewardTaskEvent) {
  activePayload.value = event.payload
  showPayload.value = true
}

onMounted(() => {
  void loadEvents()
  void loadPartnerOptions()
})

onBeforeUnmount(() => {
  requestSequence += 1
})
</script>
