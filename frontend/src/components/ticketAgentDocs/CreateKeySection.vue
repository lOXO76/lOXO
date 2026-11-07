<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.createKey.lead') }}</p>

    <ol class="list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
      <li>{{ t('ticketAgentDocs.createKey.step1') }}</li>
      <li>{{ t('ticketAgentDocs.createKey.step2') }}</li>
      <li>{{ t('ticketAgentDocs.createKey.step3') }}</li>
      <li>{{ t('ticketAgentDocs.createKey.step4') }}</li>
    </ol>

    <p class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300">
      {{ t('ticketAgentDocs.createKey.note') }}
    </p>

    <DocsCodeBlock label="key-prefix.txt" :code="keyShape" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'

const { t } = useI18n()

const keyShape = `lxt_AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_  # 完整密钥，仅创建时返回一次
lxt_AbCdEfGh                                         # key_prefix，列表里展示前 12 个字符`
</script>
