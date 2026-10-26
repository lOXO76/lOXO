<template>
  <AppLayout>
    <div class="space-y-5">
      <!-- 控制栏：功能状态 + 统计周期（筛选项统一放在图表上方一行） -->
      <div class="flex flex-wrap items-center gap-3">
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-2 text-xs" data-testid="check-in-state">
          <template v-if="stats">
            <span :class="['badge', stats.summary.check_in_enabled ? 'badge-success' : 'badge-gray']">
              {{ stats.summary.check_in_enabled ? t('admin.checkIns.state.enabled') : t('admin.checkIns.state.disabled') }}
            </span>
            <span :class="['badge', stats.summary.captcha_enabled ? 'badge-primary' : 'badge-gray']">
              {{ stats.summary.captcha_enabled ? t('admin.checkIns.state.captchaOn') : t('admin.checkIns.state.captchaOff') }}
            </span>
            <span class="text-gray-500 dark:text-dark-400">
              {{ stats.summary.configured_tiers > 0
                ? t('admin.checkIns.state.configuredRange', {
                    count: stats.summary.configured_tiers,
                    min: formatCredit(stats.summary.configured_reward_min),
                    max: formatCredit(stats.summary.configured_reward_max),
                  })
                : t('admin.checkIns.state.noTiers') }}
            </span>
            <router-link to="/admin/settings" class="text-primary-600 hover:underline dark:text-primary-400">
              {{ t('admin.checkIns.state.goSettings') }}
            </router-link>
          </template>
        </div>

        <div class="flex items-center gap-2">
          <div class="inline-flex rounded-lg border border-gray-200 p-0.5 dark:border-dark-600" role="group" :aria-label="t('admin.checkIns.range.label')">
            <button
              v-for="option in rangeOptions"
              :key="option"
              type="button"
              :data-testid="`range-${option}`"
              :aria-pressed="days === option"
              :class="[
                'rounded-md px-3 py-1 text-xs font-medium transition-colors',
                days === option
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 hover:bg-gray-100 dark:text-dark-300 dark:hover:bg-dark-700',
              ]"
              @click="changeRange(option)"
            >
              {{ t('admin.checkIns.range.days', { days: option }) }}
            </button>
          </div>
          <button class="btn btn-secondary px-2.5" type="button" :title="t('admin.checkIns.refresh')" :disabled="statsLoading" @click="refreshAll">
            <Icon name="refresh" size="sm" :class="statsLoading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <!-- 概览 -->
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" data-testid="check-in-summary">
        <StatCard :title="t('admin.checkIns.summary.todayUsers')" :value="summary.today_users" :icon="UsersStatIcon">
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkIns.summary.todayReward') }} {{ formatCredit(summary.today_reward) }}
            </p>
          </template>
        </StatCard>
        <StatCard :title="t('admin.checkIns.summary.last7DaysUsers')" :value="summary.last_7_days_users" :icon="CalendarStatIcon" />
        <StatCard
          :title="t('admin.checkIns.summary.periodReward')"
          :value="summary.period_reward"
          :format-value="formatCreditValue"
          :icon="CoinsStatIcon"
          icon-variant="warning"
        >
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkIns.summary.periodCheckIns', { count: summary.period_check_ins }) }}
            </p>
          </template>
        </StatCard>
        <StatCard
          :title="t('admin.checkIns.summary.totalReward')"
          :value="summary.total_reward"
          :format-value="formatCreditValue"
          :icon="WalletStatIcon"
          icon-variant="success"
        >
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.checkIns.summary.totalCheckIns', { count: summary.total_check_ins, users: summary.total_users }) }}
              · {{ t('admin.checkIns.summary.averageReward') }} {{ formatCredit(summary.average_reward) }}
            </p>
          </template>
        </StatCard>
      </div>

      <!-- 趋势：人数与签到额度量纲不同，拆成两张单系列小图，不用双纵轴 -->
      <div class="grid gap-4 lg:grid-cols-2">
        <section class="card p-5" data-testid="chart-users">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.checkIns.charts.dailyUsers') }}</h2>
          <CheckInDailyChart
            class="mt-3"
            :title="t('admin.checkIns.charts.dailyUsers')"
            :labels="dailyLabels"
            :values="dailyUsers"
            :empty-text="t('admin.checkIns.charts.empty')"
            :format-value="formatUsers"
            integer
          />
        </section>
        <section class="card p-5" data-testid="chart-reward">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.checkIns.charts.dailyReward') }}</h2>
          <CheckInDailyChart
            class="mt-3"
            :title="t('admin.checkIns.charts.dailyReward')"
            :labels="dailyLabels"
            :values="dailyRewards"
            :empty-text="t('admin.checkIns.charts.empty')"
            :format-value="formatCredit"
          />
        </section>
      </div>

      <!-- 档位实际分布：子弹图，实心条为实际占比，竖线为配置概率 -->
      <section class="card p-5" data-testid="check-in-tiers">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.checkIns.tiers.title') }}</h2>
            <p class="mt-1 max-w-3xl text-xs text-gray-500 dark:text-dark-400">{{ t('admin.checkIns.tiers.hint') }}</p>
          </div>
          <div class="flex items-center gap-4 text-xs text-gray-600 dark:text-dark-300" aria-hidden="true">
            <span class="inline-flex items-center gap-1.5">
              <span class="h-2.5 w-4 rounded-sm bg-primary-600 dark:bg-[#14a394]"></span>{{ t('admin.checkIns.tiers.legendActual') }}
            </span>
            <span class="inline-flex items-center gap-1.5">
              <span class="h-3 w-0.5 bg-gray-900 dark:bg-gray-100"></span>{{ t('admin.checkIns.tiers.legendConfigured') }}
            </span>
          </div>
        </div>

        <div v-if="stats && stats.tiers.length" class="mt-4 overflow-x-auto">
          <table class="w-full min-w-[720px] text-left text-sm">
            <thead>
              <tr class="border-b border-gray-200 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">
                <th class="py-2 pr-3 font-medium">{{ t('admin.checkIns.tiers.tier') }}</th>
                <th class="py-2 pr-3 font-medium">{{ t('admin.checkIns.tiers.configuredRange') }}</th>
                <th class="w-[34%] py-2 pr-3 font-medium">{{ t('admin.checkIns.tiers.actual') }}</th>
                <th class="py-2 pr-3 text-right font-medium">{{ t('admin.checkIns.tiers.configuredPercent') }}</th>
                <th class="py-2 pr-3 text-right font-medium">{{ t('admin.checkIns.tiers.count') }}</th>
                <th class="py-2 pr-3 text-right font-medium">{{ t('admin.checkIns.tiers.reward') }}</th>
                <th class="py-2 text-right font-medium">{{ t('admin.checkIns.tiers.observedRange') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="tier in stats.tiers"
                :key="tier.tier_index"
                class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                :data-testid="`tier-row-${tier.tier_index}`"
              >
                <td class="py-3 pr-3 font-medium text-gray-900 dark:text-white">
                  #{{ tier.tier_index + 1 }}
                  <span v-if="tier.configured_percent === null" class="ml-1 text-xs font-normal text-gray-400 dark:text-dark-500">
                    {{ t('admin.checkIns.tiers.legacy') }}
                  </span>
                </td>
                <td class="py-3 pr-3 tabular-nums text-gray-600 dark:text-dark-300">
                  {{ tier.configured_min === null ? '-' : `${formatCredit(tier.configured_min)} ~ ${formatCredit(tier.configured_max ?? 0)}` }}
                </td>
                <td class="py-3 pr-3">
                  <div class="flex items-center gap-3">
                    <div class="relative h-2.5 flex-1 rounded-full bg-gray-100 dark:bg-dark-700">
                      <div
                        class="h-full rounded-full bg-primary-600 dark:bg-[#14a394]"
                        :style="{ width: `${clampPercent(tier.actual_percent)}%` }"
                      ></div>
                      <div
                        v-if="tier.configured_percent !== null"
                        class="absolute -top-1 h-[18px] w-0.5 -translate-x-1/2 rounded-full bg-gray-900 dark:bg-gray-100"
                        :style="{ left: `${clampPercent(tier.configured_percent)}%` }"
                        :title="`${t('admin.checkIns.tiers.configuredPercent')} ${tier.configured_percent}%`"
                      ></div>
                    </div>
                    <span class="w-14 text-right text-sm font-medium tabular-nums text-gray-900 dark:text-white">{{ tier.actual_percent }}%</span>
                  </div>
                </td>
                <td class="py-3 pr-3 text-right tabular-nums text-gray-600 dark:text-dark-300">
                  {{ tier.configured_percent === null ? '-' : `${tier.configured_percent}%` }}
                </td>
                <td class="py-3 pr-3 text-right tabular-nums text-gray-900 dark:text-white">{{ tier.count.toLocaleString() }}</td>
                <td class="py-3 pr-3 text-right tabular-nums text-gray-900 dark:text-white">{{ formatCredit(tier.reward) }}</td>
                <td class="py-3 text-right tabular-nums text-gray-500 dark:text-dark-400">
                  {{ tier.count > 0 ? `${formatCredit(tier.observed_min)} ~ ${formatCredit(tier.observed_max)}` : '-' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else-if="!statsLoading" class="mt-6 text-center text-sm text-gray-400 dark:text-dark-500">
          {{ t('admin.checkIns.tiers.empty') }}
        </p>
      </section>

      <!-- 签到明细 -->
      <section class="card" data-testid="check-in-records">
        <div class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="mr-auto text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.checkIns.records.title') }}</h2>
          <div class="relative w-full sm:w-72">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-10"
              data-testid="records-search"
              :placeholder="t('admin.checkIns.records.searchPlaceholder')"
              @input="debounceLoadRecords"
            />
          </div>
          <input v-model="filters.start_date" type="date" class="input w-full sm:w-40" :title="t('admin.checkIns.records.startDate')" @change="reloadRecordsFromFirstPage" />
          <input v-model="filters.end_date" type="date" class="input w-full sm:w-40" :title="t('admin.checkIns.records.endDate')" @change="reloadRecordsFromFirstPage" />
        </div>

        <DataTable
          :columns="recordColumns"
          :data="records"
          :loading="recordsLoading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="sortStorageKey"
          @sort="handleSort"
        >
          <template #cell-user="{ row }">
            <div class="min-w-0">
              <div class="truncate text-sm font-medium text-gray-900 dark:text-white">
                {{ row.user_email || t('admin.checkIns.records.deletedUser') }}
              </div>
              <div class="text-xs text-gray-500 dark:text-dark-400">#{{ row.user_id }}<template v-if="row.username"> · {{ row.username }}</template></div>
            </div>
          </template>
          <template #cell-check_in_date="{ row }">
            <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">{{ row.check_in_date }}</span>
          </template>
          <template #cell-reward="{ row }">
            <span class="tabular-nums text-sm font-medium text-gray-900 dark:text-white">{{ formatCredit(row.reward) }}</span>
          </template>
          <template #cell-tier_index="{ row }">
            <span class="text-sm text-gray-600 dark:text-dark-300">#{{ row.tier_index + 1 }}</span>
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-sm text-gray-600 dark:text-dark-300">{{ formatDateTime(row.created_at) }}</span>
          </template>
          <template #empty>
            <p class="py-8 text-center text-sm text-gray-400 dark:text-dark-500">{{ t('admin.checkIns.records.empty') }}</p>
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
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarCheck2, Coins, Users, Wallet } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import StatCard from '@/components/common/StatCard.vue'
import Icon from '@/components/icons/Icon.vue'
import CheckInDailyChart from '@/components/admin/checkIn/CheckInDailyChart.vue'
import type { Column } from '@/components/common/types'
import { checkInsAPI, type AdminCheckInRecord, type CheckInStats, type CheckInStatsSummary } from '@/api/admin/checkIns'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const UsersStatIcon = { render: () => h(Users, { strokeWidth: 1.75 }) }
const CalendarStatIcon = { render: () => h(CalendarCheck2, { strokeWidth: 1.75 }) }
const CoinsStatIcon = { render: () => h(Coins, { strokeWidth: 1.75 }) }
const WalletStatIcon = { render: () => h(Wallet, { strokeWidth: 1.75 }) }

const rangeOptions = [7, 30, 90] as const
const days = ref<number>(30)
const stats = ref<CheckInStats | null>(null)
const statsLoading = ref(false)

const emptySummary: CheckInStatsSummary = {
  today_date: '',
  today_users: 0,
  today_reward: 0,
  last_7_days_users: 0,
  total_check_ins: 0,
  total_users: 0,
  total_reward: 0,
  average_reward: 0,
  period_check_ins: 0,
  period_reward: 0,
  period_start_date: '',
  check_in_enabled: false,
  captcha_enabled: false,
  configured_tiers: 0,
  configured_reward_min: 0,
  configured_reward_max: 0,
}
const summary = computed(() => stats.value?.summary ?? emptySummary)

// 签到额度与站内余额一致用「$」；不足 $1 的小额最多保留 4 位小数，避免显示成 $0.00。
const preciseFormatter = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 4 })
const compactFormatter = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 2 })
function formatCredit(value: number): string {
  const amount = Number.isFinite(value) ? value : 0
  return Math.abs(amount) >= 1 || amount === 0 ? compactFormatter.format(amount) : preciseFormatter.format(amount)
}
const formatCreditValue = (value: number | string) => formatCredit(Number(value))
const formatUsers = (value: number) => t('admin.checkIns.charts.users', { count: Math.round(value) })
const clampPercent = (value: number) => Math.max(0, Math.min(100, value))

