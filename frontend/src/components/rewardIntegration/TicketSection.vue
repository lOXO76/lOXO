<template>
  <div class="space-y-5">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.ticket.intro') }}</p>

    <div class="space-y-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.ticket.paramTitle') }}</h3>
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.ticket.paramDesc') }}</p>
      <DocsCodeBlock :label="t('rewardIntegration.ticket.paramTitle')" :code="queryExample" />
    </div>

    <div class="space-y-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.ticket.placeholderTitle') }}</h3>
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.ticket.placeholderDesc') }}</p>
      <DocsCodeBlock label="{ticket}" :code="placeholderExample" />
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.ticket.bindTitle') }}</h3>
      <ol class="mt-2 list-decimal space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.ticket.bind1') }}</li>
        <li>{{ t('rewardIntegration.ticket.bind2') }}</li>
        <li>{{ t('rewardIntegration.ticket.bind3') }}</li>
      </ol>
    </div>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.ticket.validityTitle') }}</h3>
      <DocsTable
        class="mt-2"
        :headers="[
          t('rewardIntegration.ticket.validityColTask'),
          t('rewardIntegration.ticket.validityColTtl'),
          t('rewardIntegration.ticket.validityColNote'),
        ]"
      >
        <tr>
          <td>browse</td>
          <td class="whitespace-nowrap">{{ t('rewardIntegration.ticket.validityBrowseTtl') }}</td>
          <td>{{ t('rewardIntegration.ticket.validityBrowseNote') }}</td>
        </tr>
        <tr>
          <td>register / recharge</td>
          <td class="whitespace-nowrap">{{ t('rewardIntegration.ticket.validityCallbackTtl') }}</td>
          <td>{{ t('rewardIntegration.ticket.validityCallbackNote') }}</td>
        </tr>
      </DocsTable>
      <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.ticket.validityNote') }}</p>
    </div>

    <div class="space-y-2">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.ticket.exampleTitle') }}</h3>
      <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.ticket.exampleNote') }}</p>
      <DocsCodeBlock label="landing.js" :code="landingSnippet" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import DocsTable from './DocsTable.vue'
import { currentOrigin } from './origin'

const { t } = useI18n()

const queryExample = computed(() => `https://example.com/landing?from=loxo&reward_ticket=rt_0123456789abcdef0123456789abcdef01234567#signup`)
const placeholderExample = computed(() => `${currentOrigin()}/task?ticket={ticket}`)

const landingSnippet = String.raw`// 落地页读取票据：存进会话，注册/登录成功后与你的用户 ID 绑定
const url = new URL(window.location.href)
const ticket = url.searchParams.get('reward_ticket') || sessionStorage.getItem('reward_ticket') || ''
if (ticket) {
  sessionStorage.setItem('reward_ticket', ticket)
  // 从地址栏去掉票据，避免被复制分享（其它参数保持不变）
  url.searchParams.delete('reward_ticket')
  history.replaceState(history.state, '', url.pathname + url.search + url.hash)
}`
</script>
