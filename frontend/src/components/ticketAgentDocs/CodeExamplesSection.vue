<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.examples.lead') }}</p>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.examples.curlTitle') }}</h3>
      <DocsCodeBlock class="mt-2" label="example.sh" code-test-id="ticket-agent-docs-curl" :code="CURL_EXAMPLE" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.examples.pythonTitle') }}</h3>
      <DocsCodeBlock class="mt-2" label="agent.py" code-test-id="ticket-agent-docs-python" :code="PYTHON_EXAMPLE" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.examples.nodeTitle') }}</h3>
      <DocsCodeBlock class="mt-2" label="agent.mjs" code-test-id="ticket-agent-docs-node" :code="NODE_EXAMPLE" />
    </div>

    <p class="text-xs leading-5 text-gray-400 dark:text-dark-500">{{ t('ticketAgentDocs.examples.note') }}</p>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import { CURL_EXAMPLE, NODE_EXAMPLE, PYTHON_EXAMPLE } from './examples'

const { t } = useI18n()
</script>
