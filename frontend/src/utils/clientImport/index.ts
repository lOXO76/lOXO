/**
 * 客户端导入：把某个 API Key 展开成各 AI 客户端可直接使用的配置。
 *
 * 页面只需要 registry（客户端定义 + 可用性判定）和 models（模型清单）；
 * 这里做统一出口，避免视图层同时引用多个内部模块路径。
 */
export type {
  ClientImportId,
  GenerateInput,
  ImportFile,
  ImportMode,
  ImportResult,
  ManualField,
  ManualStep,
  ModelMode,
  ResolvedEndpoints
} from './types'

export {
  CLIENT_DEFINITIONS,
  base64Utf8,
  ccSwitchAppLabel,
  evaluateClient,
  findClientDefinition,
  listCcSwitchApps,
  supportsAnthropicMessages,
  supportsTextProtocol,
  type ClientAvailability,
  type ClientAvailabilityGroup,
  type ClientBlockReason,
  type ClientDefinition,
  type ClientProtocol
} from './registry'

export { resolveCatalog, resolveEndpoints, siteRoot } from './endpoints'
export { fetchGroupModels, type GroupModel } from './models'
