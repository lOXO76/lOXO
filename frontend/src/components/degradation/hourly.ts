import type { DegradationHourBucket, DegradationOverview } from '@/api/degradation'

/**
 * 小时条的等级：
 * - healthy  该小时判定题全部通过
 * - warning  有未通过，但通过率仍不低于阈值
 * - degraded 通过率低于阈值
 * - error    只有请求失败（无法判定）
 * - empty    该小时没有检测
 */
export type HourlyLevel = 'healthy' | 'warning' | 'degraded' | 'error' | 'empty'

export const HOURLY_LEVELS: HourlyLevel[] = ['healthy', 'warning', 'degraded', 'error', 'empty']

/** 与渠道状态页一致的「高度 + 颜色」双编码：越高越好，灰色表示无法判定或无数据。 */
export const HOURLY_LEVEL_STYLE: Record<HourlyLevel, { bar: string; height: number }> = {
  healthy: { bar: 'bg-emerald-500', height: 100 },
  warning: { bar: 'bg-amber-500', height: 65 },
  degraded: { bar: 'bg-red-500', height: 35 },
  error: { bar: 'bg-gray-400 dark:bg-gray-500', height: 35 },
  empty: { bar: 'bg-gray-200 dark:bg-dark-600', height: 15 },
}

export interface HourlyWindow {
  /** 第一个桶起点（毫秒时间戳） */
  startMs: number
  count: number
  stepMs: number
}

export interface HourlySlot {
  startMs: number
  endMs: number
  bucket?: DegradationHourBucket
  level: HourlyLevel
}

const MAX_BUCKETS = 24 * 31

/**
 * 解析 overview 中的小时桶窗口；旧版后端没有 bucket_* 字段时，
 * 按 generated_at 所在整点往前推 window_hours 个桶。
 */
export function resolveHourlyWindow(
  overview: Pick<DegradationOverview, 'window_hours' | 'generated_at' | 'bucket_start' | 'bucket_count' | 'bucket_seconds'>,
): HourlyWindow | null {
  const stepSeconds = overview.bucket_seconds && overview.bucket_seconds > 0 ? overview.bucket_seconds : 3600
  const stepMs = stepSeconds * 1000
  const rawCount = overview.bucket_count && overview.bucket_count > 0 ? overview.bucket_count : overview.window_hours
  const count = Math.min(MAX_BUCKETS, Math.max(1, Math.floor(rawCount || 24)))

  const explicitStart = overview.bucket_start ? Date.parse(overview.bucket_start) : NaN
  if (Number.isFinite(explicitStart) && explicitStart > 0) {
    return { startMs: explicitStart, count, stepMs }
  }
  const generatedAt = Date.parse(overview.generated_at)
  if (!Number.isFinite(generatedAt)) return null
  const currentBucket = Math.floor(generatedAt / stepMs) * stepMs
  return { startMs: currentBucket - (count - 1) * stepMs, count, stepMs }
}

export function classifyHourBucket(bucket: DegradationHourBucket | undefined, threshold: number): HourlyLevel {
  if (!bucket) return 'empty'
  const judged = (bucket.passed || 0) + (bucket.failed || 0)
  if (judged <= 0) return (bucket.error || 0) > 0 ? 'error' : 'empty'
  if (bucket.pass_rate < threshold) return 'degraded'
  if ((bucket.failed || 0) > 0) return 'warning'
  return 'healthy'
}

/** 把稀疏的小时桶对齐到完整窗口（最左最旧、最右为当前小时）。 */
export function buildHourlySlots(
  buckets: DegradationHourBucket[] | null | undefined,
  window: HourlyWindow,
  threshold: number,
): HourlySlot[] {
  const byIndex = new Map<number, DegradationHourBucket>()
  for (const bucket of buckets ?? []) {
    const at = Date.parse(bucket.start)
    if (!Number.isFinite(at)) continue
    const index = Math.round((at - window.startMs) / window.stepMs)
    if (index >= 0 && index < window.count) byIndex.set(index, bucket)
  }
  const slots: HourlySlot[] = []
  for (let index = 0; index < window.count; index += 1) {
    const startMs = window.startMs + index * window.stepMs
    const bucket = byIndex.get(index)
    slots.push({ startMs, endMs: startMs + window.stepMs, bucket, level: classifyHourBucket(bucket, threshold) })
  }
  return slots
}
