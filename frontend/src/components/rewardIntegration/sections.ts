// 「任务接入文档」的九个章节（顺序即目录顺序）
export interface RewardDocsSection {
  /** 锚点 id（页面内为 reward-docs-<id>） */
  id: string
  /** 章节标题的 i18n key */
  titleKey: string
}

export const REWARD_DOCS_SECTIONS: RewardDocsSection[] = [
  { id: 'overview', titleKey: 'rewardIntegration.sections.overview' },
  { id: 'preparation', titleKey: 'rewardIntegration.sections.preparation' },
  { id: 'ticket', titleKey: 'rewardIntegration.sections.ticket' },
  { id: 'sdk', titleKey: 'rewardIntegration.sections.sdk' },
  { id: 'signature', titleKey: 'rewardIntegration.sections.signature' },
  { id: 'api', titleKey: 'rewardIntegration.sections.api' },
  { id: 'errors', titleKey: 'rewardIntegration.sections.errors' },
  { id: 'examples', titleKey: 'rewardIntegration.sections.examples' },
  { id: 'checklist', titleKey: 'rewardIntegration.sections.checklist' },
]

export function rewardDocsAnchor(id: string): string {
  return `reward-docs-${id}`
}
