// 开放接口相关错误码（契约 §5.6）。reason / HTTP 与契约逐字一致，说明文案走 i18n。
export interface RewardOpenErrorCode {
  reason: string
  http: string
  /** 场景说明的 i18n key */
  scenarioKey: string
  /** 处理建议的 i18n key */
  actionKey: string
}

function errorCode(id: string, reason: string, http: string): RewardOpenErrorCode {
  return {
    reason,
    http,
    scenarioKey: `rewardIntegration.errorItems.${id}.scenario`,
    actionKey: `rewardIntegration.errorItems.${id}.action`,
  }
}

export const REWARD_OPEN_ERROR_CODES: RewardOpenErrorCode[] = [
  errorCode('signatureMissing', 'REWARD_SIGNATURE_MISSING', '401'),
  errorCode('signatureInvalid', 'REWARD_SIGNATURE_INVALID', '401'),
  errorCode('timestampExpired', 'REWARD_TIMESTAMP_EXPIRED', '401'),
  errorCode('nonceReplayed', 'REWARD_NONCE_REPLAYED', '401'),
  errorCode('partnerInvalid', 'REWARD_PARTNER_INVALID', '401'),
  errorCode('tasksDisabled', 'REWARD_TASKS_DISABLED', '403'),
  errorCode('ticketNotFound', 'REWARD_TICKET_NOT_FOUND', '404'),
  errorCode('ticketExpired', 'REWARD_TICKET_EXPIRED', '400'),
  errorCode('ticketPartnerMismatch', 'REWARD_TICKET_PARTNER_MISMATCH', '403'),
  errorCode('eventInvalid', 'REWARD_EVENT_INVALID', '400'),
  errorCode('eventTypeMismatch', 'REWARD_EVENT_TYPE_MISMATCH', '400'),
  errorCode('externalUserUsed', 'REWARD_EXTERNAL_USER_USED', '409'),
  errorCode('externalUserMismatch', 'REWARD_EXTERNAL_USER_MISMATCH', '409'),
  errorCode('taskUnavailable', 'REWARD_TASK_UNAVAILABLE', '400'),
  errorCode('taskAlreadyCompleted', 'REWARD_TASK_ALREADY_COMPLETED', '409'),
  errorCode('quotaExhausted', 'REWARD_TASK_QUOTA_EXHAUSTED', '409'),
  errorCode('originNotAllowed', 'REWARD_ORIGIN_NOT_ALLOWED', '403'),
]
