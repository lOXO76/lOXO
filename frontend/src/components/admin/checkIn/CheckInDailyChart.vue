<template>
  <div class="h-56">
    <Bar v-if="hasData" :key="themeKey" :data="chartData" :options="chartOptions" :aria-label="title" role="img" />
    <div v-else class="flex h-full items-center justify-center text-sm text-gray-400 dark:text-dark-500">
      {{ emptyText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { BarElement, CategoryScale, Chart as ChartJS, LinearScale, Tooltip, type TooltipItem } from 'chart.js'
import { Bar } from 'vue-chartjs'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip)

// 单系列柱状图：标题即系列名，不需要图例。颜色经 dataviz 校验脚本验证，
// 浅色 primary-600 / 深色 #14a394 在各自卡片底色上均通过亮度、色度与 3:1 对比度检查。
const props = defineProps<{
  title: string
  labels: string[]
  values: number[]
  emptyText: string
  formatValue: (value: number) => string
  /** 人数等计数型数据刻度取整；金额类（签到额度）保留小数 */
  integer?: boolean
}>()

const isDark = ref(document.documentElement.classList.contains('dark'))
let observer: MutationObserver | null = null
onMounted(() => {
  // classList 本身不是响应式的，监听 class 变化以便切换主题时重绘
  observer = new MutationObserver(() => {
    isDark.value = document.documentElement.classList.contains('dark')
  })
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => observer?.disconnect())

const theme = computed(() =>
  isDark.value
    ? { bar: '#14a394', text: '#94a3b8', grid: 'rgba(148, 163, 184, 0.12)' }
    : { bar: '#0d9488', text: '#6b7280', grid: 'rgba(107, 114, 128, 0.12)' }
)
const themeKey = computed(() => (isDark.value ? 'dark' : 'light'))
const hasData = computed(() => props.values.some((value) => value > 0))

const chartData = computed(() => ({
  labels: props.labels,
  datasets: [
    {
      label: props.title,
      data: props.values,
      backgroundColor: theme.value.bar,
      hoverBackgroundColor: theme.value.bar,
      borderRadius: { topLeft: 4, topRight: 4 },
      borderSkipped: 'bottom' as const,
      maxBarThickness: 18,
      barPercentage: 0.8,
      categoryPercentage: 0.9,
    },
  ],
}))

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: { duration: 250 },
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: { display: false },
    tooltip: {
      displayColors: false,
      callbacks: {
        label: (context: TooltipItem<'bar'>) => props.formatValue(context.parsed.y ?? 0),
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      border: { display: false },
      ticks: { color: theme.value.text, maxRotation: 0, autoSkipPadding: 12 },
    },
    y: {
      beginAtZero: true,
      border: { display: false },
      grid: { color: theme.value.grid },
      ticks: {
        color: theme.value.text,
        maxTicksLimit: 5,
        precision: props.integer ? 0 : undefined,
        callback: (value: string | number) => props.formatValue(Number(value)),
      },
    },
  },
}))
</script>
