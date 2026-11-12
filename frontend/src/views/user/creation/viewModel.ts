import type {
  CreationGroup,
  CreationKind,
  CreationModel,
  CreationModelPricing,
  CreationProtocol,
  CreationVideoCapability,
  CreationVideoProtocol,
} from '@/features/creation/types'
import type { AsyncTask } from '@/api/asyncTasks'
import type { CreationHistoryItem, CreationSubmission } from './types'

const SELECTION_PREFIX = 'sub2api.creation.selection.'
const HISTORY_PREFIX = 'sub2api.creation.history.'
const HISTORY_LIMIT = 20

export interface CreationSessionRecord {
  id: string
  kind: CreationKind
  model: string
  protocol: CreationProtocol
  prompt: string
  createdAt: number
  /** 异步任务的任务 ID，用来与服务端历史合并；同步生成的记录没有。 */
  taskId?: string
  assets: Array<{
    kind: CreationKind
    url: string
    mimeType?: string
  }>
}

/** 单价最多保留 6 位小数（图片单价常见 $0.035 这类三位小数），至少 2 位。 */
export function formatCreationPrice(price: number): string {
  return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 6 }).format(price)
}

/** 把模型实付价展开成展示条目；不分档时 label 为空。 */
export function creationPriceEntries(pricing: CreationModelPricing | null | undefined): Array<{ label: string; price: string }> {
  if (!pricing) return []
  if (pricing.tiers.length) return pricing.tiers.map(tier => ({ label: tier.label, price: formatCreationPrice(tier.price) }))
  return typeof pricing.price === 'number' ? [{ label: '', price: formatCreationPrice(pricing.price) }] : []
}

/** 模型实付价的区间摘要（不含单位）：单一价格返回 "$0.04"，分档返回 "$0.04 – $0.12"，无定价返回空串。 */
export function creationPriceRange(pricing: CreationModelPricing | null | undefined): string {
  if (!pricing) return ''
  const prices = pricing.tiers.length
    ? pricing.tiers.map(tier => tier.price)
    : typeof pricing.price === 'number' ? [pricing.price] : []
  if (!prices.length) return ''
  const min = Math.min(...prices)
  const max = Math.max(...prices)
  return min === max ? formatCreationPrice(min) : `${formatCreationPrice(min)} – ${formatCreationPrice(max)}`
}

/** 生图尺寸对应的计费档位，与后端 ClassifyImageBillingTier 一致；auto 等无法判断的值返回 null。 */
export function creationImageTier(size: string): '1K' | '2K' | '4K' | null {
  const value = size.trim().toLowerCase()
  if (value === '1k') return '1K'
  if (value === '2k') return '2K'
  if (value === '4k') return '4K'
  const match = /^(\d+)\s*x\s*(\d+)$/.exec(value)
  if (!match) return null
  const width = Number(match[1])
  const height = Number(match[2])
  if (width <= 0 || height <= 0) return null
  const edge = Math.max(width, height)
  return edge <= 1024 ? '1K' : edge <= 2048 ? '2K' : '4K'
}

/** 请求里没有指定分辨率 / 时长时，后端计费采用的默认值（与各协议的请求解析一致）。 */
const BILLING_DEFAULT_RESOLUTION: Record<CreationVideoProtocol, string> = {
  seedance: '720p', gemini: '720p', minimax_v1: '768p', minimax_v2: '768p', grok: '480p',
}
const BILLING_DEFAULT_SECONDS: Record<CreationVideoProtocol, number> = {
  seedance: 5, gemini: 8, minimax_v1: 6, minimax_v2: 6, grok: 8,
}
/** 尺寸为 auto 时后端按 2K 档兜底，实际以出图尺寸为准。 */
const BILLING_DEFAULT_IMAGE_TIER = '2K'

export type CreationCostInput =
  | { kind: 'image'; size: string; count: number }
  | { kind: 'video'; protocol: CreationVideoProtocol; resolution: string; duration?: number }

export interface CreationCostEstimate {
  /** 预计费用（美元）。 */
  amount: number
  unit: CreationModelPricing['unit']
  unitPrice: number
  /** 张数或秒数；按次计费为 1。 */
  quantity: number
  /** 命中的档位（1K / 720p …）；不分档时为空串。 */
  tier: string
  /** 档位或时长取了默认值（尺寸自动、未选分辨率、未填时长），金额只是近似。 */
  assumed: boolean
}

