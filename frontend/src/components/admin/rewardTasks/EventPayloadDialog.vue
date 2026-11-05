<template>
  <BaseDialog :show="show" :title="t('admin.rewardTasks.events.payloadTitle')" width="wide" @close="emit('close')">
    <!-- payload 为后端返回的任意 JSON，只用 <pre> 纯文本渲染，禁止 v-html -->
    <pre
      class="max-h-[28rem] overflow-auto rounded-xl bg-gray-50 p-4 font-mono text-xs leading-relaxed text-gray-800 dark:bg-dark-900 dark:text-gray-200"
      data-testid="event-payload"
    >{{ formatted }}</pre>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-primary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  payload: unknown
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()

const formatted = computed(() => {
  try {
    return JSON.stringify(props.payload ?? {}, null, 2)
  } catch {
    return String(props.payload ?? '')
  }
})
</script>
