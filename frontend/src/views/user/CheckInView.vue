<template>
  <AppLayout>
    <div class="mx-auto max-w-2xl space-y-5">
      <!-- 加载中 -->
      <div v-if="loading" class="card flex items-center justify-center py-20" data-testid="check-in-loading">
        <div class="text-center">
          <div class="mx-auto h-9 w-9 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          <p class="mt-4 text-sm text-gray-500 dark:text-dark-400">{{ t('checkIn.loading') }}</p>
        </div>
      </div>

      <!-- 加载失败 -->
      <div v-else-if="loadError" class="card px-6 py-16 text-center" data-testid="check-in-error">
        <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-red-50 dark:bg-red-900/20">
          <Icon name="exclamationCircle" size="lg" class="text-red-500 dark:text-red-400" />
        </div>
        <h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">{{ t('checkIn.loadFailed') }}</h2>
        <button class="btn btn-secondary mt-5" type="button" @click="loadStatus()">
          <Icon name="refresh" size="sm" />
          {{ t('checkIn.retry') }}
        </button>
      </div>

      <!-- 功能未开启 -->
      <div v-else-if="!status?.enabled" class="card px-6 py-16 text-center" data-testid="check-in-disabled">
        <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800">
          <Icon name="calendar" size="xl" class="text-gray-400 dark:text-dark-500" />
        </div>
        <h2 class="mt-5 text-lg font-semibold text-gray-900 dark:text-white">{{ t('checkIn.disabledTitle') }}</h2>
        <p class="mx-auto mt-2 max-w-sm text-sm text-gray-500 dark:text-dark-400">{{ t('checkIn.disabledDescription') }}</p>
      </div>

      <template v-else-if="status">
        <!-- 主签到卡：今日奖励 + 连续天数 + 唯一的操作按钮 -->
        <section class="card relative overflow-hidden" data-testid="check-in-hero">
          <div
            class="pointer-events-none absolute -right-20 -top-24 h-56 w-56 rounded-full blur-3xl transition-colors duration-500"
            :class="status.checked_in_today ? 'bg-emerald-400/15' : 'bg-primary-400/15'"
            aria-hidden="true"
          ></div>

          <div class="relative px-6 py-8 text-center sm:px-10">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('checkIn.todayReward') }}</p>
            <p
              class="mt-2 text-4xl font-bold tabular-nums tracking-tight"
              :class="status.checked_in_today ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-900 dark:text-white'"
              data-testid="check-in-today-reward"
            >
              {{ status.checked_in_today ? formatReward(status.today_reward) : t('checkIn.rewardPending') }}
            </p>
            <p v-if="!status.checked_in_today" class="mt-2 text-sm text-gray-500 dark:text-dark-400" data-testid="check-in-range">
              {{ rewardRangeHint }}
            </p>

            <p class="mt-4 inline-flex items-center gap-1.5 text-sm text-gray-600 dark:text-dark-300" data-testid="check-in-streak">
              <Icon name="fire" size="sm" :class="status.streak_days > 0 ? 'text-orange-500' : 'text-gray-300 dark:text-dark-600'" />
              <template v-if="status.streak_days > 0">
                {{ t('checkIn.streakLine', { days: status.streak_days }) }}
              </template>
              <template v-else>{{ t('checkIn.streakNone') }}</template>
            </p>

            <div
              v-if="!status.checked_in_today && status.captcha_required"
              class="mx-auto mt-6 max-w-sm rounded-xl border border-gray-200 bg-gray-50/70 p-4 text-left dark:border-dark-700 dark:bg-dark-800/40"
            >
              <p class="mb-3 text-xs text-gray-600 dark:text-dark-300">{{ t('checkIn.captchaHint') }}</p>
              <CaptchaChallenge
                ref="captchaRef"
                :turnstile-enabled="turnstileEnabled"
                :turnstile-site-key="turnstileSiteKey"
                :tencent-enabled="tencentEnabled"
                :tencent-app-id="tencentAppId"
                :tencent-region="tencentRegion"
                :aliyun-enabled="aliyunEnabled"
                :aliyun-scene-id="aliyunSceneId"
                :aliyun-prefix="aliyunPrefix"
                :aliyun-region="aliyunRegion"
                @verify="onCaptchaVerify"
                @error="onCaptchaError"
                @expire="onCaptchaExpire"
              />
            </div>

            <div class="mx-auto mt-6 max-w-sm">
              <button
                v-if="!status.checked_in_today"
                class="btn btn-primary w-full py-3 text-base"
                type="button"
                data-testid="check-in-button"
                :disabled="submitting"
                @click="handleCheckIn"
              >
                <Icon v-if="submitting" name="refresh" size="md" class="animate-spin" />
                {{ submitting ? t('checkIn.checkingIn') : t('checkIn.checkInButton') }}
              </button>
              <div
                v-else
                class="flex w-full items-center justify-center gap-2 rounded-xl bg-gray-100 px-4 py-3 text-sm text-gray-500 dark:bg-dark-800 dark:text-dark-400"
                data-testid="check-in-today-done"
              >
                <Icon name="checkCircle" size="sm" class="text-emerald-500" />
                {{ t('checkIn.doneButton', { time: resetTime }) }}
              </div>
            </div>
          </div>
        </section>

        <!-- 月历 -->
        <section class="card px-4 pb-4 pt-5 sm:px-6" data-testid="check-in-calendar">
          <div class="flex items-center justify-between gap-3">
            <div class="min-w-0">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white" data-testid="calendar-title">
                {{ calendarTitle }}
              </h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                {{ t('checkIn.calendarSummary', { days: calendar?.checked_days ?? 0, amount: formatReward(calendar?.total_reward ?? 0) }) }}
              </p>
            </div>
            <div class="flex items-center gap-0.5">
              <button
                type="button"
                class="btn btn-ghost h-8 w-8 !p-0"
                :aria-label="t('checkIn.prevMonth')"
                data-testid="calendar-prev"
                :disabled="calendarLoading"
                @click="shiftMonth(-1)"
              >
                <Icon name="chevronLeft" size="sm" />
              </button>
              <button
                v-if="calendarMonth !== todayMonth"
                type="button"
                class="btn btn-ghost h-8 !px-2 text-xs"
                data-testid="calendar-today"
                :disabled="calendarLoading"
                @click="loadCalendar(todayMonth)"
              >
                {{ t('checkIn.thisMonth') }}
              </button>
              <button
                type="button"
                class="btn btn-ghost h-8 w-8 !p-0"
                :aria-label="t('checkIn.nextMonth')"
                data-testid="calendar-next"
                :disabled="calendarLoading || calendarMonth >= todayMonth"
                @click="shiftMonth(1)"
              >
                <Icon name="chevronRight" size="sm" />
              </button>
            </div>
          </div>

          <div class="mt-4 grid grid-cols-7 gap-1 text-center" aria-hidden="true">
            <span
              v-for="weekday in weekdayHeaders"
              :key="weekday"
              class="pb-1 text-[11px] font-medium text-gray-400 dark:text-dark-500"
            >
              {{ t('checkIn.weekday.' + weekday) }}
            </span>
          </div>

          <div
            class="grid grid-cols-7 gap-1 transition-opacity"
            :class="calendarLoading ? 'opacity-50' : ''"
            data-testid="calendar-grid"
          >
            <div v-for="blank in leadingBlanks" :key="'blank-' + blank" aria-hidden="true"></div>
            <div
              v-for="cell in calendarCells"
              :key="cell.date"
              class="flex h-14 flex-col items-center justify-center rounded-lg text-xs"
              :class="cellClass(cell)"
              :title="cell.reward !== null ? `${cell.date} · ${formatReward(cell.reward)}` : cell.date"
              :data-date="cell.date"
              :data-signed="cell.reward !== null ? 'true' : undefined"
            >
              <span class="tabular-nums leading-none">{{ cell.dayOfMonth }}</span>
              <span
                v-if="cell.reward !== null"
                class="mt-1 text-[10px] font-medium tabular-nums leading-none"
              >{{ formatCompactReward(cell.reward) }}</span>
            </div>
          </div>

          <p
            v-if="!calendarLoading && calendar && calendar.checked_days === 0"
            class="mt-3 text-center text-xs text-gray-400 dark:text-dark-500"
          >
            {{ t('checkIn.calendarEmpty') }}
          </p>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CaptchaChallenge from '@/components/CaptchaChallenge.vue'
