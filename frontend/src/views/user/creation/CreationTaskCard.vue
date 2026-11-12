<template>
  <article
    class="card cs-task"
    :class="{ 'cs-enter': fresh }"
    :data-status="task.status"
    :aria-busy="processing"
    data-testid="creation-task"
  >
    <header class="cs-task-header">
      <span class="cs-task-kind">
        <Film v-if="task.kind === 'video'" aria-hidden="true" />
        <ImageIcon v-else aria-hidden="true" />
      </span>
      <h3 class="cs-task-title" :title="`${t('creation.result.model')}: ${task.model}`">{{ task.model }}</h3>
      <span class="cs-status" :class="statusClasses[task.status]">
        <component :is="statusIcons[task.status]" :class="{ 'animate-spin': processing }" aria-hidden="true" />
        <span>{{ t(`creation.result.status.${task.status}`) }}</span>
      </span>
      <button
        v-if="task.status === 'failed' || task.status === 'cancelled'"
        type="button"
        class="cs-icon-btn -mr-1.5"
        :aria-label="t('creation.result.dismiss')"
        :title="t('creation.result.dismiss')"
        @click="emit('dismiss', task.id)"
      >
        <X aria-hidden="true" />
      </button>
    </header>

    <div class="cs-task-meta cs-task-submeta">
      <span :title="t('creation.result.group')">{{ task.groupName }}</span>
      <span class="cs-dot-sep" aria-hidden="true" />
      <span>{{ task.protocol }}</span>
      <span class="cs-dot-sep" aria-hidden="true" />
      <time :datetime="new Date(task.createdAt).toISOString()">{{ formatTime(task.createdAt) }}</time>
    </div>

    <!-- 提示词默认两行截断，点击展开 -->
    <p class="cs-task-prompt" :data-expanded="expanded" :title="task.prompt" @click="expanded = !expanded">{{ task.prompt }}</p>

    <div v-if="hasBody" class="cs-task-body">
      <!-- 生成中：按请求的张数与比例占位，结果到位时不跳动 -->
      <template v-if="processing">
        <div
          v-if="task.kind === 'video'"
          class="cs-video cs-video--pending cs-media-skeleton"
          :data-orientation="orientation"
          :style="videoStyle"
          aria-hidden="true"
        >
          <Film />
        </div>
        <div v-else class="cs-media-grid" :data-count="gridCount(task.previewCount)" :style="gridStyle" aria-hidden="true">
          <div v-for="index in task.previewCount" :key="index" class="cs-media-tile cs-media-skeleton"><ImageIcon /></div>
        </div>
        <div>
          <div
            class="cs-progress"
            role="progressbar"
            :aria-label="processingText"
            :aria-valuenow="task.progress"
            :aria-valuemin="task.progress === undefined ? undefined : 0"
            :aria-valuemax="task.progress === undefined ? undefined : 100"
            :data-indeterminate="task.progress === undefined ? 'true' : undefined"
            :style="task.progress === undefined ? undefined : { '--cs-progress': `${task.progress}%` }"
          >
            <div class="cs-progress-bar" />
          </div>
          <div class="cs-progress-meta">
            <span>{{ processingText }}</span>
            <span>{{ t('creation.result.elapsed', { time: elapsed }) }}</span>
          </div>
        </div>
        <p v-if="task.sync" class="cs-field-warning">
          <TriangleAlert aria-hidden="true" />
          <span>{{ t('creation.result.syncNotice') }}</span>
        </p>
      </template>

      <div v-if="task.cancellationNoticed" role="status" class="cs-callout cs-callout--warning">
        <TriangleAlert class="cs-callout-icon" aria-hidden="true" />
        <div class="cs-callout-content">
          <p class="cs-callout-body !mt-0">{{ t('creation.result.cancelledNotice') }}</p>
          <div class="cs-callout-actions">
            <router-link to="/async-tasks" class="cs-callout-link">{{ t('creation.result.asyncLink') }}</router-link>
          </div>
        </div>
      </div>

      <div v-if="task.error" role="alert" class="cs-callout cs-callout--danger">
        <CircleX class="cs-callout-icon" aria-hidden="true" />
        <div class="cs-callout-content">
          <p class="cs-callout-title">{{ task.error.message }}</p>
          <p v-if="task.error.status === 402" class="cs-callout-body">{{ t('creation.result.balance402') }}</p>
          <p v-else-if="task.error.status === 403" class="cs-callout-body">{{ t('creation.result.balance403') }}</p>
          <dl v-if="task.error.code || task.error.status || task.error.requestId" class="cs-callout-meta">
            <div v-if="task.error.code"><dt class="inline">{{ t('creation.result.errorCode') }}: </dt><dd class="inline font-mono">{{ task.error.code }}</dd></div>
            <div v-if="task.error.status"><dt class="inline">{{ t('creation.result.httpStatus') }}: </dt><dd class="inline">{{ task.error.status }}</dd></div>
            <div v-if="task.error.requestId" class="break-all"><dt class="inline">{{ t('creation.result.requestId') }}: </dt><dd class="inline font-mono">{{ task.error.requestId }}</dd></div>
          </dl>
        </div>
      </div>

      <CreationMedia
        :images="images"
        :videos="videos"
        :aspect-w="task.aspectW"
        :aspect-h="task.aspectH"
        @preview="(items, index) => emit('preview', items, index)"
      />
    </div>

    <footer class="cs-task-footer">
      <span v-if="task.handle" class="cs-task-meta">
        <span class="shrink-0">{{ t('creation.result.taskId') }}</span>
        <span class="break-all font-mono">{{ task.handle.taskId }}</span>
      </span>
      <div class="cs-task-actions">
        <button v-if="processing" type="button" class="btn btn-ghost btn-sm" @click="emit('stop', task.id)">{{ t('creation.result.stopTracking') }}</button>
        <button type="button" class="btn btn-ghost btn-sm" @click="copyPrompt">
          <Copy aria-hidden="true" />{{ t('creation.result.copyPrompt') }}
        </button>
      </div>
    </footer>
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { Ban, CircleCheck, CircleX, Copy, Film, ImageIcon, LoaderCircle, TriangleAlert, X } from '@lucide/vue'
import { useClipboard } from '@/composables/useClipboard'
import type { CreationAsset, CreationKind, CreationTaskStatus } from '@/features/creation/types'
import CreationMedia from './CreationMedia.vue'
import { formatCreationElapsed, safeMediaUrl } from './viewModel'
import type { CreationCurrentTask, CreationLightboxItem } from './types'

