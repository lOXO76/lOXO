<template>
  <div
    class="cs-chip-group"
    :class="{ 'cs-chip-group--grid': grid }"
    role="radiogroup"
    :aria-labelledby="labelledby"
    :style="chipMin ? { '--cs-chip-min': chipMin } : undefined"
  >
    <label v-for="option in options" :key="option.value" :title="option.title">
      <input
        type="radio"
        class="cs-chip-input"
        :name="name"
        :value="option.value"
        :checked="modelValue === option.value"
        :disabled="disabled"
        @change="emit('update:modelValue', option.value)"
      />
      <span class="cs-chip">
        <span
          v-if="glyph"
          class="cs-ratio-glyph"
          :data-auto="glyphStyle(option.value) ? undefined : 'true'"
          :style="glyphStyle(option.value)"
          aria-hidden="true"
        />
        <span class="truncate">{{ option.label }}</span>
      </span>
    </label>
  </div>
</template>

<script setup lang="ts">
/** 单选胶囊组：选项少且短时替代下拉框；glyph 为 true 时按选项值（16:9、1536x1024）画比例示意框。 */
import { parseCreationAspect } from './viewModel'
import type { CreationChipOption } from './types'

withDefaults(defineProps<{
  name: string
  modelValue: string
  options: CreationChipOption[]
  labelledby?: string
  disabled?: boolean
  /** 等宽网格排布（选项较多时更整齐）。 */
  grid?: boolean
  /** 网格单元最小宽度，如 7rem。 */
  chipMin?: string
  glyph?: boolean
}>(), { labelledby: undefined, disabled: false, grid: false, chipMin: undefined, glyph: false })

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

function glyphStyle(value: string): Record<string, string> | undefined {
  const aspect = parseCreationAspect(value)
  return aspect ? { '--cs-ratio-w': String(aspect.w), '--cs-ratio-h': String(aspect.h) } : undefined
}
</script>
