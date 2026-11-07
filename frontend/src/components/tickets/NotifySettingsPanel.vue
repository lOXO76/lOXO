<template>
  <section class="space-y-5">
    <div v-if="loading" class="card px-6 py-12 text-center text-sm text-gray-500 dark:text-dark-400" role="status">
      {{ t('tickets.notify.loading') }}
    </div>

    <div v-else-if="loadError" class="card px-6 py-12 text-center" role="alert">
      <CircleAlert class="mx-auto h-10 w-10 text-red-500" aria-hidden="true" />
      <p class="mt-4 text-sm text-gray-600 dark:text-dark-300">{{ loadError }}</p>
      <button type="button" class="btn btn-secondary mt-4" data-testid="notify-retry" @click="load">
        {{ t('tickets.common.retry') }}
      </button>
    </div>

    <template v-else-if="preferences">
      <!-- 邮件通知 -->
      <div class="card p-5" data-testid="notify-email">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('tickets.notify.email.title') }}</h2>
            <p v-if="preferences.email" class="mt-1 break-all text-sm text-gray-500 dark:text-dark-400">
              {{ t('tickets.notify.email.address', { email: preferences.email }) }}
            </p>
            <p v-else class="mt-1 text-sm text-amber-600 dark:text-amber-400">
              {{ t('tickets.notify.email.missing') }}
              <RouterLink to="/profile" class="ml-1 font-medium underline">{{ t('tickets.notify.email.bindEmail') }}</RouterLink>
            </p>
          </div>
          <Toggle v-model="form.email_enabled" :disabled="saving" data-testid="notify-email-toggle" />
        </div>
        <div class="mt-4">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="testing !== null || !preferences.email"
            data-testid="notify-test-email"
            @click="sendTest('email')"
          >
            {{ testing === 'email' ? t('tickets.notify.feishu.testing') : t('tickets.notify.feishu.test') }}
          </button>
        </div>
      </div>

      <!-- 飞书通知 -->
      <div class="card p-5" data-testid="notify-feishu">
        <div class="flex items-start justify-between gap-4">
          <div class="min-w-0">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('tickets.notify.feishu.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.notify.description') }}</p>
          </div>
          <Toggle v-model="form.feishu_enabled" :disabled="saving" data-testid="notify-feishu-toggle" />
        </div>

        <div class="mt-4 space-y-4">
          <div>
            <label class="input-label mb-1.5 block" for="notify-feishu-webhook">{{ t('tickets.notify.feishu.webhook') }}</label>
            <input
              id="notify-feishu-webhook"
              v-model="form.feishu_webhook_url"
              type="url"
              inputmode="url"
              autocomplete="off"
              spellcheck="false"
              class="input w-full font-mono text-xs"
              :placeholder="t('tickets.notify.feishu.webhookPlaceholder')"
              data-testid="notify-feishu-webhook"
            />
            <p v-if="preferences.feishu_webhook_configured" class="input-hint mt-1.5 break-all">
              {{ t('tickets.notify.feishu.webhookConfigured') }}
              <span v-if="preferences.feishu_webhook_url_masked" class="font-mono">
                · {{ t('tickets.notify.feishu.webhookMasked', { url: preferences.feishu_webhook_url_masked }) }}
              </span>
            </p>
            <p class="input-hint mt-1.5">{{ t('tickets.notify.feishu.webhookHint') }}</p>
          </div>

          <div>
            <label class="input-label mb-1.5 block" for="notify-feishu-secret">
              {{ t('tickets.notify.feishu.secret') }}
              <span class="ml-1 text-xs font-normal text-gray-400 dark:text-dark-500">({{ t('tickets.notify.feishu.secretOptional') }})</span>
            </label>
            <input
              id="notify-feishu-secret"
              v-model="form.feishu_secret"
              type="password"
              autocomplete="new-password"
              class="input w-full font-mono text-xs"
              :placeholder="t('tickets.notify.feishu.secretPlaceholder')"
              data-testid="notify-feishu-secret"
            />
            <p v-if="preferences.feishu_secret_configured" class="input-hint mt-1.5">{{ t('tickets.notify.feishu.secretConfigured') }}</p>
            <p class="input-hint mt-1.5">{{ t('tickets.notify.feishu.secretHint') }}</p>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-2">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="testing !== null || !preferences.feishu_webhook_configured"
            data-testid="notify-test-feishu"
            @click="sendTest('feishu')"
          >
            {{ testing === 'feishu' ? t('tickets.notify.feishu.testing') : t('tickets.notify.feishu.test') }}
          </button>
          <button
            v-if="preferences.feishu_webhook_configured || preferences.feishu_secret_configured"
            type="button"
            class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
            :disabled="saving"
            data-testid="notify-feishu-clear"
            @click="showClearConfirm = true"
          >
            {{ t('tickets.notify.feishu.clear') }}
          </button>
        </div>
      </div>

      <!-- 保存 -->
      <div class="flex flex-wrap items-center gap-3">
        <button type="button" class="btn btn-primary" :disabled="saving || !dirty" data-testid="notify-save" @click="save">
          {{ saving ? t('tickets.common.saving') : t('tickets.notify.feishu.save') }}
        </button>
        <p v-if="statusMessage" class="text-sm" :class="statusIsError ? 'text-red-600 dark:text-red-400' : 'text-emerald-600 dark:text-emerald-400'" role="status">
          {{ statusMessage }}
        </p>
      </div>

      <!-- 飞书配置说明 -->
      <div class="card p-5">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('tickets.notify.feishu.helpTitle') }}</h2>
        <ol class="mt-3 space-y-2 text-sm text-gray-600 dark:text-dark-300">
          <li v-for="step in 4" :key="step" class="flex gap-2">
            <span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ step }}</span>
            <span class="min-w-0 break-words">{{ t(`tickets.notify.feishu.helpSteps.step${step}`) }}</span>
          </li>
        </ol>
        <p class="mt-4 rounded-xl bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
          {{ t('tickets.notify.feishu.securityNote') }}
        </p>
      </div>
    </template>

    <ConfirmDialog
      :show="showClearConfirm"
      :title="t('tickets.notify.feishu.clearConfirmTitle')"
      :message="t('tickets.notify.feishu.clearConfirmMessage')"
      :confirm-text="t('tickets.notify.feishu.clear')"
      :cancel-text="t('tickets.common.cancel')"
      danger
      @cancel="showClearConfirm = false"
      @confirm="clearFeishu"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { CircleAlert } from '@lucide/vue'
