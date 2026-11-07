<template>
  <div class="space-y-4" data-testid="ticket-settings-tab">
    <div v-if="loading" class="card flex items-center justify-center gap-2 !rounded-2xl px-4 py-10 text-sm text-gray-500 dark:text-dark-300">
      <Icon name="refresh" size="sm" class="animate-spin" />
      {{ t('common.loading') }}
    </div>

    <template v-else>
      <!-- 总开关 -->
      <section class="card !rounded-2xl space-y-4 p-4 sm:p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.settings.general.title') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.general.hint') }}</p>
          </div>
          <div class="flex items-center gap-3">
            <span class="badge" :class="form.enabled ? 'badge-success' : 'badge-gray'" data-testid="ticket-system-state">
              {{ form.enabled ? t('admin.tickets.settings.general.enabled') : t('admin.tickets.settings.general.disabled') }}
            </span>
            <Toggle v-model="form.enabled" data-testid="ticket-system-toggle" />
          </div>
        </div>
      </section>

      <!-- 管理员通知 -->
      <section class="card !rounded-2xl space-y-5 p-4 sm:p-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.settings.notify.title') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.notify.hint') }}</p>
        </div>

        <!-- 邮件 -->
        <div class="space-y-3 rounded-xl border border-gray-100 p-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.tickets.settings.notify.emailTitle') }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.notify.emailHint') }}</p>
            </div>
            <Toggle v-model="form.admin_email_enabled" data-testid="ticket-email-toggle" />
          </div>

          <div class="space-y-2">
            <label class="input-label block text-xs font-medium text-gray-600 dark:text-dark-300">
              {{ t('admin.tickets.settings.notify.emailList') }}
            </label>
            <div class="flex flex-wrap gap-2">
              <input
                v-model="emailDraft"
                type="email"
                class="input w-full sm:w-72"
                data-testid="ticket-email-input"
                :placeholder="t('admin.tickets.settings.notify.emailPlaceholder')"
                @keydown.enter.prevent="addEmail"
              />
              <button type="button" class="btn btn-secondary" data-testid="ticket-email-add" @click="addEmail">
                <Icon name="plus" size="sm" />
                {{ t('admin.tickets.settings.notify.addEmail') }}
              </button>
            </div>
            <p v-if="emailError" class="text-xs text-red-600 dark:text-red-400">{{ emailError }}</p>
            <ul v-if="form.admin_notify_emails.length" class="flex flex-wrap gap-2" data-testid="ticket-email-list">
              <li
                v-for="email in form.admin_notify_emails"
                :key="email"
                class="inline-flex items-center gap-1 rounded-full bg-gray-100 py-1 pl-3 pr-1.5 text-xs text-gray-700 dark:bg-dark-700 dark:text-dark-200"
              >
                <span class="font-mono">{{ email }}</span>
                <button
                  type="button"
                  class="rounded-full p-0.5 text-gray-400 transition hover:bg-gray-200 hover:text-red-600 dark:hover:bg-dark-600 dark:hover:text-red-400"
                  :aria-label="`${t('common.delete')} ${email}`"
                  @click="removeEmail(email)"
                >
                  <Icon name="x" size="xs" />
                </button>
              </li>
            </ul>
            <p v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.tickets.settings.notify.emailListEmpty') }}</p>
          </div>
        </div>

        <!-- 飞书 -->
        <div class="space-y-3 rounded-xl border border-gray-100 p-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.tickets.settings.notify.feishuTitle') }}</p>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.notify.feishuHint') }}</p>
            </div>
            <Toggle v-model="form.admin_feishu_enabled" data-testid="ticket-feishu-toggle" />
          </div>

          <div class="grid gap-3 lg:grid-cols-2">
            <div>
              <label class="input-label mb-1.5 block text-xs font-medium text-gray-600 dark:text-dark-300">
                {{ t('admin.tickets.settings.notify.webhookUrl') }}
              </label>
              <input
                v-model="webhookUrlInput"
                type="url"
                class="input w-full font-mono text-xs"
                data-testid="ticket-feishu-webhook"
                :disabled="clearWebhook"
                :placeholder="t('admin.tickets.settings.notify.webhookPlaceholder')"
                @input="clearWebhook = false"
              />
              <p v-if="clearWebhook" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
                {{ t('admin.tickets.settings.notify.willClear') }}
                <button type="button" class="underline" @click="clearWebhook = false">{{ t('admin.tickets.settings.notify.undoClear') }}</button>
              </p>
              <p v-else-if="settings?.admin_feishu_webhook_configured" class="mt-1 break-all text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.tickets.settings.notify.configured') }}
                <span class="font-mono">{{ settings.admin_feishu_webhook_url_masked }}</span>
              </p>
              <p v-else class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.tickets.settings.notify.notConfigured') }}</p>
            </div>

            <div>
              <label class="input-label mb-1.5 block text-xs font-medium text-gray-600 dark:text-dark-300">
                {{ t('admin.tickets.settings.notify.secret') }}
              </label>
              <input
                v-model="secretInput"
                type="password"
                autocomplete="new-password"
                class="input w-full font-mono text-xs"
                data-testid="ticket-feishu-secret"
                :disabled="clearSecret"
                :placeholder="t('admin.tickets.settings.notify.secretPlaceholder')"
                @input="clearSecret = false"
              />
              <p v-if="clearSecret" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
                {{ t('admin.tickets.settings.notify.willClear') }}
                <button type="button" class="underline" @click="clearSecret = false">{{ t('admin.tickets.settings.notify.undoClear') }}</button>
              </p>
              <p v-else-if="settings?.admin_feishu_secret_configured" class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.tickets.settings.notify.secretConfigured') }}
              </p>
              <p v-else class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.tickets.settings.notify.secretNotConfigured') }}</p>
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="testingChannel !== null"
              data-testid="ticket-test-webhook"
              @click="requestClearWebhook"
            >
              {{ t('admin.tickets.settings.notify.clearWebhook') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="testingChannel !== null"
              @click="requestClearSecret"
            >
              {{ t('admin.tickets.settings.notify.clearSecret') }}
            </button>
            <span class="mx-1 hidden h-6 w-px bg-gray-200 dark:bg-dark-700 sm:block" />
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="testingChannel !== null"
              data-testid="ticket-test-email"
              @click="sendTest('email')"
            >
              <Icon name="mail" size="sm" />
              {{ t('admin.tickets.settings.notify.testEmail') }}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="testingChannel !== null"
              data-testid="ticket-test-feishu"
              @click="sendTest('feishu')"
            >
              <Icon name="arrowRight" size="sm" />
              {{ t('admin.tickets.settings.notify.testFeishu') }}
            </button>
          </div>
          <p class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.tickets.settings.notify.testHint') }}</p>
        </div>
      </section>

      <!-- 业务限流 -->
      <section class="card !rounded-2xl space-y-4 p-4 sm:p-5">
        <div>
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.settings.limits.title') }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.limits.hint') }}</p>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <div v-for="field in limitFields" :key="field.key">
            <label class="input-label mb-1.5 block text-xs font-medium text-gray-600 dark:text-dark-300">
              {{ t(field.labelKey) }}
            </label>
            <input
              v-model.number="numbers[field.key]"
              type="number"
              class="input w-full"
              :min="field.min"
              :max="field.max"
              :data-testid="`ticket-setting-${field.key}`"
            />
            <p v-if="fieldErrors[field.key]" class="mt-1 text-xs text-red-600 dark:text-red-400">{{ fieldErrors[field.key] }}</p>
            <p v-else class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t(field.hintKey, { min: field.min, max: field.max }) }}</p>
          </div>
        </div>
      </section>

      <!-- Agent 限流 -->
      <section class="card !rounded-2xl space-y-4 p-4 sm:p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.tickets.settings.agent.title') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.agent.hint') }}</p>
          </div>
          <div class="flex items-center gap-3">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.tickets.settings.agent.enabled') }}</span>
            <Toggle v-model="form.agent_enabled" data-testid="ticket-agent-toggle" />
          </div>
        </div>
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          <div v-for="field in agentFields" :key="field.key">
            <label class="input-label mb-1.5 block text-xs font-medium text-gray-600 dark:text-dark-300">
              {{ t(field.labelKey) }}
            </label>
            <input
              v-model.number="numbers[field.key]"
              type="number"
              class="input w-full"
              :min="field.min"
              :max="field.max"
              :data-testid="`ticket-setting-${field.key}`"
            />
            <p v-if="fieldErrors[field.key]" class="mt-1 text-xs text-red-600 dark:text-red-400">{{ fieldErrors[field.key] }}</p>
            <p v-else class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t(field.hintKey, { min: field.min, max: field.max }) }}</p>
          </div>
        </div>
      </section>

      <div class="flex flex-wrap items-center justify-end gap-2">
        <p v-if="hasErrors" class="mr-auto text-xs text-red-600 dark:text-red-400">{{ t('admin.tickets.settings.fixErrors') }}</p>
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="loadSettings">
          {{ t('admin.tickets.settings.reset') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="saving || hasErrors"
          data-testid="ticket-settings-save"
          @click="save"
        >
          <Icon name="check" size="sm" />
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { TicketSettings, TicketSettingsUpdate } from '@/types/tickets'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type NumericKey =
  | 'notify_throttle_seconds'
  | 'max_open_tickets_per_user'
  | 'create_per_hour_per_user'
  | 'messages_per_minute_per_user'
  | 'max_subject_length'
  | 'max_body_length'
  | 'agent_default_qps'
  | 'agent_max_qps'
  | 'agent_user_total_qps'
  | 'agent_admin_total_qps'
  | 'agent_max_keys_per_user'
  | 'agent_failed_auth_per_minute_per_ip'
  | 'agent_ip_requests_per_minute'

interface NumericField {
  key: NumericKey
  labelKey: string
  hintKey: string
  min: number
  max: number
}

const DEFAULT_MAX_NOTIFY_EMAILS = 100

const limitFields: NumericField[] = [
  { key: 'notify_throttle_seconds', labelKey: 'admin.tickets.settings.fields.notifyThrottle', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 0, max: 3600 },
  { key: 'max_open_tickets_per_user', labelKey: 'admin.tickets.settings.fields.maxOpenTickets', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'create_per_hour_per_user', labelKey: 'admin.tickets.settings.fields.createPerHour', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 10000 },
  { key: 'messages_per_minute_per_user', labelKey: 'admin.tickets.settings.fields.messagesPerMinute', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 10000 },
  { key: 'max_subject_length', labelKey: 'admin.tickets.settings.fields.maxSubjectLength', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'max_body_length', labelKey: 'admin.tickets.settings.fields.maxBodyLength', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 100000 },
]

const agentFields: NumericField[] = [
  { key: 'agent_default_qps', labelKey: 'admin.tickets.settings.fields.agentDefaultQps', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'agent_max_qps', labelKey: 'admin.tickets.settings.fields.agentMaxQps', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'agent_user_total_qps', labelKey: 'admin.tickets.settings.fields.agentUserTotalQps', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'agent_admin_total_qps', labelKey: 'admin.tickets.settings.fields.agentAdminTotalQps', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'agent_max_keys_per_user', labelKey: 'admin.tickets.settings.fields.agentMaxKeys', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 1000 },
  { key: 'agent_failed_auth_per_minute_per_ip', labelKey: 'admin.tickets.settings.fields.agentFailedAuth', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 1, max: 10000 },
  { key: 'agent_ip_requests_per_minute', labelKey: 'admin.tickets.settings.fields.agentIpRequests', hintKey: 'admin.tickets.settings.fields.rangeHint', min: 0, max: 100000 },
]

const { t } = useI18n()
const appStore = useAppStore()

const settings = ref<TicketSettings | null>(null)
const loading = ref(false)
const saving = ref(false)
const testingChannel = ref<'email' | 'feishu' | null>(null)
const emailDraft = ref('')
const emailError = ref('')
const webhookUrlInput = ref('')
const secretInput = ref('')
const clearWebhook = ref(false)
const clearSecret = ref(false)

const form = reactive({
  enabled: false,
  admin_email_enabled: true,
  admin_notify_emails: [] as string[],
  admin_feishu_enabled: false,
  agent_enabled: true,
})

const numbers = reactive<Record<NumericKey, number | string>>({
  notify_throttle_seconds: 60,
  max_open_tickets_per_user: 5,
  create_per_hour_per_user: 10,
  messages_per_minute_per_user: 20,
  max_subject_length: 200,
  max_body_length: 5000,
  agent_default_qps: 2,
  agent_max_qps: 10,
  agent_user_total_qps: 5,
  agent_admin_total_qps: 20,
  agent_max_keys_per_user: 5,
  agent_failed_auth_per_minute_per_ip: 20,
  agent_ip_requests_per_minute: 600,
})

function numberValue(key: NumericKey): number | null {
  const raw = numbers[key]
  if (raw === '' || raw === null || raw === undefined) return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}

const fieldErrors = computed<Partial<Record<NumericKey, string>>>(() => {
  const errors: Partial<Record<NumericKey, string>> = {}
  for (const field of [...limitFields, ...agentFields]) {
    const value = numberValue(field.key)
    if (value === null || !Number.isInteger(value) || value < field.min || value > field.max) {
      errors[field.key] = t('admin.tickets.settings.fields.invalid', { min: field.min, max: field.max })
    }
  }
  if (!errors.agent_default_qps && !errors.agent_max_qps) {
    const defaultQps = numberValue('agent_default_qps') ?? 0
    const maxQps = numberValue('agent_max_qps') ?? 0
    if (defaultQps > maxQps) errors.agent_default_qps = t('admin.tickets.settings.fields.defaultQpsTooHigh')
  }
  return errors
})

const hasErrors = computed(() => Object.keys(fieldErrors.value).length > 0)

function applySettings(value: TicketSettings) {
  settings.value = value
  form.enabled = Boolean(value.enabled)
  form.admin_email_enabled = Boolean(value.admin_email_enabled)
  form.admin_notify_emails = Array.isArray(value.admin_notify_emails) ? [...value.admin_notify_emails] : []
  form.admin_feishu_enabled = Boolean(value.admin_feishu_enabled)
  form.agent_enabled = Boolean(value.agent_enabled)
  for (const field of [...limitFields, ...agentFields]) {
    numbers[field.key] = Number(value[field.key])
  }
  webhookUrlInput.value = ''
  secretInput.value = ''
  clearWebhook.value = false
  clearSecret.value = false
}

async function loadSettings() {
  loading.value = true
  try {
    const result = await adminAPI.tickets.getSettings()
    applySettings(result)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.settings.loadFailed')))
  } finally {
    loading.value = false
  }
}

function addEmail() {
  emailError.value = ''
  const email = emailDraft.value.trim()
  if (!email) return
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) || email.length > 254) {
    emailError.value = t('admin.tickets.settings.notify.emailInvalid')
    return
  }
  if (form.admin_notify_emails.includes(email)) {
    emailDraft.value = ''
    return
  }
  // 与后端 validateTicketSettings 对齐：最多 100 个通知邮箱
  if (form.admin_notify_emails.length >= DEFAULT_MAX_NOTIFY_EMAILS) {
    emailError.value = t('admin.tickets.settings.notify.emailLimit', { max: DEFAULT_MAX_NOTIFY_EMAILS })
    return
  }
  form.admin_notify_emails = [...form.admin_notify_emails, email]
  emailDraft.value = ''
}

