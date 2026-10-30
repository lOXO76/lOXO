<template>
  <section
    class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
    :class="isFullscreen ? 'fixed inset-0 z-[100] flex flex-col rounded-none' : ''"
    data-testid="visual-artifact-frame"
  >
    <header class="flex items-center justify-between gap-3 border-b border-gray-100 px-3 py-2 dark:border-dark-700">
      <h4 class="min-w-0 truncate text-xs font-semibold text-gray-700 dark:text-gray-300">{{ title }}</h4>
      <button
        v-if="renderUrl"
        type="button"
        class="btn btn-ghost btn-sm shrink-0 !px-2 !py-1 text-xs"
        @click="isFullscreen = !isFullscreen"
      >
        {{ isFullscreen ? t('degradation.visual.exitFullscreen') : t('degradation.visual.fullscreen') }}
      </button>
    </header>

    <div :class="isFullscreen ? 'min-h-0 flex-1' : ''">
      <div v-if="!renderUrl" class="flex min-h-56 flex-col items-center justify-center gap-3 bg-gray-50 px-4 py-8 text-center dark:bg-dark-900/60">
        <template v-if="loading">
          <LoadingSpinner size="md" />
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('degradation.visual.loading') }}</p>
        </template>
        <template v-else>
          <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('degradation.visual.placeholder') }}</p>
          <p v-if="loadError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ loadError }}</p>
          <button type="button" class="btn btn-secondary btn-sm" data-testid="visual-load" @click="loadPreview">
            {{ loadError ? t('degradation.visual.retry') : t('degradation.visual.load') }}
          </button>
        </template>
      </div>
      <iframe
        v-else
        :src="renderUrl"
        :title="title"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        class="block w-full border-0 bg-white"
        :class="isFullscreen ? 'h-full' : 'h-96'"
      ></iframe>
    </div>

    <footer class="border-t border-gray-100 px-3 py-2 text-[11px] text-gray-400 dark:border-dark-700 dark:text-gray-500">
      {{ t('degradation.visual.sandboxNote') }}
    </footer>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { buildVisualRenderUrl, degradationAPI } from '@/api/degradation'
import { degradationAdminAPI } from '@/api/admin/degradation'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  runId: number
  scope: 'user' | 'admin'
  title: string
}>()
const { t } = useI18n()
const renderUrl = ref('')
const loading = ref(false)
const loadError = ref('')
const isFullscreen = ref(false)
let requestSequence = 0

async function loadPreview() {
  if (loading.value || renderUrl.value) return
  const sequence = ++requestSequence
  loading.value = true
  loadError.value = ''
  try {
    const result = props.scope === 'admin'
      ? await degradationAdminAPI.createRunViewToken(props.runId)
      : await degradationAPI.createVisualViewToken(props.runId)
    if (sequence !== requestSequence) return
    renderUrl.value = buildVisualRenderUrl(result.token)
  } catch (error) {
    if (sequence === requestSequence) {
      loadError.value = extractApiErrorMessage(error, t('degradation.visual.loadFailed'))
    }
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

watch(
  () => props.runId,
  () => {
    requestSequence += 1
    renderUrl.value = ''
    loading.value = false
    loadError.value = ''
    isFullscreen.value = false
  },
)

onBeforeUnmount(() => {
  requestSequence += 1
})
</script>
