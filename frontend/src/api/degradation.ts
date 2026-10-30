import { apiClient } from './client'
import { buildApiUrl } from './url'

export type DegradationVerdict = 'healthy' | 'degraded' | 'unknown'
export type DegradationProbeType = 'regex' | 'visual'
export type DegradationArtifactKind = 'html' | 'svg' | 'text'
export type DegradationRunStatus = 'passed' | 'failed' | 'error' | 'rendered'
export interface DegradationShowcase {
  run_id: number
  probe_id: number
  probe_name: string
  artifact_kind: DegradationArtifactKind
  latency_ms: number
  created_at: string
}
export interface DegradationVisualViewToken { token: string; expires_at: string }

export interface DegradationModelOverview {
  model: string
  verdict: DegradationVerdict
  pass_rate: number
  total: number
  passed: number
  failed: number
  error: number
  avg_latency_ms: number
  last_checked_at: string | null
  last_run_status: DegradationRunStatus | ''
  showcases: DegradationShowcase[]
}
export interface DegradationGroupOverview {
  group_id: number
  group_name: string
  platform: string
  verdict: DegradationVerdict
  last_checked_at: string | null
  models: DegradationModelOverview[]
}
export interface DegradationOverview {
  enabled: boolean
  window_hours: number
  threshold: number
  generated_at: string
  groups: DegradationGroupOverview[]
}
export interface DegradationUserRun {
  id: number
  group_id: number
  model: string
  probe_name: string
  status: DegradationRunStatus
  latency_ms: number
  created_at: string
  probe_type: DegradationProbeType
  artifact_kind: DegradationArtifactKind | ''
}

export async function getOverview(params?: { window_hours?: number }): Promise<DegradationOverview> {
  const { data } = await apiClient.get<DegradationOverview>('/degradation/overview', { params })
  return data
}

export async function listRuns(params: { group_id?: number; model?: string; limit?: number }): Promise<{ items: DegradationUserRun[] }> {
  const { data } = await apiClient.get<{ items: DegradationUserRun[] }>('/degradation/runs', { params })
  return data
}

export async function listVisualRuns(params: { group_id: number; model: string; probe_id: number; limit?: number }): Promise<{ items: DegradationShowcase[] }> {
  const { data } = await apiClient.get<{ items: DegradationShowcase[] }>('/degradation/visual-runs', { params })
  return data
}

export async function createVisualViewToken(runId: number): Promise<DegradationVisualViewToken> {
  const { data } = await apiClient.post<DegradationVisualViewToken>(`/degradation/visual-runs/${runId}/view-token`)
  return data
}

export function buildVisualRenderUrl(token: string): string {
  return buildApiUrl(`/degradation/visual-render/${encodeURIComponent(token)}`)
}

export const degradationAPI = {
  getOverview,
  listRuns,
  listVisualRuns,
  createVisualViewToken,
}

export default degradationAPI
