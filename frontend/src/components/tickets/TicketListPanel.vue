<template>
  <section class="space-y-4" :aria-busy="loading">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex flex-1 flex-col gap-3 sm:flex-row sm:items-center">
        <div class="relative w-full sm:max-w-xs">
          <SearchIcon class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400 dark:text-dark-500" aria-hidden="true" />
          <input
            v-model="keyword"
            type="search"
            class="input w-full pl-9"
            :placeholder="t('tickets.list.searchPlaceholder')"
            :aria-label="t('tickets.list.searchPlaceholder')"
            data-testid="tickets-keyword"
            @keyup.enter="applyFilters"
          />
        </div>
        <select
          v-model="statusFilter"
          class="input w-full sm:w-44"
          :aria-label="t('tickets.list.allStatuses')"
          data-testid="tickets-status-filter"
          @change="applyFilters"
        >
          <option value="">{{ t('tickets.list.allStatuses') }}</option>
          <option v-for="value in statusOptions" :key="value" :value="value">
            {{ t(`tickets.status.${value}`) }}
          </option>
        </select>
        <button type="button" class="btn btn-secondary shrink-0" :disabled="loading" @click="load(page)">
          <RefreshCw class="h-4 w-4" :class="{ 'animate-spin': loading }" aria-hidden="true" />
          {{ t('tickets.common.refresh') }}
        </button>
      </div>
      <button type="button" class="btn btn-primary shrink-0" data-testid="tickets-new" @click="emit('create')">
        <PlusIcon class="h-4 w-4" aria-hidden="true" />
        {{ t('tickets.list.newTicket') }}
      </button>
    </div>

    <div v-if="errorMessage" class="card px-6 py-12 text-center" role="alert">
      <CircleAlert class="mx-auto h-10 w-10 text-red-500" aria-hidden="true" />
      <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">{{ errorMessage }}</p>
      <button type="button" class="btn btn-secondary mt-4" data-testid="tickets-retry" @click="load(page)">
        {{ t('tickets.common.retry') }}
      </button>
    </div>

    <div v-else-if="loading && !result" class="card space-y-4 p-5" role="status" :aria-label="t('tickets.list.loading')">
      <div v-for="index in 3" :key="index" class="space-y-3 motion-safe:animate-pulse">
        <div class="h-4 w-1/3 rounded bg-gray-100 dark:bg-dark-700"></div>
        <div class="h-3 w-2/3 rounded bg-gray-100 dark:bg-dark-700"></div>
      </div>
      <span class="sr-only">{{ t('tickets.list.loading') }}</span>
    </div>

    <div v-else-if="!result || result.items.length === 0" class="card px-6 py-16 text-center" data-testid="tickets-empty">
      <LifeBuoy class="mx-auto h-12 w-12 text-gray-400 dark:text-dark-500" aria-hidden="true" />
      <p class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">
        {{ filtered ? t('tickets.list.emptyFiltered') : t('tickets.list.empty') }}
      </p>
      <p v-if="!filtered" class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.list.emptyDescription') }}</p>
    </div>

    <div v-else class="card overflow-hidden">
      <ul class="divide-y divide-gray-100 dark:divide-dark-700">
        <li v-for="ticket in result.items" :key="ticket.id">
          <RouterLink
            :to="`/tickets/${ticket.id}`"
            class="flex flex-col gap-3 px-4 py-4 transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/60 sm:px-5"
            :data-testid="`ticket-row-${ticket.id}`"
          >
            <div class="flex items-start gap-3">
              <span
                v-if="ticket.unread"
                class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-red-500"
                :aria-label="t('tickets.list.unread')"
                data-testid="ticket-unread-dot"
              ></span>
              <span v-else class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-transparent" aria-hidden="true"></span>

              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="min-w-0 break-words text-sm font-semibold text-gray-900 dark:text-white">{{ ticket.subject }}</h3>
                  <TicketStatusBadge :status="ticket.status" />
                  <TicketPriorityBadge :priority="ticket.priority" />
                </div>
                <p class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                  <span class="font-mono">{{ ticket.ticket_no }}</span>
                  <span>{{ t(`tickets.category.${ticket.category}`) }}</span>
                  <span>{{ t('tickets.list.messageCount', { count: ticket.message_count }) }}</span>
                  <span>{{ t('tickets.list.lastMessageAt', { time: formatTime(ticket.last_message_at) }) }}</span>
                </p>
              </div>
            </div>
          </RouterLink>
        </li>
      </ul>
      <Pagination
        v-if="result.total > pageSize"
        :total="result.total"
        :page="page"
        :page-size="pageSize"
        :show-page-size-selector="false"
        @update:page="onPageChange"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { CircleAlert, LifeBuoy, Plus as PlusIcon, RefreshCw, Search as SearchIcon } from '@lucide/vue'
import Pagination from '@/components/common/Pagination.vue'
import TicketStatusBadge from './TicketStatusBadge.vue'
import TicketPriorityBadge from './TicketPriorityBadge.vue'
import { listTickets } from '@/api/tickets'
import { formatDateTime } from '@/utils/format'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import { TICKET_STATUSES, type TicketStatus, type TicketSummary } from '@/types/tickets'
import type { PaginatedResponse } from '@/types'

const emit = defineEmits<{
  (e: 'create'): void
  (e: 'loaded', total: number): void
}>()

const { t } = useI18n()

const PAGE_SIZE = 10
const statusOptions = TICKET_STATUSES

const result = ref<PaginatedResponse<TicketSummary> | null>(null)
const page = ref(1)
const pageSize = PAGE_SIZE
const keyword = ref('')
const statusFilter = ref<TicketStatus | ''>('')
const appliedKeyword = ref('')
const loading = ref(true)
const errorMessage = ref('')

const filtered = computed(() => statusFilter.value !== '' || appliedKeyword.value.trim() !== '')

function formatTime(value: string | null | undefined): string {
  return formatDateTime(value) || '-'
}

async function load(targetPage = page.value): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await listTickets({
      page: targetPage,
      page_size: PAGE_SIZE,
      status: statusFilter.value,
      keyword: appliedKeyword.value.trim() || undefined,
    })
    result.value = data
    page.value = data.page || targetPage
    emit('loaded', data.total)
  } catch (error) {
    errorMessage.value = resolveTicketErrorMessage(error, t, 'tickets.list.loadFailed')
  } finally {
    loading.value = false
  }
}

function applyFilters(): void {
  appliedKeyword.value = keyword.value
  void load(1)
}

function onPageChange(next: number): void {
  void load(next)
}

onMounted(() => {
  void load(1)
})

defineExpose({ load })
</script>
