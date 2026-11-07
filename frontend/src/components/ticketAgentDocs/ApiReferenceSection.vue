<template>
  <div class="space-y-6">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.endpoints.lead') }}</p>

    <div
      v-for="endpoint in endpoints"
      :key="endpoint.id"
      class="space-y-3 border-t border-gray-200 pt-5 first:border-t-0 first:pt-0 dark:border-dark-700"
      :data-test="`ticket-agent-endpoint-${endpoint.id}`"
    >
      <div>
        <div class="flex flex-wrap items-center gap-2 font-mono text-xs">
          <span class="rounded px-1.5 py-0.5 font-semibold" :class="methodBadgeClass(endpoint.method)">{{ endpoint.method }}</span>
          <span class="break-all text-gray-700 dark:text-dark-200">{{ endpoint.path }}</span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300">
            {{ t(scopeLabelKey(endpoint.scope)) }}
          </span>
          <span
            v-if="endpoint.isPublic"
            class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300"
          >public</span>
        </div>
        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t(endpoint.descKey) }}</p>
      </div>

      <DocsCodeBlock
        v-if="endpoint.requestExample"
        :label="endpoint.requestLabel || 'request.json'"
        :code="endpoint.requestExample"
      />
      <DocsCodeBlock
        v-if="endpoint.responseExample"
        :label="endpoint.responseLabel || 'response.json'"
        :code="endpoint.responseExample"
      />

      <ul
        v-if="endpoint.noteKeys?.length"
        class="list-disc space-y-1 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200"
      >
        <li v-for="noteKey in endpoint.noteKeys" :key="noteKey">{{ t(noteKey) }}</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import { AGENT_ENDPOINTS } from './examples'
import { methodBadgeClass, scopeLabelKey } from './sections'

const { t } = useI18n()
const endpoints = AGENT_ENDPOINTS
</script>
