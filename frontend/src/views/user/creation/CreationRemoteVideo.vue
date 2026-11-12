<template>
  <figure ref="root" class="m-0">
    <div class="cs-video" :class="{ 'cs-video--pending': !src }" :data-orientation="orientation" :style="videoStyle">
      <video
        v-if="src"
        :src="src"
        controls
        preload="metadata"
        playsinline
        :aria-label="t('creation.result.video')"
        @loadedmetadata="adoptAspect"
      />
      <div v-else class="cs-media-placeholder" :role="failed ? 'alert' : 'status'">
        <TriangleAlert v-if="failed" aria-hidden="true" />
        <LoaderCircle v-else class="animate-spin" aria-hidden="true" />
        <span>{{ failed ? t('creation.history.videoLoadFailed') : t('creation.history.videoLoading') }}</span>
        <button v-if="failed" type="button" class="cs-callout-link" @click="load">{{ t('creation.history.retry') }}</button>
      </div>
    </div>
    <figcaption v-if="src" class="cs-video-actions">
      <a :href="src" :download="downloadName" class="btn btn-secondary btn-sm">
        <Download aria-hidden="true" />{{ t('creation.result.download') }}
      </a>
    </figcaption>
  </figure>
</template>

<script setup lang="ts">
/**
 * 需要登录态才能取回的视频：经站内下载接口拿到字节后转成 blob 地址播放，不把上游链接写进 <video>。
 * 滚动到附近才开始下载，避免历史里的视频一次全部拉取。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Download, LoaderCircle, TriangleAlert } from '@lucide/vue'
import { asyncTasksAPI } from '@/api/asyncTasks'

const props = defineProps<{
  /** 异步任务的目录 ID。 */
  taskId: string
  downloadName: string
}>()
const { t } = useI18n()

const root = ref<HTMLElement | null>(null)
const src = ref('')
const failed = ref(false)
const aspect = ref({ w: 16, h: 9 })
const videoStyle = computed(() => ({ '--cs-aspect': `${aspect.value.w} / ${aspect.value.h}` }))
const orientation = computed(() => aspect.value.w < aspect.value.h ? 'portrait' : undefined)

let observer: IntersectionObserver | undefined
let disposed = false
let request = 0

function release(): void {
  if (!src.value) return
  URL.revokeObjectURL(src.value)
  src.value = ''
}

async function load(): Promise<void> {
  const current = ++request
  failed.value = false
  try {
    const blob = await asyncTasksAPI.download(props.taskId)
    const video = blob.type.startsWith('video/') ? blob : new Blob([blob], { type: 'video/mp4' })
    const url = URL.createObjectURL(video)
    // 组件已卸载或又发起了新的加载：丢弃这次结果并释放地址。
    if (disposed || current !== request) {
      URL.revokeObjectURL(url)
      return
    }
    release()
    src.value = url
  } catch {
    if (!disposed && current === request) failed.value = true
  }
}

function adoptAspect(event: Event): void {
  const video = event.target as HTMLVideoElement
  if (video.videoWidth > 0 && video.videoHeight > 0) aspect.value = { w: video.videoWidth, h: video.videoHeight }
}

onMounted(() => {
  if (typeof IntersectionObserver === 'undefined' || !root.value) {
    void load()
    return
  }
  observer = new IntersectionObserver(entries => {
    if (!entries.some(entry => entry.isIntersecting)) return
    observer?.disconnect()
    observer = undefined
    void load()
  }, { rootMargin: '200px' })
  observer.observe(root.value)
})

onBeforeUnmount(() => {
  disposed = true
  observer?.disconnect()
  release()
})
</script>
