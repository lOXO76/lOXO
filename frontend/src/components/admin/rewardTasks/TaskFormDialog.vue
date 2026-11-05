<template>
  <BaseDialog
    :show="show"
    :title="task ? t('admin.rewardTasks.tasks.form.editTitle') : t('admin.rewardTasks.tasks.form.createTitle')"
    width="wide"
    @close="emit('close')"
  >
    <form id="reward-task-form" class="space-y-5" @submit.prevent="handleSubmit">
      <!-- 任务类型：创建后不可修改 -->
      <div>
        <span class="input-label">{{ t('admin.rewardTasks.tasks.form.taskType') }} <span class="text-red-500">*</span></span>
        <div class="tabs inline-flex w-full sm:w-auto" role="group" :aria-label="t('admin.rewardTasks.tasks.form.taskType')">
          <button
            v-for="type in taskTypeOptions"
            :key="type"
            type="button"
            class="tab flex-1 sm:flex-none"
            :class="form.task_type === type ? 'tab-active' : ''"
            :aria-pressed="form.task_type === type"
            :disabled="Boolean(task)"
            data-testid="reward-task-type-tab"
            @click="setTaskType(type)"
          >
            {{ t(`admin.rewardTasks.taskTypes.${type}`) }}
          </button>
        </div>
        <p v-if="task" class="mt-1 text-xs text-gray-400 dark:text-dark-500">
          {{ t('admin.rewardTasks.tasks.form.taskTypeLocked') }}
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="reward-task-title" class="input-label">
            {{ t('admin.rewardTasks.tasks.form.title') }} <span class="text-red-500">*</span>
          </label>
          <input
            id="reward-task-title"
            v-model="form.title"
            type="text"
            maxlength="100"
            required
            class="input"
            :placeholder="t('admin.rewardTasks.tasks.form.titlePlaceholder')"
            data-testid="reward-task-title"
          />
        </div>
        <div>
          <label for="reward-task-reward" class="input-label">
            {{ t('admin.rewardTasks.tasks.form.rewardAmount') }} <span class="text-red-500">*</span>
          </label>
          <input
            id="reward-task-reward"
            v-model.number="form.reward_amount"
            type="number"
            min="0.00000001"
            max="10000"
            step="0.00000001"
            required
            class="input"
          />
        </div>
      </div>

      <div>
        <label for="reward-task-description" class="input-label">{{ t('admin.rewardTasks.tasks.form.description') }}</label>
        <textarea
          id="reward-task-description"
          v-model="form.description"
          rows="3"
          maxlength="2000"
          class="input resize-y"
          :placeholder="t('admin.rewardTasks.tasks.form.descriptionPlaceholder')"
        />
        <p class="mt-1 text-right text-xs text-gray-400 dark:text-dark-500">{{ form.description.length }} / 2000</p>
      </div>

      <div>
        <label for="reward-task-target-url" class="input-label">
          {{ t('admin.rewardTasks.tasks.form.targetUrl') }} <span class="text-red-500">*</span>
        </label>
        <input
          id="reward-task-target-url"
          v-model="form.target_url"
          type="text"
          maxlength="2048"
          required
          class="input font-mono text-sm"
          :placeholder="t('admin.rewardTasks.tasks.form.targetUrlPlaceholder')"
          data-testid="reward-task-target-url"
        />
        <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.targetUrlHint') }}</p>
      </div>

      <!-- 校验方式 + 类型联动字段 -->
      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <span class="input-label">{{ t('admin.rewardTasks.tasks.form.verifyMode') }}</span>
            <template v-if="form.task_type === 'browse'">
              <Select
                id="reward-task-verify-mode"
                v-model="form.verify_mode"
                :options="verifyModeOptions"
                @change="handleVerifyModeChange"
              />
            </template>
            <p v-else class="input flex items-center bg-gray-50 text-gray-500 dark:bg-dark-900 dark:text-dark-400">
              {{ t('admin.rewardTasks.tasks.form.verifyModeFixed', { mode: t('admin.rewardTasks.verifyModes.callback') }) }}
            </p>
          </div>
          <div v-if="form.task_type === 'browse'">
            <label for="reward-task-browse-seconds" class="input-label">
              {{ t('admin.rewardTasks.tasks.form.browseSeconds') }} <span class="text-red-500">*</span>
            </label>
            <input
              id="reward-task-browse-seconds"
              v-model.number="form.browse_seconds"
              type="number"
              min="5"
              max="86400"
              step="1"
              required
              class="input"
              data-testid="reward-task-browse-seconds"
            />
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.browseSecondsHint') }}</p>
          </div>
          <!-- 挂机检测：仅 SDK 浏览任务使用（R1.6） -->
          <div v-if="showIdleTimeout" data-testid="reward-task-idle-timeout-field">
            <label for="reward-task-idle-timeout" class="input-label">
              {{ t('admin.rewardTasks.tasks.form.idleTimeout') }} <span class="text-red-500">*</span>
            </label>
            <input
              id="reward-task-idle-timeout"
              v-model.number="form.idle_timeout_seconds"
              type="number"
              min="0"
              max="3600"
              step="1"
              class="input"
              data-testid="reward-task-idle-timeout"
            />
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.idleTimeoutHint') }}</p>
          </div>
          <div v-if="form.task_type === 'recharge'">
            <label for="reward-task-min-recharge" class="input-label">
              {{ t('admin.rewardTasks.tasks.form.minRechargeAmount') }} <span class="text-red-500">*</span>
            </label>
            <input
              id="reward-task-min-recharge"
              v-model.number="form.min_recharge_amount"
              type="number"
              min="0.01"
              max="100000000"
              step="0.01"
              required
              class="input"
              data-testid="reward-task-min-recharge"
            />
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.minRechargeAmountHint') }}</p>
          </div>
        </div>

        <!-- 接入站点：sdk / callback 必填，timer 必须为空 -->
        <div v-if="needsPartner" class="mt-4">
          <label for="reward-task-partner" class="input-label">
            {{ t('admin.rewardTasks.tasks.form.partner') }} <span class="text-red-500">*</span>
          </label>
          <Select
            id="reward-task-partner"
            v-model="form.partner_id"
            :options="partnerOptions"
            :placeholder="t('admin.rewardTasks.tasks.form.partnerPlaceholder')"
          />
          <p v-if="partners.length === 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
            {{ t('admin.rewardTasks.tasks.form.noPartners') }}
          </p>
        </div>

        <p
          v-if="form.task_type === 'browse' && form.verify_mode === 'timer'"
          class="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:bg-amber-950/30 dark:text-amber-300"
          data-testid="reward-task-timer-warning"
        >
          {{ t('admin.rewardTasks.tasks.form.timerWarning') }}
        </p>
        <p v-else-if="form.task_type === 'register'" class="mt-3 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.rewardTasks.tasks.form.registerHint') }}
        </p>
        <p v-else-if="form.task_type === 'recharge'" class="mt-3 text-xs text-gray-500 dark:text-dark-400">
          {{ t('admin.rewardTasks.tasks.form.rechargeHint') }}
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <span class="input-label">{{ t('admin.rewardTasks.tasks.form.period') }}</span>
          <Select v-model="form.period" :options="periodOptions" :disabled="form.task_type === 'register'" />
        </div>
        <div>
          <label for="reward-task-icon" class="input-label">{{ t('admin.rewardTasks.tasks.form.iconUrl') }}</label>
          <input
            id="reward-task-icon"
            v-model="form.icon_url"
            type="text"
            maxlength="2048"
            class="input font-mono text-sm"
            :placeholder="t('admin.rewardTasks.tasks.form.iconUrlPlaceholder')"
          />
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.iconUrlHint') }}</p>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="reward-task-max-completions" class="input-label">{{ t('admin.rewardTasks.tasks.form.maxCompletions') }}</label>
          <input
            id="reward-task-max-completions"
            v-model.number="form.max_completions"
            type="number"
            min="0"
            step="1"
            class="input"
          />
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.maxCompletionsHint') }}</p>
        </div>
        <div>
          <label for="reward-task-sort-order" class="input-label">{{ t('admin.rewardTasks.tasks.form.sortOrder') }}</label>
          <input id="reward-task-sort-order" v-model.number="form.sort_order" type="number" step="1" class="input" />
          <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.sortOrderHint') }}</p>
        </div>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="reward-task-starts-at" class="input-label">{{ t('admin.rewardTasks.tasks.form.startsAt') }}</label>
          <input id="reward-task-starts-at" v-model="form.starts_at" type="datetime-local" class="input" />
        </div>
        <div>
          <label for="reward-task-ends-at" class="input-label">{{ t('admin.rewardTasks.tasks.form.endsAt') }}</label>
          <input id="reward-task-ends-at" v-model="form.ends_at" type="datetime-local" class="input" />
        </div>
      </div>
      <p class="-mt-3 text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.tasks.form.timeHint') }}</p>

      <div class="flex items-center justify-between">
        <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.rewardTasks.tasks.form.enabled') }}</span>
        <Toggle v-model="form.enabled" />
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="reward-task-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? t('common.submitting') : task ? t('common.update') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type {
  AdminRewardTask,
  AdminRewardTaskPartner,
  AdminRewardTaskPayload,
  RewardTaskPeriod,
  RewardTaskType,
  RewardTaskVerifyMode,
} from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  task: AdminRewardTask | null
  partners: AdminRewardTaskPartner[]
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const submitting = ref(false)

