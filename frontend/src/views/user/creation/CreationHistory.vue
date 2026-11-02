<template>
  <section class="card p-5" :aria-busy="loading">
    <div class="flex items-center justify-between gap-3">
      <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('creation.history.title') }}</h2>
      <div class="flex items-center gap-1">
        <button type="button" class="btn btn-ghost btn-sm" :disabled="loading" @click="emit('refresh')"><Icon name="refresh" size="sm" class="mr-1" :class="loading ? 'animate-spin' : ''" />{{ t('creation.history.refresh') }}</button>
        <router-link to="/async-tasks" class="btn btn-ghost btn-sm">{{ t('creation.history.viewAll') }}<Icon name="arrowRight" size="sm" class="ml-1" /></router-link>
      </div>
    </div>

    <div v-if="error" role="alert" class="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
      {{ error }}
      <button type="button" class="ml-2 font-medium underline" @click="emit('refresh')">{{ t('creation.history.refresh') }}</button>
    </div>

    <div v-if="loading && !tasks.length && !records.length" class="mt-5 space-y-3" role="status">
      <span class="sr-only">{{ t('creation.loading') }}</span>
      <div v-for="index in 3" :key="index" class="h-20 animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800" />
    </div>
    <div v-else-if="!tasks.length && !records.length" class="py-12 text-center" role="status">
      <Icon name="clock" size="lg" class="mx-auto text-gray-400 dark:text-dark-500" />
      <h3 class="mt-3 text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('creation.history.empty') }}</h3>
      <p class="mx-auto mt-2 max-w-md text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('creation.history.emptyHint') }}</p>
    </div>

    <div v-else class="mt-5 space-y-6">
      <section v-if="records.length" aria-labelledby="creation-session-history">
        <h3 id="creation-session-history" class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('creation.history.session') }}</h3>
        <ul class="mt-3 space-y-3" role="list">
          <li v-for="record in records" :key="record.id" class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="flex gap-3">
              <div class="h-16 w-16 shrink-0 overflow-hidden rounded-md bg-gray-100 dark:bg-dark-800">
                <img v-if="previewRecordUrl(record)" :src="previewRecordUrl(record)" :alt="t('creation.history.imageAlt')" class="h-full w-full object-cover" loading="lazy" />
                <div v-else class="flex h-full w-full items-center justify-center text-gray-400"><Icon :name="record.kind === 'video' ? 'play' : 'sparkles'" size="md" /></div>
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-600 dark:bg-dark-800 dark:text-gray-300">{{ t('creation.history.local') }}</span>
                  <span class="truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="record.model">{{ record.model }}</span>
                </div>
                <p class="mt-1 truncate text-sm text-gray-600 dark:text-gray-400" :title="record.prompt">{{ record.prompt }}</p>
                <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
                  <time class="text-xs text-gray-500 dark:text-gray-400" :datetime="new Date(record.createdAt).toISOString()">{{ formatTime(record.createdAt) }}</time>
                  <div class="flex gap-1">
                    <a v-for="(asset, index) in record.assets.slice(0, 2)" :key="`${record.id}-${index}`" :href="safeMediaUrl(asset.url)" target="_blank" rel="noopener noreferrer" class="btn btn-ghost btn-sm">{{ t('creation.history.openResult') }}<span v-if="record.assets.length > 1">&nbsp;{{ index + 1 }}</span></a>
                    <span v-if="!record.assets.length" class="text-xs text-gray-400">{{ t('creation.history.noAsset') }}</span>
                  </div>
                </div>
              </div>
            </div>
          </li>
        </ul>
      </section>

      <section v-if="tasks.length" aria-labelledby="creation-server-history">
        <h3 id="creation-server-history" class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('creation.history.server') }}</h3>
        <ul class="mt-3 divide-y divide-gray-100 dark:divide-dark-700" role="list">
          <li v-for="task in tasks" :key="task.id" class="flex gap-3 py-3 first:pt-0 last:pb-0">
            <div class="h-14 w-14 shrink-0 overflow-hidden rounded-md bg-gray-100 dark:bg-dark-800">
              <img v-if="task.kind === 'image' && directUrl(task)" :src="directUrl(task)" :alt="t('creation.history.imageAlt')" class="h-full w-full object-cover" loading="lazy" />
              <div v-else class="flex h-full w-full items-center justify-center text-gray-400"><Icon :name="task.kind === 'video' ? 'play' : 'sparkles'" size="md" /></div>
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="statusClass(task.status)">{{ t(`creation.history.status.${task.status}`) }}</span>
                <span class="truncate font-mono text-xs text-gray-700 dark:text-gray-300" :title="task.model">{{ task.model || '—' }}</span>
              </div>
              <p class="mt-1 break-all text-xs text-gray-500 dark:text-gray-400" :title="task.task_id">{{ task.task_id }}</p>
              <p v-if="task.error" class="mt-1 break-words text-xs text-red-600 dark:text-red-400">{{ task.error }}</p>
              <div class="mt-2 flex flex-wrap items-center justify-between gap-2">
                <time class="text-xs text-gray-500 dark:text-gray-400" :datetime="new Date(task.created_at * 1000).toISOString()">{{ formatTime(task.created_at * 1000) }}</time>
                <div class="flex gap-1">
                  <a v-if="directUrl(task)" :href="directUrl(task)" target="_blank" rel="noopener noreferrer" class="btn btn-ghost btn-sm">{{ t('creation.history.openResult') }}</a>
                  <button v-if="task.status === 'completed' && task.result_url" type="button" class="btn btn-ghost btn-sm" :disabled="downloadingId === task.id" @click="emit('download', task)"><Icon name="download" size="sm" class="mr-1" :class="downloadingId === task.id ? 'animate-pulse' : ''" />{{ t('creation.history.download') }}</button>
                </div>
              </div>
            </div>
          </li>
        </ul>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { AsyncTask } from '@/api/asyncTasks'
import { safeMediaUrl, type CreationSessionRecord } from './viewModel'

defineProps<{
  tasks: AsyncTask[]
  records: CreationSessionRecord[]
  loading: boolean
  error: string
  downloadingId: string
}>()
const emit = defineEmits<{ refresh: []; download: [task: AsyncTask] }>()
const { t, locale } = useI18n()

function directUrl(task: AsyncTask): string {
  return task.status === 'completed' ? safeMediaUrl(task.result_url) : ''
}

function previewRecordUrl(record: CreationSessionRecord): string {
  if (record.kind !== 'image') return ''
  const asset = record.assets.find(item => item.kind === 'image')
  return asset ? safeMediaUrl(asset.url) : ''
}

function formatTime(value: number): string {
  return new Date(value).toLocaleString(locale.value)
}

function statusClass(status: AsyncTask['status']): string {
  return ({
    processing: 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
    completed: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300',
    failed: 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300',
    cancelled: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
  })[status]
}
</script>
