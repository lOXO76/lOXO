import type { GroupPlatform } from '@/types'
import type { CcSwitchApp } from '@/utils/ccswitchImport'

/**
 * 客户端导入目标。
 *
 * 每个客户端生成配置所需的信息都在 GenerateInput 里，生成器本身不读
 * props / store，便于在单测里直接断言生成的字符串。
 */
export type ClientImportId =
  | 'claude-code'
  | 'codex'
  | 'pi'
  | 'cherry-studio'
  | 'chatbox'
  | 'magpie'
  | 'workbuddy'
  | 'zcode'
  | 'deepseek-hermes'
  | 'cc-switch'

/**
 * 导入方式：
 * - deeplink：客户端注册了自定义协议，可一键唤起并完成配置
 * - command：提供一条终端命令直接写好配置，同时也能下载配置文件
 * - file：客户端从磁盘读配置，需要下载文件后放到指定路径
 * - manual：没有公开的配置文件格式，只能在客户端 UI 里逐项填写
 */
export type ImportMode = 'deeplink' | 'command' | 'file' | 'manual'

/**
 * 模型选择方式：
 * - none：配置里不需要模型
 * - single：只能填一个模型
 * - multi：可配置多个候选模型，并指定其中一个为默认模型
 */
export type ModelMode = 'none' | 'single' | 'multi'

export interface ImportFile {
  /** 展示用的文件路径；deeplink/manual 客户端没有文件 */
  path: string
  content: string
  /** 覆盖或合并已有配置时的注意事项 */
  hint?: string
}

/** 一键配置命令；内容含 API Key，页面需要提示不要外传 */
export interface ImportCommand {
  shell: 'bash' | 'powershell'
  content: string
}

export interface ManualField {
  label: string
  value: string
  /** 等宽字体展示 + 一键复制（端点、密钥、模型 ID） */
  mono?: boolean
}

export interface ManualStep {
  text: string
}

export interface ImportResult {
  files: ImportFile[]
  /** 有值时页面展示「一键导入」按钮 */
  deeplink?: string
  /** 有值时页面展示「一键命令」区块，按 shell 分 tab */
  commands?: ImportCommand[]
  /** 手动填写字段；与 files 可同时存在（如 Hermes 既给终端变量也给待填字段） */
  manual: ManualField[]
  steps: ManualStep[]
}

export interface GenerateInput {
  /** 站点根地址，已去掉尾部斜杠与 /v1 */
  baseUrl: string
  apiKey: string
  siteName: string
  /** 已选中的模型 ID，顺序即页面展示顺序 */
  models: string[]
  /** 默认模型；multi 客户端从 models 里选，single 客户端即唯一模型 */
  defaultModel: string
  platform: GroupPlatform
  /** 用于生成稳定且不冲突的 provider id（Magpie 按 id 判重） */
  keyId?: number
  keyName?: string
  /** 仅 cc-switch 使用：导入到 CC Switch 中的哪个应用 */
  ccSwitchApp?: CcSwitchApp
}

export interface ResolvedEndpoints {
  anthropic?: string
  chat?: string
  responses?: string
  gemini?: string
}