const props = defineProps<{ task: CreationCurrentTask }>()
const emit = defineEmits<{
  stop: [id: number]
  dismiss: [id: number]
  preview: [items: CreationLightboxItem[], index: number]
}>()
const { t, locale } = useI18n()
const { copyToClipboard } = useClipboard()

// 完整类名写成字面量，Tailwind 才能在扫描源码时保留对应规则。
const statusClasses: Record<CreationTaskStatus, string> = {
  processing: 'cs-status--processing',
  succeeded: 'cs-status--succeeded',
  failed: 'cs-status--failed',
  cancelled: 'cs-status--cancelled',
}
const statusIcons: Record<CreationTaskStatus, Component> = {
  processing: LoaderCircle,
  succeeded: CircleCheck,
  failed: CircleX,
  cancelled: Ban,
}

const expanded = ref(false)
/** 只有刚提交的任务播放入场动画；切换生图 / 生视频重新渲染列表时不重播。 */
const fresh = Date.now() - props.task.createdAt < 1500
const processing = computed(() => props.task.status === 'processing')

const images = computed(() => mediaOf('image'))
const videos = computed(() => mediaOf('video'))
const hasBody = computed(() => processing.value
  || props.task.cancellationNoticed
  || Boolean(props.task.error)
  || images.value.length > 0
  || videos.value.length > 0)

/** 生成中的占位按请求的比例画；结果到位后由 CreationMedia 改用真实宽高比。 */
const gridStyle = computed(() => ({ '--cs-aspect-w': String(props.task.aspectW), '--cs-aspect-h': String(props.task.aspectH) }))
const videoStyle = computed(() => ({ '--cs-aspect': `${props.task.aspectW} / ${props.task.aspectH}` }))
const orientation = computed(() => props.task.aspectW < props.task.aspectH ? 'portrait' : undefined)

const processingText = computed(() => {
  if (props.task.sync) return t('creation.result.syncRunning')
  if (props.task.submitting) return t('creation.result.submitting')
  return props.task.progress === undefined
    ? t('creation.result.indeterminate')
    : t('creation.result.progress', { value: props.task.progress })
})

const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined
const elapsed = computed(() => formatCreationElapsed(now.value - props.task.createdAt))

function stopTimer(): void {
  if (timer === undefined) return
  clearInterval(timer)
  timer = undefined
}

watch(processing, active => {
  if (!active) {
    stopTimer()
  } else if (timer === undefined) {
    now.value = Date.now()
    timer = setInterval(() => { now.value = Date.now() }, 1000)
  }
}, { immediate: true })
onBeforeUnmount(stopTimer)

function mediaOf(kind: CreationKind): CreationLightboxItem[] {
  return props.task.assets.flatMap((asset, index) => {
    if (asset.kind !== kind) return []
    const url = safeMediaUrl(asset.url)
    if (!url) return []
    return [{
      url,
      alt: kind === 'image' ? t('creation.result.imageAlt', { index: index + 1 }) : t('creation.result.video'),
      downloadName: downloadName(asset, index),
    }]
  })
}

/** 1–4 用固定排布；更多走 auto-fill 兜底（不设 data-count）。 */
function gridCount(count: number): number | undefined {
  return count >= 1 && count <= 4 ? count : undefined
}

function formatTime(value: number): string {
  return new Date(value).toLocaleString(locale.value)
}

function downloadName(asset: CreationAsset, index: number): string {
  const model = (props.task.model || 'creation').replace(/[^a-zA-Z0-9-_]/g, '-')
  const extension = asset.kind === 'video' ? 'mp4' : asset.mimeType?.split('/')[1]?.replace('jpeg', 'jpg') || 'png'
  return `${model}-${props.task.id}-${index + 1}.${extension}`
}

function copyPrompt(): void {
  void copyToClipboard(props.task.prompt, t('creation.result.copied'))
}
</script>
