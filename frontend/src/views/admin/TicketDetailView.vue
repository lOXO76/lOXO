<template>
  <AppLayout>
    <div class="w-full min-w-0 space-y-4 pb-8">
      <!-- 头部 -->
      <header class="page-header mb-0 rounded-3xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 text-xs text-gray-400 dark:text-dark-500">
              <button type="button" class="inline-flex items-center gap-1 hover:text-gray-600 dark:hover:text-dark-200" @click="goBack">
                <Icon name="arrowLeft" size="xs" />
                {{ t('admin.tickets.detail.back') }}
              </button>
              <span aria-hidden="true">/</span>
              <span class="font-mono">{{ detail?.ticket.ticket_no ?? `#${ticketId}` }}</span>
            </div>
            <h1 class="mt-1.5 break-words text-lg font-bold text-gray-900 dark:text-white" data-testid="ticket-detail-subject">
              {{ detail?.ticket.subject ?? t('admin.tickets.detail.loading') }}
            </h1>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <span v-if="detail" class="badge" :class="TICKET_STATUS_BADGE[detail.ticket.status]">
                {{ t(statusLabelKey(detail.ticket.status)) }}
              </span>
              <span v-if="detail" class="badge" :class="TICKET_PRIORITY_BADGE[detail.ticket.priority]">
                {{ t(priorityLabelKey(detail.ticket.priority)) }}
              </span>
              <span v-if="detail" class="badge badge-gray">{{ t(categoryLabelKey(detail.ticket.category)) }}</span>
              <span v-if="detail" class="text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.tickets.detail.messageCount', { count: detail.ticket.message_count }) }}
              </span>
            </div>
          </div>
          <button type="button" class="btn btn-secondary" :disabled="loading" @click="refresh">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            {{ t('common.refresh') }}
          </button>
        </div>
      </header>

      <div v-if="notFound" class="card !rounded-2xl">
        <EmptyState
          :title="t('admin.tickets.detail.notFound')"
          :description="t('admin.tickets.detail.notFoundDescription')"
          :action-text="t('admin.tickets.detail.back')"
          @action="goBack"
        />
      </div>

      <div v-else class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <!-- 消息流 + 回复 -->
        <section class="min-w-0 space-y-4">
          <div class="card !rounded-2xl space-y-3 p-4 sm:p-5">
            <div v-if="loading && !detail" class="flex items-center justify-center gap-2 py-10 text-sm text-gray-500 dark:text-dark-300">
              <Icon name="refresh" size="sm" class="animate-spin" />
              {{ t('common.loading') }}
            </div>

            <div v-else-if="!messages.length" class="py-8 text-center text-sm text-gray-400 dark:text-dark-500">
              {{ t('admin.tickets.detail.noMessages') }}
            </div>

            <div
              v-else
              ref="messageScrollRef"
              class="max-h-[60vh] space-y-3 overflow-y-auto pr-1"
              data-testid="ticket-message-scroll"
            >
              <ul class="space-y-3" data-testid="ticket-message-list">
              <li
                v-for="message in messages"
                :key="message.id"
                :class="[
                  'flex',
                  message.author_type === 'system' ? 'justify-center' : message.author_type === 'admin' ? 'justify-end' : 'justify-start',
                ]"
              >
                <!-- 系统消息 -->
                <div
                  v-if="message.author_type === 'system'"
                  class="max-w-xl rounded-full bg-gray-100 px-3 py-1 text-center text-xs text-gray-500 dark:bg-dark-700 dark:text-dark-300"
                >
                  {{ systemMessageText(message.body) }}
                </div>

                <!-- 普通消息 -->
                <div
                  v-else
                  class="max-w-[42rem] min-w-0 rounded-2xl border px-4 py-3"
                  :class="[
                    message.internal
                      ? 'border-amber-200 bg-amber-50 dark:border-amber-500/30 dark:bg-amber-500/10'
                      : message.author_type === 'admin'
                        ? 'border-primary-100 bg-primary-50 dark:border-primary-500/30 dark:bg-primary-500/10'
                        : 'border-gray-100 bg-gray-50 dark:border-dark-700 dark:bg-dark-900/60',
                  ]"
                >
                  <div class="flex flex-wrap items-center gap-2 text-xs">
                    <span class="font-medium text-gray-900 dark:text-white">{{ message.author_name || t('admin.tickets.detail.unknownAuthor') }}</span>
                    <span class="badge" :class="message.author_type === 'admin' ? 'badge-primary' : 'badge-gray'">
                      {{ t(`admin.tickets.authors.${message.author_type}`) }}
                    </span>
                    <span v-if="message.internal" class="badge badge-warning" data-testid="ticket-internal-badge">
                      {{ t('admin.tickets.detail.internal') }}
                    </span>
                    <span v-if="message.via === 'agent'" class="badge badge-purple">{{ t('admin.tickets.detail.viaAgent') }}</span>
                    <span class="ml-auto whitespace-nowrap text-gray-400 dark:text-dark-500">{{ formatTicketDateTime(message.created_at) }}</span>
                  </div>
                  <!-- 纯文本渲染，禁止 v-html -->
                  <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-dark-100" data-testid="ticket-message-body">{{ message.body }}</p>
                </div>
              </li>
              </ul>
            </div>
          </div>

          <!-- 回复框 -->
          <div class="card !rounded-2xl space-y-3 p-4 sm:p-5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.detail.replyTitle') }}</h2>
              <label class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-600 dark:text-dark-300">
                <input
                  v-model="replyInternal"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-amber-500 focus:ring-amber-500 dark:border-dark-600 dark:bg-dark-800"
                  data-testid="ticket-reply-internal"
                />
                {{ t('admin.tickets.detail.internalToggle') }}
              </label>
            </div>
            <p v-if="replyInternal" class="rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-500/10 dark:text-amber-300">
              {{ t('admin.tickets.detail.internalHint') }}
            </p>
            <textarea
              v-model="replyBody"
              rows="4"
              class="input w-full resize-y"
              data-testid="ticket-reply-body"
              :placeholder="replyInternal ? t('admin.tickets.detail.internalPlaceholder') : t('admin.tickets.detail.replyPlaceholder')"
              @keydown.ctrl.enter.prevent="sendReply"
              @keydown.meta.enter.prevent="sendReply"
            ></textarea>
            <div class="flex flex-wrap items-center justify-between gap-2">
              <span class="text-xs text-gray-400 dark:text-dark-500" data-testid="ticket-reply-counter">
                {{ replyCharCount }}<template v-if="maxBodyLength > 0"> / {{ maxBodyLength }}</template>
              </span>
              <span class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.replyShortcut') }}</span>
              <button
                type="button"
                class="btn btn-primary"
                :disabled="sending || !replyBody.trim()"
                data-testid="ticket-reply-send"
                @click="sendReply"
              >
                <Icon name="arrowRight" size="sm" />
                {{ sending ? t('common.saving') : t('admin.tickets.detail.send') }}
              </button>
            </div>
          </div>
        </section>

        <!-- 右侧信息栏 -->
        <aside class="min-w-0 space-y-4">
          <div class="card !rounded-2xl space-y-4 p-4 sm:p-5">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.detail.infoTitle') }}</h2>

            <div class="space-y-1">
              <p class="text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.user') }}</p>
              <p class="break-all text-sm text-gray-900 dark:text-white" data-testid="ticket-detail-user-email">{{ detail?.ticket.user_email || '-' }}</p>
              <p v-if="detail?.ticket.user_username" class="text-xs text-gray-500 dark:text-dark-400">{{ detail.ticket.user_username }}</p>
            </div>

            <div class="space-y-1">
              <p class="text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.status') }}</p>
              <Select
                v-model="statusDraft"
                :options="statusOptions"
                :disabled="updating"
                :aria-label="t('admin.tickets.detail.status')"
                data-testid="ticket-status-select"
                @change="saveStatus"
              />
            </div>

            <div class="space-y-1">
              <p class="text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.priority') }}</p>
              <Select
                v-model="priorityDraft"
                :options="priorityOptions"
                :disabled="updating"
                :aria-label="t('admin.tickets.detail.priority')"
                data-testid="ticket-priority-select"
                @change="savePriority"
              />
            </div>

            <div class="space-y-1">
              <p class="text-xs font-medium uppercase tracking-wide text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.assignee') }}</p>
              <Select
                v-model="assigneeDraft"
                :options="assigneeOptions"
                :disabled="updating || assigneesLoading"
                :aria-label="t('admin.tickets.detail.assignee')"
                data-testid="ticket-assignee-select"
                @change="saveAssignee"
              />
            </div>

            <dl class="space-y-2 border-t border-gray-100 pt-3 text-xs dark:border-dark-700">
              <div class="flex items-center justify-between gap-2">
                <dt class="text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.createdAt') }}</dt>
                <dd class="text-gray-700 dark:text-dark-200">{{ formatTicketDateTime(detail?.ticket.created_at) }}</dd>
              </div>
              <div class="flex items-center justify-between gap-2">
                <dt class="text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.lastMessageAt') }}</dt>
                <dd class="text-gray-700 dark:text-dark-200">{{ formatTicketDateTime(detail?.ticket.last_message_at) }}</dd>
              </div>
              <div class="flex items-center justify-between gap-2">
                <dt class="text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.closedAt') }}</dt>
                <dd class="text-gray-700 dark:text-dark-200">{{ formatTicketDateTime(detail?.ticket.closed_at) }}</dd>
              </div>
            </dl>
          </div>

          <p class="text-xs leading-5 text-gray-400 dark:text-dark-500">{{ t('admin.tickets.detail.pollHint') }}</p>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type {
  AdminTicketDetail,
  TicketAssignee,
  TicketMessage,
  TicketPriority,
  TicketStatus,
} from '@/types/tickets'
import { TICKET_PRIORITIES, TICKET_STATUSES } from '@/types/tickets'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  TICKET_PRIORITY_BADGE,
  TICKET_STATUS_BADGE,
  categoryLabelKey,
  formatTicketDateTime,
  parseSystemMessage,
  priorityLabelKey,
  statusLabelKey,
} from '@/components/admin/tickets/format'