import Toggle from '@/components/common/Toggle.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import {
  getTicketNotifyPreferences,
  sendTicketNotifyTest,
  updateTicketNotifyPreferences,
} from '@/api/tickets'
import { useAppStore } from '@/stores/app'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import type { TicketNotifyChannel, TicketNotifyPreference } from '@/types/tickets'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const loadError = ref('')
const preferences = ref<TicketNotifyPreference | null>(null)
const saving = ref(false)
const testing = ref<TicketNotifyChannel | null>(null)
const statusMessage = ref('')
const statusIsError = ref(false)
const showClearConfirm = ref(false)

const form = reactive({
  email_enabled: false,
  feishu_enabled: false,
  feishu_webhook_url: '',
  feishu_secret: '',
})

const dirty = computed(() => {
  const current = preferences.value
  if (!current) return false
  return (
    form.email_enabled !== current.email_enabled ||
    form.feishu_enabled !== current.feishu_enabled ||
    form.feishu_webhook_url.trim() !== '' ||
    form.feishu_secret.trim() !== ''
  )
})

function applyPreference(next: TicketNotifyPreference): void {
  preferences.value = next
  form.email_enabled = next.email_enabled
  form.feishu_enabled = next.feishu_enabled
  form.feishu_webhook_url = ''
  form.feishu_secret = ''
}

function setStatus(message: string, isError = false): void {
  statusMessage.value = message
  statusIsError.value = isError
}

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    applyPreference(await getTicketNotifyPreferences())
  } catch (error) {
    loadError.value = resolveTicketErrorMessage(error, t, 'tickets.notify.loadFailed')
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (saving.value) return
  saving.value = true
  statusMessage.value = ''
  try {
    const updated = await updateTicketNotifyPreferences({
      email_enabled: form.email_enabled,
      feishu_enabled: form.feishu_enabled,
      // 留空表示不修改；填写则覆盖原配置。
      ...(form.feishu_webhook_url.trim() ? { feishu_webhook_url: form.feishu_webhook_url.trim() } : {}),
      ...(form.feishu_secret.trim() ? { feishu_secret: form.feishu_secret.trim() } : {}),
    })
    applyPreference(updated)
    setStatus(t('tickets.notify.feishu.saved'))
    appStore.showSuccess(t('tickets.notify.feishu.saved'))
  } catch (error) {
    setStatus(resolveTicketErrorMessage(error, t, 'tickets.notify.feishu.saveFailed'), true)
  } finally {
    saving.value = false
  }
}

async function clearFeishu(): Promise<void> {
  showClearConfirm.value = false
  saving.value = true
  statusMessage.value = ''
  try {
    // 空字符串 = 清除（SPEC 6.1：缺省/null 保持不变，空串清除）。
    const updated = await updateTicketNotifyPreferences({
      email_enabled: form.email_enabled,
      feishu_enabled: false,
      feishu_webhook_url: '',
      feishu_secret: '',
    })
    applyPreference(updated)
    setStatus(t('tickets.notify.feishu.cleared'))
    appStore.showSuccess(t('tickets.notify.feishu.cleared'))
  } catch (error) {
    setStatus(resolveTicketErrorMessage(error, t, 'tickets.notify.feishu.saveFailed'), true)
  } finally {
    saving.value = false
  }
}

async function sendTest(channel: TicketNotifyChannel): Promise<void> {
  if (testing.value) return
  if (channel === 'email' && !preferences.value?.email) {
    appStore.showWarning(t('tickets.notify.feishu.testNeedEmail'))
    return
  }
  if (channel === 'feishu' && !preferences.value?.feishu_webhook_configured) {
    appStore.showWarning(t('tickets.notify.feishu.testNeedWebhook'))
    return
  }
  testing.value = channel
  statusMessage.value = ''
  try {
    await sendTicketNotifyTest(channel)
    appStore.showSuccess(
      channel === 'email' ? t('tickets.notify.feishu.testEmailSuccess') : t('tickets.notify.feishu.testFeishuSuccess'),
    )
  } catch (error) {
    appStore.showError(resolveTicketErrorMessage(error, t, 'tickets.notify.feishu.testFailed'))
  } finally {
    testing.value = null
  }
}

onMounted(() => {
  void load()
})

defineExpose({ load })
</script>
