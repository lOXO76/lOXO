<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-5">
      <div>
        <RouterLink to="/tickets" class="inline-flex items-center gap-1.5 text-sm font-medium text-gray-500 transition-colors hover:text-primary-600 dark:text-dark-400 dark:hover:text-primary-400" data-testid="ticket-back">
          <ArrowLeftIcon class="h-4 w-4" aria-hidden="true" />
          {{ t('tickets.detail.back') }}
        </RouterLink>
      </div>

      <!-- 加载 / 错误 -->
      <div v-if="loading && !detail" class="card px-6 py-16 text-center" role="status">
        <LoadingSpinner size="md" class="mx-auto" />
        <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.detail.loading') }}</p>
      </div>

      <div v-else-if="loadError" class="card px-6 py-16 text-center" role="alert">
        <CircleAlert class="mx-auto h-10 w-10 text-red-500" aria-hidden="true" />
        <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">{{ loadError }}</p>
        <button type="button" class="btn btn-secondary mt-4" data-testid="ticket-detail-retry" @click="load(false)">
          {{ t('tickets.common.retry') }}
        </button>
      </div>

      <template v-else-if="detail">
        <!-- 头部信息 -->
        <header class="card p-5">
          <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0 space-y-2">
              <h1 class="break-words text-xl font-bold tracking-tight text-gray-900 dark:text-white">{{ detail.ticket.subject }}</h1>
              <div class="flex flex-wrap items-center gap-2">
                <TicketStatusBadge :status="detail.ticket.status" />
                <TicketPriorityBadge :priority="detail.ticket.priority" />
                <span class="rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">
                  {{ t(`tickets.category.${detail.ticket.category}`) }}
                </span>
              </div>
              <p class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
                <span class="font-mono">{{ t('tickets.detail.ticketNo', { no: detail.ticket.ticket_no }) }}</span>
                <span>{{ t('tickets.detail.created', { time: formatDateTime(detail.ticket.created_at) }) }}</span>
                <span>{{ t('tickets.detail.updated', { time: formatDateTime(detail.ticket.updated_at) }) }}</span>
              </p>
            </div>

            <div class="flex shrink-0 flex-wrap gap-2">
              <button
                v-if="detail.ticket.status === 'resolved' || detail.ticket.status === 'closed'"
                type="button"
                class="btn btn-primary"
                :disabled="actionBusy"
                data-testid="ticket-reopen"
                @click="reopen"
              >
                {{ t('tickets.detail.reopen') }}
              </button>
              <button
                v-else
                type="button"
                class="btn btn-secondary"
                :disabled="actionBusy"
                data-testid="ticket-close"
                @click="showCloseConfirm = true"
              >
                {{ t('tickets.detail.close') }}
              </button>
            </div>
          </div>
        </header>

        <!-- 消息流：内部滚动容器，进入时定位到最新消息，轮询时不打断向上翻看 -->
        <section ref="messageListRef" class="card max-h-[65vh] space-y-4 overflow-y-auto overscroll-contain p-5" :aria-label="t('tickets.title')">
          <p v-if="detail.messages.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
            {{ t('tickets.detail.emptyMessages') }}
          </p>
          <TicketMessageItem v-for="message in detail.messages" :key="message.id" :message="message" />
        </section>

        <!-- 回复框 -->
        <section class="card p-5">
          <p v-if="detail.ticket.status === 'closed'" class="rounded-xl bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300" data-testid="ticket-closed-hint">
            {{ t('tickets.detail.closedHint') }}
          </p>
          <form v-else class="space-y-3" @submit.prevent="send">
            <label class="sr-only" for="ticket-reply">{{ t('tickets.detail.replyPlaceholder') }}</label>
            <textarea
              id="ticket-reply"
              ref="replyInputRef"
              :value="reply"
              rows="4"
              class="input w-full resize-y"
              :placeholder="t('tickets.detail.replyPlaceholder')"
              :disabled="sending"
              data-testid="ticket-reply-input"
              @input="onReplyInput"
            ></textarea>
            <div class="flex flex-wrap items-center justify-between gap-3">
              <p class="text-xs text-gray-400 dark:text-dark-500">
                <span class="tabular-nums" data-testid="ticket-reply-counter">{{ replyLength }}/{{ maxBodyLength }}</span>
                <span class="ml-3">{{ t('tickets.detail.polling') }}</span>
              </p>
              <button type="submit" class="btn btn-primary" :disabled="sending || !canSend" data-testid="ticket-reply-submit">
                {{ sending ? t('tickets.detail.sending') : t('tickets.detail.send') }}
              </button>
            </div>
            <p v-if="replyError" class="rounded-xl bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300" role="alert" data-testid="ticket-reply-error">
              {{ replyError }}
            </p>
          </form>
        </section>
      </template>

      <ConfirmDialog
        :show="showCloseConfirm"
        :title="t('tickets.detail.closeConfirmTitle')"
        :message="t('tickets.detail.closeConfirmMessage')"
        :confirm-text="t('tickets.detail.close')"
        :cancel-text="t('tickets.common.cancel')"
        danger
        @cancel="showCloseConfirm = false"
        @confirm="closeTicketAction"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, RouterLink } from 'vue-router'