const dailyLabels = computed(() => (stats.value?.daily ?? []).map((item) => item.date.slice(5)))
const dailyUsers = computed(() => (stats.value?.daily ?? []).map((item) => item.users))
const dailyRewards = computed(() => (stats.value?.daily ?? []).map((item) => item.reward))

async function loadStats(): Promise<void> {
  statsLoading.value = true
  try {
    stats.value = await checkInsAPI.getStats(days.value)
  } catch (error) {
    appStore.showError(t('admin.checkIns.loadFailed'))
    console.error('Failed to load check-in stats:', error)
  } finally {
    statsLoading.value = false
  }
}

function changeRange(value: number): void {
  if (days.value === value) return
  days.value = value
  void loadStats()
}

// ---- 明细 ----
const records = ref<AdminCheckInRecord[]>([])
const recordsLoading = ref(false)
const filters = reactive({ search: '', start_date: '', end_date: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let recordsRequest = 0

const recordColumns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.checkIns.records.user'), sortable: true },
  { key: 'check_in_date', label: t('admin.checkIns.records.checkInDate'), sortable: true },
  { key: 'reward', label: t('admin.checkIns.records.reward'), sortable: true },
  { key: 'tier_index', label: t('admin.checkIns.records.tier'), sortable: true },
  { key: 'created_at', label: t('admin.checkIns.records.createdAt'), sortable: true },
])

