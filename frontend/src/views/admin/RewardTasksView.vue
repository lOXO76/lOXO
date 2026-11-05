<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-emerald-50 text-emerald-500 dark:bg-emerald-900/30 dark:text-emerald-400">
                <Icon name="gift" size="sm" />
              </span>
              {{ t('admin.rewardTasks.title') }}
            </h1>
            <p class="page-description mt-1.5 max-w-3xl text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.rewardTasks.description') }}
            </p>
          </div>
          <router-link
            :to="{ name: 'RewardIntegrationDocs' }"
            target="_blank"
            rel="noopener"
            class="btn btn-secondary"
            data-testid="reward-tasks-docs-link"
          >
            <Icon name="externalLink" size="sm" />
            {{ t('admin.rewardTasks.docsLink') }}
          </router-link>
        </div>

        <!-- 总开关：切换即保存（带确认） -->
        <div class="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-gray-100 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
          <div class="min-w-0">
            <p class="flex flex-wrap items-center gap-2 text-sm font-medium text-gray-900 dark:text-white">
              {{ t('admin.rewardTasks.state.toggleLabel') }}
              <span
                class="badge"
                :class="settingsEnabled ? 'badge-success' : 'badge-gray'"
                data-testid="reward-tasks-state"
              >
                {{ settingsEnabled ? t('admin.rewardTasks.state.enabled') : t('admin.rewardTasks.state.disabled') }}
              </span>
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.rewardTasks.state.toggleHint') }}</p>
          </div>
          <Toggle
            :model-value="settingsEnabled"
            :disabled="settingsLoading || settingsSaving"
            data-testid="reward-tasks-toggle"
            @update:model-value="requestToggleSettings"
          />
        </div>

        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <nav class="tabs inline-flex w-full max-w-2xl sm:w-auto" role="tablist" :aria-label="t('admin.rewardTasks.tabs.aria')">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="activeTab === tab.value ? 'tab-active' : ''"
              :aria-selected="activeTab === tab.value"
              :data-testid="`reward-tasks-tab-${tab.value}`"
              @click="activeTab = tab.value"
            >
              {{ tab.label }}
            </button>
          </nav>
        </div>
      </header>

      <!-- 概览 -->
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4" data-testid="reward-tasks-stats">
        <StatCard :title="t('admin.rewardTasks.stats.totalTasks')" :value="stats.total_tasks" :icon="TasksStatIcon">
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.rewardTasks.stats.enabledHint', { count: stats.enabled_tasks }) }}
            </p>
          </template>
        </StatCard>
        <StatCard
          :title="t('admin.rewardTasks.stats.completedToday')"
          :value="stats.completed_today"
          :icon="TodayStatIcon"
          icon-variant="warning"
        >
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.rewardTasks.stats.todayRewardHint', { amount: formatCredit(stats.reward_today) }) }}
            </p>
          </template>
        </StatCard>
        <StatCard
          :title="t('admin.rewardTasks.stats.completedTotal')"
          :value="stats.completed_total"
          :format-value="formatCount"
          :icon="TotalStatIcon"
          icon-variant="success"
        >
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ t('admin.rewardTasks.stats.rewardTotalHint', { amount: formatCredit(stats.reward_total) }) }}
            </p>
          </template>
        </StatCard>
        <StatCard :title="t('admin.rewardTasks.stats.partners')" :value="stats.partners" :icon="PartnersStatIcon">
          <template #footer>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.rewardTasks.stats.partnersHint') }}</p>
          </template>
        </StatCard>
      </div>

      <TasksTab v-if="activeTab === 'tasks'" @changed="refreshStats" />
      <PartnersTab v-else-if="activeTab === 'partners'" @changed="refreshStats" />
      <RecordsTab v-else-if="activeTab === 'records'" />
      <EventsTab v-else />
    </div>

    <ConfirmDialog
      :show="showToggleConfirm"
      :title="pendingEnabled ? t('admin.rewardTasks.state.confirmEnableTitle') : t('admin.rewardTasks.state.confirmDisableTitle')"
      :message="pendingEnabled ? t('admin.rewardTasks.state.confirmEnableMessage') : t('admin.rewardTasks.state.confirmDisableMessage')"
      :confirm-text="t('common.confirm')"
      :cancel-text="t('common.cancel')"
      :danger="!pendingEnabled"
      @confirm="confirmToggleSettings"
      @cancel="showToggleConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { CalendarCheck2, ListChecks, Users, Wallet } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatCard from '@/components/common/StatCard.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import TasksTab from '@/components/admin/rewardTasks/TasksTab.vue'
