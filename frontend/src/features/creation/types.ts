/**
 * 创作页（生图 / 生视频）共享契约。
 *
 * - 后端 GET /api/v1/user/creation/catalog（JWT）返回 CreationCatalog：
 *   当前用户可用、且支持生图或生视频的分组，以及各分组的协议、模型与能力。
 * - 前端用「该分组下的用户 API Key」直接调用网关（与批量生图页一致），
 *   计费、限流、异步任务记录都沿用网关原有链路。
 */

export type CreationKind = 'image' | 'video'

/**
 * 生图协议：
 * - openai：/v1/images/generations、/v1/images/edits（及 /async 变体）
 * - gemini：/v1beta/models/{model}:generateContent
 */
export type CreationImageProtocol = 'openai' | 'gemini'

/** 生视频协议，与 API 文档「按协议接入」一致。 */
export type CreationVideoProtocol = 'seedance' | 'gemini' | 'minimax_v1' | 'minimax_v2' | 'grok'

export type CreationProtocol = CreationImageProtocol | CreationVideoProtocol

// ---------------------------------------------------------------------------
// 后端目录（JSON 字段为 snake_case，与后端 DTO 一一对应）
// ---------------------------------------------------------------------------

export interface CreationModel {
  /** 公开模型名，直接作为请求中的 model。 */
  id: string
  /** 该模型可走的协议；空数组表示分组内该类型的全部协议都可用。 */
  protocols: string[]
  /** 实付口径的展示价；未配置图片／视频定价时为 null。 */
  pricing: CreationModelPricing | null
}

/** 与模型广场「实付」列同口径：渠道／分组定价 × 生效倍率（独立生图／视频倍率、用户专属倍率或分组倍率）。 */
export interface CreationModelPricing {
  /** image 按张、second 按秒、request 按次。 */
  unit: 'image' | 'second' | 'request'
  /** 不分档时的单价。 */
  price?: number
  /** 分档单价：图片按 1K／2K／4K，视频按分辨率。 */
  tiers: CreationPriceTier[]
}

export interface CreationPriceTier {
  label: string
  price: number
}

export interface CreationImageCapability {
  protocols: CreationImageProtocol[]
  models: CreationModel[]
  /** openai 协议能否走异步提交（站点开启了异步生图且分组平台支持）。 */
  async_supported: boolean
  /** 是否支持上传参考图（图片编辑 / 图生图）。 */
  edit_supported: boolean
}

export interface CreationVideoCapability {
  protocols: CreationVideoProtocol[]
  models: CreationModel[]
  /** 分组内可用账号支持的分辨率并集（小写：480p/720p/768p/1080p/2k）；空数组表示未配置，按协议默认。 */
  resolutions: string[]
  /** 分组内可用账号的时长区间（秒）；null 表示未配置，按协议默认。 */
  min_duration: number | null
  max_duration: number | null
}

export interface CreationGroup {
  id: number
  name: string
  description: string
  platform: string
  /** 'standard' | 'subscription' */
  subscription_type: string
  rate_multiplier: number
  /** 不支持生图时为 null。 */
  image: CreationImageCapability | null
  /** 不支持生视频时为 null。 */
  video: CreationVideoCapability | null
}

export interface CreationCatalog {
  groups: CreationGroup[]
}

// ---------------------------------------------------------------------------
// 前端网关调用（gateway.ts）
// ---------------------------------------------------------------------------

export type CreationTaskStatus = 'processing' | 'succeeded' | 'failed' | 'cancelled'

export interface CreationAsset {
  kind: CreationKind
  /** 可直接给 <img>/<video> 使用的地址：http(s)、data: 或 blob:。 */
  url: string
  /** url 是 URL.createObjectURL 生成的，调用方不再展示时需 revoke。 */
  objectUrl?: boolean
  mimeType?: string
  /** 原始结果地址（上游链接或站内受保护路径），仅供展示/排障。 */
  sourceUrl?: string
}

export interface CreationTaskHandle {
  kind: CreationKind
  protocol: CreationProtocol
  /** imgtask_… / Seedance task id / Veo operation name / MiniMax task_id / xAI request_id */
  taskId: string
  model: string
}

export interface CreationTaskSnapshot {
  status: CreationTaskStatus
  assets: CreationAsset[]
  /** 失败原因，已是可直接展示的文本。 */
  error?: string
  /** 0–100，协议不提供时为空。 */
  progress?: number
  /** 建议的下一次轮询间隔（毫秒），来自 Retry-After 或协议默认。 */
  retryAfterMs?: number
}

export interface CreationSubmitResult {
  /** 同步出图时为 null，此时 snapshot 已是终态。 */
  handle: CreationTaskHandle | null
  snapshot: CreationTaskSnapshot
}

export type CreationImageRole = 'first_frame' | 'last_frame' | 'reference_image'

export interface CreationReferenceImage {
  file: File
  /** 仅视频使用：首帧 / 尾帧 / 参考图；生图忽略。 */
  role?: CreationImageRole
}

export interface ImageGenerationInput {
  apiKey: string
  protocol: CreationImageProtocol
  /** 分组平台（openai / grok / gemini / composite …），用于按平台微调请求字段。 */
  platform: string
  model: string
  prompt: string
  /** openai 协议（Grok 分组除外）：如 1024x1024、1536x1024、1024x1536、auto。 */
  size?: string
  /** gemini 协议与 Grok 分组：如 1:1、16:9。 */
  aspectRatio?: string
  /** gemini 协议：1K / 2K / 4K；Grok 分组：1K / 2K（发送为 resolution）。 */
  imageSize?: string
  /** openai 协议（Grok 分组除外）：low / medium / high / auto。 */
  quality?: string
  /** 出图张数，默认 1。 */
  n?: number
  referenceImages?: CreationReferenceImage[]
  /** openai 协议优先走异步接口（仅当目录 async_supported 为 true 时传 true）。 */
  preferAsync?: boolean
  signal?: AbortSignal
}

export interface VideoGenerationInput {
  apiKey: string
  protocol: CreationVideoProtocol
  model: string
  prompt: string
  /** 秒；不填按协议默认。 */
  duration?: number
  /** 如 720p / 1080p / 768P；不填按协议默认。 */
  resolution?: string
  /** 画面比例，如 16:9、9:16、1:1、adaptive。 */
  ratio?: string
  referenceImages?: CreationReferenceImage[]
  signal?: AbortSignal
}

export interface WaitForTaskOptions {
  signal?: AbortSignal
  /** 每次轮询拿到新快照时回调（含非终态）。 */
  onUpdate?: (snapshot: CreationTaskSnapshot) => void
  /** 默认轮询间隔（毫秒），快照带 retryAfterMs 时以其为准。 */
  intervalMs?: number
  /** 最长等待时间（毫秒），超时抛错但不取消上游任务。 */
  timeoutMs?: number
}
