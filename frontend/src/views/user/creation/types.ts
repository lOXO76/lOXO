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
