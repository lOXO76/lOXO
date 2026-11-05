<template>
  <div class="space-y-4">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.examples.intro') }}</p>

    <div class="flex gap-1 overflow-x-auto border-b border-gray-200 pb-px dark:border-dark-700" role="tablist" :aria-label="t('rewardIntegration.examples.tabLabel')">
      <button
        v-for="example in examples"
        :key="example.id"
        type="button"
        role="tab"
        :aria-selected="activeId === example.id"
        :data-test="`code-tab-${example.id}`"
        class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium transition"
        :class="activeId === example.id
          ? 'border-primary-600 text-primary-700 dark:border-primary-400 dark:text-primary-300'
          : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-dark-300 dark:hover:text-white'"
        @click="activeId = example.id"
      >{{ t(example.labelKey) }}</button>
    </div>

    <div v-if="activeExample" class="space-y-2" :data-test="`code-panel-${activeExample.id}`">
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t(activeExample.runKey) }}</p>
      <DocsCodeBlock :label="activeExample.filename" :code="activeExample.code" :code-test-id="`code-${activeExample.id}`" />
    </div>

    <p class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm leading-6 text-gray-600 dark:border-dark-700 dark:bg-dark-800 dark:text-dark-200">
      {{ t('rewardIntegration.examples.note') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import { buildRewardCodeExamples, type RewardCodeExampleId } from './codeExamples'
import { currentOrigin } from './origin'

const { t } = useI18n()

const examples = computed(() => buildRewardCodeExamples(currentOrigin()))
const activeId = ref<RewardCodeExampleId>('curl')
const activeExample = computed(() => examples.value.find((example) => example.id === activeId.value) ?? examples.value[0])
</script>
