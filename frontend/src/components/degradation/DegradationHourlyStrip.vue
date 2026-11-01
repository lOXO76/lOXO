<template>
  <div class="w-full" data-testid="degradation-hourly">
    <div
      class="flex h-7 w-full items-end"
      :style="{ gap: `${gapPx}px` }"
      role="img"
      :aria-label="ariaLabel"
      @mouseleave="hideTooltip"
    >
      <div
        v-for="slot in slots"
        :key="slot.startMs"
        class="hourly-slot flex h-full min-w-0 flex-1 cursor-help items-end"
        aria-hidden="true"
        @mouseenter="showTooltip($event, slot)"
        @mousemove="moveTooltip"
      >
        <span
          class="block w-full rounded-sm transition-opacity"
          :class="[LEVEL_STYLE[slot.level].bar, activeStart === slot.startMs ? 'opacity-70' : '']"
          :style="{ height: `${LEVEL_STYLE[slot.level].height}%` }"
          data-testid="hourly-bar"
          :data-level="slot.level"
        ></span>
      </div>
    </div>
    <div class="mt-1 flex justify-between text-[10px] tabular-nums text-gray-400 dark:text-gray-500">
      <span>{{ startLabel }}</span>
      <span>{{ t('degradation.hourly.now') }}</span>
    </div>

    <Teleport to="body">
      <div
        v-if="tooltip.visible"
        class="pointer-events-none fixed z-[9999] min-w-[11rem] max-w-[min(18rem,calc(100vw-1.5rem))] -translate-x-1/2 -translate-y-full whitespace-nowrap rounded-xl border border-gray-200 bg-white px-2.5 py-2 shadow-xl dark:border-dark-700 dark:bg-dark-800"
        :style="{ left: `${tooltip.x}px`, top: `${tooltip.y}px` }"
        role="tooltip"
      >
        <span
          v-for="(line, index) in tooltip.lines"
          :key="`${index}:${line}`"
          class="flex items-center gap-1.5 text-[11px] leading-normal"
          :class="index === 0 ? 'mb-0.5 font-semibold text-gray-900 dark:text-gray-100' : 'text-gray-600 dark:text-gray-300'"
        >
          <i v-if="index === 1" class="inline-block h-2 w-2 flex-none rounded-full" :class="LEVEL_STYLE[tooltip.level].bar"></i>{{ line }}
        </span>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DegradationHourBucket } from '@/api/degradation'
import { HOURLY_LEVEL_STYLE, buildHourlySlots, type HourlyLevel, type HourlySlot, type HourlyWindow } from './hourly'

const props = defineProps<{
  buckets: DegradationHourBucket[] | null | undefined
  bucketWindow: HourlyWindow
  threshold: number
}>()

const { t, locale } = useI18n()

const LEVEL_STYLE = HOURLY_LEVEL_STYLE

const slots = computed(() => buildHourlySlots(props.buckets, props.bucketWindow, props.threshold))
const gapPx = computed(() => (props.bucketWindow.count > 72 ? 1 : 2))
const startLabel = computed(() => (slots.value.length ? formatHour(slots.value[0].startMs, true) : ''))
const ariaLabel = computed(() => t('degradation.hourly.aria', { n: props.bucketWindow.count }))

const activeStart = ref<number | null>(null)
const tooltip = reactive({
  visible: false,
  x: 0,
  y: 0,
  level: 'empty' as HourlyLevel,
  lines: [] as string[],
})

function formatHour(ms: number, withDate: boolean) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    ...(withDate ? { month: '2-digit', day: '2-digit' } : {}),
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(ms))
}

function formatLatency(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return value >= 1000 ? `${(value / 1000).toFixed(1)}s` : `${Math.round(value)}ms`
}

function tooltipLines(slot: HourlySlot): string[] {
  const now = Date.now()
  const isCurrent = slot.startMs <= now && now < slot.endMs
  const range = `${formatHour(slot.startMs, true)} – ${formatHour(slot.endMs, false)}`
  const lines = [isCurrent ? `${range} · ${t('degradation.hourly.inProgress')}` : range]
  lines.push(t(`degradation.hourly.levels.${slot.level}`))
  const bucket = slot.bucket
  if (!bucket || slot.level === 'empty') {
    lines.push(t('degradation.hourly.noData'))
    return lines
  }
  if (slot.level === 'error') {
    lines.push(t('degradation.hourly.errorsOnly'))
  } else {
    lines.push(t('degradation.hourly.passRateValue', { value: `${(bucket.pass_rate * 100).toFixed(1)}%` }))
    lines.push(t('degradation.hourly.avgLatencyValue', { value: formatLatency(bucket.avg_latency_ms) }))
  }
  lines.push(t('degradation.sampleCounts', { passed: bucket.passed, failed: bucket.failed, error: bucket.error }))
  return lines
}

function showTooltip(event: MouseEvent, slot: HourlySlot) {
  activeStart.value = slot.startMs
  tooltip.level = slot.level
  tooltip.lines = tooltipLines(slot)
  tooltip.visible = true
  moveTooltip(event)
}

function moveTooltip(event: MouseEvent) {
  if (!tooltip.visible) return
  tooltip.x = Math.min(window.innerWidth - 12, Math.max(12, event.clientX))
  tooltip.y = Math.min(window.innerHeight - 12, Math.max(12, event.clientY)) - 12
}

function hideTooltip() {
  activeStart.value = null
  tooltip.visible = false
}

onBeforeUnmount(hideTooltip)
</script>