import { ArrowLeft as ArrowLeftIcon, CircleAlert } from '@lucide/vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import TicketStatusBadge from '@/components/tickets/TicketStatusBadge.vue'
import TicketPriorityBadge from '@/components/tickets/TicketPriorityBadge.vue'
import TicketMessageItem from '@/components/tickets/TicketMessageItem.vue'
import {
  closeTicket,
  getTicket,
  getTicketConfig,
  reopenTicket,
  sendTicketMessage,
} from '@/api/tickets'
import { formatDateTime } from '@/utils/format'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import { countTicketChars, isBlankTicketText, isTicketTextWithinLimit, truncateTicketText } from '@/utils/ticketText'
import { useAppStore } from '@/stores/app'
import { refreshTicketUnreadCount } from '@/composables/useTicketUnread'
import type { TicketDetail } from '@/types/tickets'

const POLL_INTERVAL_MS = 15_000
const DEFAULT_MAX_BODY_LENGTH = 5000
/** 距离底部多少像素内算「正在看最新消息」，用于决定轮询后是否自动跟随。 */
const NEAR_BOTTOM_PX = 96

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const ticketId = computed(() => {
  const raw = route.params.id
  const id = Number.parseInt(Array.isArray(raw) ? raw[0] : String(raw ?? ''), 10)
  return Number.isFinite(id) && id > 0 ? id : null
})

const detail = ref<TicketDetail | null>(null)
const loading = ref(true)
const loadError = ref('')
const reply = ref('')
const sending = ref(false)
const replyError = ref('')
const actionBusy = ref(false)
const showCloseConfirm = ref(false)
const maxBodyLength = ref(DEFAULT_MAX_BODY_LENGTH)
const messageListRef = ref<HTMLElement | null>(null)
const replyInputRef = ref<HTMLTextAreaElement | null>(null)

const replyLength = computed(() => countTicketChars(reply.value))
const canSend = computed(() => !isBlankTicketText(reply.value) && isTicketTextWithinLimit(reply.value, maxBodyLength.value))

let pollTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
let loadVersion = 0

