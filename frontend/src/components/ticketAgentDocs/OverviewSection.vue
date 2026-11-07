<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.overview.lead') }}</p>

    <ul class="list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('ticketAgentDocs.overview.point1') }}</li>
      <li>{{ t('ticketAgentDocs.overview.point2') }}</li>
      <li>{{ t('ticketAgentDocs.overview.point3') }}</li>
      <li>{{ t('ticketAgentDocs.overview.point4') }}</li>
      <li>{{ t('ticketAgentDocs.overview.point5') }}</li>
    </ul>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.overview.baseTitle') }}</h3>
      <p class="mt-1.5 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.overview.baseHint') }}</p>
      <DocsCodeBlock class="mt-2" label="base-url" code-test-id="ticket-agent-docs-base-url" :code="baseUrl" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.overview.envelopeTitle') }}</h3>
      <p class="mt-1.5 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.overview.envelopeHint') }}</p>
      <DocsCodeBlock class="mt-2" label="envelope.json" :code="envelope" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import { DOCS_ORIGIN_PLACEHOLDER } from './examples'

const { t } = useI18n()

const baseUrl = `${DOCS_ORIGIN_PLACEHOLDER}/api/v1/agent`

const envelope = `// 成功
{
  "code": 0,
  "message": "success",
  "data": { }
}

// 失败
{
  "code": 401,
  "message": "agent key is invalid",
  "reason": "AGENT_KEY_INVALID",
  "data": null
}`
</script>
