<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-6 pb-8">
      <header class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="min-w-0">
            <h1 class="page-title flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-primary-50 text-primary-500 dark:bg-primary-900/30 dark:text-primary-400">
                <Icon name="chat" size="sm" />
              </span>
              {{ t('admin.tickets.title') }}
            </h1>
            <p class="page-description mt-1.5 max-w-3xl text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tickets.description') }}
            </p>
          </div>
          <router-link
            :to="{ name: 'TicketAgentDocs' }"
            target="_blank"
            rel="noopener"
            class="btn btn-secondary"
            data-testid="tickets-agent-docs-link"
          >
            <Icon name="externalLink" size="sm" />
            {{ t('admin.tickets.docsLink') }}
          </router-link>
        </div>

        <div class="mt-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <nav class="tabs inline-flex w-full max-w-2xl sm:w-auto" role="tablist" :aria-label="t('admin.tickets.tabs.aria')">
            <button
              v-for="tab in tabs"
              :key="tab.value"
              type="button"
              role="tab"
              class="tab flex-1 sm:flex-none"
              :class="activeTab === tab.value ? 'tab-active' : ''"
              :aria-selected="activeTab === tab.value"
              :data-testid="`tickets-tab-${tab.value}`"
              @click="selectTab(tab.value)"
            >
              {{ tab.label }}
            </button>
          </nav>
        </div>
      </header>

      <TicketsListTab v-if="activeTab === 'tickets'" />
      <TicketSettingsTab v-else-if="activeTab === 'settings'" />
      <TicketAgentKeysTab v-else />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import TicketsListTab from '@/components/admin/tickets/TicketsListTab.vue'
import TicketSettingsTab from '@/components/admin/tickets/TicketSettingsTab.vue'
import TicketAgentKeysTab from '@/components/admin/tickets/TicketAgentKeysTab.vue'

type AdminTab = 'tickets' | 'settings' | 'agentKeys'

const VALID_TABS: AdminTab[] = ['tickets', 'settings', 'agentKeys']

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

function initialTab(): AdminTab {
  const query = route.query.tab
  if (typeof query === 'string' && (VALID_TABS as string[]).includes(query)) return query as AdminTab
  return 'tickets'
}

const activeTab = ref<AdminTab>(initialTab())

const tabs = computed(() => [
  { value: 'tickets' as AdminTab, label: t('admin.tickets.tabs.tickets') },
  { value: 'settings' as AdminTab, label: t('admin.tickets.tabs.settings') },
  { value: 'agentKeys' as AdminTab, label: t('admin.tickets.tabs.agentKeys') },
])

// Tab 写回 query，刷新/分享链接时停留在同一分区
function selectTab(tab: AdminTab) {
  activeTab.value = tab
  void router.replace({ query: { ...route.query, tab } })
}
</script>
