import type { UsageLog } from '@/types'
import { BILLING_MODE_VIDEO } from '@/utils/billingMode'

type VideoUsageRow = Pick<UsageLog, 'billing_mode' | 'video_count' | 'video_resolution' | 'video_duration_seconds'>

/** 按秒计费的视频用量行（按次计费的视频任务 billing_mode 为 per_request，不适用）。 */
export const isVideoPerSecondUsage = (row: VideoUsageRow | null | undefined): boolean =>
  row?.billing_mode === BILLING_MODE_VIDEO

/** 计费单位数 = 视频数量 × 单个时长（秒）；缺少记录时按 1 处理，避免除零。 */
export const videoBillingUnits = (row: VideoUsageRow | null | undefined): number => {
  const count = Math.max(1, row?.video_count ?? 1)
  const duration = Math.max(1, row?.video_duration_seconds ?? 1)
  return count * duration
}

/**
 * 每秒单价 = 总价 ÷ (视频数量 × 时长)。
 * 历史记录可能缺少时长字段，此时退回展示总价（units 为 1）。
 */
export const videoUnitPricePerSecond = (
  row: (VideoUsageRow & Pick<UsageLog, 'total_cost'>) | null | undefined,
): number => {
  const total = row?.total_cost ?? 0
  if (!Number.isFinite(total)) return 0
  return total / videoBillingUnits(row)
}

/** 视频分辨率展示（未记录时返回空串，由调用方决定占位）。 */
export const formatVideoResolution = (row: VideoUsageRow | null | undefined): string =>
  row?.video_resolution?.trim() ?? ''
