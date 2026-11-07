<template>
  <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
    <div class="flex items-center justify-between gap-3 border-b border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-800">
      <span class="truncate font-mono text-xs text-gray-500 dark:text-dark-300">{{ label }}</span>
      <button
        type="button"
        class="shrink-0 rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-600 transition hover:bg-gray-100 dark:border-dark-600 dark:bg-dark-900 dark:text-dark-200 dark:hover:bg-dark-700"
        :aria-label="`${t('ticketAgentDocs.copy')} ${label}`"
        :data-test="`ticket-agent-docs-copy-${label}`"
        @click="copy"
      >{{ copied ? t('ticketAgentDocs.copied') : t('ticketAgentDocs.copy') }}</button>
    </div>
    <pre class="overflow-x-auto bg-gray-950 p-4 text-xs leading-5 text-gray-100"><code :data-test="codeTestId">{{ code }}</code></pre>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'

const props = defineProps<{
  code: string
  label: string
  /** 可选：给 <code> 元素加 data-test，便于测试断言 */
  codeTestId?: string
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const copied = ref(false)
let copiedTimer: ReturnType<typeof setTimeout> | null = null

async function copy(): Promise<void> {
  const success = await copyToClipboard(props.code, t('ticketAgentDocs.copied'))
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
