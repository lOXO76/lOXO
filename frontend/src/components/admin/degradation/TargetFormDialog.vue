<template>
  <BaseDialog
    :show="show"
    :title="target ? t('admin.degradation.targets.edit') : t('admin.degradation.targets.create')"
    width="wide"
    @close="emit('close')"
  >
    <form id="degradation-target-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="degradation-target-group" class="input-label">{{ t('admin.degradation.fields.group') }} <span class="text-red-500">*</span></label>
          <Select
            id="degradation-target-group"
            v-model="selectedGroupId"
            :options="groupOptions"
            :placeholder="t('admin.degradation.targets.groupPlaceholder')"
            :loading="groupsLoading"
            @change="handleGroupChange"
          />
        </div>
        <div>
          <label for="degradation-target-model" class="input-label">{{ t('admin.degradation.fields.model') }} <span class="text-red-500">*</span></label>
          <Select
            id="degradation-target-model"
            v-model="selectedModel"
            :options="modelOptions"
            :placeholder="t('admin.degradation.targets.modelPlaceholder')"
            :loading="modelsLoading"
            :disabled="!selectedGroupId"
            searchable
            creatable
            @change="preview = null"
          />
        </div>
      </div>

      <div>
        <label for="degradation-target-interval" class="input-label">{{ t('admin.degradation.fields.interval') }} <span class="text-red-500">*</span></label>
        <input
          id="degradation-target-interval"
          v-model.number="intervalSeconds"
          type="number"
          min="300"
          max="86400"
          step="60"
          required
          class="input"
        />
        <p class="mt-1 text-xs text-gray-400">{{ t('admin.degradation.targets.intervalHint') }}</p>
      </div>

      <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
        <div class="flex items-center justify-between gap-4">
          <div>
                        <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.degradation.targets.allProbes') }}</span>
            <p class="mt-0.5 text-xs text-gray-400">{{ t('admin.degradation.targets.allProbesHint') }}</p>
          </div>
          <Toggle :model-value="useAllProbes" @update:model-value="handleAllProbesChange" />
        </div>
        <div class="mt-4 max-h-48 space-y-2 overflow-y-auto" :class="useAllProbes ? 'opacity-50' : ''">
          <label
            v-for="probe in enabledProbes"
            :key="probe.id"
            class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"
          >
            <input
              v-model="selectedProbeIds"
              type="checkbox"
              :value="probe.id"
              :disabled="useAllProbes"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
            />
            <span>{{ probe.name }}</span>
          </label>
          <p v-if="enabledProbes.length === 0" class="text-sm text-amber-600 dark:text-amber-400">
            {{ t('admin.degradation.targets.noEnabledProbes') }}
          </p>
        </div>
      </div>

      <div class="flex items-center justify-between">
        <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.degradation.fields.enabled') }}</span>
        <Toggle v-model="enabled" />
      </div>

      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900/40">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.degradation.targets.previewTitle') }}</p>
            <p class="mt-0.5 text-xs text-gray-400">{{ t('admin.degradation.targets.previewHint') }}</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="previewLoading || !selectedGroupId || !selectedModel.trim()"
            @click="runPreview"
          >
            {{ previewLoading ? t('admin.degradation.targets.previewing') : t('admin.degradation.targets.previewAction') }}
          </button>
        </div>
        <div v-if="preview" class="mt-3 rounded-lg px-3 py-2 text-sm" :class="preview.available ? 'bg-green-50 text-green-700 dark:bg-green-950/30 dark:text-green-300' : 'bg-red-50 text-red-700 dark:bg-red-950/30 dark:text-red-300'">
          <template v-if="preview.available">
            {{ t('admin.degradation.targets.previewAvailable', { name: preview.account_name, id: preview.account_id, platform: preview.platform }) }}
          </template>
          <template v-else>
            {{ preview.reason || t('admin.degradation.targets.previewUnavailable') }}
          </template>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="degradation-target-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? t('common.submitting') : target ? t('common.update') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { DegradationAccountPreview, DegradationProbe, DegradationTarget, DegradationTargetInput } from '@/api/admin/degradation'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  target: DegradationTarget | null
  probes: DegradationProbe[]
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)
const models = ref<string[]>([])
const modelsLoading = ref(false)
const selectedGroupId = ref('')
const selectedModel = ref('')
const selectedProbeIds = ref<number[]>([])
const useAllProbes = ref(true)
const intervalSeconds = ref(1800)
const enabled = ref(true)
const submitting = ref(false)
const preview = ref<DegradationAccountPreview | null>(null)
const previewLoading = ref(false)
let modelSequence = 0
let previewSequence = 0