const POLL_INTERVAL_MS = 15000

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()

// 路由参数必须响应式：SPA 内 /admin/tickets/1 → /admin/tickets/2 会复用同一个组件实例，
// 若在 setup 时只算一次，后续的轮询、回复和状态修改都会打到旧工单。
const ticketId = computed(() => {
  const raw = Array.isArray(route.params.id) ? route.params.id[0] : route.params.id
  const parsed = Number(raw)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 0
})
const detail = ref<AdminTicketDetail | null>(null)
const messages = ref<TicketMessage[]>([])
const assignees = ref<TicketAssignee[]>([])
const loading = ref(false)
const notFound = ref(false)
const sending = ref(false)
const updating = ref(false)
const assigneesLoading = ref(false)
const replyBody = ref('')
const replyInternal = ref(false)
const statusDraft = ref<TicketStatus | ''>('')
const priorityDraft = ref<TicketPriority | ''>('')
const assigneeDraft = ref('')
const messageScrollRef = ref<HTMLElement | null>(null)
/** 回复正文上限（来自全局设置 max_body_length）；0 表示未知，不做限制。 */
const maxBodyLength = ref(0)
let firstLoadDone = false

let pollTimer: ReturnType<typeof setInterval> | null = null
let requestSequence = 0

const statusOptions = computed(() =>
  TICKET_STATUSES.map((status) => ({ value: status, label: t(statusLabelKey(status)) })),
)
const priorityOptions = computed(() =>
  TICKET_PRIORITIES.map((priority) => ({ value: priority, label: t(priorityLabelKey(priority)) })),
)
const assigneeOptions = computed(() => [
  { value: '0', label: t('admin.tickets.unassigned') },
  ...assignees.value.map((assignee) => ({
    value: String(assignee.id),
    label: assignee.username ? `${assignee.username} (${assignee.email})` : assignee.email,
  })),
])

