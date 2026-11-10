<template>
  <section class="card p-5" :aria-busy="activeCount > 0">
    <div class="flex items-center justify-between gap-3">
      <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('creation.result.title') }}</h2>
      <span v-if="activeCount" class="rounded-md bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 dark:bg-blue-950 dark:text-blue-300" data-testid="creation-active-count">
        {{ t('creation.result.activeCount', { count: activeCount }) }}
      </span>
    </div>

    <div v-if="!tasks.length" class="flex min-h-72 flex-col items-center justify-center py-10 text-center">
      <div class="flex h-14 w-14 items-center justify-center rounded-xl bg-primary-50 text-primary-600 dark:bg-primary-950 dark:text-primary-300">
        <Icon name="sparkles" size="lg" />
      </div>
      <h3 class="mt-4 text-sm font-medium text-gray-900 dark:text-gray-100">{{ t('creation.result.emptyTitle') }}</h3>
      <p class="mt-2 max-w-sm text-sm leading-6 text-gray-500 dark:text-gray-400">{{ t('creation.result.emptyDescription') }}</p>
    </div>

    <template v-else>
      <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('creation.result.backgroundHint') }}</p>
      <ul class="mt-4 space-y-4" role="list">
        <li
          v-for="task in tasks"
          :key="task.id"
          class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-dark-700"
          :aria-busy="task.status === 'processing'"
          data-testid="creation-task"
        >
          <div class="flex items-start justify-between gap-3">
            <dl class="grid min-w-0 flex-1 gap-3 text-xs text-gray-500 dark:text-gray-400 sm:grid-cols-3">
              <div class="min-w-0"><dt>{{ t('creation.result.group') }}</dt><dd class="mt-1 truncate text-sm text-gray-800 dark:text-gray-200" :title="task.groupName">{{ task.groupName }}</dd></div>
              <div class="min-w-0"><dt>{{ t('creation.result.model') }}</dt><dd class="mt-1 truncate font-mono text-sm text-gray-800 dark:text-gray-200" :title="task.model">{{ task.model }}</dd></div>
              <div><dt>{{ task.protocol }}</dt><dd class="mt-1 text-sm text-gray-800 dark:text-gray-200">{{ formatTime(task.createdAt) }}</dd></div>
            </dl>
            <div class="flex shrink-0 items-center gap-1">
              <span class="rounded-md px-2 py-1 text-xs font-medium" :class="statusClass(task)">{{ t(`creation.result.status.${task.status}`) }}</span>
              <button
                v-if="task.status !== 'processing'"
                type="button"
                class="btn btn-ghost btn-sm px-2"
                :aria-label="t('creation.result.dismiss')"
                :title="t('creation.result.dismiss')"
                @click="emit('dismiss', task.id)"
              ><Icon name="x" size="sm" /></button>
            </div>
          </div>

          <div v-if="task.status === 'processing'" class="rounded-lg border border-primary-200 bg-primary-50 p-4 dark:border-primary-900 dark:bg-primary-950/40">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-sm text-primary-800 dark:text-primary-200">{{ processingText(task) }}</p>
              <button type="button" class="btn btn-secondary btn-sm" @click="emit('stop', task.id)">{{ t('creation.result.stopTracking') }}</button>
            </div>
            <div
              class="mt-3 h-1.5 overflow-hidden rounded-full bg-primary-100 dark:bg-primary-900"
              role="progressbar"
              :aria-valuenow="task.progress"
              :aria-valuemin="task.progress === undefined ? undefined : 0"
              :aria-valuemax="task.progress === undefined ? undefined : 100"
            >
              <div class="h-full rounded-full bg-primary-500 transition-all" :class="task.progress === undefined ? 'w-1/3 animate-pulse' : ''" :style="task.progress === undefined ? undefined : { width: `${task.progress}%` }" />
            </div>
            <p v-if="task.sync" class="mt-3 text-xs leading-5 text-amber-700 dark:text-amber-300">{{ t('creation.result.syncNotice') }}</p>
          </div>

          <div v-if="task.cancellationNoticed" role="status" class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm leading-6 text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-200">
            {{ t('creation.result.cancelledNotice') }}
            <router-link to="/async-tasks" class="ml-1 font-medium underline">{{ t('creation.result.asyncLink') }}</router-link>
          </div>

          <div v-if="task.error" role="alert" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950/40">
            <p class="font-medium text-red-800 dark:text-red-200">{{ task.error.message }}</p>
            <p v-if="task.error.status === 402" class="mt-2 leading-6 text-red-700 dark:text-red-300">{{ t('creation.result.balance402') }}</p>
            <p v-else-if="task.error.status === 403" class="mt-2 leading-6 text-red-700 dark:text-red-300">{{ t('creation.result.balance403') }}</p>
            <dl class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-red-600 dark:text-red-300">
              <div v-if="task.error.code"><dt class="inline">{{ t('creation.result.errorCode') }}: </dt><dd class="inline font-mono">{{ task.error.code }}</dd></div>
              <div v-if="task.error.status"><dt class="inline">{{ t('creation.result.httpStatus') }}: </dt><dd class="inline">{{ task.error.status }}</dd></div>
              <div v-if="task.error.requestId" class="break-all"><dt class="inline">{{ t('creation.result.requestId') }}: </dt><dd class="inline font-mono">{{ task.error.requestId }}</dd></div>
            </dl>
          </div>

          <div v-if="task.assets.length" class="grid gap-4 sm:grid-cols-2">
            <figure v-for="(asset, index) in task.assets" :key="`${asset.url}-${index}`" class="min-w-0 overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-900">
              <a v-if="asset.kind === 'image' && mediaUrl(asset.url)" :href="mediaUrl(asset.url)" target="_blank" rel="noopener noreferrer" class="block bg-checkered" :aria-label="t('creation.result.open')">
                <img :src="mediaUrl(asset.url)" :alt="t('creation.result.imageAlt', { index: index + 1 })" class="max-h-[32rem] w-full object-contain" />
              </a>
              <video v-else-if="mediaUrl(asset.url)" :src="mediaUrl(asset.url)" controls preload="metadata" class="max-h-[32rem] w-full bg-black" :aria-label="t('creation.result.video')" />
              <figcaption class="flex flex-wrap items-center justify-end gap-2 px-3 py-2">
                <a v-if="mediaUrl(asset.url)" :href="mediaUrl(asset.url)" target="_blank" rel="noopener noreferrer" class="btn btn-ghost btn-sm"><Icon name="externalLink" size="sm" class="mr-1" />{{ t('creation.result.open') }}</a>
                <a v-if="mediaUrl(asset.url)" :href="mediaUrl(asset.url)" :download="downloadName(task, asset, index)" class="btn btn-secondary btn-sm"><Icon name="download" size="sm" class="mr-1" />{{ t('creation.result.download') }}</a>
              </figcaption>
            </figure>
          </div>

          <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900/70">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('creation.composer.prompt') }}</span>
              <button type="button" class="btn btn-ghost btn-sm" @click="copyPrompt(task)"><Icon name="copy" size="sm" class="mr-1" />{{ t('creation.result.copyPrompt') }}</button>
            </div>
            <p class="mt-2 max-h-32 overflow-y-auto whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-300">{{ task.prompt }}</p>
          </div>
          <div v-if="task.handle" class="break-all text-xs text-gray-500 dark:text-gray-400">{{ t('creation.result.taskId') }}: <span class="font-mono">{{ task.handle.taskId }}</span></div>
        </li>
      </ul>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { CreationAsset } from '@/features/creation/types'