import { getCheckInCalendar, getCheckInStatus, performCheckIn, type CheckInCalendar, type CheckInPayload, type CheckInStatus } from '@/api/checkIn'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorCode } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<CheckInStatus | null>(null)
const loading = ref(true)
const loadError = ref(false)
const submitting = ref(false)
const captchaToken = ref('')
const captchaRandstr = ref('')
const captchaRef = ref<InstanceType<typeof CaptchaChallenge> | null>(null)

const publicSettings = computed(() => appStore.cachedPublicSettings)
const turnstileEnabled = computed(() => publicSettings.value?.turnstile_enabled === true)
const turnstileSiteKey = computed(() => publicSettings.value?.turnstile_site_key || '')
const tencentEnabled = computed(() => publicSettings.value?.tencent_captcha_enabled === true)
const tencentAppId = computed(() => publicSettings.value?.tencent_captcha_app_id || '')
const tencentRegion = computed(() => publicSettings.value?.tencent_captcha_region || 'cn')
const aliyunEnabled = computed(() => publicSettings.value?.aliyun_captcha_enabled === true)
const aliyunSceneId = computed(() => publicSettings.value?.aliyun_captcha_scene_id || '')
const aliyunPrefix = computed(() => publicSettings.value?.aliyun_captcha_prefix || '')
const aliyunRegion = computed(() => publicSettings.value?.aliyun_captcha_region || 'cn')