function clearPollTimer(): void {
  if (pollTimer !== undefined) {
    clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

function schedulePoll(): void {
  clearPollTimer()
  if (disposed) return
  if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
  pollTimer = setTimeout(() => {
    void load(true)
  }, POLL_INTERVAL_MS)
}

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const el = messageListRef.value
  if (!el) return
  try {
    if (typeof el.scrollTo === 'function') el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' })
    else el.scrollTop = el.scrollHeight
  } catch {
    el.scrollTop = el.scrollHeight
  }
}

/** 用户是否已经在（或接近）消息列表底部。 */
function isNearBottom(): boolean {
  const el = messageListRef.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight <= NEAR_BOTTOM_PX
}

/** 首次加载直接贴底（不要平滑动画），避免进页时先看到顶部。 */
async function jumpToLatest(): Promise<void> {
  await nextTick()
  const el = messageListRef.value
  if (el) el.scrollTop = el.scrollHeight
}

/** 回复框按 code point 截断（原生 maxlength 会按 UTF-16 计算）。 */
function onReplyInput(event: Event): void {
  const el = event.target as HTMLTextAreaElement
  const next = truncateTicketText(el.value, maxBodyLength.value)
  reply.value = next
  if (el.value !== next) el.value = next
}

/** 拉取详情；`silent=true` 为轮询刷新（不显示 loading、失败不报错）。 */
async function load(silent = false): Promise<void> {
  const id = ticketId.value
  if (id === null) {
    loading.value = false
    loadError.value = t('tickets.detail.notFound')
    return
  }
  const version = ++loadVersion
  if (!silent) loading.value = true
  if (!silent) loadError.value = ''
  // 先记录是否为首次加载；是否跟随到底部要等数据回来、替换 DOM 之前再量一次
  // （请求期间用户可能已经向上翻看）。
  const firstLoad = detail.value === null
  try {
    const next = await getTicket(id)
    if (disposed || version !== loadVersion) return
    const shouldFollow = firstLoad || isNearBottom()
    const previousLastId = detail.value?.messages.at(-1)?.id ?? 0
    const nextLastId = next.messages.at(-1)?.id ?? 0
    detail.value = next
    loadError.value = ''
    if (firstLoad) {
      void jumpToLatest()
    } else if (nextLastId > previousLastId && shouldFollow) {
      void scrollToBottom()
    }
    // 详情接口会清除 user_unread，同步角标。
    void refreshTicketUnreadCount()
  } catch (error) {
    if (disposed || version !== loadVersion) return
    if (!silent || !detail.value) {
      loadError.value = resolveTicketErrorMessage(error, t, 'tickets.detail.loadFailed')
    }
  } finally {
    if (!disposed && version === loadVersion) {
      loading.value = false
      schedulePoll()
    }
  }
}

async function loadConfig(): Promise<void> {
  try {
    const config = await getTicketConfig()
    if (!disposed && config?.max_body_length > 0) maxBodyLength.value = config.max_body_length
  } catch {
    // 配置失败不影响阅读与回复，回退到默认长度。
  }
}

async function send(): Promise<void> {
  const id = ticketId.value
  const body = reply.value
  if (id === null || sending.value) return
  if (isBlankTicketText(body)) return
  if (!isTicketTextWithinLimit(body, maxBodyLength.value)) {
    replyError.value = t('tickets.create.errors.tooLong', { max: maxBodyLength.value })
    return
  }
  sending.value = true
  replyError.value = ''
  try {
    await sendTicketMessage(id, body)
    if (disposed) return
    // 期间切换到别的工单：不要把「回复成功」的状态和草稿改到新工单上。
    if (ticketId.value !== id) return
    reply.value = ''
    appStore.showSuccess(t('tickets.detail.replySent'))
    await load(true)
    void scrollToBottom()
  } catch (error) {
    if (ticketId.value === id) replyError.value = resolveTicketErrorMessage(error, t)
  } finally {
    sending.value = false
  }
}

async function closeTicketAction(): Promise<void> {
  showCloseConfirm.value = false
  const id = ticketId.value
  if (id === null || actionBusy.value) return
  actionBusy.value = true
  try {
    const summary = await closeTicket(id)
    if (detail.value) detail.value.ticket = { ...detail.value.ticket, ...summary }
    appStore.showSuccess(t('tickets.detail.closedSuccess'))
    await load(true)
  } catch (error) {
    appStore.showError(resolveTicketErrorMessage(error, t))
  } finally {
    actionBusy.value = false
  }
}

async function reopen(): Promise<void> {
  const id = ticketId.value
  if (id === null || actionBusy.value) return
  actionBusy.value = true
  try {
    const summary = await reopenTicket(id)
    if (detail.value) detail.value.ticket = { ...detail.value.ticket, ...summary }
    appStore.showSuccess(t('tickets.detail.reopenSuccess'))
    await load(true)
  } catch (error) {
    appStore.showError(resolveTicketErrorMessage(error, t))
  } finally {
    actionBusy.value = false
  }
}

function onVisibilityChange(): void {
  if (document.visibilityState === 'visible') void load(true)
  else clearPollTimer()
}

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  void loadConfig()
  void load(false)
})

// 在 /tickets/:id 之间跳转时复用同一个组件实例：作废上一张工单的在途请求、
// 清空草稿与错误，再重新拉取。
watch(ticketId, (next, previous) => {
  if (next === previous) return
  ++loadVersion
  detail.value = null
  reply.value = ''
  replyError.value = ''
  showCloseConfirm.value = false
  void load(false)
})

onBeforeUnmount(() => {
  disposed = true
  ++loadVersion
  clearPollTimer()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>