import { safeMediaUrl } from './viewModel'
import type { CreationCurrentTask } from './types'

const props = defineProps<{ tasks: CreationCurrentTask[] }>()
const emit = defineEmits<{ stop: [id: number]; dismiss: [id: number] }>()
const { t, locale } = useI18n()
const { copyToClipboard } = useClipboard()

const activeCount = computed(() => props.tasks.filter(task => task.status === 'processing').length)

function statusClass(task: CreationCurrentTask): string {
  return ({
    processing: 'bg-blue-50 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
    succeeded: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300',
    failed: 'bg-red-50 text-red-700 dark:bg-red-950 dark:text-red-300',
    cancelled: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
  })[task.status]
}

function processingText(task: CreationCurrentTask): string {
  if (task.sync) return t('creation.result.syncRunning')
  if (task.submitting) return t('creation.result.submitting')
  return task.progress === undefined ? t('creation.result.indeterminate') : t('creation.result.progress', { value: task.progress })
}

function mediaUrl(value: string): string {
  return safeMediaUrl(value)
}

function formatTime(value: number): string {
  return new Date(value).toLocaleString(locale.value)
}

function downloadName(task: CreationCurrentTask, asset: CreationAsset, index: number): string {
  const model = (task.model || 'creation').replace(/[^a-zA-Z0-9-_]/g, '-')
  const extension = asset.kind === 'video' ? 'mp4' : asset.mimeType?.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
  return `${model}-${task.id}-${index + 1}.${extension}`
}

function copyPrompt(task: CreationCurrentTask): void {
  void copyToClipboard(task.prompt, t('creation.result.copied'))
}
</script>