const captchaProvider = computed<'turnstile' | 'tencent' | 'aliyun' | null>(() => {
  if (turnstileEnabled.value && turnstileSiteKey.value) return 'turnstile'
  if (tencentEnabled.value && tencentAppId.value) return 'tencent'
  if (aliyunEnabled.value && aliyunSceneId.value && aliyunPrefix.value) return 'aliyun'
  return null
})

// 与站内其它余额展示一致使用「$」；不足 $1 的小额奖励最多保留 4 位小数，避免显示成 $0.00。
const preciseRewardFormatter = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 4,
})
const compactRewardFormatter = new Intl.NumberFormat('en-US', {
  style: 'currency',
  currency: 'USD',
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})

function formatReward(amount: number | null | undefined): string {
  const value = amount ?? 0
  return Math.abs(value) >= 1 ? compactRewardFormatter.format(value) : preciseRewardFormatter.format(value)
}

function formatCompactReward(amount: number): string {
  return compactRewardFormatter.format(amount)
}

// 只展示合并后的奖励区间，不涉及各档位权重与概率。
const rewardRangeHint = computed(() => {
  const min = status.value?.reward_min ?? 0
  const max = status.value?.reward_max ?? 0
  if (max <= 0) return t('checkIn.rewardRangeUnknown')
  if (min === max) return t('checkIn.rewardFixed', { amount: formatReward(min) })
  return t('checkIn.rewardRange', { min: formatReward(min), max: formatReward(max) })
})

const resetTime = computed(() => {
  const value = status.value?.next_reset_at
  const date = value ? new Date(value) : null
  if (!date || Number.isNaN(date.getTime())) return '00:00'
  return new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit', hour12: false }).format(date)
})

// ---- 月历 ----
// 周一为一周起点；按月向后端拉取整月记录。
const weekdayHeaders = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const
const calendar = ref<CheckInCalendar | null>(null)
const calendarMonth = ref('')
const calendarLoading = ref(false)
const todayMonth = computed(() => (status.value?.today_date ?? '').slice(0, 7))

function monthParts(month: string): [number, number] {
  const [year, monthIndex] = month.split('-').map(Number)
  return [year, monthIndex]
}

const calendarTitle = computed(() => {
  if (!calendarMonth.value) return ''
  const [year, month] = monthParts(calendarMonth.value)
  return new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long' }).format(new Date(year, month - 1, 1))
})

const leadingBlanks = computed(() => {
  if (!calendar.value) return []
  const [year, month] = monthParts(calendar.value.month)
  const offset = (new Date(year, month - 1, 1).getDay() + 6) % 7
  return Array.from({ length: offset }, (_, index) => index + 1)
})

const calendarCells = computed(() => {
  const current = calendar.value
  if (!current) return []
  const rewards = new Map(current.days.map((day) => [day.date, day.reward]))
  const [year, month] = monthParts(current.month)
  const daysInMonth = new Date(year, month, 0).getDate()

  return Array.from({ length: daysInMonth }, (_, index) => {
    const dayOfMonth = index + 1
    const date = `${current.month}-${String(dayOfMonth).padStart(2, '0')}`
    return {
      date,
      dayOfMonth,
      reward: rewards.get(date) ?? null,
      isToday: date === current.today,
      isFuture: date > current.today,
    }
  })
})

