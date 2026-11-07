<template>
  <span
    class="inline-flex items-center whitespace-nowrap rounded-md px-2 py-0.5 text-xs font-medium"
    :class="classes"
    :data-testid="`ticket-priority-${priority}`"
  >
    {{ t(`tickets.priority.${priority}`) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TicketPriority } from '@/types/tickets'

const props = defineProps<{ priority: TicketPriority }>()
const { t } = useI18n()

const classes = computed(() => {
  switch (props.priority) {
    case 'urgent':
      return 'bg-red-50 text-red-700 ring-1 ring-inset ring-red-200 dark:bg-red-900/20 dark:text-red-300 dark:ring-red-900/50'
    case 'high':
      return 'bg-orange-50 text-orange-700 ring-1 ring-inset ring-orange-200 dark:bg-orange-900/20 dark:text-orange-300 dark:ring-orange-900/50'
    case 'low':
      return 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'
    default:
      return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
  }
})
</script>
