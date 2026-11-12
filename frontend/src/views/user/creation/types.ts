import type {
  CreationAsset,
  CreationImageRole,
  CreationKind,
  CreationProtocol,
  CreationTaskHandle,
  CreationTaskStatus,
  ImageGenerationInput,
  VideoGenerationInput,
} from '@/features/creation/types'

export interface CreationErrorState {
  message: string
  code: string
  requestId: string
  status: number
}

export interface CreationCurrentTask {
  id: number
  kind: CreationKind
  protocol: CreationProtocol
  model: string
  prompt: string
  groupName: string
  createdAt: number
  status: CreationTaskStatus
  assets: CreationAsset[]
  progress?: number
  error: CreationErrorState | null
  handle: CreationTaskHandle | null
  /** 提交请求尚未返回（异步任务尚未拿到任务 ID，或同步请求仍在进行）。 */
  submitting: boolean
  /** 网关不支持异步提交（如 Gemini 生图），结果只能由当前页面的同步请求带回。 */
  sync: boolean
  cancellationNoticed: boolean
  /** 生成中占位的格数（按请求张数，视频为 1）。 */
  previewCount: number
  /** 占位与结果单元的宽高比，取自请求的尺寸 / 比例。 */
  aspectW: number
  aspectH: number
}

interface CreationSubmissionBase {
  groupId: number
  groupName: string
  apiKey: string
}

export type CreationSubmission = CreationSubmissionBase & (
  | { kind: 'image'; input: ImageGenerationInput }
  | { kind: 'video'; input: VideoGenerationInput }
)

export interface ReferenceImageDraft {
  id: number
  file: File
  previewUrl: string
  role: CreationImageRole
}

/** 单选胶囊组（CreationChipGroup）的选项。 */
export interface CreationChipOption {
  value: string
  label: string
  /** 悬停时的完整说明。 */
  title?: string
}

/** 灯箱（CreationLightbox）里的一张图片。 */
export interface CreationLightboxItem {
  /** 已通过 safeMediaUrl 校验的地址。 */
  url: string
  alt: string
  downloadName: string
}

/** 历史记录的类型筛选。 */
export type CreationFeedFilter = 'all' | CreationKind

/** 历史记录里一条已归档的记录：来自服务端的异步任务，或只保存在本机的生成记录。 */
export interface CreationHistoryItem {
  key: string
  source: 'server' | 'local'
  kind: CreationKind
  protocol: string
  model: string
  status: 'processing' | 'completed' | 'failed' | 'cancelled'
  /** 只有在本机提交过的任务才有提示词。 */
  prompt: string
  taskId: string
  /** 毫秒时间戳。 */
  createdAt: number
  /** 可直接展示的图片地址。 */
  images: string[]
  /** 可直接播放的视频地址。 */
  videoUrl: string
  /** 视频需要经站内下载接口取回后才能播放时，为异步任务的目录 ID。 */
  downloadId: string
  error: string
}