function syncDrafts() {
  const ticket = detail.value?.ticket
  if (!ticket) return
  statusDraft.value = ticket.status
  priorityDraft.value = ticket.priority
  assigneeDraft.value = ticket.assignee_id ? String(ticket.assignee_id) : '0'
}

// 消息容器滚动位置：只有用户本来就在底部附近（或首次加载 / 自己刚发完消息）才自动跟随，
// 避免 15s 轮询把正在往上翻看历史的管理员拽回底部。
const AUTO_SCROLL_THRESHOLD_PX = 80

function isMessageListNearBottom(): boolean {
  const el = messageScrollRef.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight <= AUTO_SCROLL_THRESHOLD_PX
}

function scrollMessagesToBottom(smooth = false) {
  const el = messageScrollRef.value
  if (!el) return
  if (typeof el.scrollTo === 'function') {
    el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
  } else {
    el.scrollTop = el.scrollHeight
  }
}

function applyDetail(value: AdminTicketDetail, follow: boolean) {
  const isFirstLoad = !firstLoadDone
  detail.value = value
  messages.value = Array.isArray(value.messages) ? value.messages : []
  syncDrafts()
  firstLoadDone = true
  void nextTick(() => {
    if (follow) scrollMessagesToBottom(!isFirstLoad)
  })
}

async function loadDetail(options: { silent?: boolean; follow?: boolean } = {}) {
  const id = ticketId.value
  if (!id) {
    notFound.value = true
    return
  }
  const sequence = ++requestSequence
  // 在请求发出前先读一次滚动位置：轮询回来后 DOM 已变，旧位置无法再判断
  const follow = options.follow ?? (!firstLoadDone || isMessageListNearBottom())
  if (!options.silent) loading.value = true
  try {
    const result = await adminAPI.tickets.getTicket(id)
    // 序号变化（新请求/切换工单）或路由已切到别的工单 → 丢弃旧响应
    if (sequence !== requestSequence || ticketId.value !== id) return
    applyDetail(result, follow)
    notFound.value = false
  } catch (error) {
    if (sequence !== requestSequence || ticketId.value !== id) return
    // 静默轮询失败（网络抖动）不把页面切成「不存在」，保留已渲染的内容
    if (options.silent) return
    notFound.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.detail.loadFailed')))
  } finally {
    if (sequence === requestSequence && ticketId.value === id) loading.value = false
  }
}

