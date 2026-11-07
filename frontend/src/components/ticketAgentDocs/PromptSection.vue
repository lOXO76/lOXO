<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('ticketAgentDocs.prompt.lead') }}</p>

    <div class="flex flex-wrap items-center justify-between gap-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('ticketAgentDocs.prompt.templateTitle') }}</h3>
      <button type="button" class="btn btn-secondary btn-sm" data-testid="ticket-agent-docs-copy-prompt" @click="copyPrompt">
        <Icon name="copy" size="sm" />
        {{ copied ? t('ticketAgentDocs.prompt.copiedPrompt') : t('ticketAgentDocs.prompt.copyPrompt') }}
      </button>
    </div>

    <p class="text-xs leading-5 text-gray-400 dark:text-dark-500">{{ t('ticketAgentDocs.prompt.hint') }}</p>

    <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
      <pre class="max-h-[32rem] overflow-auto bg-gray-950 p-4 text-xs leading-5 text-gray-100"><code data-testid="ticket-agent-docs-prompt">{{ prompt }}</code></pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { agentPromptTemplate } from './examples'

const { t, locale } = useI18n()
const { copyToClipboard } = useClipboard()

const prompt = computed(() => agentPromptTemplate(locale.value))
const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copyPrompt() {
  const success = await copyToClipboard(prompt.value, t('ticketAgentDocs.prompt.copiedPrompt'))
  if (!success) return
  copied.value = true
  if (copiedTimer) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copied.value = false
    copiedTimer = null
  }, 2000)
}

onBeforeUnmount(() => {
  if (copiedTimer) clearTimeout(copiedTimer)
})
</script>
