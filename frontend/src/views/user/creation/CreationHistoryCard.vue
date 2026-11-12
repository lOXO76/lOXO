<template>
  <article class="card cs-task" :data-status="item.status" :aria-busy="processing" data-testid="creation-history-item">
    <header class="cs-task-header">
      <span class="cs-task-kind">
        <Film v-if="item.kind === 'video'" aria-hidden="true" />
        <ImageIcon v-else aria-hidden="true" />
      </span>
      <h3 class="cs-task-title" :title="item.model">{{ item.model || item.taskId || '—' }}</h3>
      <span class="cs-status" :class="statusClasses[item.status]">
        <component :is="statusIcons[item.status]" :class="{ 'animate-spin': processing }" aria-hidden="true" />
        <span>{{ t(`creation.history.status.${item.status}`) }}</span>
      </span>
    </header>

    <div class="cs-task-meta cs-task-submeta">
      <span v-if="item.source === 'local'" class="cs-param-pill">{{ t('creation.history.local') }}</span>
      <span>{{ item.protocol }}</span>
      <span class="cs-dot-sep" aria-hidden="true" />
      <time :datetime="new Date(item.createdAt).toISOString()">{{ formatTime(item.createdAt) }}</time>
    </div>

    <!-- 提示词默认两行截断，点击展开 -->
    <p v-if="item.prompt" class="cs-task-prompt" :data-expanded="expanded" :title="item.prompt" @click="expanded = !expanded">{{ item.prompt }}</p>

    <div v-if="processing || item.error || hasMedia || missingText" class="cs-task-body">
      <template v-if="processing">
        <div v-if="item.kind === 'video'" class="cs-video cs-video--pending cs-media-skeleton" aria-hidden="true"><Film /></div>
        <div v-else class="cs-media-grid" data-count="1" aria-hidden="true">
          <div class="cs-media-tile cs-media-skeleton"><ImageIcon /></div>
        </div>
        <div>
          <div class="cs-progress" role="progressbar" :aria-label="t('creation.history.processingHint')" data-indeterminate="true">
            <div class="cs-progress-bar" />
          </div>
          <div class="cs-progress-meta"><span>{{ t('creation.history.processingHint') }}</span></div>
        </div>
      </template>

      <p v-if="item.error" class="cs-task-error">{{ item.error }}</p>

      <CreationMedia :images="images" :videos="videos" :aspect-w="item.kind === 'video' ? 16 : 1" :aspect-h="item.kind === 'video' ? 9 : 1" @preview="(items, index) => emit('preview', items, index)" />
      <CreationRemoteVideo v-if="item.downloadId" :task-id="item.downloadId" :download-name="`${fileStem}.mp4`" />

      <p v-if="missingText" class="cs-section-hint">{{ missingText }}</p>
    </div>

    <footer v-if="item.taskId || item.prompt" class="cs-task-footer">
      <span v-if="item.taskId" class="cs-task-meta">
        <span class="shrink-0">{{ t('creation.result.taskId') }}</span>
        <span class="break-all font-mono">{{ item.taskId }}</span>
      </span>
      <div v-if="item.prompt" class="cs-task-actions">
        <button type="button" class="btn btn-ghost btn-sm" @click="copyPrompt">
          <Copy aria-hidden="true" />{{ t('creation.result.copyPrompt') }}
        </button>
      </div>
    </footer>
  </article>
</template>

<script setup lang="ts">
/** 历史记录里已归档的一条：图片直接显示、视频直接播放；样式与任务卡片一致。 */
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { Ban, CircleCheck, CircleX, Copy, Film, ImageIcon, LoaderCircle } from '@lucide/vue'
import { useClipboard } from '@/composables/useClipboard'
import CreationMedia from './CreationMedia.vue'
import CreationRemoteVideo from './CreationRemoteVideo.vue'
import type { CreationHistoryItem, CreationLightboxItem } from './types'

const props = defineProps<{ item: CreationHistoryItem }>()
const emit = defineEmits<{ preview: [items: CreationLightboxItem[], index: number] }>()
const { t, locale } = useI18n()
const { copyToClipboard } = useClipboard()

// 完整类名写成字面量，Tailwind 才能在扫描源码时保留对应规则。
const statusClasses: Record<CreationHistoryItem['status'], string> = {
  processing: 'cs-status--processing',
  completed: 'cs-status--completed',
  failed: 'cs-status--failed',
  cancelled: 'cs-status--cancelled',
}
const statusIcons: Record<CreationHistoryItem['status'], Component> = {
  processing: LoaderCircle,
  completed: CircleCheck,
  failed: CircleX,
  cancelled: Ban,
}

const expanded = ref(false)
const processing = computed(() => props.item.status === 'processing')

const fileStem = computed(() => {
  const clean = (value: string) => value.replace(/[^a-zA-Z0-9-_]/g, '-')
  return `${clean(props.item.model || props.item.kind)}-${clean(props.item.taskId || String(props.item.createdAt))}`
})
const images = computed<CreationLightboxItem[]>(() => props.item.images.map((url, index) => ({
  url,
  alt: t('creation.result.imageAlt', { index: index + 1 }),
  downloadName: `${fileStem.value}-${index + 1}.png`,
})))
const videos = computed<CreationLightboxItem[]>(() => props.item.videoUrl
  ? [{ url: props.item.videoUrl, alt: t('creation.result.video'), downloadName: `${fileStem.value}.mp4` }]
  : [])
const hasMedia = computed(() => images.value.length > 0 || videos.value.length > 0 || Boolean(props.item.downloadId))
/** 已完成却没有可展示的结果时的说明。 */
const missingText = computed(() => {
  if (props.item.status !== 'completed' || hasMedia.value) return ''
  return props.item.source === 'local' ? t('creation.history.noAsset') : t('creation.history.noPreview')
})

function formatTime(value: number): string {
  return new Date(value).toLocaleString(locale.value)
}

function copyPrompt(): void {
  void copyToClipboard(props.item.prompt, t('creation.result.copied'))
}
</script>