// DataTable 只在内部恢复持久化的排序、不会回传，首次请求需自行读取保持一致
const sortStorageKey = 'admin-check-in-records-sort'
function loadInitialSort(): { by: string; order: 'asc' | 'desc' } {
  const fallback = { by: 'created_at', order: 'desc' as const }
  try {
    const parsed = JSON.parse(localStorage.getItem(sortStorageKey) || 'null') as { key?: string; order?: string } | null
    const key = typeof parsed?.key === 'string' ? parsed.key : ''
    if (!recordColumns.value.some((column) => column.key === key && column.sortable)) return fallback
    return { by: key, order: parsed?.order === 'asc' ? 'asc' : 'desc' }
  } catch {
    return fallback
  }
}
const sort = reactive<{ by: string; order: 'asc' | 'desc' }>(loadInitialSort())

async function loadRecords(): Promise<void> {
  const request = ++recordsRequest
  recordsLoading.value = true
  try {
    const result = await checkInsAPI.listRecords({
      page: pagination.page,
      page_size: pagination.page_size,
      search: filters.search.trim() || undefined,
      start_date: filters.start_date || undefined,
      end_date: filters.end_date || undefined,
      sort_by: sort.by,
      sort_order: sort.order,
    })
    // 搜索输入较快时只采用最后一次请求的结果
    if (request !== recordsRequest) return
    records.value = result.items
    pagination.total = result.total
  } catch (error) {
    if (request !== recordsRequest) return
    appStore.showError(t('admin.checkIns.loadFailed'))
    console.error('Failed to load check-in records:', error)
  } finally {
    if (request === recordsRequest) recordsLoading.value = false
  }
}

function reloadRecordsFromFirstPage(): void {
  pagination.page = 1
  void loadRecords()
}

function debounceLoadRecords(): void {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(reloadRecordsFromFirstPage, 300)
}

function handleSort(key: string, order: 'asc' | 'desc'): void {
  sort.by = key
  sort.order = order
  reloadRecordsFromFirstPage()
}

function handlePageChange(page: number): void {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(pageSize: number): void {
  pagination.page_size = pageSize
  reloadRecordsFromFirstPage()
}

function refreshAll(): void {
  void loadStats()
  void loadRecords()
}

onMounted(refreshAll)
onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
})
</script>
