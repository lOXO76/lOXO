import { apiClient } from '@/api/client'
import type { ApiKey } from '@/types'
import type { CreationCatalog, CreationGroup, CreationKind, CreationModel, CreationProtocol } from './types'

export async function getCreationCatalog(signal?: AbortSignal): Promise<CreationCatalog> {
  const { data } = await apiClient.get<CreationCatalog>('/user/creation/catalog', { signal })
  return data
}

export const creationAPI = { getCatalog: getCreationCatalog }

export function groupSupports(group: CreationGroup, kind: CreationKind): boolean {
  return Boolean(group[kind]?.protocols.length)
}

export function resolveModels(group: CreationGroup, kind: CreationKind, protocol: CreationProtocol): CreationModel[] {
  const capability = group[kind]
  if (!capability?.protocols.some((supported) => supported === protocol)) return []
  return capability.models.filter((model) => model.protocols.length === 0 || model.protocols.includes(protocol))
}

export function creationPlatformLabel(protocol: CreationProtocol): string {
  return protocol
}

export function pickKeysForGroup(keys: ApiKey[], groupId: number): ApiKey[] {
  return keys.filter((key) => key.status === 'active' && key.group_id === groupId)
}
