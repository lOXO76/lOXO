<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.auth.lead') }}</p>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <h3 class="font-mono text-xs font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.auth.header1Title') }}</h3>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.auth.header1Desc') }}</p>
      </div>
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
        <h3 class="font-mono text-xs font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.auth.header2Title') }}</h3>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.auth.header2Desc') }}</p>
      </div>
    </div>

    <DocsCodeBlock :label="t('ticketAgentDocs.auth.exampleLabel')" code-test-id="ticket-agent-docs-auth-headers" :code="AUTH_HEADER_EXAMPLE" />

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.auth.securityTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('ticketAgentDocs.auth.security1') }}</li>
        <li>{{ t('ticketAgentDocs.auth.security2') }}</li>
        <li>{{ t('ticketAgentDocs.auth.security3') }}</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import { AUTH_HEADER_EXAMPLE } from './examples'

const { t } = useI18n()
</script>