import PartnersTab from '@/components/admin/rewardTasks/PartnersTab.vue'
import RecordsTab from '@/components/admin/rewardTasks/RecordsTab.vue'
import EventsTab from '@/components/admin/rewardTasks/EventsTab.vue'
import { formatCredit } from '@/components/admin/rewardTasks/format'
import { adminAPI } from '@/api/admin'
import type { AdminRewardTaskStats } from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type AdminTab = 'tasks' | 'partners' | 'records' | 'events'

const TasksStatIcon = { render: () => h(ListChecks, { strokeWidth: 1.75 }) }
const TodayStatIcon = { render: () => h(CalendarCheck2, { strokeWidth: 1.75 }) }
const TotalStatIcon = { render: () => h(Wallet, { strokeWidth: 1.75 }) }
const PartnersStatIcon = { render: () => h(Users, { strokeWidth: 1.75 }) }

const { t } = useI18n()
const appStore = useAppStore()

const activeTab = ref<AdminTab>('tasks')
const stats = reactive<AdminRewardTaskStats>({
  total_tasks: 0,
  enabled_tasks: 0,
  completed_today: 0,
  reward_today: 0,
  completed_total: 0,
  reward_total: 0,
  partners: 0,
})
const settingsEnabled = ref(false)
const settingsLoading = ref(false)
const settingsSaving = ref(false)
const showToggleConfirm = ref(false)
const pendingEnabled = ref(false)
let statsSequence = 0
let settingsSequence = 0

const tabs = computed(() => [
  { value: 'tasks' as AdminTab, label: t('admin.rewardTasks.tabs.tasks') },
  { value: 'partners' as AdminTab, label: t('admin.rewardTasks.tabs.partners') },
  { value: 'records' as AdminTab, label: t('admin.rewardTasks.tabs.records') },
  { value: 'events' as AdminTab, label: t('admin.rewardTasks.tabs.events') },
])

const formatCount = (value: number | string) => Number(value).toLocaleString()

async function loadStats() {
  const sequence = ++statsSequence
  try {
    const result = await adminAPI.rewardTasks.getStats()
    if (sequence !== statsSequence) return
    Object.assign(stats, result)
  } catch (error) {
    if (sequence !== statsSequence) return
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.loadFailed')))
  }
}

function refreshStats() {
  void loadStats()
}

async function loadSettings() {
  const sequence = ++settingsSequence
  settingsLoading.value = true
  try {
    const result = await adminAPI.rewardTasks.getSettings()
    if (sequence !== settingsSequence) return
    settingsEnabled.value = Boolean(result.enabled)
  } catch (error) {
    if (sequence !== settingsSequence) return
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.state.settingsLoadFailed')))
  } finally {
    if (sequence === settingsSequence) settingsLoading.value = false
  }
}

// 切换开关先弹确认，确认后才落库
function requestToggleSettings(value: boolean) {
  if (settingsSaving.value || value === settingsEnabled.value) return
  pendingEnabled.value = value
  showToggleConfirm.value = true
}

async function confirmToggleSettings() {
  if (settingsSaving.value) return
  const next = pendingEnabled.value
  const previous = settingsEnabled.value
  settingsSaving.value = true
  // 乐观更新，失败回滚
  settingsEnabled.value = next
  try {
    const result = await adminAPI.rewardTasks.updateSettings(next)
    settingsEnabled.value = Boolean(result.enabled)
    appStore.showSuccess(
      settingsEnabled.value ? t('admin.rewardTasks.state.enableSuccess') : t('admin.rewardTasks.state.disableSuccess'),
    )
    showToggleConfirm.value = false
    void loadStats()
  } catch (error) {
    settingsEnabled.value = previous
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.state.updateFailed')))
  } finally {
    settingsSaving.value = false
  }
}

onMounted(() => {
  void loadStats()
  void loadSettings()
})

onBeforeUnmount(() => {
  statsSequence += 1
  settingsSequence += 1
})
</script>
