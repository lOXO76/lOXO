<template>
  <div class="min-h-screen bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-gray-200 bg-white/95 dark:border-dark-800 dark:bg-dark-900/95">
      <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-3">
          <template v-if="settings">
            <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-dark-800 dark:ring-dark-700">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </span>
            <span class="truncate text-base font-semibold text-gray-950 dark:text-white">{{ siteName }}</span>
          </template>
          <template v-else>
            <span class="h-10 w-10 flex-shrink-0 animate-pulse rounded-xl bg-gray-200 dark:bg-dark-700" aria-hidden="true"></span>
            <span class="h-5 w-28 animate-pulse rounded bg-gray-200 dark:bg-dark-700" aria-hidden="true"></span>
          </template>
        </RouterLink>
        <RouterLink
          to="/home"
          class="inline-flex flex-shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white px-4 py-2 text-sm font-semibold text-gray-700 transition hover:bg-gray-100 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700"
        >
          {{ t('rewardIntegration.backHome') }}
        </RouterLink>
      </div>
    </header>

    <main class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:py-10">
      <div class="mb-8 border-b border-gray-200 pb-6 dark:border-dark-800">
        <p class="text-sm font-medium text-primary-700 dark:text-primary-300">{{ t('rewardIntegration.badge') }}</p>
        <h1 class="mt-2 break-words text-2xl font-bold tracking-normal text-gray-950 dark:text-white sm:text-3xl">
          {{ t('rewardIntegration.title') }}
        </h1>
        <p class="mt-3 max-w-3xl text-sm leading-6 text-gray-600 dark:text-dark-300">{{ t('rewardIntegration.subtitle') }}</p>
        <p class="mt-2 text-xs text-gray-500 dark:text-dark-400">{{ t('rewardIntegration.originHint', { origin }) }}</p>
      </div>

      <div class="lg:flex lg:items-start lg:gap-8">
        <nav
          :aria-label="t('rewardIntegration.tocLabel')"
          data-test="reward-docs-toc"
          class="mb-5 flex gap-2 overflow-x-auto pb-1 lg:sticky lg:top-6 lg:mb-0 lg:max-h-[calc(100vh-3rem)] lg:w-60 lg:shrink-0 lg:flex-col lg:overflow-y-auto lg:pb-0"
        >
          <p class="hidden px-3 pb-1 text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-dark-500 lg:block">
            {{ t('rewardIntegration.tocLabel') }}
          </p>
          <a
            v-for="(section, index) in sections"
            :key="section.id"
            :href="`#${rewardDocsAnchor(section.id)}`"
            :data-test="`reward-docs-toc-${section.id}`"
            class="shrink-0 whitespace-nowrap rounded-md px-3 py-1.5 text-sm text-gray-600 transition hover:bg-gray-100 hover:text-gray-900 dark:text-dark-300 dark:hover:bg-dark-800 dark:hover:text-white lg:whitespace-normal"
          >
            <span class="mr-1.5 text-xs text-gray-400 dark:text-dark-500">{{ index + 1 }}.</span>{{ t(section.titleKey) }}
          </a>
        </nav>

        <div class="min-w-0 flex-1 space-y-6">
          <section
            v-for="(section, index) in sections"
            :id="rewardDocsAnchor(section.id)"
            :key="section.id"
            :data-test="`reward-docs-section-${section.id}`"
            class="scroll-mt-6 rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900"
          >
            <h2 class="flex items-baseline gap-2 text-lg font-semibold text-gray-900 dark:text-white">
              <span class="text-sm font-mono text-primary-600 dark:text-primary-400">{{ String(index + 1).padStart(2, '0') }}</span>
              {{ t(section.titleKey) }}
            </h2>
            <div class="mt-4">
              <component :is="sectionComponents[section.id]" />
            </div>
          </section>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { sanitizeUrl } from '@/utils/url'
import ApiReferenceSection from '@/components/rewardIntegration/ApiReferenceSection.vue'
import ChecklistSection from '@/components/rewardIntegration/ChecklistSection.vue'
import CodeExamplesSection from '@/components/rewardIntegration/CodeExamplesSection.vue'
import ErrorCodesSection from '@/components/rewardIntegration/ErrorCodesSection.vue'
import OverviewSection from '@/components/rewardIntegration/OverviewSection.vue'
import PreparationSection from '@/components/rewardIntegration/PreparationSection.vue'
import SdkSection from '@/components/rewardIntegration/SdkSection.vue'
import SignatureSection from '@/components/rewardIntegration/SignatureSection.vue'
import TicketSection from '@/components/rewardIntegration/TicketSection.vue'
import { currentOrigin } from '@/components/rewardIntegration/origin'
import { REWARD_DOCS_SECTIONS, rewardDocsAnchor } from '@/components/rewardIntegration/sections'

const { t } = useI18n()
const appStore = useAppStore()

const sections = REWARD_DOCS_SECTIONS
const origin = currentOrigin()

const sectionComponents: Record<string, Component> = {
  overview: OverviewSection,
  preparation: PreparationSection,
  ticket: TicketSection,
  sdk: SdkSection,
  signature: SignatureSection,
  api: ApiReferenceSection,
  errors: ErrorCodesSection,
  examples: CodeExamplesSection,
  checklist: ChecklistSection,
}

const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => settings.value?.site_name || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(settings.value?.site_logo || '', {
  allowRelative: true,
  allowDataUrl: true,
}))

onMounted(async () => {
  // 公开页不阻塞渲染：拿不到公开设置时用默认站点名兜底
  await appStore.fetchPublicSettings()
})
</script>