function validPrice(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

/**
 * 按模型实付单价估算本次提交的费用；没有可用单价时返回 null。
 * 图片 = 档位单价 × 张数；视频 = 分辨率单价 × 时长；按次计费取固定价。
 * 只是提交前的预估：高峰时段倍率、上游实际出图尺寸等以最终扣费为准。
 */
export function estimateCreationCost(
  pricing: CreationModelPricing | null | undefined,
  input: CreationCostInput,
): CreationCostEstimate | null {
  if (!pricing) return null
  const tiered = pricing.tiers.length > 0
  const priceOf = (label: string): number | null => {
    const price = tiered
      ? pricing.tiers.find(tier => tier.label.trim().toLowerCase() === label.toLowerCase())?.price
      : pricing.price
    return validPrice(price) ? price : null
  }

  if (pricing.unit === 'request') {
    const price = validPrice(pricing.price) ? pricing.price : pricing.tiers[0]?.price
    return validPrice(price) ? { amount: price, unit: 'request', unitPrice: price, quantity: 1, tier: '', assumed: false } : null
  }

  if (input.kind === 'image') {
    if (pricing.unit !== 'image') return null
    const classified = creationImageTier(input.size)
    const tier = classified ?? BILLING_DEFAULT_IMAGE_TIER
    const unitPrice = priceOf(tier)
    if (unitPrice === null) return null
    const count = Number.isFinite(input.count) ? Math.max(1, Math.floor(input.count)) : 1
    return { amount: unitPrice * count, unit: 'image', unitPrice, quantity: count, tier: tiered ? tier : '', assumed: tiered && classified === null }
  }

  if (pricing.unit !== 'second') return null
  const requested = input.resolution.trim().toLowerCase()
  const resolution = requested || BILLING_DEFAULT_RESOLUTION[input.protocol]
  const unitPrice = priceOf(resolution)
  if (unitPrice === null) return null
  const specified = typeof input.duration === 'number' && Number.isFinite(input.duration) && input.duration > 0
  const seconds = specified ? Math.ceil(input.duration as number) : BILLING_DEFAULT_SECONDS[input.protocol]
  return {
    amount: unitPrice * seconds, unit: 'second', unitPrice, quantity: seconds,
    tier: tiered ? resolution : '', assumed: !specified || (tiered && !requested),
  }
}

export function filterCreationGroups(
  groups: CreationGroup[] | null | undefined,
  kind: CreationKind,
): CreationGroup[] {
  return groups?.filter(group => Boolean(group[kind]?.protocols.length)) ?? []
}

export function filterCreationModels(
  group: CreationGroup | null | undefined,
  kind: CreationKind,
  protocol: CreationProtocol,
): CreationModel[] {
  const capability = group?.[kind]
  if (!capability?.protocols.some(supported => supported === protocol)) return []

  const seen = new Set<string>()
  const models: CreationModel[] = []
  for (const model of capability.models) {
    if (model.protocols.length > 0 && !model.protocols.includes(protocol)) continue
    if (seen.has(model.id)) continue
    seen.add(model.id)
    models.push(model)
  }
  return models
}

/** 模型在分组内可走的生视频协议；模型没有声明协议时，分组支持的协议都可用。 */
function videoProtocolsOf(group: CreationGroup, modelId: string): CreationVideoProtocol[] {
  const capability = group.video
  if (!capability) return []
  const entries = capability.models.filter(model => model.id === modelId)
  if (!entries.length) return []
  return capability.protocols.filter(protocol =>
    entries.some(model => model.protocols.length === 0 || model.protocols.includes(protocol)))
}

/** 分组内全部可用的生视频模型（去重）。不按协议分开：用户只选模型，协议由模型决定。 */
export function creationVideoModels(group: CreationGroup | null | undefined): CreationModel[] {
  if (!group?.video) return []
  const seen = new Set<string>()
  const models: CreationModel[] = []
  for (const model of group.video.models) {
    if (seen.has(model.id) || videoProtocolsOf(group, model.id).length === 0) continue
    seen.add(model.id)
    models.push(model)
  }
  return models
}

/** 多个协议都可用时的兜底顺序。 */
const VIDEO_PROTOCOL_ORDER: CreationVideoProtocol[] = ['seedance', 'gemini', 'grok', 'minimax_v1', 'minimax_v2']

/** 按模型名判断它原生属于哪类接口，与后端 creationVideoModelProtocols 的分类一致。 */
function nativeVideoProtocols(modelId: string): CreationVideoProtocol[] {
  const name = modelId.trim().toLowerCase().replace(/^(?:grok|xai)\//, '')
  if (name.startsWith('veo')) return ['gemini']
  if (name.includes('seedance')) return ['seedance']
  // H3 系列只有 v2 接口支持；其余 MiniMax 模型用原生的 v1 接口。
  if (name.startsWith('minimax-h3')) return ['minimax_v2', 'minimax_v1']
  if (name.startsWith('minimax') || name.startsWith('hailuo')) return ['minimax_v1', 'minimax_v2']
  if (name.startsWith('grok-') || name.startsWith('imagine')) return ['grok']
  return []
}

/** 所选生视频模型应走的协议；模型不在分组内或没有可用协议时返回 null。 */
export function resolveCreationVideoProtocol(
  group: CreationGroup | null | undefined,
  modelId: string,
): CreationVideoProtocol | null {
  if (!group) return null
  const available = videoProtocolsOf(group, modelId)
  if (available.length <= 1) return available[0] ?? null
  return [...nativeVideoProtocols(modelId), ...VIDEO_PROTOCOL_ORDER].find(protocol => available.includes(protocol))
    ?? available[0]
    ?? null
}

export function creationSelectionKey(userId: number | string, kind: CreationKind): string {
  return `${SELECTION_PREFIX}${userId}.${kind}`
}

export function restoreSelectedGroupId(
  storage: Pick<Storage, 'getItem'>,
  userId: number | string,
  kind: CreationKind,
  groups: CreationGroup[],
): number | null {
  try {
    const stored = storage.getItem(creationSelectionKey(userId, kind))
    if (stored === null || !stored.trim()) return null
    const groupId = Number(stored)
    if (!Number.isSafeInteger(groupId)) return null
    return groups.some(group => group.id === groupId) ? groupId : null
  } catch {
    return null
  }
}

export function saveSelectedGroupId(
  storage: Pick<Storage, 'setItem'>,
  userId: number | string,
  kind: CreationKind,
  groupId: number,
): void {
  try {
    storage.setItem(creationSelectionKey(userId, kind), String(groupId))
  } catch {
    return
  }
}

function configuredDuration(value: number | null | undefined): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function hasConfiguredDuration(capability?: CreationVideoCapability | null): boolean {
  return configuredDuration(capability?.min_duration) !== null
    || configuredDuration(capability?.max_duration) !== null
}

function isSeedance25(model: string): boolean {
  return /2[._-]5/i.test(model)
}

function protocolDurationRange(
  protocol: CreationVideoProtocol,
  model: string,
): { min: number | null; max: number | null } {
  switch (protocol) {
    case 'seedance':
      if (isSeedance25(model)) return { min: 4, max: 30 }
      if (/2[._-]0/i.test(model)) return { min: 4, max: 15 }
      if (/1[._-]5/i.test(model)) return { min: 4, max: 12 }
      if (/1[._-]0/i.test(model)) return { min: 2, max: 12 }
      return { min: null, max: null }
    case 'gemini':
      if (/3[._-]1/i.test(model)) return { min: 4, max: 8 }
      if (/3[._-]0/i.test(model)) return { min: 8, max: 8 }
      if (/2[._-]0/i.test(model)) return { min: 5, max: 8 }
      return { min: null, max: null }
    case 'minimax_v1':
      return { min: 6, max: 10 }
    case 'minimax_v2':
      if (/h3[-_ ]?max/i.test(model)) return { min: 5, max: 15 }
      if (/h3/i.test(model)) return { min: 4, max: 15 }
      return { min: null, max: null }
    case 'grok':
      return { min: null, max: null }
  }
}

export function videoDurationRange(
  protocol: CreationVideoProtocol,
  model: string,
  capability?: CreationVideoCapability | null,
): { min: number | null; max: number | null } {
  const defaults = protocolDurationRange(protocol, model)
  return {
    min: configuredDuration(capability?.min_duration) ?? defaults.min,
    max: configuredDuration(capability?.max_duration) ?? defaults.max,
  }
}

function allowedDurations(
  protocol: CreationVideoProtocol,
  model: string,
  capability?: CreationVideoCapability | null,
): number[] | null {
  if (protocol === 'gemini') {
    if (/3[._-]1/i.test(model)) return [4, 6, 8]
    if (/3[._-]0/i.test(model)) return [8]
  }
  if (protocol === 'minimax_v1' && !hasConfiguredDuration(capability)) return [6, 10]
  return null
}

function clampToRange(
  value: number,
  range: { min: number | null; max: number | null },
): number {
  let result = value
  if (range.min !== null) result = Math.max(range.min, result)
  if (range.max !== null) result = Math.min(range.max, result)
  return result
}

function nearestAllowedDuration(
  value: number,
  allowed: number[],
  range: { min: number | null; max: number | null },
): number {
  const candidates = allowed.filter(option =>
    (range.min === null || option >= range.min)
    && (range.max === null || option <= range.max),
  )
  if (candidates.length === 0) return value

  let nearest = candidates[0] ?? value
  for (const candidate of candidates.slice(1)) {
    const distance = Math.abs(candidate - value)
    const nearestDistance = Math.abs(nearest - value)
    if (distance < nearestDistance || (distance === nearestDistance && candidate > nearest)) {
      nearest = candidate
    }
  }
  return nearest
}

function applyDurationRules(
  value: number,
  protocol: CreationVideoProtocol,
  model: string,
  capability?: CreationVideoCapability | null,
): number {
  const range = videoDurationRange(protocol, model, capability)
  const clamped = clampToRange(value, range)
  const allowed = allowedDurations(protocol, model, capability)
  return allowed ? nearestAllowedDuration(clamped, allowed, range) : clamped
}

export function defaultVideoDuration(
  protocol: CreationVideoProtocol,
  model: string,
  capability?: CreationVideoCapability | null,
): number | undefined {
  if (protocol === 'seedance' && isSeedance25(model)) return undefined

  let value: number
  switch (protocol) {
    case 'seedance':
      value = 5
      break
    case 'gemini':
      value = 8
      break
    case 'minimax_v1':
    case 'minimax_v2':
      value = 6
      break
    case 'grok':
      return undefined
  }
  return applyDurationRules(value, protocol, model, capability)
}

export function clampVideoDuration(
  value: unknown,
  protocol: CreationVideoProtocol,
  model: string,
  capability?: CreationVideoCapability | null,
): number | undefined {
  if (value === null || value === undefined) return undefined
  if (typeof value === 'string' && !value.trim()) return undefined

  let numeric: number
  if (typeof value === 'number') {
    numeric = value
  } else if (typeof value === 'string') {
    numeric = Number(value)
  } else {
    return defaultVideoDuration(protocol, model, capability)
  }

  if (!Number.isFinite(numeric)) return defaultVideoDuration(protocol, model, capability)
  return applyDurationRules(Math.round(numeric), protocol, model, capability)
}

export function defaultVideoResolutions(
  protocol: CreationVideoProtocol,
  capability?: CreationVideoCapability | null,
): string[] {
  const configured = Array.from(new Set(capability?.resolutions ?? []))
  if (configured.length > 0) return configured

  switch (protocol) {
    case 'seedance':
      return ['480p', '720p', '1080p']
    case 'gemini':
      return ['720p', '1080p']
    case 'minimax_v1':
    case 'minimax_v2':
      return ['480p', '768p', '1080p', '2k']
    case 'grok':
      return ['480p', '720p', '1080p']
  }
}

export function defaultVideoRatios(protocol: CreationVideoProtocol): string[] {
  if (protocol === 'gemini') return ['16:9', '9:16']
  return ['16:9', '9:16', '1:1', 'adaptive']
}

/** 解析 16:9、19.5:9、1536x1024 这类比例或尺寸；auto、adaptive 等没有固定比例的值返回 null。 */
export function parseCreationAspect(value: unknown): { w: number; h: number } | null {
  if (typeof value !== 'string') return null
  const match = /^\s*(\d+(?:\.\d+)?)\s*[:x×]\s*(\d+(?:\.\d+)?)\s*$/i.exec(value)
  if (!match) return null
  const w = Number(match[1])
  const h = Number(match[2])
  return w > 0 && h > 0 ? { w, h } : null
}

/** 任务卡片里生成中占位最多画几格（张数本身没有上限）。 */
const PREVIEW_TILE_LIMIT = 8

/** 任务卡片的占位形状：按请求的张数与比例画骨架，结果到位时不跳动。没有固定比例时生图按 1:1、视频按 16:9。 */
export function creationPreviewShape(
  submission: CreationSubmission,
): { previewCount: number; aspectW: number; aspectH: number } {
  if (submission.kind === 'video') {
    const aspect = parseCreationAspect(submission.input.ratio) ?? { w: 16, h: 9 }
    return { previewCount: 1, aspectW: aspect.w, aspectH: aspect.h }
  }
  const requested = Math.floor(Number(submission.input.n ?? 1))
  const previewCount = Number.isFinite(requested) ? Math.min(Math.max(requested, 1), PREVIEW_TILE_LIMIT) : 1
  const aspect = parseCreationAspect(submission.input.size)
    ?? parseCreationAspect(submission.input.aspectRatio)
    ?? { w: 1, h: 1 }
  return { previewCount, aspectW: aspect.w, aspectH: aspect.h }
}

/** 已用时长：不足 1 小时为 mm:ss，否则 h:mm:ss。 */
export function formatCreationElapsed(milliseconds: number): string {
  const total = Number.isFinite(milliseconds) ? Math.max(0, Math.floor(milliseconds / 1000)) : 0
  const hours = Math.floor(total / 3600)
  const minutes = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const seconds = String(total % 60).padStart(2, '0')
  return hours > 0 ? `${hours}:${minutes}:${seconds}` : `${minutes}:${seconds}`
}

export function safeMediaUrl(value: unknown): string {
  if (typeof value !== 'string') return ''
  const url = value.trim()
  if (!url) return ''
  if (/^data:image\/(?:png|jpeg|webp|gif);base64,/i.test(url)) return url
  if (!/^https?:\/\//i.test(url) && !/^blob:/i.test(url)) return ''

  try {
    const parsed = new URL(url)
    return ['http:', 'https:', 'blob:'].includes(parsed.protocol) ? url : ''
  } catch {
    return ''
  }
}

export function creationHistoryKey(userId: number | string): string {
  return `${HISTORY_PREFIX}${userId}`
}

function isCreationKind(value: unknown): value is CreationKind {
  return value === 'image' || value === 'video'
}

function isCreationProtocol(value: unknown): value is CreationProtocol {
  return value === 'openai'
    || value === 'gemini'
    || value === 'seedance'
    || value === 'minimax_v1'
    || value === 'minimax_v2'
    || value === 'grok'
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function persistentAsset(value: unknown): CreationSessionRecord['assets'][number] | null {
  if (!isObject(value) || !isCreationKind(value.kind)) return null
  if (value.mimeType !== undefined && typeof value.mimeType !== 'string') return null

  const url = safeMediaUrl(value.url)
  if (!url || /^blob:/i.test(url)) return null
  return {
    kind: value.kind,
    url,
    ...(typeof value.mimeType === 'string' ? { mimeType: value.mimeType } : {}),
  }
}

function parseHistoryRecord(value: unknown): CreationSessionRecord | null {
  if (!isObject(value)
    || typeof value.id !== 'string'
    || !isCreationKind(value.kind)
    || typeof value.model !== 'string'
    || !isCreationProtocol(value.protocol)
    || typeof value.prompt !== 'string'
    || typeof value.createdAt !== 'number'
    || !Number.isFinite(value.createdAt)
    || !Array.isArray(value.assets)) {
    return null
  }

  const assets: CreationSessionRecord['assets'] = []
  for (const item of value.assets) {
    const asset = persistentAsset(item)
    if (asset) assets.push(asset)
  }
  return {
    id: value.id,
    kind: value.kind,
    model: value.model,
    protocol: value.protocol,
    prompt: value.prompt,
    createdAt: value.createdAt,
    ...(typeof value.taskId === 'string' && value.taskId ? { taskId: value.taskId } : {}),
    assets,
  }
}

export function readCreationHistory(
  storage: Pick<Storage, 'getItem'>,
  userId: number | string,
  kind: CreationKind,
): CreationSessionRecord[] {
  try {
    const stored = storage.getItem(creationHistoryKey(userId))
    if (!stored) return []
    const parsed: unknown = JSON.parse(stored)
    if (!Array.isArray(parsed)) return []

    const records: CreationSessionRecord[] = []
    for (const value of parsed) {
      const record = parseHistoryRecord(value)
      if (record?.kind !== kind) continue
      records.push(record)
      if (records.length === HISTORY_LIMIT) break
    }
    return records
  } catch {
    return []
  }
}

export function writeCreationHistory(
  storage: Pick<Storage, 'setItem'>,
  userId: number | string,
  records: CreationSessionRecord[],
): void {
  try {
    const whitelisted = records.slice(0, HISTORY_LIMIT).map(record => {
      const assets: CreationSessionRecord['assets'] = []
      for (const value of record.assets) {
        const asset = persistentAsset(value)
        if (asset) assets.push(asset)
      }
      return {
        id: record.id,
        kind: record.kind,
        model: record.model,
        protocol: record.protocol,
        prompt: record.prompt,
        createdAt: record.createdAt,
        ...(typeof record.taskId === 'string' && record.taskId ? { taskId: record.taskId } : {}),
        assets,
      }
    })
    storage.setItem(creationHistoryKey(userId), JSON.stringify(whitelisted))
  } catch {
    return
  }
}

/**
 * 合并出历史记录：服务端的异步任务为主，本机记录补上提示词和全部图片；
 * 没有对应服务端任务的本机记录（如同步生图）单独成条。按创建时间倒序。
 *
 * hidden 是本页已经用任务卡片展示的任务与记录，这里不再重复。
 */
export function buildCreationHistory(
  tasks: AsyncTask[],
  records: CreationSessionRecord[],
  hidden: { taskIds: ReadonlySet<string>; recordIds: ReadonlySet<string> },
): CreationHistoryItem[] {
  const recordByTask = new Map<string, CreationSessionRecord>()
  for (const record of records) {
    if (record.taskId && !recordByTask.has(record.taskId)) recordByTask.set(record.taskId, record)
  }

  const items: CreationHistoryItem[] = []
  const merged = new Set<string>()
  for (const task of tasks) {
    const record = recordByTask.get(task.task_id)
    if (record) merged.add(record.id)
    if (hidden.taskIds.has(task.task_id)) continue

    const completed = task.status === 'completed'
    const direct = safeMediaUrl(task.result_url)
    let images: string[] = []
    let videoUrl = ''
    let downloadId = ''
    if (completed && task.kind === 'image') {
      // 任务目录只记第一张图；本机提交过的任务保存了全部结果。
      const saved = record?.assets.flatMap(asset => asset.kind === 'image' ? [asset.url] : []) ?? []
      images = saved.length ? saved : direct ? [direct] : []
    } else if (completed && task.kind === 'video' && task.result_url) {
      // 与异步任务页一致：只有 Seedance 的直链可以直接播放，其余走站内下载接口。
      if (task.protocol === 'seedance' && direct) videoUrl = direct
      else downloadId = task.id
    }
    items.push({
      key: `server:${task.id}`,
      source: 'server',
      kind: task.kind,
      protocol: task.protocol,
      model: task.model || record?.model || '',
      status: task.status,
      prompt: record?.prompt ?? '',
      taskId: task.task_id,
      createdAt: task.created_at * 1000,
      images,
      videoUrl,
      downloadId,
      error: task.error ?? '',
    })
  }

  for (const record of records) {
    if (merged.has(record.id) || hidden.recordIds.has(record.id)) continue
    if (record.taskId && hidden.taskIds.has(record.taskId)) continue
    items.push({
      key: `local:${record.id}`,
      source: 'local',
      kind: record.kind,
      protocol: record.protocol,
      model: record.model,
      status: 'completed',
      prompt: record.prompt,
      taskId: record.taskId ?? '',
      createdAt: record.createdAt,
      images: record.assets.flatMap(asset => asset.kind === 'image' ? [asset.url] : []),
      videoUrl: record.assets.find(asset => asset.kind === 'video')?.url ?? '',
      downloadId: '',
      error: '',
    })
  }

  return items.sort((left, right) => right.createdAt - left.createdAt)
}
