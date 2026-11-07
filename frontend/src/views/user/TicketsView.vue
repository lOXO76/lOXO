<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <header class="min-w-0">
        <h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">{{ t('tickets.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.description') }}</p>
      </header>

      <!-- 配置加载失败 -->
      <div v-if="configError" class="card px-6 py-16 text-center" role="alert">
        <CircleAlert class="mx-auto h-10 w-10 text-red-500" aria-hidden="true" />
        <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">{{ configError }}</p>
        <button type="button" class="btn btn-secondary mt-4" data-testid="tickets-config-retry" @click="loadConfig">
          {{ t('tickets.common.retry') }}
        </button>
      </div>

      <!-- 加载中 -->
      <div v-else-if="loadingConfig" class="card px-6 py-16 text-center" role="status">
        <LoadingSpinner size="md" class="mx-auto" />
        <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.loading') }}</p>
      </div>

      <!-- 系统未开启 -->
      <div v-else-if="!config?.enabled" class="card px-6 py-16 text-center" data-testid="tickets-disabled">
        <LifeBuoy class="mx-auto h-12 w-12 text-gray-400 dark:text-dark-500" aria-hidden="true" />
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('tickets.disabledTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.disabledDescription') }}</p>
      </div>

      <!-- Tab 容器 -->
      <template v-else>
        <div class="border-b border-gray-200 dark:border-dark-700">
          <nav class="-mb-px flex gap-2 overflow-x-auto" role="tablist" :aria-label="t('tickets.title')">
            <button
              v-for="tab in tabs"
              :key="tab.id"
              type="button"
              role="tab"
              :aria-selected="activeTab === tab.id"
              :class="[
                'whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-medium transition-colors',
                activeTab === tab.id
                  ? 'border-primary-500 text-primary-600 dark:border-primary-400 dark:text-primary-400'
                  : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200',
              ]"
              :data-testid="`tickets-tab-${tab.id}`"
              @click="selectTab(tab.id)"
            >
              {{ t(tab.labelKey) }}
            </button>
          </nav>
        </div>

        <TicketListPanel
          v-if="config"
          v-show="activeTab === 'list'"
          @create="showCreateDialog = true"
        />

        <NotifySettingsPanel v-if="config && visited.has('notify')" v-show="activeTab === 'notify'" />

        <AgentKeysPanel v-if="config && visited.has('agent')" v-show="activeTab === 'agent'" :config="config" />
      </template>

      <CreateTicketDialog
        v-if="config"
        :show="showCreateDialog"
        :categories="config.categories"
        :priorities="config.priorities"
        :max-subject-length="config.max_subject_length"
        :max-body-length="config.max_body_length"
        :max-open-tickets="config.max_open_tickets_per_user"
        @close="showCreateDialog = false"
        @created="onCreated"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { CircleAlert, LifeBuoy } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import TicketListPanel from '@/components/tickets/TicketListPanel.vue'
import NotifySettingsPanel from '@/components/tickets/NotifySettingsPanel.vue'
import AgentKeysPanel from '@/components/tickets/AgentKeysPanel.vue'
import CreateTicketDialog from '@/components/tickets/CreateTicketDialog.vue'
import { getTicketConfig } from '@/api/tickets'
import { useAppStore } from '@/stores/app'
import { refreshTicketUnreadCount } from '@/composables/useTicketUnread'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import type { TicketDetail, TicketPublicConfig } from '@/types/tickets'

type TabId = 'list' | 'notify' | 'agent'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const tabs: Array<{ id: TabId; labelKey: string }> = [
  { id: 'list', labelKey: 'tickets.tabs.list' },
  { id: 'notify', labelKey: 'tickets.tabs.notify' },
  { id: 'agent', labelKey: 'tickets.tabs.agent' },
]

const config = ref<TicketPublicConfig | null>(null)
const loadingConfig = ref(true)
const configError = ref('')
const activeTab = ref<TabId>('list')
const visited = ref<Set<TabId>>(new Set<TabId>(['list']))
const showCreateDialog = ref(false)

function selectTab(tab: TabId): void {
  activeTab.value = tab
  visited.value = new Set(visited.value).add(tab)
}

async function loadConfig(): Promise<void> {
  loadingConfig.value = true
  configError.value = ''
  try {
    config.value = await getTicketConfig()
  } catch (error) {
    configError.value = resolveTicketErrorMessage(error, t, 'tickets.configLoadFailed')
  } finally {
    loadingConfig.value = false
  }
}

function onCreated(detail: TicketDetail): void {
  showCreateDialog.value = false
  appStore.showSuccess(t('tickets.create.success'))
  void refreshTicketUnreadCount()
  if (detail?.ticket?.id) void router.push(`/tickets/${detail.ticket.id}`)
}

onMounted(() => {
  void loadConfig()
})
</script>
