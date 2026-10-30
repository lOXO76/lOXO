<template>
  <BaseDialog
    :show="show"
    :title="probe ? t('admin.degradation.probes.edit') : t('admin.degradation.probes.create')"
    width="wide"
    @close="emit('close')"
  >
    <form id="degradation-probe-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div>
        <span class="input-label">{{ t('admin.degradation.fields.probeType') }}</span>
        <div class="tabs inline-flex w-full sm:w-auto" role="group" :aria-label="t('admin.degradation.fields.probeType')">
          <button type="button" class="tab flex-1 sm:flex-none" :class="form.probe_type === 'regex' ? 'tab-active' : ''" :aria-pressed="form.probe_type === 'regex'" @click="setProbeType('regex')">
            {{ t('admin.degradation.probes.typeRegex') }}
          </button>
          <button type="button" class="tab flex-1 sm:flex-none" :class="form.probe_type === 'visual' ? 'tab-active' : ''" :aria-pressed="form.probe_type === 'visual'" @click="setProbeType('visual')">
            {{ t('admin.degradation.probes.typeVisual') }}
          </button>
        </div>
      </div>

      <section v-if="form.probe_type === 'visual'" class="rounded-xl border border-blue-100 bg-blue-50/60 p-4 dark:border-blue-900/40 dark:bg-blue-950/20">
        <p class="text-sm leading-relaxed text-blue-800 dark:text-blue-200">{{ t('admin.degradation.probes.visualHint') }}</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="applyPreset('pelican')">{{ t('admin.degradation.probes.presets.pelican.button') }}</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="applyPreset('chair')">{{ t('admin.degradation.probes.presets.chair.button') }}</button>
        </div>
      </section>

      <div class="grid gap-4 sm:grid-cols-[1fr_10rem]">
        <div>
          <label for="degradation-probe-name" class="input-label">{{ t('admin.degradation.fields.name') }} <span class="text-red-500">*</span></label>
          <input id="degradation-probe-name" v-model="form.name" type="text" maxlength="100" required class="input" />
        </div>
        <div>
          <label for="degradation-probe-sort" class="input-label">{{ t('admin.degradation.fields.sortOrder') }}</label>
          <input id="degradation-probe-sort" v-model.number="form.sort_order" type="number" step="1" class="input" />
        </div>
      </div>

      <div>
        <label for="degradation-probe-prompt" class="input-label">{{ t('admin.degradation.fields.prompt') }} <span class="text-red-500">*</span></label>
        <textarea id="degradation-probe-prompt" v-model="form.prompt" rows="5" maxlength="20000" required class="input resize-y" />
        <p class="mt-1 text-right text-xs text-gray-400">{{ form.prompt.length }} / 20000</p>
      </div>

      <template v-if="form.probe_type === 'regex'">
        <div>
          <label for="degradation-probe-regex" class="input-label">{{ t('admin.degradation.fields.regex') }} <span class="text-red-500">*</span></label>
          <textarea id="degradation-probe-regex" v-model="form.expect_regex" rows="3" maxlength="2000" required class="input resize-y font-mono text-sm" />
          <p class="mt-1 text-xs text-gray-400">{{ t('admin.degradation.probes.regexHint') }}</p>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <fieldset>
            <legend class="input-label">{{ t('admin.degradation.fields.flags') }}</legend>
            <div class="flex gap-4">
              <label v-for="flag in flagOptions" :key="flag.value" class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <input v-model="flagList" type="checkbox" :value="flag.value" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800" />
                <span>{{ flag.value }} — {{ flag.label }}</span>
              </label>
            </div>
          </fieldset>
          <div class="flex items-center justify-between gap-4 sm:justify-end">
            <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.degradation.fields.negate') }}</span>
            <Toggle v-model="form.negate" />
          </div>
        </div>
        <p class="-mt-3 text-xs text-gray-400">{{ t('admin.degradation.probes.negateHint') }}</p>
      </template>

      <details class="rounded-xl border border-gray-200 bg-gray-50/50 p-4 dark:border-dark-700 dark:bg-dark-900/30">
        <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.degradation.probes.advanced') }}</summary>
        <div class="mt-4 grid gap-4 sm:grid-cols-3">
          <div>
            <label for="degradation-probe-max-tokens" class="input-label">{{ t('admin.degradation.fields.maxOutputTokens') }}</label>
            <input id="degradation-probe-max-tokens" v-model.number="form.max_output_tokens" type="number" min="0" max="128000" step="1" class="input" />
            <p class="mt-1 text-xs text-gray-400">{{ defaultMaxTokens }}</p>
          </div>
          <div>
            <label for="degradation-probe-timeout" class="input-label">{{ t('admin.degradation.fields.timeoutSeconds') }}</label>
            <input id="degradation-probe-timeout" v-model.number="form.timeout_seconds" type="number" min="0" max="900" step="1" class="input" />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.degradation.probes.defaultSeconds', { value: form.probe_type === 'visual' ? 300 : 90 }) }}</p>
          </div>
          <div>
            <label for="degradation-probe-run-interval" class="input-label">{{ t('admin.degradation.fields.runIntervalSeconds') }}</label>
            <input id="degradation-probe-run-interval" v-model.number="form.run_interval_seconds" type="number" min="0" max="604800" step="1" class="input" />
            <p class="mt-1 text-xs text-gray-400">{{ form.probe_type === 'visual' ? t('admin.degradation.probes.defaultSeconds', { value: 86400 }) : t('admin.degradation.probes.defaultEveryTargetRun') }}</p>
          </div>
        </div>
        <p class="mt-3 text-xs text-gray-400">{{ t('admin.degradation.probes.advancedHint') }}</p>
      </details>

      <div class="flex items-center justify-between">
        <span class="text-sm font-medium text-gray-800 dark:text-gray-200">{{ t('admin.degradation.fields.enabled') }}</span>
        <Toggle v-model="form.enabled" />
      </div>

      <section v-if="form.probe_type === 'regex'" class="rounded-xl border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-900/40">
        <div class="flex items-center justify-between gap-3">
          <div>
            <h4 class="text-sm font-semibold text-gray-800 dark:text-gray-200">{{ t('admin.degradation.probes.testTitle') }}</h4>
            <p class="mt-0.5 text-xs text-gray-400">{{ t('admin.degradation.probes.testHint') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="testing || !canTest" @click="runTest">
            {{ testing ? t('admin.degradation.probes.testing') : t('admin.degradation.probes.testAction') }}
          </button>
        </div>
        <label for="degradation-probe-sample" class="input-label mt-4">{{ t('admin.degradation.probes.sampleText') }}</label>
        <textarea id="degradation-probe-sample" v-model="sampleText" rows="4" class="input resize-y" :placeholder="t('admin.degradation.probes.samplePlaceholder')" />
        <div v-if="testing" class="mt-3 text-sm text-gray-400" role="status">{{ t('admin.degradation.probes.testing') }}</div>
        <div v-else-if="testError" class="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/30 dark:text-red-300" role="alert">
          {{ testError }}
        </div>
        <div v-else-if="testResult" class="mt-3 space-y-2 rounded-lg bg-white px-3 py-3 text-sm dark:bg-dark-800" aria-live="polite">
          <p v-if="testResult.error" class="text-red-600 dark:text-red-400">{{ testResult.error }}</p>
          <template v-else>
            <p class="text-gray-600 dark:text-gray-300">
              {{ t('admin.degradation.probes.matched') }}:
              <strong :class="testResult.matched ? 'text-green-600 dark:text-green-400' : 'text-gray-500'">{{ yesNo(testResult.matched) }}</strong>
            </p>
            <p class="text-gray-600 dark:text-gray-300">
              {{ t('admin.degradation.probes.passed') }}:
              <strong :class="testResult.passed ? 'text-green-600 dark:text-green-400' : 'text-red-600 dark:text-red-400'">{{ yesNo(testResult.passed) }}</strong>
            </p>
          </template>
        </div>
      </section>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="submit" form="degradation-probe-form" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? t('common.submitting') : probe ? t('common.update') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { DegradationProbe, DegradationProbeInput, DegradationProbeTestResult } from '@/api/admin/degradation'
import type { DegradationProbeType } from '@/api/degradation'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  probe: DegradationProbe | null
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const form = reactive<DegradationProbeInput>({
  name: '',
  prompt: '',
  expect_regex: '',
  regex_flags: '',
  negate: false,
  enabled: true,
  sort_order: 0,
  probe_type: 'regex',
  max_output_tokens: 0,
  timeout_seconds: 0,
  run_interval_seconds: 0,
})
const sampleText = ref('')
const submitting = ref(false)
const testing = ref(false)
const testResult = ref<DegradationProbeTestResult | null>(null)
const testError = ref('')
let testTimer: ReturnType<typeof setTimeout> | null = null
let testSequence = 0

const flagOptions = computed(() => [
  { value: 'i', label: t('admin.degradation.probes.flagI') },
  { value: 'm', label: t('admin.degradation.probes.flagM') },
  { value: 's', label: t('admin.degradation.probes.flagS') },
])
const flagList = computed<string[]>({
  get: () => form.regex_flags.split('').filter((flag) => 'ims'.includes(flag)),
  set: (flags) => {
    form.regex_flags = 'ims'.split('').filter((flag) => flags.includes(flag)).join('')
  },
})
const canTest = computed(() => form.probe_type === 'regex' && Boolean(form.expect_regex.trim() && sampleText.value.trim()))
const defaultMaxTokens = computed(() => form.probe_type === 'visual'
  ? t('admin.degradation.probes.defaultTokens', { value: 8192 })
  : t('admin.degradation.probes.defaultProtocolLimit'))

function setProbeType(type: DegradationProbeType) {
  if (form.probe_type === type) return
  form.probe_type = type
  clearTestTimer()
  testSequence += 1
  testing.value = false
  testResult.value = null
  testError.value = ''
}

function applyPreset(preset: 'pelican' | 'chair') {
  setProbeType('visual')
  form.name = t(`admin.degradation.probes.presets.${preset}.name`)
  form.prompt = t(`admin.degradation.probes.presets.${preset}.prompt`)
}

function yesNo(value: boolean) {
  return value ? t('admin.degradation.yes') : t('admin.degradation.no')
}

function clearTestTimer() {
  if (testTimer !== null) {
    clearTimeout(testTimer)
    testTimer = null
  }
}

function resetForm() {
  form.name = props.probe?.name ?? ''
  form.prompt = props.probe?.prompt ?? ''
  form.expect_regex = props.probe?.expect_regex ?? ''
  form.regex_flags = props.probe?.regex_flags ?? ''
  form.negate = props.probe?.negate ?? false
  form.enabled = props.probe?.enabled ?? true
  form.sort_order = props.probe?.sort_order ?? 0
  form.probe_type = props.probe?.probe_type ?? 'regex'
  form.max_output_tokens = props.probe?.max_output_tokens ?? 0
  form.timeout_seconds = props.probe?.timeout_seconds ?? 0
  form.run_interval_seconds = props.probe?.run_interval_seconds ?? 0
  sampleText.value = ''
  testResult.value = null
  testError.value = ''
  testing.value = false
  clearTestTimer()
  testSequence += 1
}

async function runTest() {
  clearTestTimer()
  if (!canTest.value || !props.show) return
  const sequence = ++testSequence
  testing.value = true
  testError.value = ''
  try {
    const result = await adminAPI.degradation.testProbe({
      regex: form.expect_regex,
      flags: form.regex_flags,
      negate: form.negate,
      sample_text: sampleText.value,
    })
    if (sequence === testSequence) testResult.value = result
  } catch (error) {
    if (sequence === testSequence) {
      testResult.value = null
      testError.value = extractApiErrorMessage(error, t('admin.degradation.probes.testFailed'))
    }
  } finally {
    if (sequence === testSequence) testing.value = false
  }
}

function scheduleTest() {
  clearTestTimer()
  testSequence += 1
  testing.value = false
  testResult.value = null
  testError.value = ''
  if (!props.show || !canTest.value) return
  testTimer = setTimeout(() => void runTest(), 400)
}

function isZeroOrRange(value: number, min: number, max: number) {
  return value === 0 || (Number.isInteger(value) && value >= min && value <= max)
}

async function handleSubmit() {
  if (submitting.value) return
  if (!form.name.trim() || !form.prompt.trim() || (form.probe_type === 'regex' && !form.expect_regex.trim())) {
    appStore.showError(t('admin.degradation.probes.required'))
    return
  }
  if (!Number.isInteger(form.sort_order)) {
    appStore.showError(t('admin.degradation.probes.invalidSortOrder'))
    return
  }
  if (!isZeroOrRange(form.max_output_tokens, 256, 128000)) {
    appStore.showError(t('admin.degradation.probes.invalidMaxOutputTokens'))
    return
  }
  if (!isZeroOrRange(form.timeout_seconds, 30, 900)) {
    appStore.showError(t('admin.degradation.probes.invalidTimeout'))
    return
  }
  if (!isZeroOrRange(form.run_interval_seconds, 300, 604800)) {
    appStore.showError(t('admin.degradation.probes.invalidRunInterval'))
    return
  }
  submitting.value = true
  const isVisual = form.probe_type === 'visual'
  const input: DegradationProbeInput = {
    name: form.name.trim(),
    prompt: form.prompt,
    expect_regex: isVisual ? '' : form.expect_regex,
    regex_flags: isVisual ? '' : form.regex_flags,
    negate: isVisual ? false : form.negate,
    enabled: form.enabled,
    sort_order: form.sort_order,
    probe_type: form.probe_type,
    max_output_tokens: form.max_output_tokens,
    timeout_seconds: form.timeout_seconds,
    run_interval_seconds: form.run_interval_seconds,
  }
  try {
    if (props.probe) {
      await adminAPI.degradation.updateProbe(props.probe.id, input)
      appStore.showSuccess(t('admin.degradation.probes.updateSuccess'))
    } else {
      await adminAPI.degradation.createProbe(input)
      appStore.showSuccess(t('admin.degradation.probes.createSuccess'))
    }
    emit('saved')
    emit('close')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.degradation.probes.saveFailed')))
  } finally {
    submitting.value = false
  }
}

watch(
  () => [props.show, props.probe] as const,
  ([show]) => {
    if (show) resetForm()
    else {
      clearTestTimer()
      testSequence += 1
    }
  },
  { immediate: true },
)
watch(
  [() => form.probe_type, () => form.expect_regex, () => form.regex_flags, () => form.negate, sampleText],
  scheduleTest,
)

onBeforeUnmount(() => {
  clearTestTimer()
  testSequence += 1
})
</script>