const taskTypes: RewardTaskType[] = ['browse', 'register', 'recharge']
const periods: RewardTaskPeriod[] = ['once', 'daily', 'weekly', 'monthly']
const taskTypeOptions = taskTypes
const verifyModeOptions = computed(() => [
  { value: 'timer', label: t('admin.rewardTasks.verifyModes.timer') },
  { value: 'sdk', label: t('admin.rewardTasks.verifyModes.sdk') },
])
const periodOptions = computed(() =>
  periods.map((period) => ({ value: period, label: t(`admin.rewardTasks.periods.${period}`) })),
)
const partnerOptions = computed(() =>
  props.partners.map((partner) => ({
    value: String(partner.id),
    label: partner.enabled ? partner.name : `${partner.name} (${t('common.disabled')})`,
  })),
)

const form = reactive({
  title: '',
  description: '',
  task_type: 'browse' as RewardTaskType,
  verify_mode: 'timer' as RewardTaskVerifyMode,
  partner_id: '' as string,
  target_url: '',
  icon_url: '',
  reward_amount: 0.5,
  period: 'once' as RewardTaskPeriod,
  browse_seconds: 30,
  // 挂机检测阈值：新建默认 15 秒（R1.6），0 = 不检测
  idle_timeout_seconds: 15,
  min_recharge_amount: 1,
  max_completions: 0,
  starts_at: '',
  ends_at: '',
  enabled: true,
  sort_order: 0,
})