function cellClass(cell: { reward: number | null; isToday: boolean; isFuture: boolean }): string {
  const classes: string[] = []
  if (cell.reward !== null) {
    classes.push('bg-emerald-50 font-medium text-emerald-700 dark:bg-emerald-900/25 dark:text-emerald-300')
  } else if (cell.isFuture) {
    classes.push('text-gray-300 dark:text-dark-600')
  } else {
    classes.push('text-gray-500 dark:text-dark-400')
  }
  if (cell.isToday) {
    classes.push('ring-2 ring-inset ring-primary-500/70 dark:ring-primary-400/70')
  }
  return classes.join(' ')
}

async function loadCalendar(month: string): Promise<void> {
  calendarLoading.value = true
  try {
    calendar.value = await getCheckInCalendar(month || undefined)
    calendarMonth.value = calendar.value.month
  } catch (error) {
    appStore.showError(t('checkIn.calendarLoadFailed'))
    console.error('Failed to load check-in calendar:', error)
  } finally {
    calendarLoading.value = false
  }
}

function shiftMonth(delta: number): void {
  if (!calendarMonth.value) return
  const [year, month] = monthParts(calendarMonth.value)
  const target = new Date(year, month - 1 + delta, 1)
  const value = `${target.getFullYear()}-${String(target.getMonth() + 1).padStart(2, '0')}`
  if (value > todayMonth.value) return
  void loadCalendar(value)
}

// silent=true 时只在后台刷新数据，不切换到整页加载态，避免签到后页面闪烁。
async function loadStatus(options: { silent?: boolean } = {}): Promise<void> {
  if (!options.silent) loading.value = true
  loadError.value = false
  try {
    if (!appStore.publicSettingsLoaded) await appStore.fetchPublicSettings()
    status.value = await getCheckInStatus()
    if (status.value.enabled) {
      await loadCalendar(calendarMonth.value || todayMonth.value)
    }
  } catch (error) {
    if (!options.silent) loadError.value = true
    appStore.showError(t('checkIn.loadFailed'))
    console.error('Failed to load check-in status:', error)
  } finally {
    loading.value = false
  }
}

function onCaptchaVerify(tokenOrTicket: string, randstr: string): void {
  captchaToken.value = tokenOrTicket
  captchaRandstr.value = randstr
}

function onCaptchaExpire(): void {
  captchaToken.value = ''
  captchaRandstr.value = ''
}

function onCaptchaError(): void {
  captchaToken.value = ''
  captchaRandstr.value = ''
  appStore.showError(t('checkIn.errors.captchaFailed'))
}

async function acquireCaptchaPayload(): Promise<CheckInPayload | null> {
  if (!status.value?.captcha_required) return {}

  if (captchaProvider.value === 'tencent' || captchaProvider.value === 'aliyun') {
    const proof = await captchaRef.value?.verifyAction()
    if (!proof?.token) {
      appStore.showError(t('checkIn.errors.captchaRequired'))
      return null
    }
    if (captchaProvider.value === 'tencent') {
      return { tencent_ticket: proof.token, tencent_randstr: proof.randstr }
    }
    return { turnstile_token: proof.token }
  }

  if (captchaProvider.value === 'turnstile' && captchaToken.value) {
    return { turnstile_token: captchaToken.value }
  }

  appStore.showError(t('checkIn.errors.captchaRequired'))
  return null
}

async function handleCheckIn(): Promise<void> {
  if (!status.value || status.value.checked_in_today || submitting.value) return
  submitting.value = true
  try {
    const payload = await acquireCaptchaPayload()
    if (!payload) return

    const result = await performCheckIn(payload)
    appStore.showSuccess(t('checkIn.successDescription', { amount: formatReward(result.reward), days: result.streak_days }))
    captchaToken.value = ''
    captchaRandstr.value = ''
    await Promise.all([
      loadStatus({ silent: true }),
      authStore.refreshUser({ fresh: true }).catch((error: unknown) => {
        console.error('Failed to refresh user after check-in:', error)
      }),
    ])
  } catch (error: unknown) {
    const code = extractApiErrorCode(error)
    if (code === 'CHECK_IN_ALREADY_DONE') {
      appStore.showInfo(t('checkIn.errors.alreadyDone'))
      await loadStatus({ silent: true })
    } else if (code === 'CHECK_IN_DISABLED') {
      appStore.showInfo(t('checkIn.errors.disabled'))
      status.value = status.value ? { ...status.value, enabled: false } : null
    } else if (code === 'CHECK_IN_CAPTCHA_REQUIRED') {
      appStore.showError(t('checkIn.errors.captchaRequired'))
    } else {
      appStore.showError(t('checkIn.errors.submitFailed'))
    }
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  void loadStatus()
})
</script>
