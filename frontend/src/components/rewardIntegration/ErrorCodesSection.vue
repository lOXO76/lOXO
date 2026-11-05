<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.errors.intro') }}</p>

    <DocsTable
      :headers="[
        t('rewardIntegration.errors.colReason'),
        t('rewardIntegration.errors.colHttp'),
        t('rewardIntegration.errors.colScenario'),
        t('rewardIntegration.errors.colAction'),
      ]"
    >
      <tr v-for="item in errorCodes" :key="item.reason">
        <td>{{ item.reason }}</td>
        <td>
          <span class="rounded px-1.5 py-0.5 text-xs font-semibold" :class="statusClass(item.http)">{{ item.http }}</span>
        </td>
        <td>{{ t(item.scenarioKey) }}</td>
        <td>{{ t(item.actionKey) }}</td>
      </tr>
    </DocsTable>

    <p class="text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('rewardIntegration.errors.footer') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsTable from './DocsTable.vue'
import { REWARD_OPEN_ERROR_CODES } from './errorCodes'

const { t } = useI18n()
const errorCodes = REWARD_OPEN_ERROR_CODES

function statusClass(http: string): string {
  if (http.startsWith('401')) return 'bg-amber-100 text-amber-800 dark:bg-amber-500/15 dark:text-amber-300'
  if (http.startsWith('403')) return 'bg-orange-100 text-orange-800 dark:bg-orange-500/15 dark:text-orange-300'
  if (http.startsWith('404')) return 'bg-gray-200 text-gray-700 dark:bg-dark-700 dark:text-dark-200'
  if (http.startsWith('409')) return 'bg-red-100 text-red-700 dark:bg-red-500/15 dark:text-red-300'
  return 'bg-sky-100 text-sky-800 dark:bg-sky-500/15 dark:text-sky-300'
}
</script>
