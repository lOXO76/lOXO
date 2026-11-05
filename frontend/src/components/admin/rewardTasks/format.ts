/**
 * 任务中心管理页共用的格式化工具。
 * 奖励额度与站内余额一致用「$」；不足 $1 的小额最多保留 4 位小数，避免显示成 $0.00。
 */

const preciseFormatter = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
})
const compactFormatter = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

export function formatCredit(value: number | null | undefined): string {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return Math.abs(amount) >= 1 || amount === 0 ? compactFormatter.format(amount) : preciseFormatter.format(amount)
}

/** 对方站点充值金额单位为元，保留 2 位小数。 */
export function formatYuan(value: number | null | undefined): string {
  const amount = typeof value === 'number' && Number.isFinite(value) ? value : 0
  return amount.toFixed(2)
}