const enabledProbes = computed(() => props.probes.filter((probe) => probe.enabled))
const groupOptions = computed(() => groups.value.map((group) => ({
  value: String(group.id),
  label: `${group.name} · ${group.platform}`,
})))
const modelOptions = computed(() => {
  const values = [...models.value]
  if (selectedModel.value && !values.includes(selectedModel.value)) values.unshift(selectedModel.value)
  return values.map((model) => ({ value: model, label: model }))
})

async function loadGroups() {
  groupsLoading.value = true
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.groupsLoadFailed')))
  } finally {
    groupsLoading.value = false
  }
}

async function loadModels() {
  const groupId = Number(selectedGroupId.value)
  const sequence = ++modelSequence
  if (!groupId) {
    models.value = []
    return
  }
  modelsLoading.value = true
  try {
    const result = await adminAPI.groups.getModelAllowlistCandidates(groupId)
    if (sequence === modelSequence) models.value = result
  } catch (error) {
    if (sequence !== modelSequence) return
    models.value = []
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.modelsLoadFailed')))
  } finally {
    if (sequence === modelSequence) modelsLoading.value = false
  }
}

function handleGroupChange() {
  selectedModel.value = ''
  preview.value = null
  void loadModels()
}

function handleAllProbesChange(value: boolean) {
  useAllProbes.value = value
  if (value) selectedProbeIds.value = []
  else if (selectedProbeIds.value.length === 0) selectedProbeIds.value = enabledProbes.value.map((probe) => probe.id)
}

function resetForm() {
  selectedGroupId.value = props.target ? String(props.target.group_id) : ''
  selectedModel.value = props.target?.model ?? ''
  selectedProbeIds.value = [...(props.target?.probe_ids ?? [])]
  useAllProbes.value = (props.target?.probe_ids.length ?? 0) === 0
  intervalSeconds.value = props.target?.interval_seconds ?? 1800
  enabled.value = props.target?.enabled ?? true
  preview.value = null
  if (selectedGroupId.value) void loadModels()
  else models.value = []
}

async function runPreview() {
  const groupId = Number(selectedGroupId.value)
  const model = selectedModel.value.trim()
  if (!groupId || !model) return
  const sequence = ++previewSequence
  previewLoading.value = true
  try {
    const result = await adminAPI.degradation.previewAccount({ group_id: groupId, model })
    if (sequence === previewSequence) preview.value = result
  } catch (error) {
    if (sequence === previewSequence) {
      preview.value = null
      appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.previewFailed')))
    }
  } finally {
    if (sequence === previewSequence) previewLoading.value = false
  }
}

async function handleSubmit() {
  if (submitting.value) return
  const groupId = Number(selectedGroupId.value)
  const model = selectedModel.value.trim()
  if (!groupId || !model) {
    appStore.showError(t('admin.degradation.targets.required'))
    return
  }
  if (!Number.isInteger(intervalSeconds.value) || intervalSeconds.value < 300 || intervalSeconds.value > 86400) {
    appStore.showError(t('admin.degradation.targets.invalidInterval'))
    return
  }
  if (enabledProbes.value.length === 0) {
    appStore.showError(t('admin.degradation.targets.noEnabledProbes'))
    return
  }
  if (!useAllProbes.value && selectedProbeIds.value.length === 0) {
    appStore.showError(t('admin.degradation.targets.probesRequired'))
    return
  }
  const input: DegradationTargetInput = {
    group_id: groupId,
    model,
    probe_ids: useAllProbes.value ? [] : [...selectedProbeIds.value],
    interval_seconds: intervalSeconds.value,
    enabled: enabled.value,
  }
  submitting.value = true
  try {
    if (props.target) {
      await adminAPI.degradation.updateTarget(props.target.id, input)
      appStore.showSuccess(t('admin.degradation.targets.updateSuccess'))
    } else {
      await adminAPI.degradation.createTarget(input)
      appStore.showSuccess(t('admin.degradation.targets.createSuccess'))
    }
    emit('saved')
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.targets.saveFailed')))
  } finally {
    submitting.value = false
  }
}

watch(
  () => [props.show, props.target] as const,
  ([show]) => {
    if (!show) {
      modelSequence += 1
      previewSequence += 1
      return
    }
    resetForm()
    if (groups.value.length === 0) void loadGroups()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  modelSequence += 1
  previewSequence += 1
})
</script>
