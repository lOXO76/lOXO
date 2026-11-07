<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.rateLimits.lead') }}</p>

    <ul class="list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('ticketAgentDocs.rateLimits.item1') }}</li>
      <li>{{ t('ticketAgentDocs.rateLimits.item2') }}</li>
      <li>{{ t('ticketAgentDocs.rateLimits.item3') }}</li>
      <li>{{ t('ticketAgentDocs.rateLimits.item4') }}</li>
      <li>{{ t('ticketAgentDocs.rateLimits.item5') }}</li>
      <li>{{ t('ticketAgentDocs.rateLimits.item6') }}</li>
    </ul>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.rateLimits.headersTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('ticketAgentDocs.rateLimits.header1') }}</li>
        <li>{{ t('ticketAgentDocs.rateLimits.header2') }}</li>
        <li>{{ t('ticketAgentDocs.rateLimits.header3') }}</li>
      </ul>
      <DocsCodeBlock class="mt-2" label="429.response" code-test-id="ticket-agent-docs-429" :code="rateLimitExample" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.rateLimits.backoffTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('ticketAgentDocs.rateLimits.backoff1') }}</li>
        <li>{{ t('ticketAgentDocs.rateLimits.backoff2') }}</li>
        <li>{{ t('ticketAgentDocs.rateLimits.backoff3') }}</li>
        <li>{{ t('ticketAgentDocs.rateLimits.backoff4') }}</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'

const { t } = useI18n()

const rateLimitExample = `HTTP/1.1 429 Too Many Requests
Retry-After: 2
X-RateLimit-Limit: 2
X-RateLimit-Remaining: 0
Content-Type: application/json

{
  "code": 429,
  "message": "agent rate limit exceeded",
  "reason": "AGENT_RATE_LIMITED",
  "data": null
}`
</script>
