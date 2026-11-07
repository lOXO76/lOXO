<template>
  <!-- 系统消息：居中提示条 -->
  <div v-if="message.author_type === 'system'" class="flex justify-center" data-testid="ticket-message-system">
    <p class="inline-flex max-w-full items-center gap-1.5 rounded-full bg-gray-100 px-3 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">
      <InfoIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span class="min-w-0 break-words">{{ systemText }}</span>
      <span class="whitespace-nowrap text-gray-400 dark:text-dark-500">· {{ formatDateTime(message.created_at) }}</span>
    </p>
  </div>

  <!-- 用户 / 客服消息 -->
  <div v-else class="flex gap-3" :class="isOwn ? 'flex-row-reverse' : 'flex-row'" :data-testid="`ticket-message-${message.id}`">
    <div
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
      :class="isOwn
        ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
        : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-200'"
      aria-hidden="true"
    >
      {{ isOwn ? t('tickets.detail.you').slice(0, 1) : t('tickets.detail.support').slice(0, 1) }}
    </div>

    <div class="min-w-0 max-w-[min(100%,42rem)] space-y-1" :class="isOwn ? 'items-end text-right' : 'text-left'">
      <div class="flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400" :class="isOwn ? 'justify-end' : ''">
        <span class="font-medium text-gray-700 dark:text-dark-200">{{ authorLabel }}</span>
        <span v-if="message.via === 'agent'" class="inline-flex items-center gap-1 rounded bg-violet-50 px-1.5 py-0.5 font-medium text-violet-700 dark:bg-violet-900/25 dark:text-violet-300" data-testid="ticket-message-agent-tag">
          <BotIcon class="h-3 w-3" aria-hidden="true" />
          {{ t('tickets.detail.viaAgent') }}
        </span>
        <span>{{ formatDateTime(message.created_at) }}</span>
      </div>

      <div
        class="inline-block max-w-full whitespace-pre-wrap break-words rounded-2xl px-4 py-2.5 text-sm leading-relaxed"
        :class="isOwn
          ? 'bg-primary-600 text-white'
          : 'bg-gray-100 text-gray-800 dark:bg-dark-800 dark:text-dark-100'"
        data-testid="ticket-message-body"
      >{{ message.body }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bot as BotIcon, Info as InfoIcon } from '@lucide/vue'
import { formatDateTime } from '@/utils/format'
import type { TicketMessage } from '@/types/tickets'

const props = defineProps<{ message: TicketMessage }>()

const { t } = useI18n()

const isOwn = computed(() => props.message.author_type === 'user')

const authorLabel = computed(() => {
  if (props.message.author_type === 'user') return t('tickets.detail.you')
  // 用户端接口里 admin 消息的 author_name 恒为 "Support"，仍用 i18n 文案统一兜底。
  return props.message.author_name || t('tickets.detail.support')
})

/**
 * 系统消息 body 形如 `status_changed:resolved`，按 i18n 渲染；
 * 无法识别的 body 原样展示（纯文本，绝不解析 HTML）。
 */
const systemText = computed(() => {
  const raw = props.message.body?.trim() ?? ''
  const [event, param] = raw.split(':', 2)
  if (event === 'status_changed' && param) {
    const statusKey = `tickets.status.${param}`
    const translated = t(statusKey)
    return t('tickets.detail.systemEvents.status_changed', {
      status: translated === statusKey ? param : translated,
    })
  }
  return raw || t('tickets.detail.system')
})
</script>
