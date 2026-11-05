<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.checklist.intro') }}</p>

    <ol class="list-decimal space-y-2 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('rewardIntegration.checklist.step1') }}</li>
      <li>{{ t('rewardIntegration.checklist.step2') }}</li>
      <li>{{ t('rewardIntegration.checklist.step3') }}</li>
      <li>{{ t('rewardIntegration.checklist.step4') }}</li>
      <li>{{ t('rewardIntegration.checklist.step5') }}</li>
      <li>{{ t('rewardIntegration.checklist.step6') }}</li>
    </ol>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
        <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.checklist.retryTitle') }}</p>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.checklist.retryDesc') }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
        <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.checklist.supportTitle') }}</p>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.checklist.supportDesc') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
</script>
