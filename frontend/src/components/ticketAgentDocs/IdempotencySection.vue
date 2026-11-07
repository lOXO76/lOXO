<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.idempotency.lead') }}</p>

    <ul class="list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('ticketAgentDocs.idempotency.point1') }}</li>
      <li>{{ t('ticketAgentDocs.idempotency.point2') }}</li>
      <li>{{ t('ticketAgentDocs.idempotency.point3') }}</li>
      <li>{{ t('ticketAgentDocs.idempotency.point4') }}</li>
    </ul>

    <DocsCodeBlock label="idempotency.sh" code-test-id="ticket-agent-docs-idempotency" :code="idempotencyExample" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'

const { t } = useI18n()

const idempotencyExample = `curl -sS -X POST "$BASE_URL/api/v1/agent/tickets/1/messages" \\
  -H "Authorization: Bearer $AGENT_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: reply-ticket-1-20261106-001" \\
  -d '{"body":"Thanks for the report — we are checking it now."}'

# 24 小时内用同一个 Idempotency-Key 重试：返回第一次的消息，不会重复写入`
</script>