// timer 模式不绑定站点；sdk / callback 必须绑定
const needsPartner = computed(() => form.verify_mode !== 'timer')
// 挂机阈值只对 browse + sdk 可见；校验与模板必须用同一个条件
const showIdleTimeout = computed(() => form.task_type === 'browse' && form.verify_mode === 'sdk')

function setTaskType(type: RewardTaskType) {
  if (props.task) return
  form.task_type = type
  if (type === 'browse') {
    form.verify_mode = 'timer'
  } else {
    form.verify_mode = 'callback'
    if (type === 'register') form.period = 'once'
  }
}

function handleVerifyModeChange() {
  if (form.verify_mode === 'timer') form.partner_id = ''
}

function decimalPlaces(value: number): number {
  const text = String(value)
  if (text.includes('e-')) {
    const [mantissa, exponent] = text.split('e-')
    const fraction = mantissa.includes('.') ? mantissa.split('.')[1].length : 0
    return fraction + Number(exponent)
  }
  return text.includes('.') ? text.split('.')[1].length : 0
}

function toDateTimeLocal(value: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (input: number) => String(input).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toIsoString(value: string): string | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

function isHttpUrl(value: string): boolean {
  return /^https?:\/\/\S+$/i.test(value)
}

// 与契约 §1.2 一致的前端校验，避免把明显非法的参数发给后端
function validate(): string | null {
  const title = form.title.trim()
  if (!title) return t('admin.rewardTasks.tasks.form.required')
  if (title.length > 100) return t('admin.rewardTasks.tasks.form.invalidTitle')
  if (form.description.length > 2000) return t('admin.rewardTasks.tasks.form.invalidDescription')

  const targetUrl = form.target_url.trim()
  if (!targetUrl) return t('admin.rewardTasks.tasks.form.required')
  if (!isHttpUrl(targetUrl) || targetUrl.length > 2048) return t('admin.rewardTasks.tasks.form.invalidTargetUrl')

  const iconUrl = form.icon_url.trim()
  if (iconUrl && !isHttpUrl(iconUrl) && !iconUrl.startsWith('/')) return t('admin.rewardTasks.tasks.form.invalidIconUrl')

  const reward = Number(form.reward_amount)
  if (!Number.isFinite(reward) || reward <= 0 || reward > 10000 || decimalPlaces(reward) > 8) {
    return t('admin.rewardTasks.tasks.form.invalidReward')
  }

  if (form.task_type === 'browse') {
    const seconds = Number(form.browse_seconds)
    if (!Number.isInteger(seconds) || seconds < 5 || seconds > 86400) {
      return t('admin.rewardTasks.tasks.form.invalidBrowseSeconds')
    }
  }
  // 挂机阈值：只允许 0（不检测）或 5 ~ 3600 的整数，与后端校验一致。
  // 仅在字段可见（browse + sdk）时校验；空值 / NaN 一律拒绝，
  // 必须由管理员明确填 0 才表示「不检测」，避免清空后误关检测。
  if (showIdleTimeout.value) {
    const idleTimeout = parseIdleTimeout()
    if (
      idleTimeout === null ||
      !Number.isInteger(idleTimeout) ||
      (idleTimeout !== 0 && (idleTimeout < 5 || idleTimeout > 3600))
    ) {
      return t('admin.rewardTasks.tasks.form.invalidIdleTimeout')
    }
  }
  if (form.task_type === 'recharge') {
    const minAmount = Number(form.min_recharge_amount)
    if (!Number.isFinite(minAmount) || minAmount < 0.01 || minAmount > 100000000 || decimalPlaces(minAmount) > 2) {
      return t('admin.rewardTasks.tasks.form.invalidRechargeAmount')
    }
  }

  const maxCompletions = Number(form.max_completions)
  if (!Number.isInteger(maxCompletions) || maxCompletions < 0) return t('admin.rewardTasks.tasks.form.invalidMaxCompletions')
  if (!Number.isInteger(Number(form.sort_order))) return t('admin.rewardTasks.tasks.form.invalidSortOrder')

  if (needsPartner.value && !form.partner_id) return t('admin.rewardTasks.tasks.form.partnerRequired')

  const startsAt = toIsoString(form.starts_at)
  const endsAt = toIsoString(form.ends_at)
  if (startsAt && endsAt && new Date(startsAt) >= new Date(endsAt)) {
    return t('admin.rewardTasks.tasks.form.invalidTimeRange')
  }
  return null
}

// 挂机阈值：空字符串 / null / NaN 一律返回 null（不当成 0 也不当成 15）
function parseIdleTimeout(): number | null {
  const raw: unknown = form.idle_timeout_seconds
  if (raw === null || raw === undefined || raw === '') return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}

// 字段可见时非法值已被 validate() 拦下；不可见时保留表单值，缺失回退 15（非 sdk 任务不使用该字段）
function normalizeIdleTimeout(): number {
  return parseIdleTimeout() ?? 15
}

function buildPayload(): AdminRewardTaskPayload {
  return {
    title: form.title.trim(),
    description: form.description.trim(),
    task_type: form.task_type,
    verify_mode: form.verify_mode,
    partner_id: needsPartner.value ? Number(form.partner_id) : null,
    target_url: form.target_url.trim(),
    icon_url: form.icon_url.trim(),
    reward_amount: Number(form.reward_amount),
    period: form.period,
    browse_seconds: form.task_type === 'browse' ? Number(form.browse_seconds) : 0,
    // 所有类型都保存该值（非 sdk 任务不使用）；保留表单值，非法 / 缺失时回退 15
    idle_timeout_seconds: normalizeIdleTimeout(),
    min_recharge_amount: form.task_type === 'recharge' ? Number(form.min_recharge_amount) : 0,
    max_completions: Number(form.max_completions),
    starts_at: toIsoString(form.starts_at),
    ends_at: toIsoString(form.ends_at),
    enabled: form.enabled,
    sort_order: Number(form.sort_order),
  }
}

async function handleSubmit() {
  if (submitting.value) return
  const error = validate()
  if (error) {
    appStore.showError(error)
    return
  }
  submitting.value = true
  try {
    if (props.task) {
      await adminAPI.rewardTasks.updateTask(props.task.id, buildPayload())
      appStore.showSuccess(t('admin.rewardTasks.tasks.updateSuccess'))
    } else {
      await adminAPI.rewardTasks.createTask(buildPayload())
      appStore.showSuccess(t('admin.rewardTasks.tasks.createSuccess'))
    }
    emit('saved')
    emit('close')
  } catch (caught) {
    // 后端 message 直接展示（字段级英文短句）
    appStore.showError(extractApiErrorMessage(caught, t('admin.rewardTasks.tasks.saveFailed')))
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  const task = props.task
  form.title = task?.title ?? ''
  form.description = task?.description ?? ''
  form.task_type = task?.task_type ?? 'browse'
  form.verify_mode = task?.verify_mode ?? 'timer'
  form.partner_id = task?.partner_id ? String(task.partner_id) : ''
  form.target_url = task?.target_url ?? ''
  form.icon_url = task?.icon_url ?? ''
  form.reward_amount = task?.reward_amount ?? 0.5
  form.period = task?.period ?? 'once'
  form.browse_seconds = task?.browse_seconds || 30
  // 编辑时回填挂机阈值；历史 / 缺失数据回退 15
  form.idle_timeout_seconds = task?.idle_timeout_seconds ?? 15
  form.min_recharge_amount = task?.min_recharge_amount || 1
  form.max_completions = task?.max_completions ?? 0
  form.starts_at = toDateTimeLocal(task?.starts_at ?? null)
  form.ends_at = toDateTimeLocal(task?.ends_at ?? null)
  form.enabled = task?.enabled ?? true
  form.sort_order = task?.sort_order ?? 0
  // 兼容历史数据：register 任务的周期固定 once
  if (form.task_type === 'register') form.period = 'once'
}

watch(
  () => [props.show, props.task] as const,
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
