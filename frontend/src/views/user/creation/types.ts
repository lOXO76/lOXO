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
  submitting: boolean
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
