<template>
  <div v-if="images.length || videos.length" class="space-y-3">
    <!-- 图片：点开灯箱看全图，悬停显示下载 / 新窗口打开 -->
    <div v-if="images.length" class="cs-media-grid" :data-count="gridCount" :style="gridStyle">
      <div v-for="(image, index) in images" :key="`${image.url}-${index}`" class="cs-media-tile">
        <!-- 链接过期或加载失败：用占位代替浏览器的裂图图标 -->
        <div v-if="broken.has(image.url)" class="cs-media-placeholder">
          <ImageOff aria-hidden="true" />
          <span>{{ t('creation.history.imageUnavailable') }}</span>
        </div>
        <template v-else>
          <img :src="image.url" :alt="image.alt" loading="lazy" @load="adoptImageAspect" @error="broken.add(image.url)" />
          <button
            type="button"
            class="cs-media-open"
            :aria-label="t('creation.result.preview', { index: index + 1 })"
            @click="emit('preview', images, index)"
          />
        </template>
        <span v-if="images.length > 1" class="cs-media-index">{{ index + 1 }}</span>
        <div v-if="!broken.has(image.url)" class="cs-media-overlay">
          <a
            class="cs-media-action"
            :href="image.url"
            :download="image.downloadName"
            :aria-label="t('creation.result.download')"
            :title="t('creation.result.download')"
          >
            <Download aria-hidden="true" />
          </a>
          <a
            class="cs-media-action"
            :href="image.url"
            target="_blank"
            rel="noopener noreferrer"
            :aria-label="t('creation.result.open')"
            :title="t('creation.result.open')"
          >
            <ExternalLink aria-hidden="true" />
          </a>
        </div>
      </div>
    </div>

    <figure v-for="(video, index) in videos" :key="`${video.url}-${index}`" class="m-0">
      <div class="cs-video" :data-orientation="orientation" :style="videoStyle">
        <video
          :src="video.url"
          controls
          preload="metadata"
          playsinline
          :aria-label="video.alt"
          @loadedmetadata="adoptVideoAspect"
        />
      </div>
      <figcaption class="cs-video-actions">
        <a :href="video.url" :download="video.downloadName" class="btn btn-secondary btn-sm">
          <Download aria-hidden="true" />{{ t('creation.result.download') }}
        </a>
        <a :href="video.url" target="_blank" rel="noopener noreferrer" class="btn btn-ghost btn-sm">
          <ExternalLink aria-hidden="true" />{{ t('creation.result.open') }}
        </a>
      </figcaption>
    </figure>
  </div>
</template>

<script setup lang="ts">
/** 生成结果：图片网格与视频播放器，任务卡片和历史卡片共用。 */
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Download, ExternalLink, ImageOff } from '@lucide/vue'
import type { CreationLightboxItem } from './types'

const props = withDefaults(defineProps<{
  /** 地址都已通过 safeMediaUrl 校验。 */
  images: CreationLightboxItem[]
  videos: CreationLightboxItem[]
  /** 结果加载前的占位比例（取自请求参数；未知时图片按 1:1）。 */
  aspectW?: number
  aspectH?: number
}>(), { aspectW: 1, aspectH: 1 })
const emit = defineEmits<{ preview: [items: CreationLightboxItem[], index: number] }>()
const { t } = useI18n()

/** 加载失败的图片地址。 */
const broken = reactive(new Set<string>())
/** 结果加载后改用真实宽高比：实际尺寸可能与请求不同（如 size=auto、ratio=adaptive）。 */
const naturalAspect = ref<{ w: number; h: number } | null>(null)
const aspect = computed(() => naturalAspect.value ?? { w: props.aspectW, h: props.aspectH })
const gridStyle = computed(() => ({ '--cs-aspect-w': String(aspect.value.w), '--cs-aspect-h': String(aspect.value.h) }))
const videoStyle = computed(() => ({ '--cs-aspect': `${aspect.value.w} / ${aspect.value.h}` }))
const orientation = computed(() => aspect.value.w < aspect.value.h ? 'portrait' : undefined)
/** 1–4 用固定排布；更多走 auto-fill 兜底（不设 data-count）。 */
const gridCount = computed(() => props.images.length <= 4 ? props.images.length : undefined)

function adoptAspect(width: number, height: number): void {
  if (!naturalAspect.value && width > 0 && height > 0) naturalAspect.value = { w: width, h: height }
}

function adoptImageAspect(event: Event): void {
  const image = event.target as HTMLImageElement
  adoptAspect(image.naturalWidth, image.naturalHeight)
}

function adoptVideoAspect(event: Event): void {
  const video = event.target as HTMLVideoElement
  adoptAspect(video.videoWidth, video.videoHeight)
}
</script>
