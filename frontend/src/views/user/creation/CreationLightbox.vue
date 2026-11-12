<template>
  <DialogRoot :open="open" @update:open="emit('update:open', $event)">
    <DialogPortal>
      <DialogOverlay class="cs-lightbox-overlay" />
      <DialogContent class="cs-lightbox" :aria-describedby="undefined" @keydown="onKeydown">
        <DialogTitle class="sr-only">{{ t('creation.lightbox.title') }}</DialogTitle>
        <div class="cs-lightbox-toolbar">
          <span class="cs-lightbox-counter">
            {{ t('creation.lightbox.counter', { index: safeIndex + 1, total: items.length }) }}
          </span>
          <div class="flex items-center gap-1">
            <a
              v-if="current"
              class="cs-lightbox-btn"
              :href="current.url"
              :download="current.downloadName"
              :aria-label="t('creation.result.download')"
              :title="t('creation.result.download')"
            >
              <Download aria-hidden="true" />
            </a>
            <a
              v-if="current"
              class="cs-lightbox-btn"
              :href="current.url"
              target="_blank"
              rel="noopener noreferrer"
              :aria-label="t('creation.result.open')"
              :title="t('creation.result.open')"
            >
              <ExternalLink aria-hidden="true" />
            </a>
            <button
              type="button"
              class="cs-lightbox-btn"
              :aria-label="t('creation.lightbox.close')"
              :title="t('creation.lightbox.close')"
              @click="emit('update:open', false)"
            >
              <X aria-hidden="true" />
            </button>
          </div>
        </div>

        <div class="cs-lightbox-stage" @touchstart.passive="onTouchStart" @touchend.passive="onTouchEnd">
          <img v-if="current" :src="current.url" :alt="current.alt" />
          <template v-if="items.length > 1">
            <button
              type="button"
              class="cs-lightbox-nav cs-lightbox-nav--prev"
              :disabled="safeIndex <= 0"
              :aria-label="t('creation.lightbox.prev')"
              @click="step(-1)"
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <button
              type="button"
              class="cs-lightbox-nav cs-lightbox-nav--next"
              :disabled="safeIndex >= items.length - 1"
              :aria-label="t('creation.lightbox.next')"
              @click="step(1)"
            >
              <ChevronRight aria-hidden="true" />
            </button>
          </template>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<script setup lang="ts">
/**
 * 图片灯箱：基于 reka-ui 的 Dialog 原语（自带焦点陷阱与 Esc 关闭），样式全用 `cs-lightbox*`。
 * 键盘 ←/→ 切换、Esc 关闭；触屏左右滑动切换；关闭后焦点由 Dialog 还给触发元素。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, Download, ExternalLink, X } from '@lucide/vue'
import { DialogContent, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import type { CreationLightboxItem } from './types'

const props = defineProps<{
  open: boolean
  items: CreationLightboxItem[]
  index: number
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'update:index': [value: number]
}>()

const { t } = useI18n()

let touchStartX: number | null = null

const safeIndex = computed(() => Math.min(Math.max(props.index, 0), Math.max(props.items.length - 1, 0)))
const current = computed<CreationLightboxItem | undefined>(() => props.items[safeIndex.value])

function step(direction: 1 | -1): void {
  const next = safeIndex.value + direction
  if (next < 0 || next >= props.items.length) return
  emit('update:index', next)
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    step(-1)
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    step(1)
  }
}

function onTouchStart(event: TouchEvent): void {
  touchStartX = event.changedTouches[0]?.clientX ?? null
}

function onTouchEnd(event: TouchEvent): void {
  if (touchStartX === null) return
  const delta = (event.changedTouches[0]?.clientX ?? touchStartX) - touchStartX
  touchStartX = null
  if (Math.abs(delta) < 40) return
  step(delta > 0 ? -1 : 1)
}
</script>
