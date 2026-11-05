<template>
  <BaseDialog
    :show="show"
    :title="t('admin.rewardTasks.partners.secret.title')"
    width="wide"
    :close-on-click-outside="false"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <p class="rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
        {{ t('admin.rewardTasks.partners.secret.warning') }}
      </p>

      <div v-if="partner">
        <p class="input-label">{{ t('admin.rewardTasks.partners.secret.appKey') }}</p>
        <div class="flex items-center gap-2">
          <code class="min-w-0 flex-1 truncate rounded-lg bg-gray-50 px-3 py-2 font-mono text-sm text-gray-800 dark:bg-dark-900 dark:text-gray-200" data-testid="partner-secret-app-key">
            {{ partner.app_key }}
          </code>
          <button type="button" class="btn btn-secondary btn-sm" @click="copy(partner.app_key)">
            <Icon name="copy" size="sm" />
            {{ t('common.copy') }}
          </button>
        </div>
      </div>

      <div v-if="partner?.app_secret">
        <p class="input-label">{{ t('admin.rewardTasks.partners.secret.appSecret') }}</p>
        <div class="flex items-center gap-2">
          <code class="min-w-0 flex-1 truncate rounded-lg bg-gray-50 px-3 py-2 font-mono text-sm text-gray-800 dark:bg-dark-900 dark:text-gray-200" data-testid="partner-secret-value">
            {{ partner.app_secret }}
          </code>
          <button type="button" class="btn btn-secondary btn-sm" @click="copy(partner.app_secret ?? '')">
            <Icon name="copy" size="sm" />
            {{ t('common.copy') }}
          </button>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminRewardTaskPartner } from '@/api/admin/rewardTasks'
import { useClipboard } from '@/composables/useClipboard'

defineProps<{
  show: boolean
  partner: AdminRewardTaskPartner | null
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

function copy(text: string) {
  void copyToClipboard(text)
}
</script>
