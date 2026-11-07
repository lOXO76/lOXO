<template>
  <span
    class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset"
    :class="classes"
    :data-testid="`ticket-status-${status}`"
  >
    <span class="h-1.5 w-1.5 rounded-full" :class="dotClass" aria-hidden="true"></span>
    {{ t(`tickets.status.${status}`) }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TicketStatus } from '@/types/tickets'

const props = defineProps<{ status: TicketStatus }>()
const { t } = useI18n()

const classes = computed(() => {
  switch (props.status) {
    case 'pending_admin':
      return 'bg-amber-50 text-amber-700 ring-amber-200 dark:bg-amber-900/20 dark:text-amber-300 dark:ring-amber-900/50'
    case 'pending_user':
      return 'bg-primary-50 text-primary-700 ring-primary-200 dark:bg-primary-900/20 dark:text-primary-300 dark:ring-primary-900/50'
    case 'resolved':
      return 'bg-emerald-50 text-emerald-700 ring-emerald-200 dark:bg-emerald-900/20 dark:text-emerald-300 dark:ring-emerald-900/50'
    default:
      return 'bg-gray-100 text-gray-600 ring-gray-200 dark:bg-dark-800 dark:text-dark-300 dark:ring-dark-700'
  }
})

const dotClass = computed(() => {
  switch (props.status) {
    case 'pending_admin':
      return 'bg-amber-500'
    case 'pending_user':
      return 'bg-primary-500'
    case 'resolved':
      return 'bg-emerald-500'
    default:
      return 'bg-gray-400 dark:bg-dark-500'
  }
})
</script>