function removeEmail(email: string) {
  form.admin_notify_emails = form.admin_notify_emails.filter((item) => item !== email)
}

// 写入专用字段的清除是「保存后生效」：只标记状态，不立刻调用接口。
function requestClearWebhook() {
  clearWebhook.value = true
  webhookUrlInput.value = ''
}

function requestClearSecret() {
  clearSecret.value = true
  secretInput.value = ''
}

function buildPayload(): TicketSettingsUpdate {
  const payload: TicketSettingsUpdate = {
    enabled: form.enabled,
    admin_email_enabled: form.admin_email_enabled,
    admin_notify_emails: [...form.admin_notify_emails],
    admin_feishu_enabled: form.admin_feishu_enabled,
    agent_enabled: form.agent_enabled,
  }
  for (const field of [...limitFields, ...agentFields]) {
    const value = numberValue(field.key)
    if (value !== null) payload[field.key] = value
  }
  // 写入专用字段：空字符串 = 清除，未改动 = 不发送（后端保持原值）
  if (clearWebhook.value) {
    payload.admin_feishu_webhook_url = ''
  } else if (webhookUrlInput.value.trim()) {
    payload.admin_feishu_webhook_url = webhookUrlInput.value.trim()
  }
  if (clearSecret.value) {
    payload.admin_feishu_secret = ''
  } else if (secretInput.value.trim()) {
    payload.admin_feishu_secret = secretInput.value.trim()
  }
  return payload
}

async function save() {
  if (saving.value || hasErrors.value) return
  saving.value = true
  try {
    const result = await adminAPI.tickets.updateSettings(buildPayload())
    applySettings(result)
    appStore.showSuccess(t('admin.tickets.settings.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.settings.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function sendTest(channel: 'email' | 'feishu') {
  if (testingChannel.value) return
  testingChannel.value = channel
  try {
    await adminAPI.tickets.testNotification(channel)
    appStore.showSuccess(t('admin.tickets.settings.notify.testSent'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.tickets.settings.notify.testFailed')))
  } finally {
    testingChannel.value = null
  }
}

onMounted(() => {
  void loadSettings()
})
</script>
