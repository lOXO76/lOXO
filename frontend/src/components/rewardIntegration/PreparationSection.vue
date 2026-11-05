<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.preparation.intro') }}</p>

    <ol class="list-decimal space-y-2 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('rewardIntegration.preparation.step1') }}</li>
      <li>{{ t('rewardIntegration.preparation.step2') }}</li>
      <li>{{ t('rewardIntegration.preparation.step3') }}</li>
      <li>{{ t('rewardIntegration.preparation.step4') }}</li>
      <li>{{ t('rewardIntegration.preparation.step5') }}</li>
    </ol>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.preparation.infoTitle') }}</h3>
      <DocsTable class="mt-2" :headers="[t('rewardIntegration.preparation.infoColItem'), t('rewardIntegration.preparation.infoColDesc')]">
        <tr>
          <td>{{ t('rewardIntegration.preparation.infoName') }}</td>
          <td>{{ t('rewardIntegration.preparation.infoNameDesc') }}</td>
        </tr>
        <tr>
          <td>allowed_domains</td>
          <td>{{ t('rewardIntegration.preparation.infoDomainsDesc') }}</td>
        </tr>
        <tr>
          <td>{{ t('rewardIntegration.preparation.infoContact') }}</td>
          <td>{{ t('rewardIntegration.preparation.infoContactDesc') }}</td>
        </tr>
        <tr>
          <td>app_key / app_secret</td>
          <td>{{ t('rewardIntegration.preparation.infoSecretDesc') }}</td>
        </tr>
        <tr>
          <td>{{ t('rewardIntegration.preparation.infoTask') }}</td>
          <td>{{ t('rewardIntegration.preparation.infoTaskDesc') }}</td>
        </tr>
      </DocsTable>
    </div>

    <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-500/30 dark:bg-amber-500/10">
      <p class="text-sm font-semibold text-amber-800 dark:text-amber-200">{{ t('rewardIntegration.preparation.tipTitle') }}</p>
      <p class="mt-1 text-sm leading-6 text-amber-700 dark:text-amber-200/90">{{ t('rewardIntegration.preparation.tip') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsTable from './DocsTable.vue'

const { t } = useI18n()
</script>
