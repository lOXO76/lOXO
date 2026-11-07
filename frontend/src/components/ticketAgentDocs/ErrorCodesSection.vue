<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.errors.lead') }}</p>

    <DocsTable :headers="headers">
      <tr v-for="code in codes" :key="code.id" :data-test="`ticket-agent-error-${code.reason}`">
        <td>{{ code.reason }}</td>
        <td class="font-mono text-xs">{{ code.http }}</td>
        <td>{{ t(`ticketAgentDocs.errors.items.${code.id}.meaning`) }}</td>
        <td>{{ t(`ticketAgentDocs.errors.items.${code.id}.action`) }}</td>
      </tr>
    </DocsTable>

    <DocsCodeBlock label="error.response.json" code-test-id="ticket-agent-docs-error-example" :code="errorExample" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import DocsTable from './DocsTable.vue'
import { AGENT_ERROR_CODES } from './examples'

const { t } = useI18n()
const codes = AGENT_ERROR_CODES

const headers = computed(() => [
  t('ticketAgentDocs.errors.columns.reason'),
  t('ticketAgentDocs.errors.columns.http'),
  t('ticketAgentDocs.errors.columns.meaning'),
  t('ticketAgentDocs.errors.columns.action'),
])

const errorExample = `HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "code": 404,
  "message": "ticket not found",
  "reason": "TICKET_NOT_FOUND",
  "data": null
}`
</script>
