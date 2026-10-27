import type { GroupPlatform } from '@/types'
import { resolveEndpoints } from './endpoints'

export interface GroupModel {
  id: string
  name?: string
}

interface ModelsPayload {
  data?: unknown
  models?: unknown
}

/**
 * 读取该 API Key 当前可用的模型。
 *
 * 直接调网关的 /v1/models，而不是在页面里内置模型清单：后端已按分组的
 * allowlist 与 Codex manifest 配置过滤，返回的就是这个 key 真正能调的模型，
 * 与用户之后在客户端里选到的一致。
 */
export async function fetchGroupModels(
  baseUrl: string,
  apiKey: string,
  platform: GroupPlatform,
  signal?: AbortSignal
): Promise<GroupModel[]> {
  const { chat } = resolveEndpoints(baseUrl, platform)
  if (!chat) throw new Error('no chat endpoint for platform')

  const response = await fetch(`${chat}/models`, {
    headers: { Accept: 'application/json', Authorization: `Bearer ${apiKey}` },
    cache: 'no-store',
    signal
  })
  if (!response.ok) {
    throw new Error(`models request failed with status ${response.status}`)
  }

  const payload = (await response.json()) as ModelsPayload
  const raw = Array.isArray(payload.data)
    ? payload.data
    : Array.isArray(payload.models)
      ? payload.models
      : []

  return raw.flatMap((item) => {
    const entry = item as { id?: unknown; slug?: unknown; name?: unknown }
    const id = entry.id ?? entry.slug
    if (typeof id !== 'string' || id.trim() === '') return []
    return [{
      id,
      name: typeof entry.name === 'string' ? entry.name : undefined
    }]
  })
}
