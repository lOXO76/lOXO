<template>
  <BaseDialog
    :show="show"
    :title="partner ? t('admin.rewardTasks.partners.form.editTitle') : t('admin.rewardTasks.partners.form.createTitle')"
    width="wide"
    @close="emit('close')"
  >
    <form id="reward-partner-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="reward-partner-name" class="input-label">
            {{ t('admin.rewardTasks.partners.form.name') }} <span class="text-red-500">*</span>
          </label>
          <input
            id="reward-partner-name"
            v-model="form.name"
            type="text"
            maxlength="100"
            required
            class="input"
            :placeholder="t('admin.rewardTasks.partners.form.namePlaceholder')"
            data-testid="reward-partner-name"
          />
        </div>
        <div>
          <label for="reward-partner-contact" class="input-label">{{ t('admin.rewardTasks.partners.form.contact') }}</label>
          <input id="reward-partner-contact" v-model="form.contact" type="text" maxlength="200" class="input" />
        </div>
      </div>

      <div>
        <label for="reward-partner-domains" class="input-label">{{ t('admin.rewardTasks.partners.form.domains') }}</label>
        <textarea
          id="reward-partner-domains"
          v-model="form.allowed_domains"
          rows="4"
          class="input resize-y font-mono text-sm"
          :placeholder="t('admin.rewardTasks.partners.form.domainsPlaceholder')"
          data-testid="reward-partner-domains"
        />
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.partners.form.domainsHint') }}</p>
      </div>

      <div>
        <label for="reward-partner-notes" class="input-label">{{ t('admin.rewardTasks.partners.form.notes') }}</label>
        <textarea id="reward-partner-notes" v-model="form.notes" rows="3" maxlength="2000" class="input resize-y" />
      </div>

      <div class="flex items-center justify-between">
        <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.rewardTasks.partners.form.enabled') }}</span>
        <Toggle v-model="form.enabled" />
      </div>

      <div v-if="partner" class="rounded-xl bg-gray-50 p-4 text-xs text-gray-500 dark:bg-dark-900/40 dark:text-dark-400">
        <p class="font-mono">{{ t('admin.rewardTasks.partners.secret.appKey') }}: {{ partner.app_key }}</p>
        <p class="mt-1 font-mono">{{ t('admin.rewardTasks.partners.secret.appSecret') }}: ****{{ partner.secret_hint }}</p>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="reward-partner-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? t('common.submitting') : partner ? t('common.update') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type {
  AdminRewardTaskPartner,
  AdminRewardTaskPartnerPayload,
} from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  partner: AdminRewardTaskPartner | null
}>()
const emit = defineEmits<{
  (e: 'close'): void
  // 携带后端返回的对象：创建时含一次性明文 app_secret
  (e: 'saved', partner: AdminRewardTaskPartner): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const submitting = ref(false)

const form = reactive({
  name: '',
  allowed_domains: '',
  contact: '',
  notes: '',
  enabled: true,
})

// 每行一个域名；后端会再做规范化（去空格、小写、去重）
function parseDomains(text: string): string[] {
  const seen = new Set<string>()
  for (const raw of text.split(/[\n,]/)) {
    const domain = raw.trim().toLowerCase()
    if (domain) seen.add(domain)
  }
  return [...seen]
}

// 合法 host，或以 *. 开头的通配子域；不含协议 / 端口 / 路径
function isValidDomain(value: string): boolean {
  const host = value.startsWith('*.') ? value.slice(2) : value
  if (!host || host.length > 253) return false
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/.test(host)
}

function validate(domains: string[]): string | null {
  const name = form.name.trim()
  if (!name || name.length > 100) return t('admin.rewardTasks.partners.form.invalidName')
  if (form.contact.length > 200) return t('admin.rewardTasks.partners.form.invalidContact')
  if (form.notes.length > 2000) return t('admin.rewardTasks.partners.form.invalidNotes')
  if (domains.length > 20 || domains.some((domain) => !isValidDomain(domain))) {
    return t('admin.rewardTasks.partners.form.invalidDomains')
  }
  return null
}

async function handleSubmit() {
  if (submitting.value) return
  const domains = parseDomains(form.allowed_domains)
  const error = validate(domains)
  if (error) {
    appStore.showError(error)
    return
  }
  const payload: AdminRewardTaskPartnerPayload = {
    name: form.name.trim(),
    allowed_domains: domains,
    contact: form.contact.trim(),
    notes: form.notes.trim(),
    enabled: form.enabled,
  }
  submitting.value = true
  try {
    const saved = props.partner
      ? await adminAPI.rewardTasks.updatePartner(props.partner.id, payload)
      : await adminAPI.rewardTasks.createPartner(payload)
    appStore.showSuccess(
      props.partner ? t('admin.rewardTasks.partners.updateSuccess') : t('admin.rewardTasks.partners.createSuccess'),
    )
    emit('saved', saved)
    emit('close')
  } catch (caught) {
    appStore.showError(extractApiErrorMessage(caught, t('admin.rewardTasks.partners.saveFailed')))
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  form.name = props.partner?.name ?? ''
  form.allowed_domains = (props.partner?.allowed_domains ?? []).join('\n')
  form.contact = props.partner?.contact ?? ''
  form.notes = props.partner?.notes ?? ''
  form.enabled = props.partner?.enabled ?? true
}

watch(
  () => [props.show, props.partner] as const,
  ([show]) => {
    if (!show) return
    resetForm()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  submitting.value = false
})
</script>
