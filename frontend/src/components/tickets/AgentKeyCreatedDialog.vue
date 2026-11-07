<template>
  <BaseDialog :show="show" :title="t('tickets.agent.createdDialog.title')" width="normal" @close="close">
    <div class="space-y-4">
      <p class="rounded-xl bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
        {{ t('tickets.agent.createdDialog.warning') }}
      </p>

      <div>
        <p class="input-label mb-1.5">{{ t('tickets.agent.createdDialog.secret') }}</p>
        <div class="flex flex-col gap-2 sm:flex-row">
          <input
            :value="secret"
            readonly
            type="text"
            spellcheck="false"
            class="input w-full font-mono text-xs"
            data-testid="agent-secret-value"
            @focus="($event.target as HTMLInputElement).select()"
          />
          <button type="button" class="btn btn-secondary shrink-0" data-testid="agent-secret-copy" @click="copy">
            <CopyIcon class="h-4 w-4" aria-hidden="true" />
            {{ copied ? t('tickets.agent.createdDialog.copied') : t('tickets.agent.createdDialog.copy') }}
          </button>
        </div>
      </div>

      <dl v-if="key" class="grid grid-cols-2 gap-3 text-sm">
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('tickets.agent.table.name') }}</dt>
          <dd class="mt-0.5 break-all font-medium text-gray-900 dark:text-white">{{ key.name }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('tickets.agent.table.scope') }}</dt>
          <dd class="mt-0.5 text-gray-900 dark:text-white">{{ t(`tickets.agent.scope.${key.scope}`) }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('tickets.agent.table.qps') }}</dt>
          <dd class="mt-0.5 tabular-nums text-gray-900 dark:text-white">{{ key.qps_limit }}</dd>
        </div>
        <div>
          <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('tickets.agent.table.expires') }}</dt>
          <dd class="mt-0.5 text-gray-900 dark:text-white">
            {{ key.expires_at ? formatDateTime(key.expires_at) : t('tickets.agent.expiresNever') }}
          </dd>
        </div>
      </dl>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" data-testid="agent-secret-done" @click="close">
          {{ t('tickets.agent.createdDialog.done') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy as CopyIcon } from '@lucide/vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'
import type { TicketAgentKeyCreated } from '@/types/tickets'

const props = defineProps<{
  show: boolean
  createdKey: TicketAgentKeyCreated | null
}>()

const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

/**
 * 明文只保存在这个局部 ref 里：对话框关闭时立即清空，
 * 父组件也不会把它写回列表数据，保证「只显示一次」。
 */
const secret = ref('')
const key = ref<TicketAgentKeyCreated | null>(null)

watch(
  () => [props.show, props.createdKey] as const,
  ([show, createdKey]) => {
    if (show && createdKey) {
      secret.value = createdKey.secret
      key.value = createdKey
    }
  },
  { immediate: true },
)

function close(): void {
  secret.value = ''
  key.value = null
  emit('close')
}

async function copy(): Promise<void> {
  if (!secret.value) return
  await copyToClipboard(secret.value, t('tickets.agent.createdDialog.copied'))
}
</script>
