import type {
  CreationGroup,
  CreationKind,
  CreationModel,
  CreationModelPricing,
  CreationProtocol,
  CreationVideoCapability,
  CreationVideoProtocol,
} from '@/features/creation/types'

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
        assets,
      }
    })
    storage.setItem(creationHistoryKey(userId), JSON.stringify(whitelisted))
  } catch {
    return
  }
}
