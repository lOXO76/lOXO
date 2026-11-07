<template>
  <div class="space-y-4">
    <!-- 概览 -->
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5" data-testid="tickets-stats">
      <StatCard
        :title="t('admin.tickets.stats.pendingAdmin')"
        :value="stats.pending_admin"
        :icon="InboxStatIcon"
        icon-variant="warning"
      />
      <StatCard
        :title="t('admin.tickets.stats.pendingUser')"
        :value="stats.pending_user"
        :icon="ClockStatIcon"
      />
      <StatCard
        :title="t('admin.tickets.stats.resolved')"
        :value="stats.resolved"
        :icon="CheckStatIcon"
        icon-variant="success"
      />
      <StatCard
        :title="t('admin.tickets.stats.closed')"
        :value="stats.closed"
        :icon="ArchiveStatIcon"
        icon-variant="danger"
      />
      <StatCard
        :title="t('admin.tickets.stats.unread')"
        :value="stats.unread"
        :icon="BellStatIcon"
        icon-variant="warning"
      />
    </div>

    <!-- 筛选 -->
    <div class="flex flex-wrap items-center gap-2">
      <div class="relative w-full sm:w-64">
        <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          v-model="filters.keyword"
          type="text"
          class="input pl-10"
          data-testid="tickets-search"
          :placeholder="t('admin.tickets.filters.keywordPlaceholder')"
          @input="debounceLoad"
        />
      </div>
      <Select
        v-model="filters.status"
        :options="statusOptions"
        class="w-full sm:w-36"
        :aria-label="t('admin.tickets.filters.status')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.priority"
        :options="priorityOptions"
        class="w-full sm:w-32"
        :aria-label="t('admin.tickets.filters.priority')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.category"
        :options="categoryOptions"
        class="w-full sm:w-32"
        :aria-label="t('admin.tickets.filters.category')"
        @change="reloadFromFirstPage"
      />
      <Select
        v-model="filters.assignee_id"
        :options="assigneeOptions"
        class="w-full sm:w-40"
        :aria-label="t('admin.tickets.filters.assignee')"
        @change="reloadFromFirstPage"
      />
      <label class="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 dark:border-dark-700 dark:text-dark-200">
        <input
          v-model="filters.unread_only"
          type="checkbox"
          class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
          data-testid="tickets-unread-only"
          @change="reloadFromFirstPage"
        />
        <span>{{ t('admin.tickets.filters.unreadOnly') }}</span>
      </label>
      <div class="ml-auto flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="refresh">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable
        :columns="columns"
        :data="tickets"
        :loading="loading"
        row-key="id"
        clickable-rows
        :actions-count="1"
        @row-click="openTicket"
      >
        <template #cell-subject="{ row }">
          <div class="min-w-0 max-w-xs">
            <p class="flex items-center gap-2 truncate font-medium text-gray-900 dark:text-white" :title="row.subject">
              <span
                v-if="row.admin_unread"
                class="inline-block h-2 w-2 flex-shrink-0 rounded-full bg-primary-500"
                :title="t('admin.tickets.unreadBadge')"
              />
              <span class="truncate">{{ row.subject }}</span>
            </p>
            <p class="mt-0.5 truncate font-mono text-xs text-gray-400 dark:text-dark-500">{{ row.ticket_no }}</p>
          </div>
        </template>
        <template #cell-user="{ row }">
          <div class="min-w-0 max-w-[14rem]">
            <p class="truncate text-sm text-gray-900 dark:text-white" :title="row.user_email">{{ row.user_email }}</p>
            <p v-if="row.user_username" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-500">
              {{ row.user_username }}
            </p>
          </div>
        </template>
        <template #cell-status="{ row }">
          <span class="badge" :class="statusBadge(row.status)">{{ t(statusLabelKey(row.status)) }}</span>
        </template>
        <template #cell-priority="{ row }">
          <span class="badge" :class="priorityBadge(row.priority)">{{ t(priorityLabelKey(row.priority)) }}</span>
        </template>
        <template #cell-category="{ row }">
          <span class="text-sm text-gray-600 dark:text-dark-300">{{ t(categoryLabelKey(row.category)) }}</span>
        </template>
        <template #cell-assignee="{ row }">
          <span class="text-sm text-gray-600 dark:text-dark-300">{{ row.assignee_name || t('admin.tickets.unassigned') }}</span>
        </template>
        <template #cell-messages="{ row }">
          <span class="tabular-nums text-sm text-gray-600 dark:text-dark-300">{{ row.message_count }}</span>
        </template>
        <template #cell-lastMessageAt="{ row }">
          <span class="whitespace-nowrap text-sm text-gray-600 dark:text-dark-300" :title="formatTicketDateTime(row.last_message_at)">
            {{ formatTicketTime(row.last_message_at) }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm" @click.stop="openTicket(row)">
              {{ t('admin.tickets.view') }}
            </button>
          </div>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.tickets.list.loadFailed') : t('admin.tickets.list.empty')"
            :description="loadFailed ? t('common.tryAgain') : t('admin.tickets.list.emptyDescription')"
            :action-text="loadFailed ? t('common.tryAgain') : t('common.refresh')"
            @action="refresh"
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
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { Archive, Bell, CheckCheck, Clock, Inbox } from '@lucide/vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import StatCard from '@/components/common/StatCard.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type {
  AdminTicketSummary,
  TicketAssignee,
  TicketCategory,
  TicketPriority,
  TicketStats,
  TicketStatus,
} from '@/types/tickets'
import { TICKET_CATEGORIES, TICKET_PRIORITIES, TICKET_STATUSES } from '@/types/tickets'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  TICKET_PRIORITY_BADGE,
  TICKET_STATUS_BADGE,
  categoryLabelKey,
  formatTicketDateTime,
  formatTicketTime,
  priorityLabelKey,
  statusLabelKey,
} from './format'

