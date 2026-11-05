<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.intro') }}</p>

    <div class="space-y-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.installTitle') }}</h3>
      <DocsCodeBlock label="index.html" :code="installSnippet" />
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.installNote') }}</p>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.attrsTitle') }}</h3>
      <DocsTable
        class="mt-2"
        :headers="[
          t('rewardIntegration.sdk.attrColName'),
          t('rewardIntegration.sdk.attrColDesc'),
        ]"
      >
        <tr>
          <td>data-app-key</td>
          <td>{{ t('rewardIntegration.sdk.attrAppKeyDesc') }}</td>
        </tr>
        <tr>
          <td>data-badge</td>
          <td>{{ t('rewardIntegration.sdk.attrBadgeDesc') }}</td>
        </tr>
        <tr>
          <td>data-lang</td>
          <td>{{ t('rewardIntegration.sdk.attrLangDesc') }}</td>
        </tr>
      </DocsTable>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.apiTitle') }}</h3>
      <DocsTable
        class="mt-2"
        :headers="[
          t('rewardIntegration.sdk.apiColName'),
          t('rewardIntegration.sdk.apiColDesc'),
        ]"
      >
        <tr>
          <td>window.RewardTaskSDK.getState()</td>
          <td>{{ t('rewardIntegration.sdk.apiGetStateDesc') }}</td>
        </tr>
        <tr>
          <td>window.RewardTaskSDK.onChange(fn)</td>
          <td>{{ t('rewardIntegration.sdk.apiOnChangeDesc') }}</td>
        </tr>
      </DocsTable>
      <div class="mt-3">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.stateTitle') }}</h4>
        <DocsTable
          class="mt-2"
          :headers="[
            t('rewardIntegration.sdk.stateColField'),
            t('rewardIntegration.sdk.stateColValue'),
            t('rewardIntegration.sdk.stateColDesc'),
          ]"
        >
          <tr>
            <td>status</td>
            <td class="font-mono text-xs">'idle' | 'in_progress' | 'paused' | 'completed' | 'error'</td>
            <td>{{ t('rewardIntegration.sdk.stateStatusDesc') }}</td>
          </tr>
          <tr>
            <td>paused_reason</td>
            <td class="font-mono text-xs">'idle' | 'hidden' | null</td>
            <td>{{ t('rewardIntegration.sdk.statePausedReasonDesc') }}</td>
          </tr>
          <tr>
            <td>progress_seconds</td>
            <td class="font-mono text-xs">number</td>
            <td>{{ t('rewardIntegration.sdk.stateProgressDesc') }}</td>
          </tr>
          <tr>
            <td>browse_seconds</td>
            <td class="font-mono text-xs">number</td>
            <td>{{ t('rewardIntegration.sdk.stateBrowseDesc') }}</td>
          </tr>
          <tr>
            <td>completed</td>
            <td class="font-mono text-xs">boolean</td>
            <td>{{ t('rewardIntegration.sdk.stateCompletedDesc') }}</td>
          </tr>
          <tr>
            <td>error</td>
            <td class="font-mono text-xs">string | null</td>
            <td>{{ t('rewardIntegration.sdk.stateErrorDesc') }}</td>
          </tr>
        </DocsTable>
      </div>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.timingTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.sdk.timing1') }}</li>
        <li>{{ t('rewardIntegration.sdk.timing2') }}</li>
        <li>{{ t('rewardIntegration.sdk.timing3') }}</li>
        <li>{{ t('rewardIntegration.sdk.timing4') }}</li>
        <li>{{ t('rewardIntegration.sdk.timing5') }}</li>
      </ul>
    </div>

    <div data-test="sdk-idle" class="space-y-4 border-t border-gray-200 pt-5 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idleTitle') }}</h3>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.idleIntro') }}</p>
      </div>

      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idleRuleTitle') }}</h4>
        <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
          <li>{{ t('rewardIntegration.sdk.idleRule1') }}</li>
          <li>{{ t('rewardIntegration.sdk.idleRule2') }}</li>
          <li>{{ t('rewardIntegration.sdk.idleRule3') }}</li>
        </ul>
      </div>

      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idleThresholdTitle') }}</h4>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.idleThresholdDesc') }}</p>
      </div>

      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idlePauseTitle') }}</h4>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.idlePauseDesc') }}</p>
      </div>

      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idleResumeTitle') }}</h4>
        <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.idleResumeDesc') }}</p>
      </div>

      <div class="space-y-2">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.idleCustomUiTitle') }}</h4>
        <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.idleCustomUiDesc') }}</p>
        <DocsCodeBlock label="custom-ui.js" :code="customUiSnippet" code-test-id="sdk-custom-ui" />
      </div>

      <div class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-500/30 dark:bg-amber-500/10">
        <p class="text-sm font-semibold text-amber-800 dark:text-amber-200">{{ t('rewardIntegration.sdk.idleLimitTitle') }}</p>
        <p class="mt-1 text-sm leading-6 text-amber-700 dark:text-amber-200/90">{{ t('rewardIntegration.sdk.idleLimitDesc') }}</p>
      </div>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.spaTitle') }}</h3>
      <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.sdk.spaDesc') }}</p>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.sdk.faqTitle') }}</h3>
      <dl class="mt-2 space-y-3">
        <div v-for="faq in faqs" :key="faq.question">
          <dt class="text-sm font-medium text-gray-900 dark:text-white">{{ faq.question }}</dt>
          <dd class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ faq.answer }}</dd>
        </div>
      </dl>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import DocsTable from './DocsTable.vue'
import { currentOrigin } from './origin'
import { buildSdkCustomUiSnippet, buildSdkInstallSnippet } from './sdkSnippets'

const { t } = useI18n()

const installSnippet = computed(() => buildSdkInstallSnippet(currentOrigin()))
const customUiSnippet = computed(() => buildSdkCustomUiSnippet(currentOrigin()))

const faqs = computed(() => [
  { question: t('rewardIntegration.sdk.faqQ1'), answer: t('rewardIntegration.sdk.faqA1') },
  { question: t('rewardIntegration.sdk.faqQ2'), answer: t('rewardIntegration.sdk.faqA2') },
  { question: t('rewardIntegration.sdk.faqQ3'), answer: t('rewardIntegration.sdk.faqA3') },
  { question: t('rewardIntegration.sdk.faqQ4'), answer: t('rewardIntegration.sdk.faqA4') },
])
</script>
