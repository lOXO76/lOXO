/**
 * 管理端侧边栏「工单」入口的待处理数角标。
 *
 * - 每 60 秒轮询一次 `/admin/tickets/stats`，取 `pending_admin`。
 * - 页面不可见时暂停轮询，回到前台立即补一次。
 * - 非管理员（enabled() 为 false）不发起请求。
 * - 任何失败都静默处理：角标只是辅助信息，不能影响导航。
 */
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { adminAPI } from '@/api/admin'

const POLL_INTERVAL_MS = 60000

export function useTicketAdminBadge(enabled: () => boolean) {
  const pendingCount = ref(0)
  let timer: ReturnType<typeof setInterval> | null = null
  let inFlight = false
  let disposed = false

  async function refresh() {
    if (disposed || inFlight || !enabled()) return
    inFlight = true
    try {
      const stats = await adminAPI.tickets.getStats()
      if (!disposed) pendingCount.value = Number(stats?.pending_admin) || 0
    } catch {
      // 静默失败：保留上一次的数字
    } finally {
      inFlight = false
    }
  }

  function handleVisibilityChange() {
    if (typeof document === 'undefined') return
    if (document.visibilityState === 'visible') void refresh()
  }

  onMounted(() => {
    void refresh()
    timer = setInterval(() => {
      if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
      void refresh()
    }, POLL_INTERVAL_MS)
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
  })

  onBeforeUnmount(() => {
    disposed = true
    if (timer) {
      clearInterval(timer)
      timer = null
    }
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
  })

  return pendingCount
}
