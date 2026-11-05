<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.signature.intro') }}</p>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.headersTitle') }}</h3>
      <DocsTable
        class="mt-2"
        :headers="[
          t('rewardIntegration.signature.headerColName'),
          t('rewardIntegration.signature.headerColDesc'),
        ]"
      >
        <tr>
          <td>X-Reward-App-Key</td>
          <td>{{ t('rewardIntegration.signature.headerAppKeyDesc') }}</td>
        </tr>
        <tr>
          <td>X-Reward-Timestamp</td>
          <td>{{ t('rewardIntegration.signature.headerTimestampDesc') }}</td>
        </tr>
        <tr>
          <td>X-Reward-Nonce</td>
          <td>{{ t('rewardIntegration.signature.headerNonceDesc') }}</td>
        </tr>
        <tr>
          <td>X-Reward-Signature</td>
          <td>{{ t('rewardIntegration.signature.headerSignatureDesc') }}</td>
        </tr>
      </DocsTable>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.stepsTitle') }}</h3>
      <ol class="mt-2 list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.signature.step1') }}</li>
        <li>{{ t('rewardIntegration.signature.step2') }}</li>
        <li>{{ t('rewardIntegration.signature.step3') }}</li>
        <li>{{ t('rewardIntegration.signature.step4') }}</li>
      </ol>
    </div>

    <div class="space-y-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.stringTitle') }}</h3>
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.signature.stringDesc') }}</p>
      <DocsCodeBlock :label="t('rewardIntegration.signature.stringTitle')" :code="signingStringExample" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.rulesTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.signature.rule1') }}</li>
        <li>{{ t('rewardIntegration.signature.rule2') }}</li>
        <li>{{ t('rewardIntegration.signature.rule3') }}</li>
        <li>{{ t('rewardIntegration.signature.rule4') }}</li>
      </ul>
    </div>

    <SignatureCalculator />

    <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800">
      <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.noteTitle') }}</p>
      <p class="mt-1 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.signature.note') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import DocsTable from './DocsTable.vue'
import SignatureCalculator from './SignatureCalculator.vue'

const { t } = useI18n()

// 待签名字符串示例：三行依次是 timestamp、nonce、原始请求体
const signingStringExample = [
  '1730790000',
  'nonce_abc123456789',
  '{"event_id":"evt_20261105_0001","event_type":"register","ticket":"rt_0123456789abcdef0123456789abcdef01234567","external_user_id":"u_1001"}',
].join('\n')
</script>
