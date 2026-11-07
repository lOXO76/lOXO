<template>
  <div class="overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
    <table class="w-full min-w-[560px] border-collapse text-left">
      <thead class="bg-gray-50 dark:bg-dark-800">
        <tr class="border-b border-gray-200 dark:border-dark-700">
          <th
            v-for="header in headers"
            :key="header"
            class="whitespace-nowrap px-3 py-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-dark-300"
          >{{ header }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
        <slot />
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
defineProps<{ headers: string[] }>()
</script>

<style scoped>
:deep(tbody td) {
  @apply px-3 py-2 align-top text-sm leading-6 text-gray-600 dark:text-dark-200;
}

:deep(tbody td:first-child) {
  @apply whitespace-nowrap font-mono text-xs text-gray-900 dark:text-white;
}
</style>