async function loadAssignees() {
  assigneesLoading.value = true
  try {
    assignees.value = await adminAPI.tickets.listAssignees()
  } catch {
    assignees.value = []
  } finally {
    assigneesLoading.value = false
  }
}

function refresh() {
  void loadDetail()
}

function systemMessageText(body: string): string {
  const parsed = parseSystemMessage(body)
  if (parsed.kind === 'status_changed' && parsed.status) {
    return t('admin.tickets.system.statusChanged', { status: t(statusLabelKey(parsed.status)) })
  }
  return parsed.raw
}

async function sendReply() {
  const id = ticketId.value
  const body = replyBody.value.trim()
  if (!id || !body || sending.value) return
  const internal = replyInternal.value
  sending.value = true
  try {
    await adminAPI.tickets.addMessage(id, { body, internal })
    // 请求期间切换了工单：丢弃旧结果，不要清空/覆盖新工单的草稿
    if (ticketId.value !== id) return
    replyBody.value = ''
    replyInternal.value = false
    appStore.showSuccess(
      internal ? t('admin.tickets.detail.internalSent') : t('admin.tickets.detail.replySent'),
    )
    await loadDetail({ silent: true, follow: true })
  } catch (error) {
    if (ticketId.value !== id) return
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.detail.sendFailed')))
  } finally {
    if (ticketId.value === id) sending.value = false
  }
}

async function patchTicket(payload: { status?: TicketStatus; priority?: TicketPriority; assignee_id?: number }) {
  const id = ticketId.value
  if (!id || updating.value) return
  updating.value = true
  const previous = detail.value
  // 乐观更新：失败时回滚到服务端已知状态
  if (detail.value) {
    detail.value = {
      ...detail.value,
      ticket: {
        ...detail.value.ticket,
        ...(payload.status ? { status: payload.status } : {}),
        ...(payload.priority ? { priority: payload.priority } : {}),
        ...(payload.assignee_id !== undefined ? { assignee_id: payload.assignee_id || null } : {}),
      },
    }
  }
  try {
    await adminAPI.tickets.updateTicket(id, payload)
    if (ticketId.value !== id) return
    appStore.showSuccess(t('admin.tickets.detail.updateSaved'))
    await loadDetail({ silent: true })
  } catch (error) {
    if (ticketId.value !== id) return
    if (previous) detail.value = previous
    syncDrafts()
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.detail.updateFailed')))
  } finally {
    if (ticketId.value === id) updating.value = false
  }
}

function saveStatus() {
  if (!statusDraft.value) return
  void patchTicket({ status: statusDraft.value })
}

function savePriority() {
  if (!priorityDraft.value) return
  void patchTicket({ priority: priorityDraft.value })
}

function saveAssignee() {
  void patchTicket({ assignee_id: Number(assigneeDraft.value || '0') })
}

function goBack() {
  void router.push({ name: 'AdminTickets' })
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(() => {
    // 页面不可见时暂停轮询，回到前台立即补一次
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
    if (sending.value || updating.value) return
    void loadDetail({ silent: true })
  }, POLL_INTERVAL_MS)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function handleVisibilityChange() {
  if (typeof document === 'undefined') return
  if (document.visibilityState === 'visible') void loadDetail({ silent: true })
}

// 后端按 rune 校验正文长度：前端用 Unicode code point 计数/截断，emoji 不会被算成 2 个，也不会截出半个代理对。
const replyCharCount = computed(() => Array.from(replyBody.value).length)

watch(replyBody, (value) => {
  if (maxBodyLength.value <= 0) return
  const chars = Array.from(value)
  if (chars.length > maxBodyLength.value) {
    replyBody.value = chars.slice(0, maxBodyLength.value).join('')
  }
})

async function loadReplyLimit() {
  try {
    const settings = await adminAPI.tickets.getSettings()
    const max = Number(settings?.max_body_length)
    if (Number.isInteger(max) && max > 0) maxBodyLength.value = max
  } catch {
    // 拿不到上限时不限制，交给后端校验
  }
}

// 路由参数变化：清空当前工单状态、作废旧请求，再按新 ID 重新加载
watch(ticketId, () => {
  requestSequence += 1
  detail.value = null
  messages.value = []
  notFound.value = false
  firstLoadDone = false
  replyBody.value = ''
  replyInternal.value = false
  statusDraft.value = ''
  priorityDraft.value = ''
  assigneeDraft.value = ''
  sending.value = false
  updating.value = false
  void loadDetail()
})

onMounted(() => {
  void loadDetail()
  void loadAssignees()
  void loadReplyLimit()
  startPolling()
  if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  requestSequence += 1
  stopPolling()
  if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>
