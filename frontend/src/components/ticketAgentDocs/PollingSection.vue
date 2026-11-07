<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.polling.lead') }}</p>

    <ol class="list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('ticketAgentDocs.polling.step1') }}</li>
      <li>{{ t('ticketAgentDocs.polling.step2') }}</li>
      <li>{{ t('ticketAgentDocs.polling.step3') }}</li>
      <li>{{ t('ticketAgentDocs.polling.step4') }}</li>
      <li>{{ t('ticketAgentDocs.polling.step5') }}</li>
      <li>{{ t('ticketAgentDocs.polling.step6') }}</li>
    </ol>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.polling.stateTitle') }}</h3>
      <p class="mt-1.5 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.polling.stateHint') }}</p>
    </div>

    <DocsCodeBlock label="updates.response.json" code-test-id="ticket-agent-docs-updates" :code="updatesExample" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'

const { t } = useI18n()

const updatesExample = `GET /api/v1/agent/updates?cursor=118&limit=50

{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "cursor": 120,
        "ticket": { "id": 1, "ticket_no": "TK20261106A1B2C3", "status": "pending_admin", "...": "..." },
        "message": { "id": 120, "ticket_id": 1, "author_type": "user", "via": "web", "internal": false, "body": "Any update?", "...": "..." }
      }
    ],
    "next_cursor": 120,
    "has_more": false
  }
}`
</script>