const InboxStatIcon = { render: () => h(Inbox, { strokeWidth: 1.75 }) }
const ClockStatIcon = { render: () => h(Clock, { strokeWidth: 1.75 }) }
const CheckStatIcon = { render: () => h(CheckCheck, { strokeWidth: 1.75 }) }
const ArchiveStatIcon = { render: () => h(Archive, { strokeWidth: 1.75 }) }
const BellStatIcon = { render: () => h(Bell, { strokeWidth: 1.75 }) }

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const tickets = ref<AdminTicketSummary[]>([])
const assignees = ref<TicketAssignee[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const stats = reactive<TicketStats>({ pending_admin: 0, pending_user: 0, resolved: 0, closed: 0, unread: 0 })
const filters = reactive<{
  keyword: string
  status: TicketStatus | ''
  priority: TicketPriority | ''
  category: TicketCategory | ''
  assignee_id: string
  unread_only: boolean
}>({
  keyword: '',
  status: '',
  priority: '',
  category: '',
  assignee_id: '',
  unread_only: false,
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let requestSequence = 0
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const columns = computed<Column[]>(() => [
  { key: 'subject', label: t('admin.tickets.columns.subject') },
  { key: 'user', label: t('admin.tickets.columns.user') },
  { key: 'status', label: t('admin.tickets.columns.status') },
  { key: 'priority', label: t('admin.tickets.columns.priority') },
  { key: 'category', label: t('admin.tickets.columns.category') },
  { key: 'assignee', label: t('admin.tickets.columns.assignee') },
  { key: 'messages', label: t('admin.tickets.columns.messages') },
  { key: 'lastMessageAt', label: t('admin.tickets.columns.lastMessageAt') },
  { key: 'actions', label: t('admin.tickets.columns.actions') },
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.tickets.filters.allStatuses') },
  ...TICKET_STATUSES.map((status) => ({ value: status, label: t(statusLabelKey(status)) })),
])
const priorityOptions = computed(() => [
  { value: '', label: t('admin.tickets.filters.allPriorities') },
  ...TICKET_PRIORITIES.map((priority) => ({ value: priority, label: t(priorityLabelKey(priority)) })),
])
const categoryOptions = computed(() => [
  { value: '', label: t('admin.tickets.filters.allCategories') },
  ...TICKET_CATEGORIES.map((category) => ({ value: category, label: t(categoryLabelKey(category)) })),
])
const assigneeOptions = computed(() => [
  { value: '', label: t('admin.tickets.filters.allAssignees') },
  ...assignees.value.map((assignee) => ({
    value: String(assignee.id),
    label: assignee.username || assignee.email,
  })),
])

async function loadTickets() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.tickets.listTickets({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: filters.keyword.trim(),
      status: filters.status,
      priority: filters.priority,
      category: filters.category,
      assignee_id: filters.assignee_id ? Number(filters.assignee_id) : undefined,
      unread_only: filters.unread_only || undefined,
    })
    if (sequence !== requestSequence) return
    tickets.value = result.items ?? []
    pagination.total = result.total ?? 0
    pagination.page = result.page ?? pagination.page
    pagination.page_size = result.page_size ?? pagination.page_size
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    tickets.value = []
    pagination.total = 0
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.list.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

async function loadStats() {
  try {
    const result = await adminAPI.tickets.getStats()
    Object.assign(stats, result)
  } catch {
    // 统计失败不打断列表使用
  }
}

async function loadAssignees() {
  try {
    assignees.value = await adminAPI.tickets.listAssignees()
  } catch {
    assignees.value = []
  }
}

function refresh() {
  void loadTickets()
  void loadStats()
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadTickets()
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
  void loadTickets()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadTickets()
}

function openTicket(row: AdminTicketSummary) {
  void router.push({ name: 'AdminTicketDetail', params: { id: row.id } })
}

// DataTable 的 slot row 是 any，这里显式收窄，避免模板里索引类型化 Record 报错。
function statusBadge(status: TicketStatus): string {
  return TICKET_STATUS_BADGE[status]
}

function priorityBadge(priority: TicketPriority): string {
  return TICKET_PRIORITY_BADGE[priority]
}

onMounted(() => {
  void loadTickets()
  void loadStats()
  void loadAssignees()
})

onBeforeUnmount(() => {
  requestSequence += 1
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
