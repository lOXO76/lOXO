<template>
  <div class="cs-stepper" :class="{ 'cs-stepper--wide': wide }" :aria-disabled="disabled ? 'true' : undefined">
    <button
      type="button"
      class="cs-stepper-btn"
      :disabled="disabled || atMin"
      :aria-label="t('creation.composer.decrease')"
      @click="emit('step', -1)"
    >
      <Minus aria-hidden="true" />
    </button>
    <input
      :id="inputId"
      class="cs-stepper-input"
      type="number"
      inputmode="numeric"
      step="1"
      :value="modelValue ?? ''"
      :min="min ?? undefined"
      :max="max ?? undefined"
      :disabled="disabled"
      :placeholder="placeholder"
      :aria-label="label"
      @input="onInput"
      @blur="emit('commit')"
    />
    <button
      type="button"
      class="cs-stepper-btn"
      :disabled="disabled || atMax"
      :aria-label="t('creation.composer.increase')"
      @click="emit('step', 1)"
    >
      <Plus aria-hidden="true" />
    </button>
  </div>
</template>

<script setup lang="ts">
/**
 * 数字步进器。只负责展示与输入：加减的取值规则（范围、离散档位）由父组件在 step 事件里决定，
 * 输入框失焦时触发 commit，父组件据此归一化。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Minus, Plus } from '@lucide/vue'

const props = withDefaults(defineProps<{
  /** null 表示留空。 */
  modelValue: number | null
  min?: number | null
  max?: number | null
  disabled?: boolean
  inputId?: string
  label?: string
  placeholder?: string
  /** 占满一行，输入框自适应宽度。 */
  wide?: boolean
}>(), { min: null, max: null, disabled: false, inputId: undefined, label: undefined, placeholder: undefined, wide: false })

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
  step: [direction: 1 | -1]
  commit: []
}>()

const { t } = useI18n()

const atMin = computed(() => props.modelValue !== null && props.min !== null && props.modelValue <= props.min)
const atMax = computed(() => props.modelValue !== null && props.max !== null && props.modelValue >= props.max)

function onInput(event: Event): void {
  const raw = (event.target as HTMLInputElement).value
  if (raw.trim() === '') {
    emit('update:modelValue', null)
    return
  }
  const value = Number(raw)
  if (Number.isFinite(value)) emit('update:modelValue', value)
}
</script>
