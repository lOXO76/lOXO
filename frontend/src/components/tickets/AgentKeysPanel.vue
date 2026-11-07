<template>
  <section class="space-y-5">
    <!-- 概要与操作 -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('tickets.agent.title') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.agent.description') }}</p>
        <p class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-500 dark:text-dark-400">
          <span>{{ t('tickets.agent.limitsHint', { max: config.agent_max_qps, total: config.agent_user_total_qps }) }}</span>
          <span>{{ t('tickets.agent.maxKeysHint', { max: config.agent_max_keys_per_user }) }}</span>
        </p>
      </div>
      <div class="flex shrink-0 flex-wrap gap-2">
        <RouterLink to="/docs/ticket-agent" class="btn btn-secondary" data-testid="agent-docs-link">
          <BookOpenIcon class="h-4 w-4" aria-hidden="true" />
          {{ t('tickets.agent.docs') }}
        </RouterLink>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!config.agent_enabled || keys.length >= config.agent_max_keys_per_user"
          data-testid="agent-key-create"
          @click="openCreate"
        >
          <PlusIcon class="h-4 w-4" aria-hidden="true" />
          {{ t('tickets.agent.create') }}
        </button>
      </div>
    </div>

    <p
      v-if="!config.agent_enabled"
      class="rounded-xl bg-amber-50 px-3 py-2 text-sm text-amber-700 dark:bg-amber-900/20 dark:text-amber-300"
      data-testid="agent-disabled-notice"
    >
      {{ t('tickets.errors.AGENT_API_DISABLED') }}
    </p>

    <!-- 密钥列表 -->
    <div class="card overflow-hidden">
      <div v-if="errorMessage" class="px-5 py-10 text-center" role="alert">
        <p class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
        <button type="button" class="btn btn-secondary mt-3" data-testid="agent-retry" @click="load">
          {{ t('tickets.common.retry') }}
        </button>
      </div>
      <p v-else-if="loading" class="px-5 py-10 text-center text-sm text-gray-500 dark:text-dark-400" role="status">
        {{ t('tickets.agent.loading') }}
      </p>
      <p v-else-if="keys.length === 0" class="px-5 py-12 text-center text-sm text-gray-500 dark:text-dark-400" data-testid="agent-empty">
        {{ t('tickets.agent.empty') }}
      </p>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[46rem] text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            <tr>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.name') }}</th>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.scope') }}</th>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.qps') }}</th>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.status') }}</th>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.lastUsed') }}</th>
              <th class="px-4 py-3 font-medium" scope="col">{{ t('tickets.agent.table.expires') }}</th>
              <th class="px-4 py-3 text-right font-medium" scope="col">{{ t('tickets.agent.table.actions') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="key in keys" :key="key.id" class="align-top text-gray-700 dark:text-dark-200" :data-testid="`agent-key-${key.id}`">
              <td class="min-w-36 max-w-[14rem] px-4 py-4">
                <p class="break-words font-medium text-gray-900 dark:text-white">{{ key.name }}</p>
                <p class="mt-0.5 font-mono text-xs text-gray-500 dark:text-dark-400">{{ key.key_prefix }}…</p>
              </td>
              <td class="whitespace-nowrap px-4 py-4">
                <span
                  class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium"
                  :class="key.scope === 'admin'
                    ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
                    : 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'"
                >
                  {{ t(`tickets.agent.scope.${key.scope}`) }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-4 tabular-nums">
                <button
                  type="button"
                  class="rounded text-primary-600 underline-offset-2 hover:underline dark:text-primary-400"
                  :data-testid="`agent-key-qps-${key.id}`"
                  @click="openQps(key)"
                >
                  {{ key.qps_limit }}
                </button>
              </td>
              <td class="whitespace-nowrap px-4 py-4">
                <span
                  class="inline-flex rounded-md px-2 py-0.5 text-xs font-medium"
                  :class="key.status === 'active'
                    ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
                    : 'bg-gray-100 text-gray-500 dark:bg-dark-800 dark:text-dark-400'"
                >
                  {{ t(`tickets.agent.status.${key.status}`) }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-xs">
                <span v-if="key.last_used_at">{{ formatDateTime(key.last_used_at) }}</span>
                <span v-else class="text-gray-400 dark:text-dark-500">{{ t('tickets.agent.lastUsedNever') }}</span>
                <p v-if="key.last_used_ip" class="mt-0.5 font-mono text-[11px] text-gray-400 dark:text-dark-500">{{ key.last_used_ip }}</p>
              </td>
              <td class="whitespace-nowrap px-4 py-4 text-xs">
                <span v-if="key.expires_at">{{ formatDateTime(key.expires_at) }}</span>
                <span v-else class="text-gray-400 dark:text-dark-500">{{ t('tickets.agent.expiresNever') }}</span>
              </td>
              <td class="px-4 py-4">
                <div class="flex flex-wrap justify-end gap-1.5">
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    :disabled="busyId === key.id"
                    :data-testid="`agent-key-toggle-${key.id}`"
                    @click="toggleStatus(key)"
                  >
                    {{ key.status === 'active' ? t('tickets.agent.disable') : t('tickets.agent.enable') }}
                  </button>
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
                    :disabled="busyId === key.id"
                    :data-testid="`agent-key-delete-${key.id}`"
                    @click="askDelete(key)"
                  >
                    {{ t('tickets.agent.delete') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 给 Agent 的提示词 -->
    <div class="card p-5" data-testid="agent-prompt">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0">
          <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('tickets.agent.prompt.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('tickets.agent.prompt.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm shrink-0" data-testid="agent-prompt-copy" @click="copyPrompt">
          <CopyIcon class="h-3.5 w-3.5" aria-hidden="true" />
          {{ t('tickets.agent.prompt.copy') }}
        </button>
      </div>
      <pre class="mt-3 max-h-64 overflow-auto whitespace-pre-wrap break-words rounded-xl bg-gray-50 p-3 text-xs leading-relaxed text-gray-700 dark:bg-dark-900 dark:text-dark-200" data-testid="agent-prompt-text">{{ promptText }}</pre>
    </div>

    <!-- 新建密钥 -->
    <BaseDialog :show="showCreate" :title="t('tickets.agent.createDialog.title')" width="normal" @close="showCreate = false">
      <form id="agent-key-form" class="space-y-4" @submit.prevent="create">
        <Input
          v-model="createForm.name"
          :label="t('tickets.agent.createDialog.name')"
          :placeholder="t('tickets.agent.createDialog.namePlaceholder')"
          required
          data-testid="agent-key-name"
        />

        <Input
          v-model="createForm.qps"
          type="number"
          :label="t('tickets.agent.createDialog.qps')"
          :hint="t('tickets.agent.createDialog.qpsHint', { min: 1, max: config.agent_max_qps })"
          data-testid="agent-key-qps"
        />

        <div v-if="isAdmin">
          <label class="input-label mb-1.5 block" :for="scopeSelectId">{{ t('tickets.agent.createDialog.scope') }}</label>
          <Select
            :id="scopeSelectId"
            v-model="createForm.scope"
            :options="scopeOptions"
            data-testid="agent-key-scope"
          />
          <p v-if="createForm.scope === 'admin'" class="mt-2 flex items-start gap-1.5 rounded-xl bg-red-50 px-3 py-2 text-xs font-medium text-red-700 dark:bg-red-900/20 dark:text-red-300" data-testid="agent-scope-warning">
            <TriangleAlertIcon class="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span>{{ t('tickets.agent.scopeAdminWarning') }}</span>
          </p>
        </div>

        <div>
          <label class="input-label mb-1.5 block" for="agent-key-expires">{{ t('tickets.agent.createDialog.expires') }}</label>
          <input
            id="agent-key-expires"
            v-model="createForm.expires_local"
            type="datetime-local"
            class="input w-full"
            data-testid="agent-key-expires"
          />
          <p class="input-hint mt-1.5">{{ t('tickets.agent.createDialog.expiresHint') }}</p>
        </div>

        <p v-if="createError" class="rounded-xl bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300" role="alert">
          {{ createError }}
        </p>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" :disabled="creating" @click="showCreate = false">
            {{ t('tickets.common.cancel') }}
          </button>
          <button
            type="submit"
            form="agent-key-form"
            class="btn btn-primary"
            :disabled="creating"
            data-testid="agent-key-submit"
          >
            {{ creating ? t('tickets.agent.createDialog.submitting') : t('tickets.agent.createDialog.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- 修改 QPS -->
    <BaseDialog :show="editingKey !== null" :title="t('tickets.agent.editQps')" width="narrow" @close="editingKey = null">
      <form id="agent-qps-form" class="space-y-4" @submit.prevent="saveQps">
        <Input
          v-model="editQps"
          type="number"
          :label="t('tickets.agent.createDialog.qps')"
          :hint="t('tickets.agent.createDialog.qpsHint', { min: 1, max: config.agent_max_qps })"
          data-testid="agent-qps-input"
        />
        <p v-if="qpsError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ qpsError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="editingKey = null">{{ t('tickets.common.cancel') }}</button>
          <button type="submit" form="agent-qps-form" class="btn btn-primary" :disabled="savingQps" data-testid="agent-qps-submit">
            {{ savingQps ? t('tickets.common.saving') : t('tickets.common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- 删除确认 -->
    <ConfirmDialog
      :show="deletingKey !== null"
      :title="t('tickets.agent.deleteTitle')"
      :message="t('tickets.agent.deleteMessage')"
      :confirm-text="t('tickets.agent.delete')"
      :cancel-text="t('tickets.common.cancel')"
      danger
      @cancel="deletingKey = null"
      @confirm="confirmDelete"
    />

    <!-- 创建成功（明文只显示一次） -->
    <AgentKeyCreatedDialog :show="createdKey !== null" :created-key="createdKey" @close="createdKey = null" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import {
  BookOpen as BookOpenIcon,
  Copy as CopyIcon,
  Plus as PlusIcon,
  TriangleAlert as TriangleAlertIcon,
} from '@lucide/vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Input from '@/components/common/Input.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import AgentKeyCreatedDialog from './AgentKeyCreatedDialog.vue'
import {
  createTicketAgentKey,
  deleteTicketAgentKey,
  listTicketAgentKeys,
  updateTicketAgentKey,
} from '@/api/tickets'
import { buildApiUrl } from '@/api/url'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatDateTime } from '@/utils/format'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import type { TicketAgentKey, TicketAgentKeyCreated, TicketPublicConfig } from '@/types/tickets'

const props = defineProps<{ config: TicketPublicConfig }>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const scopeSelectId = 'agent-key-scope-select'
const isAdmin = computed(() => authStore.user?.role === 'admin')

const keys = ref<TicketAgentKey[]>([])
const loading = ref(true)
const errorMessage = ref('')
const busyId = ref<number | null>(null)

const showCreate = ref(false)
const creating = ref(false)
const createError = ref('')
const createForm = reactive({
  name: '',
  qps: String(props.config.agent_default_qps),
  scope: 'user' as 'user' | 'admin',
  expires_local: '',
})

const createdKey = ref<TicketAgentKeyCreated | null>(null)

const editingKey = ref<TicketAgentKey | null>(null)
const editQps = ref('')
const savingQps = ref(false)
const qpsError = ref('')

const deletingKey = ref<TicketAgentKey | null>(null)

const scopeOptions = computed<SelectOption[]>(() =>
  isAdmin.value
    ? [
        { value: 'user', label: t('tickets.agent.scope.user') },
        { value: 'admin', label: t('tickets.agent.scope.admin') },
      ]
    : [{ value: 'user', label: t('tickets.agent.scope.user') }],
)

/** 给 Agent 的提示词：站点 origin + docs.md 地址 + key 占位符。 */
const promptText = computed(() => {
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  let docsUrl = '/api/v1/agent/docs.md'
  let baseUrl = '/api/v1/agent'
  try {
    docsUrl = new URL(buildApiUrl('/agent/docs.md'), origin).toString()
    baseUrl = new URL(buildApiUrl('/agent'), origin).toString().replace(/\/+$/, '')
  } catch {
    // 保底使用相对路径，仍能复制出可用提示词。
  }
  return [
    t('tickets.agent.prompt.intro'),
    t('tickets.agent.prompt.docsLine', { url: docsUrl }),
    t('tickets.agent.prompt.baseLine', { base: baseUrl }),
    t('tickets.agent.prompt.authLine'),
    t('tickets.agent.prompt.keyLine', { key: 'lxt_REPLACE_WITH_YOUR_KEY' }),
    t('tickets.agent.prompt.usageLine'),
    t('tickets.agent.prompt.limitsLine'),
  ].join('\n')
})

async function load(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    keys.value = await listTicketAgentKeys()
  } catch (error) {
    errorMessage.value = resolveTicketErrorMessage(error, t, 'tickets.agent.loadFailed')
  } finally {
    loading.value = false
  }
}

function openCreate(): void {
  createForm.name = ''
  createForm.qps = String(props.config.agent_default_qps)
  createForm.scope = 'user'
  createForm.expires_local = ''
  createError.value = ''
  showCreate.value = true
}

function parseQps(raw: string): number | null {
  const value = Number.parseInt(raw, 10)
  if (!Number.isFinite(value) || value < 1 || value > props.config.agent_max_qps) return null
  return value
}

async function create(): Promise<void> {
  if (creating.value) return
  createError.value = ''
  const name = createForm.name.trim().slice(0, 64)
  if (!name) {
    createError.value = t('tickets.agent.createDialog.nameRequired')
    return
  }
  const qps = parseQps(createForm.qps)
  if (qps === null) {
    createError.value = t('tickets.agent.createDialog.qpsInvalid', { min: 1, max: props.config.agent_max_qps })
    return
  }

  creating.value = true
  try {
    const created = await createTicketAgentKey({
      name,
      // user 是默认值，但显式传 scope 便于后端记录（admin 仅 admin 角色可选）。
      scope: isAdmin.value ? createForm.scope : 'user',
      qps_limit: qps,
      ...(createForm.expires_local ? { expires_at: new Date(createForm.expires_local).toISOString() } : {}),
    })
    showCreate.value = false
    createdKey.value = created
    await load()
  } catch (error) {
    createError.value = resolveTicketErrorMessage(error, t)
  } finally {
    creating.value = false
  }
}

function openQps(key: TicketAgentKey): void {
  editingKey.value = key
  editQps.value = String(key.qps_limit)
  qpsError.value = ''
}

async function saveQps(): Promise<void> {
  const key = editingKey.value
  if (!key || savingQps.value) return
  const qps = parseQps(editQps.value)
  if (qps === null) {
    qpsError.value = t('tickets.agent.createDialog.qpsInvalid', { min: 1, max: props.config.agent_max_qps })
    return
  }
  savingQps.value = true
  qpsError.value = ''
  try {
    const updated = await updateTicketAgentKey(key.id, { qps_limit: qps })
    keys.value = keys.value.map(item => (item.id === updated.id ? updated : item))
    editingKey.value = null
    appStore.showSuccess(t('tickets.agent.qpsUpdated'))
  } catch (error) {
    qpsError.value = resolveTicketErrorMessage(error, t)
  } finally {
    savingQps.value = false
  }
}

async function toggleStatus(key: TicketAgentKey): Promise<void> {
  if (busyId.value !== null) return
  busyId.value = key.id
  try {
    const nextStatus = key.status === 'active' ? 'disabled' : 'active'
    const updated = await updateTicketAgentKey(key.id, { status: nextStatus })
    keys.value = keys.value.map(item => (item.id === updated.id ? updated : item))
    appStore.showSuccess(nextStatus === 'active' ? t('tickets.agent.enabled') : t('tickets.agent.disabled'))
  } catch (error) {
    appStore.showError(resolveTicketErrorMessage(error, t))
  } finally {
    busyId.value = null
  }
}

function askDelete(key: TicketAgentKey): void {
  deletingKey.value = key
}

async function confirmDelete(): Promise<void> {
  const key = deletingKey.value
  deletingKey.value = null
  if (!key) return
  busyId.value = key.id
  try {
    await deleteTicketAgentKey(key.id)
    keys.value = keys.value.filter(item => item.id !== key.id)
    appStore.showSuccess(t('tickets.agent.deleted'))
  } catch (error) {
    appStore.showError(resolveTicketErrorMessage(error, t))
  } finally {
    busyId.value = null
  }
}

async function copyPrompt(): Promise<void> {
  await copyToClipboard(promptText.value, t('tickets.agent.prompt.copied'))
}

onMounted(() => {
  void load()
})

defineExpose({ load })
</script>
