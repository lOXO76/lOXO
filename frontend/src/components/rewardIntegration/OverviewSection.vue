<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.overview.intro') }}</p>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.overview.whatTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.overview.what1') }}</li>
        <li>{{ t('rewardIntegration.overview.what2') }}</li>
        <li>{{ t('rewardIntegration.overview.what3') }}</li>
      </ul>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.overview.typesTitle') }}</h3>
      <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.overview.typesIntro') }}</p>
      <DocsTable
        class="mt-3"
        :headers="[
          t('rewardIntegration.overview.colType'),
          t('rewardIntegration.overview.colVerify'),
          t('rewardIntegration.overview.colCondition'),
          t('rewardIntegration.overview.colPartner'),
        ]"
      >
        <tr v-for="row in typeRows" :key="`${row.taskType}-${row.verify}`">
          <td>{{ row.taskType }}</td>
          <td class="font-mono text-xs">{{ row.verify }}</td>
          <td>{{ row.condition }}</td>
          <td>{{ row.partner }}</td>
        </tr>
      </DocsTable>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.overview.flowTitle') }}</h3>
      <ol class="mt-2 list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.overview.flow1') }}</li>
        <li>{{ t('rewardIntegration.overview.flow2') }}</li>
        <li>{{ t('rewardIntegration.overview.flow3') }}</li>
        <li>{{ t('rewardIntegration.overview.flow4') }}</li>
        <li>{{ t('rewardIntegration.overview.flow5') }}</li>
        <li>{{ t('rewardIntegration.overview.flow6') }}</li>
      </ol>
    </div>

    <p class="rounded-lg border border-primary-200 bg-primary-50 px-4 py-3 text-sm leading-6 text-primary-800 dark:border-primary-500/30 dark:bg-primary-500/10 dark:text-primary-200">
      {{ t('rewardIntegration.overview.flowNote') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsTable from './DocsTable.vue'

const { t } = useI18n()

const typeRows = computed(() => [
  {
    taskType: 'browse',
    verify: 'timer',
    condition: t('rewardIntegration.overview.typeBrowseTimerCondition'),
    partner: t('rewardIntegration.overview.typeBrowseTimerPartner'),
  },
  {
    taskType: 'browse',
    verify: 'sdk',
    condition: t('rewardIntegration.overview.typeBrowseSdkCondition'),
    partner: t('rewardIntegration.overview.typeBrowseSdkPartner'),
  },
  {
    taskType: 'register',
    verify: 'callback',
    condition: t('rewardIntegration.overview.typeRegisterCondition'),
    partner: t('rewardIntegration.overview.typeRegisterPartner'),
  },
  {
    taskType: 'recharge',
    verify: 'callback',
    condition: t('rewardIntegration.overview.typeRechargeCondition'),
    partner: t('rewardIntegration.overview.typeRechargePartner'),
  },
])
</script>
