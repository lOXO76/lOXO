import { apiClient } from '../client'
import type {
  DegradationArtifactKind,
  DegradationOverview,
  DegradationProbeType,
  DegradationRunStatus,
  DegradationVisualViewToken,
} from '../degradation'

export interface DegradationProbe {
  id: number
  name: string
  prompt: string
  expect_regex: string
  regex_flags: string
  negate: boolean
  enabled: boolean
  sort_order: number
  created_at: string
  updated_at: string
  probe_type: DegradationProbeType
  max_output_tokens: number
  timeout_seconds: number
  run_interval_seconds: number
}
export interface DegradationProbeInput {
  name: string
  prompt: string
  expect_regex: string
  regex_flags: string
  negate: boolean
  enabled: boolean
  sort_order: number
  probe_type: DegradationProbeType
  max_output_tokens: number
  timeout_seconds: number
  run_interval_seconds: number
}
export interface DegradationTarget {
  id: number
  group_id: number
  group_name: string
  platform: string
  model: string
  probe_ids: number[]
  interval_seconds: number
  enabled: boolean
  last_run_at: string | null
  created_at: string
  updated_at: string
  enabled_probe_count: number
  visual_probe_count: number
}
export interface DegradationTargetInput {
  group_id: number
  model: string
  probe_ids: number[]
  interval_seconds: number
  enabled: boolean
}
export interface DegradationRun {
  id: number
  target_id: number
  group_id: number
  model: string
  probe_id: number
  probe_name: string
  status: DegradationRunStatus
  answer_text: string
  error_message: string
  latency_ms: number
  account_id: number
  created_at: string
  probe_type: DegradationProbeType
  artifact_kind: DegradationArtifactKind | ''
  artifact_bytes: number
}
export interface DegradationProbeTestResult {
  matched: boolean
  passed: boolean
  error: string
}
export interface DegradationAccountPreview {
  account_id: number
  account_name: string
  platform: string
  available: boolean
  reason: string
}
export interface DegradationRunTriggerResult { accepted: boolean; probe_count: number; started_at: string }

export async function listProbes(): Promise<{ items: DegradationProbe[] }> {
  const { data } = await apiClient.get<{ items: DegradationProbe[] }>('/admin/degradation/probes')
  return data
}

export async function createProbe(input: DegradationProbeInput): Promise<DegradationProbe> {
  const { data } = await apiClient.post<DegradationProbe>('/admin/degradation/probes', input)
  return data
}

export async function updateProbe(id: number, input: DegradationProbeInput): Promise<DegradationProbe> {
  const { data } = await apiClient.put<DegradationProbe>(`/admin/degradation/probes/${id}`, input)
  return data
}

export async function deleteProbe(id: number): Promise<void> {
  await apiClient.delete(`/admin/degradation/probes/${id}`)
}

export async function testProbe(input: { regex: string; flags: string; negate: boolean; sample_text: string }): Promise<DegradationProbeTestResult> {
  const { data } = await apiClient.post<DegradationProbeTestResult>('/admin/degradation/probes/test', input)
  return data
}

export async function listTargets(params?: { group_id?: number }): Promise<{ items: DegradationTarget[] }> {
  const { data } = await apiClient.get<{ items: DegradationTarget[] }>('/admin/degradation/targets', { params })
  return data
}

export async function createTarget(input: DegradationTargetInput): Promise<DegradationTarget> {
  const { data } = await apiClient.post<DegradationTarget>('/admin/degradation/targets', input)
  return data
}

export async function updateTarget(id: number, input: DegradationTargetInput): Promise<DegradationTarget> {
  const { data } = await apiClient.put<DegradationTarget>(`/admin/degradation/targets/${id}`, input)
  return data
}

export async function deleteTarget(id: number): Promise<void> {
  await apiClient.delete(`/admin/degradation/targets/${id}`)
}

export async function runTarget(id: number): Promise<DegradationRunTriggerResult> {
  const { data } = await apiClient.post<DegradationRunTriggerResult>(`/admin/degradation/targets/${id}/run`)
  return data
}

export async function createRunViewToken(runId: number): Promise<DegradationVisualViewToken> {
  const { data } = await apiClient.post<DegradationVisualViewToken>(`/admin/degradation/runs/${runId}/view-token`)
  return data
}

export async function listTargetRuns(id: number, params?: { limit?: number }): Promise<{ items: DegradationRun[] }> {
  const { data } = await apiClient.get<{ items: DegradationRun[] }>(`/admin/degradation/targets/${id}/runs`, { params })
  return data
}

export async function getOverview(params?: { window_hours?: number }): Promise<DegradationOverview> {
  const { data } = await apiClient.get<DegradationOverview>('/admin/degradation/overview', { params })
  return data
}

export async function previewAccount(params: { group_id: number; model: string }): Promise<DegradationAccountPreview> {
  const { data } = await apiClient.post<DegradationAccountPreview>('/admin/degradation/preview-account', params)
  return data
}

export const degradationAdminAPI = {
  listProbes,
  createProbe,
  updateProbe,
  deleteProbe,
  testProbe,
  listTargets,
  createTarget,
  updateTarget,
  deleteTarget,
  runTarget,
  createRunViewToken,
  listTargetRuns,
  getOverview,
  previewAccount,
}

export default degradationAdminAPI
