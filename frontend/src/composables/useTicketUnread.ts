/**
 * 工单未读数（用户端侧边栏角标 + 页面内提示）。
 *
 * 单例式实现：模块级保存计数和一个 60 秒轮询定时器，多个组件通过
 * `useTicketUnread()` 共享同一份状态，引用计数归零才停掉定时器。
 *
 * 轮询规则（对齐 SPEC 第 8 节）：
 * - 工单系统开关关闭时不请求，计数归零；
 * - 页面不可见（`document.hidden`）时暂停，重新可见时立即刷新一次。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch, type ComputedRef, type Ref } from 'vue'
import { getTicketUnreadCount } from '@/api/tickets'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'

export const TICKET_UNREAD_POLL_INTERVAL_MS = 60_000

const unreadCount = ref(0)
const lastRefreshAt = ref<number | null>(null)

let pollTimer: ReturnType<typeof setInterval> | undefined
let subscribers = 0
let refreshing: Promise<void> | null = null

type AppStore = ReturnType<typeof useAppStore>

/**
 * 取 app store；没有 active pinia 时（组件单测、极早期调用）返回 null 而不是抛出，
 * 让角标逻辑在任何环境都能安全降级为「不轮询」。
 */
function tryUseAppStore(): AppStore | null {
  try {
    return useAppStore()
  } catch {
    return null
  }
}

function isEnabled(): boolean {
  const appStore = tryUseAppStore()
  // 公开设置尚未加载时不发请求，等 watch(enabled) 在加载完成后补一次。
  if (!appStore?.cachedPublicSettings) return false
  try {
    return resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.tickets)
  } catch {
    return false
  }
}

function clearTimer(): void {
  if (pollTimer !== undefined) {
    clearInterval(pollTimer)
    pollTimer = undefined
  }
}

function startTimer(): void {
  if (pollTimer !== undefined || subscribers === 0) return
  if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
  pollTimer = setInterval(() => {
    void refreshTicketUnreadCount()
  }, TICKET_UNREAD_POLL_INTERVAL_MS)
}

/** 立即刷新未读数（失败静默，角标不该打断正常操作）。 */
export async function refreshTicketUnreadCount(): Promise<void> {
  if (refreshing) return refreshing
  if (!isEnabled()) {
    unreadCount.value = 0
    return
  }
  refreshing = (async () => {
    try {
      unreadCount.value = await getTicketUnreadCount()
      lastRefreshAt.value = Date.now()
    } catch {
      // 静默失败：保留上一次的值，等下一轮轮询。
    } finally {
      refreshing = null
    }
  })()
  return refreshing
}

/** 就地调整计数（打开详情、发消息后立即反映，无需等下一轮轮询）。 */
export function setTicketUnreadCount(count: number): void {
  unreadCount.value = Math.max(0, Math.trunc(count))
}

/** 退出登录时清零，避免下一个账号看到上一个账号的角标。 */
export function resetTicketUnreadCount(): void {
  unreadCount.value = 0
  lastRefreshAt.value = null
}

function handleVisibilityChange(): void {
  if (subscribers === 0) return
  if (document.visibilityState === 'visible') {
    startTimer()
    void refreshTicketUnreadCount()
  } else {
    clearTimer()
  }
}

export interface UseTicketUnreadResult {
  /** 当前未读工单数（只读）。 */
  unreadCount: Readonly<Ref<number>>
  /** 是否应展示角标。 */
  hasUnread: ComputedRef<boolean>
  /** 是否已开启工单系统。 */
  enabled: ComputedRef<boolean>
  refresh: () => Promise<void>
  markAllRead: () => void
}

/**
 * 在组件内使用未读数。组件挂载时自动加入轮询，卸载时自动退出。
 * 传入 `{ autoStart: false }` 可只读取状态不启动轮询（例如详情页）。
 */
export function useTicketUnread(options: { autoStart?: boolean } = {}): UseTicketUnreadResult {
  const appStore = tryUseAppStore()
  const autoStart = options.autoStart !== false

  const enabled = computed(() => {
    if (!appStore) return false
    return resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.tickets)
  })

  if (autoStart) {
    onMounted(() => {
      subscribers += 1
      startTimer()
      void refreshTicketUnreadCount()
    })
    onBeforeUnmount(() => {
      subscribers = Math.max(0, subscribers - 1)
      // 最后一个实例卸载（例如退出登录）时清定时器并归零，避免下一个账号看到残留角标。
      if (subscribers === 0) {
        clearTimer()
        resetTicketUnreadCount()
      }
    })

    if (typeof document !== 'undefined') {
      // 同一个函数引用注册多次是幂等的（浏览器按引用去重），卸载时统一移除。
      document.addEventListener('visibilitychange', handleVisibilityChange)
      onBeforeUnmount(() => {
        if (subscribers === 0) document.removeEventListener('visibilitychange', handleVisibilityChange)
      })
    }
  }

  // 公开设置是异步加载的：开关从「未加载/关闭」变成「开启」时要补一次请求。
  if (appStore) {
    watch(enabled, (value) => {
      if (!value) {
        clearTimer()
        unreadCount.value = 0
        return
      }
      if (autoStart && subscribers > 0) {
        startTimer()
        void refreshTicketUnreadCount()
      }
    })
  }

  return {
    unreadCount: unreadCount as Readonly<Ref<number>>,
    hasUnread: computed(() => unreadCount.value > 0),
    enabled,
    refresh: refreshTicketUnreadCount,
    markAllRead: () => setTicketUnreadCount(0),
  }
}
