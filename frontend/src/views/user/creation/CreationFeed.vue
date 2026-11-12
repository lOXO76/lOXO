<template>
  <section class="cs-section" :aria-busy="loading || activeCount > 0">
    <div class="cs-section-head">
      <div class="flex min-w-0 items-center gap-3">
        <h2 class="cs-section-title">
          {{ t('creation.history.title') }}
          <span v-if="count" class="cs-section-count">{{ count }}{{ hasMore ? '+' : '' }}</span>
        </h2>
        <span v-if="activeCount" class="cs-sync" data-testid="creation-active-count">
          <span class="cs-sync-dot" aria-hidden="true" />
          <span>{{ t('creation.result.activeCount', { count: activeCount }) }}</span>
        </span>
      </div>
      <div class="cs-section-actions">
        <div class="cs-segmented cs-segmented--sm" role="tablist" :aria-label="t('creation.history.filterLabel')">
          <button
            v-for="option in filters"
            :key="option"
            type="button"
            role="tab"
            class="cs-segmented-item"
            :aria-selected="filter === option"
            :data-testid="`creation-filter-${option}`"
            @click="emit('update:filter', option)"
          >
            {{ t(`creation.history.filters.${option}`) }}
          </button>
        </div>
        <button type="button" class="btn btn-ghost btn-sm" :disabled="loading" @click="emit('refresh')">
          <RefreshCw :class="{ 'animate-spin': loading }" aria-hidden="true" />{{ t('creation.history.refresh') }}
        </button>
        <router-link to="/async-tasks" class="btn btn-ghost btn-sm">
          {{ t('creation.history.viewAll') }}<ArrowRight aria-hidden="true" />
        </router-link>
      </div>
    </div>

    <p class="cs-section-hint">{{ t('creation.history.hint') }}</p>

    <div v-if="error" role="alert" class="cs-callout cs-callout--danger">
      <CircleX class="cs-callout-icon" aria-hidden="true" />
      <div class="cs-callout-content">
        <p class="cs-callout-title">{{ error }}</p>
        <div class="cs-callout-actions">
          <button type="button" class="cs-callout-link" @click="emit('refresh')">{{ t('creation.history.refresh') }}</button>
        </div>
      </div>
    </div>

    <div v-if="loading && !count" class="cs-feed" role="status">
      <span class="sr-only">{{ t('creation.loading') }}</span>
      <div v-for="index in 3" :key="index" class="card cs-task p-4 sm:p-5">
        <div class="skeleton h-4 w-1/3" />
        <div class="skeleton mt-3 h-3 w-2/3" />
        <div class="skeleton mt-3 h-36 w-full rounded-xl" />
      </div>
    </div>

    <div v-else-if="!count && !error" class="card cs-empty" role="status">
      <img class="cs-illu" :src="emptyStudio" alt="" aria-hidden="true" />
      <h3 class="cs-empty-title">{{ t('creation.history.empty') }}</h3>
      <p class="cs-empty-desc">{{ t('creation.history.emptyHint') }}</p>
    </div>

    <template v-else-if="count">
      <ul class="cs-feed" role="list">
        <!-- 本页提交、仍在跟踪的任务排在最前 -->
        <li v-for="task in tasks" :key="`live:${task.id}`">
          <CreationTaskCard
            :task="task"
            @stop="emit('stop', $event)"
            @dismiss="emit('dismiss', $event)"
            @preview="openPreview"
          />
        </li>
        <li v-for="item in items" :key="item.key">
          <CreationHistoryCard :item="item" @preview="openPreview" />
        </li>
      </ul>

      <div v-if="hasMore" class="cs-load-more">
        <button type="button" class="btn btn-secondary" :disabled="loading" data-testid="creation-history-more" @click="emit('loadMore')">
          <LoaderCircle v-if="loading" class="h-4 w-4 animate-spin" aria-hidden="true" />
          {{ t('creation.history.loadMore') }}
        </button>
      </div>
      <p v-else-if="items.length" class="cs-feed-end">{{ t('creation.history.noMore') }}</p>
    </template>

    <CreationLightbox v-model:open="lightboxOpen" v-model:index="lightboxIndex" :items="lightboxItems" />
  </section>
</template>

<script setup lang="ts">
/**
 * 创作页右侧的历史记录：本页提交的任务（带进度）和已归档的图片、视频合并成一个列表，
 * 图片直接显示、视频直接播放。
 */
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, CircleX, LoaderCircle, RefreshCw } from '@lucide/vue'
import CreationHistoryCard from './CreationHistoryCard.vue'
import CreationLightbox from './CreationLightbox.vue'
import CreationTaskCard from './CreationTaskCard.vue'
import emptyStudio from '@/assets/creation/empty-studio.svg'
import type { CreationCurrentTask, CreationFeedFilter, CreationHistoryItem, CreationLightboxItem } from './types'

const props = defineProps<{
  tasks: CreationCurrentTask[]
  items: CreationHistoryItem[]
  filter: CreationFeedFilter
  loading: boolean
  error: string
  /** 服务端还有更早的记录没有加载。 */
  hasMore: boolean
}>()
const emit = defineEmits<{
  'update:filter': [value: CreationFeedFilter]
  refresh: []
  loadMore: []
  stop: [id: number]
  dismiss: [id: number]
}>()
const { t } = useI18n()

const filters: CreationFeedFilter[] = ['all', 'image', 'video']
const count = computed(() => props.tasks.length + props.items.length)
const activeCount = computed(() => props.tasks.filter(task => task.status === 'processing').length)

const lightboxOpen = ref(false)
const lightboxIndex = ref(0)
const lightboxItems = ref<CreationLightboxItem[]>([])

function openPreview(items: CreationLightboxItem[], index: number): void {
  if (!items.length) return
  lightboxItems.value = items
  lightboxIndex.value = index
  lightboxOpen.value = true
}
</script>
