<template>
  <form class="card cs-form-panel" :aria-label="t('creation.composer.title')" @submit.prevent="submit">
    <div class="cs-form-body">
      <!-- 分组与密钥 -->
      <section class="cs-group" aria-labelledby="creation-section-access">
        <h2 id="creation-section-access" class="cs-group-title">{{ t('creation.composer.sections.access') }}</h2>

        <div class="cs-field">
          <label for="creation-group" class="cs-field-label">{{ t('creation.composer.group') }}</label>
          <Select
            id="creation-group"
            :model-value="selectedGroupId || null"
            :options="groupOptions"
            :aria-label="t('creation.composer.group')"
            @update:model-value="selectGroup"
          >
            <template #selected>
              <span v-if="selectedGroup" class="cs-option">
                <span class="cs-option-main cs-option-name">
                  <PlatformIcon :platform="platformOf(selectedGroup)" size="md" />
                  <span>{{ selectedGroup.name }}</span>
                </span>
                <span class="cs-option-aside" :title="t('creation.composer.platform', { value: selectedGroup.platform })">{{ selectedGroup.platform }}</span>
              </span>
              <template v-else>{{ t('common.selectOption') }}</template>
            </template>
            <template #option="{ option }">
              <span class="cs-option">
                <span class="cs-option-main cs-option-name">
                  <PlatformIcon :platform="option.platform" size="md" />
                  <span>{{ option.label }}</span>
                </span>
                <span class="cs-option-aside">{{ option.description }}</span>
              </span>
            </template>
          </Select>
          <p class="cs-field-hint">{{ t('creation.composer.groupHint') }}</p>
          <p v-if="selectedGroup?.description" class="cs-field-hint">{{ selectedGroup.description }}</p>
        </div>

        <div class="cs-field">
          <div class="cs-field-head">
            <label for="creation-api-key" class="cs-field-label">{{ t('creation.composer.apiKey') }}</label>
            <button type="button" class="cs-field-link" :disabled="loadingKeys" @click="emit('reloadKeys')">
              <RefreshCw :class="{ 'animate-spin': loadingKeys }" aria-hidden="true" />
              {{ t('creation.composer.reloadKeys') }}
            </button>
          </div>
          <Select
            id="creation-api-key"
            :model-value="selectedKeyId || null"
            :options="keyOptions"
            :disabled="loadingKeys || availableKeys.length === 0"
            :placeholder="t('creation.composer.selectKey')"
            :aria-label="t('creation.composer.apiKey')"
            @update:model-value="selectKey"
          >
            <template #selected>
              <span v-if="selectedKey" class="cs-option">
                <span class="cs-option-main cs-option-name"><span>{{ selectedKey.name }}</span></span>
                <span class="cs-option-aside font-mono">{{ maskKey(selectedKey.key) }}</span>
              </span>
              <template v-else>{{ t('creation.composer.selectKey') }}</template>
            </template>
            <template #option="{ option }">
              <span class="cs-option">
                <span class="cs-option-main">
                  <span class="cs-option-name"><span>{{ option.label }}</span></span>
                  <span class="cs-option-sub font-mono">{{ option.description }}</span>
                </span>
              </span>
            </template>
          </Select>
          <p class="cs-field-hint">{{ t('creation.composer.apiKeyHint') }}</p>
          <p v-if="selectedKey && selectedGroup" class="cs-task-meta gap-x-3">
            <span>{{ t('creation.composer.keyBound', { group: selectedGroup.name }) }}</span>
            <span>{{ t('creation.composer.keySource', { name: selectedKey.name }) }}</span>
          </p>
          <div v-if="!loadingKeys && availableKeys.length === 0" class="cs-callout cs-callout--warning">
            <KeyRound class="cs-callout-icon" aria-hidden="true" />
            <div class="cs-callout-content">
              <p class="cs-callout-title">{{ t('creation.composer.noKey') }}</p>
              <div class="cs-callout-actions">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="creatingKey || !selectedGroup" @click="selectedGroup && emit('createKey', selectedGroup)">
                  <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                  {{ creatingKey ? t('creation.composer.creatingKey') : t('creation.composer.createKey') }}
                </button>
              </div>
            </div>
          </div>
          <p v-if="keyError" role="alert" class="cs-field-error">
            <CircleX aria-hidden="true" />
            <span>{{ keyError }}</span>
          </p>
        </div>
      </section>

      <!-- 创作内容 -->
      <section class="cs-group" aria-labelledby="creation-section-content">
        <h2 id="creation-section-content" class="cs-group-title">{{ t('creation.composer.sections.content') }}</h2>

        <!-- 协议只在生图时可选（OpenAI / Gemini 的参数不同）；生视频由所选模型决定 -->
        <div v-if="mode === 'image' && protocolOptions.length > 1" class="cs-field">
          <span id="creation-protocol-label" class="cs-field-label">{{ t('creation.composer.protocol') }}</span>
          <CreationChipGroup
            name="creation-protocol"
            :model-value="selectedProtocol"
            :options="protocolChipOptions"
            labelledby="creation-protocol-label"
            @update:model-value="changeProtocol"
          />
        </div>

        <div class="cs-field">
          <label for="creation-model" class="cs-field-label">{{ t('creation.composer.model') }}</label>
          <Select
            id="creation-model"
            :model-value="selectedModel || null"
            :options="modelOptions"
            :disabled="!models.length"
            :placeholder="t('creation.composer.noModels')"
            :empty-text="t('creation.composer.noModels')"
            :aria-label="t('creation.composer.model')"
            @update:model-value="selectModel"
          >
            <template #selected>
              <span v-if="selectedModel" class="cs-option">
                <span class="cs-option-main cs-option-name">
                  <ModelIcon :model="selectedModel" size="16px" />
                  <span class="font-mono">{{ selectedModel }}</span>
                </span>
              </span>
              <template v-else>{{ t('creation.composer.noModels') }}</template>
            </template>
            <template #option="{ option }">
              <span class="cs-option">
                <span class="cs-option-main cs-option-name">
                  <ModelIcon :model="option.label" size="16px" />
                  <span class="font-mono">{{ option.label }}</span>
                </span>
                <span v-if="option.price" class="cs-option-aside">{{ option.price }}</span>
              </span>
            </template>
          </Select>
          <p v-if="!models.length" class="cs-field-hint">{{ t('creation.composer.noModelsHint') }}</p>
          <div v-else-if="priceEntries.length" class="flex flex-wrap items-center gap-1.5" data-testid="creation-model-price">
            <span class="cs-field-hint">{{ t('creation.composer.price') }}</span>
            <span v-for="entry in priceEntries" :key="entry.label || 'flat'" class="cs-param-pill font-mono">
              <span v-if="entry.label" class="font-sans opacity-60">{{ entry.label }}</span>{{ entry.price }}<span class="font-sans opacity-60">{{ priceUnit }}</span>
            </span>
          </div>
          <p v-else class="cs-field-hint">{{ t('creation.composer.noPrice') }}</p>
        </div>

        <div class="cs-field">
          <div class="cs-field-head">
            <label for="creation-prompt" class="cs-field-label">{{ t('creation.composer.prompt') }}</label>
            <span class="cs-field-counter">{{ t('creation.composer.shortcutHint') }}</span>
          </div>
          <textarea
            id="creation-prompt"
            v-model="prompt"
            rows="5"
            class="input cs-prompt"
            aria-keyshortcuts="Meta+Enter Control+Enter"
            :placeholder="mode === 'image' ? t('creation.composer.promptImagePlaceholder') : t('creation.composer.promptVideoPlaceholder')"
            @keydown.meta.enter.prevent="submit"
            @keydown.ctrl.enter.prevent="submit"
          />
        </div>

        <ReferenceImagePicker
          :key="referenceContext"
          :kind="mode"
          :protocol="selectedProtocol"
          :model="selectedModel"
          :platform="selectedGroup?.platform ?? ''"
          :edit-supported="mode === 'image' ? Boolean(imageCapability?.edit_supported) : true"
          @change="updateReferences"
          @require-duration="setRequiredDuration"
        />
      </section>

      <!-- 生成参数 -->
      <section class="cs-group" aria-labelledby="creation-section-params">
        <h2 id="creation-section-params" class="cs-group-title">{{ t('creation.composer.sections.params') }}</h2>

        <template v-if="mode === 'image'">
          <template v-if="imageParamStyle === 'openai'">
            <div class="cs-field">
              <span id="creation-image-size-label" class="cs-field-label">{{ t('creation.composer.size') }}</span>
              <CreationChipGroup
                v-model="imageSizeOpenAI"
                name="creation-image-size"
                :options="sizeChipOptions"
                labelledby="creation-image-size-label"
                grid
                glyph
                chip-min="7.5rem"
              />
            </div>
            <div class="cs-field">
              <span id="creation-image-quality-label" class="cs-field-label">{{ t('creation.composer.quality') }}</span>
              <CreationChipGroup
                v-model="imageQuality"
                name="creation-image-quality"
                :options="qualityChipOptions"
                labelledby="creation-image-quality-label"
              />
            </div>
          </template>
          <template v-else>
            <div class="cs-field">
              <span id="creation-image-ratio-label" class="cs-field-label">{{ t('creation.composer.aspectRatio') }}</span>
              <CreationChipGroup
                v-model="imageAspectRatio"
                name="creation-image-ratio"
                :options="imageRatioChipOptions"
                labelledby="creation-image-ratio-label"
                grid
                glyph
              />
            </div>
            <div class="cs-field">
              <span id="creation-image-resolution-label" class="cs-field-label">{{ t('creation.composer.imageSize') }}</span>
              <CreationChipGroup
                v-model="imageResolution"
                name="creation-image-resolution"
                :options="imageResolutionChipOptions"
                labelledby="creation-image-resolution-label"
              />
            </div>
          </template>
          <div class="cs-field cs-field--inline">
            <label for="creation-image-count" class="cs-field-label">{{ t('creation.composer.count') }}</label>
            <CreationStepper
              v-model="imageCount"
              input-id="creation-image-count"
              :min="1"
              :disabled="selectedProtocol === 'gemini'"
              :label="t('creation.composer.count')"
              @step="stepImageCount"
              @commit="normalizeImageCount"
            />
          </div>
          <p v-if="selectedGroup && !submitsAsync" class="cs-field-warning" data-testid="creation-sync-hint">
            <TriangleAlert aria-hidden="true" />
            <span>{{ t('creation.composer.syncOnlyHint') }}</span>
          </p>
        </template>

        <template v-else>
          <div class="cs-field">
            <label for="creation-video-duration" class="cs-field-label">{{ t('creation.composer.duration') }}</label>
            <CreationStepper
              input-id="creation-video-duration"
              wide
              :model-value="durationValue"
              :min="durationRange.min"
              :max="durationRange.max"
              :label="t('creation.composer.duration')"
              :placeholder="t('creation.composer.durationAuto')"
              @update:model-value="setDuration"
              @step="stepDuration"
              @commit="normalizeDuration"
            />
          </div>
          <div class="cs-field">
            <span id="creation-video-resolution-label" class="cs-field-label">{{ t('creation.composer.resolution') }}</span>
            <CreationChipGroup
              v-model="videoResolution"
              name="creation-video-resolution"
              :options="videoResolutionChipOptions"
              labelledby="creation-video-resolution-label"
            />
          </div>
          <div class="cs-field">
            <span id="creation-video-ratio-label" class="cs-field-label">{{ t('creation.composer.ratio') }}</span>
            <CreationChipGroup
              v-model="videoRatio"
              name="creation-video-ratio"
              :options="videoRatioChipOptions"
              labelledby="creation-video-ratio-label"
              grid
              glyph
            />
          </div>
        </template>
      </section>
    </div>

    <!-- 提交栏：桌面端固定在面板底部，移动端吸底 -->
    <div class="cs-form-footer">
      <!-- 预计费用与余额 -->
      <div class="space-y-1.5">
        <div class="cs-cost" :data-state="costState" data-testid="creation-cost">
          <span class="cs-cost-label">{{ t('creation.cost.estimate') }}</span>
          <div class="min-w-0 text-right">
            <span class="cs-cost-value">{{ costValue }}</span>
            <div class="cs-cost-detail">{{ costDetail }}</div>
          </div>
        </div>
        <div v-if="availableBalance !== null" class="cs-balance" data-testid="creation-balance">
          <span>{{ enterpriseBalance ? t('enterprise.companyBalance') : t('common.availableBalance') }}</span>
          <span>{{ formatBalance(availableBalance) }}</span>
        </div>
      </div>
      <p v-if="formNotice" role="status" class="cs-field-warning">
        <TriangleAlert aria-hidden="true" />
        <span>{{ formNotice }}</span>
      </p>
      <p v-if="formError" role="alert" class="cs-field-error">
        <CircleX aria-hidden="true" />
        <span>{{ formError }}</span>
      </p>
      <button type="submit" class="btn btn-primary cs-submit" :disabled="atCapacity || cooldown || !canSubmit">
        <Sparkles aria-hidden="true" />
        <span>{{ mode === 'image' ? t('creation.composer.submitImage') : t('creation.composer.submitVideo') }}</span>
      </button>
      <p class="cs-submit-reason">{{ atCapacity ? t('creation.composer.atCapacity', { count: maxActive }) : t('creation.composer.submitHint') }}</p>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleX, KeyRound, Plus, RefreshCw, Sparkles, TriangleAlert } from '@lucide/vue'
import Select from '@/components/common/Select.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import CreationChipGroup from './CreationChipGroup.vue'
import CreationStepper from './CreationStepper.vue'
import ReferenceImagePicker from './ReferenceImagePicker.vue'
import type { ApiKey, GroupPlatform } from '@/types'
import type { CreationGroup, CreationKind, CreationProtocol, CreationReferenceImage, ImageGenerationInput } from '@/features/creation/types'
import {
  clampVideoDuration,
  creationPriceEntries,
  creationPriceRange,
  creationVideoModels,
  defaultVideoDuration,
  estimateCreationCost,
  defaultVideoRatios,
  defaultVideoResolutions,
  filterCreationGroups,
  filterCreationModels,
  formatCreationPrice,
  resolveCreationVideoProtocol,
  restoreSelectedGroupId,
  saveSelectedGroupId,
  videoDurationRange,
} from './viewModel'
import type { CreationChipOption, CreationSubmission } from './types'

const props = defineProps<{
  mode: CreationKind
  groups: CreationGroup[]
  apiKeys: ApiKey[]
  userId: number | string
  loadingKeys: boolean
  creatingKey: boolean
  keyError: string
  /** 进行中的任务已达上限，暂不接受新提交。 */
  atCapacity: boolean
  maxActive: number
  /** 当前可用于扣费的余额（美元）；不可见或未知时为 null。 */
  balance?: number | null
  /** 余额是企业的公司余额（标签不同）。 */
  enterpriseBalance?: boolean
}>()
const emit = defineEmits<{
  submit: [submission: CreationSubmission]
  createKey: [group: CreationGroup]
  reloadKeys: []
}>()
const { t } = useI18n()
const openAISizes = ['1024x1024', '1536x1024', '1024x1536', 'auto']
const geminiRatios = ['1:1', '16:9', '9:16', '3:2', '2:3', '4:3', '3:4', 'auto']
const geminiSizes = ['1K', '2K', '4K']
// xAI Imagine 官方比例与分辨率档，与后端 grokImagineAspectRatioValues 一致。
const grokRatios = ['1:1', '16:9', '9:16', '4:3', '3:4', '3:2', '2:3', '2:1', '1:2', '19.5:9', '9:19.5', '20:9', '9:20']
const grokResolutions = ['1K', '2K']
const imageQualities = ['low', 'medium', 'high', 'auto']

const selectedGroupId = ref(0)
const selectedKeyId = ref(0)
const selectedProtocol = ref<CreationProtocol>('openai')
const selectedModel = ref('')
const prompt = ref('')
const references = ref<CreationReferenceImage[]>([])
const imageSizeOpenAI = ref('1024x1024')
const imageAspectRatio = ref('1:1')
const imageResolution = ref('1K')
const imageQuality = ref('auto')
/** null 表示输入框被清空，失焦或提交时归一化为 1。 */
const imageCount = ref<number | null>(1)
/** 提交后短暂禁用按钮，防止双击重复提交（重复提交会重复计费）。 */
const cooldown = ref(false)
let cooldownTimer: ReturnType<typeof setTimeout> | undefined
const videoDuration = ref('')
const videoResolution = ref('')
const videoRatio = ref('16:9')
const formError = ref('')
const formNotice = ref('')
let currentContext = ''

const availableGroups = computed(() => filterCreationGroups(props.groups, props.mode))
const selectedGroup = computed(() => availableGroups.value.find(group => group.id === selectedGroupId.value) ?? null)
const imageCapability = computed(() => props.mode === 'image' ? selectedGroup.value?.image ?? null : null)
const videoCapability = computed(() => props.mode === 'video' ? selectedGroup.value?.video ?? null : null)
const protocolOptions = computed<CreationProtocol[]>(() => selectedGroup.value?.[props.mode]?.protocols ?? [])
/** 生视频列出分组内的全部模型（协议跟随模型）；生图按所选协议筛选。 */
const models = computed(() => props.mode === 'video'
  ? creationVideoModels(selectedGroup.value)
  : filterCreationModels(selectedGroup.value, 'image', selectedProtocol.value))
const selectedPricing = computed(() => models.value.find(model => model.id === selectedModel.value)?.pricing ?? null)
const priceEntries = computed(() => creationPriceEntries(selectedPricing.value))
const priceUnit = computed(() => selectedPricing.value ? t(`creation.composer.priceUnit.${selectedPricing.value.unit}`) : '')
// Grok 分组虽走 OpenAI Images 协议，但 xAI Imagine 只认比例 + 分辨率档，不支持 size / quality。
const imageParamStyle = computed<'openai' | 'gemini' | 'grok'>(() => {
  if (selectedProtocol.value === 'gemini') return 'gemini'
  return selectedGroup.value?.platform === 'grok' ? 'grok' : 'openai'
})
const imageRatioOptions = computed(() => imageParamStyle.value === 'grok' ? grokRatios : geminiRatios)
const imageResolutionOptions = computed(() => imageParamStyle.value === 'grok' ? grokResolutions : geminiSizes)
const availableKeys = computed(() => props.apiKeys.filter(key => key.status === 'active' && key.group_id === selectedGroupId.value))
const selectedKey = computed(() => availableKeys.value.find(key => key.id === selectedKeyId.value) ?? null)
const durationRange = computed(() => props.mode === 'video' && selectedProtocol.value !== 'openai'
  ? videoDurationRange(selectedProtocol.value, selectedModel.value, videoCapability.value)
  : { min: null, max: null })
const resolutionOptions = computed(() => props.mode === 'video' && selectedProtocol.value !== 'openai'
  ? defaultVideoResolutions(selectedProtocol.value, videoCapability.value)
  : [])
const ratioOptions = computed(() => props.mode === 'video' && selectedProtocol.value !== 'openai'
  ? defaultVideoRatios(selectedProtocol.value)
  : [])
const referenceContext = computed(() => `${props.mode}:${selectedGroupId.value}:${selectedProtocol.value}:${selectedModel.value}`)
const canSubmit = computed(() => Boolean(selectedGroup.value && selectedKey.value && selectedModel.value.trim() && prompt.value.trim()))
/** 视频本身就是异步任务；生图仅 OpenAI 协议且站点开启异步生图时走异步任务，否则只能同步请求。 */
const submitsAsync = computed(() => props.mode === 'video' || Boolean(selectedProtocol.value === 'openai' && imageCapability.value?.async_supported))

// ---- 预计费用 ----
/** 按当前模型的实付单价和已选参数估算；与提交时发出的参数保持一致。 */
const costEstimate = computed(() => {
  const pricing = selectedPricing.value
  if (props.mode === 'image') {
    return estimateCreationCost(pricing, {
      kind: 'image',
      size: imageParamStyle.value === 'openai' ? imageSizeOpenAI.value : imageResolution.value,
      count: selectedProtocol.value === 'gemini' ? 1 : imageCount.value ?? 1,
    })
  }
  const protocol = selectedProtocol.value
  if (protocol === 'openai') return null
  return estimateCreationCost(pricing, {
    kind: 'video',
    protocol,
    resolution: videoResolution.value,
    duration: clampVideoDuration(videoDuration.value, protocol, selectedModel.value, videoCapability.value),
  })
})
/** 订阅分组走订阅额度而不是余额，不显示余额也不提示余额不足。 */
const availableBalance = computed(() => {
  if (selectedGroup.value?.subscription_type === 'subscription') return null
  return typeof props.balance === 'number' && Number.isFinite(props.balance) ? props.balance : null
})
const costShortfall = computed(() => {
  const amount = costEstimate.value?.amount
  return amount !== undefined && availableBalance.value !== null && amount > availableBalance.value ? amount - availableBalance.value : 0
})
const costState = computed(() => {
  if (!costEstimate.value) return 'unavailable'
  return costShortfall.value > 0 ? 'insufficient' : undefined
})
const costValue = computed(() => {
  const estimate = costEstimate.value
  return estimate ? `${estimate.assumed ? '≈ ' : ''}${formatCreationPrice(estimate.amount)}` : '—'
})
const costDetail = computed(() => {
  const estimate = costEstimate.value
  if (!estimate) return t('creation.cost.unavailable')
  if (costShortfall.value > 0) return t('creation.cost.insufficient', { amount: formatCreationPrice(costShortfall.value) })
  const parts = [estimate.unit === 'request'
    ? t('creation.cost.detail.request')
    : t(`creation.cost.detail.${estimate.unit}`, { count: estimate.quantity, price: formatCreationPrice(estimate.unitPrice) })]
  if (estimate.tier) parts.push(estimate.tier)
  if (estimate.assumed) parts.push(t('creation.cost.assumed'))
  return parts.join(' · ')
})

function formatBalance(value: number): string {
  return `$${value.toFixed(2)}`
}

// ---- 下拉与胶囊选项 ----
const groupOptions = computed(() => availableGroups.value.map(group => ({
  value: group.id, label: group.name, description: group.platform, platform: platformOf(group),
})))
const keyOptions = computed(() => availableKeys.value.map(apiKey => ({
  value: apiKey.id, label: apiKey.name, description: maskKey(apiKey.key),
})))
const modelOptions = computed(() => models.value.map(model => {
  const range = creationPriceRange(model.pricing)
  return {
    value: model.id,
    label: model.id,
    price: range && model.pricing ? `${range} ${t(`creation.composer.priceUnit.${model.pricing.unit}`)}` : '',
  }
}))
/** auto 显示为本地化的「自动」，其余选项原样展示。 */
function chipOptions(values: readonly string[]): CreationChipOption[] {
  return values.map(value => ({ value, label: value === 'auto' ? t('creation.composer.auto') : value }))
}
const protocolChipOptions = computed(() => chipOptions(protocolOptions.value))
const sizeChipOptions = computed(() => chipOptions(openAISizes))
const qualityChipOptions = computed(() => chipOptions(imageQualities))
const imageRatioChipOptions = computed(() => chipOptions(imageRatioOptions.value))
const imageResolutionChipOptions = computed(() => chipOptions(imageResolutionOptions.value))
const videoResolutionChipOptions = computed<CreationChipOption[]>(() => [
  { value: '', label: t('creation.composer.auto'), title: t('creation.composer.resolutionAuto') },
  ...chipOptions(resolutionOptions.value),
])
const videoRatioChipOptions = computed(() => chipOptions(ratioOptions.value))
const durationValue = computed(() => {
  if (!videoDuration.value.trim()) return null
  const value = Number(videoDuration.value)
  return Number.isFinite(value) ? value : null
})

function platformOf(group: CreationGroup): GroupPlatform {
  return group.platform as GroupPlatform
}

function selectGroup(value: unknown): void {
  if (typeof value === 'number') selectedGroupId.value = value
}

function selectKey(value: unknown): void {
  if (typeof value === 'number') selectedKeyId.value = value
}

function selectModel(value: unknown): void {
  if (typeof value !== 'string' || value === selectedModel.value) return
  selectedModel.value = value
  // 换到另一种接口的模型时，参数项和参考图规则都不同，按新协议重置。
  if (syncVideoProtocol()) {
    references.value = []
    formNotice.value = ''
    resetParameters()
  }
}

/** 生视频不让用户选协议：由所选模型决定走哪个接口。返回协议是否发生了变化。 */
function syncVideoProtocol(): boolean {
  if (props.mode !== 'video') return false
  const protocol = resolveCreationVideoProtocol(selectedGroup.value, selectedModel.value) ?? protocolOptions.value[0] ?? 'seedance'
  if (protocol === selectedProtocol.value) return false
  selectedProtocol.value = protocol
  return true
}

function syncGroupContext(): void {
  const groups = availableGroups.value
  const modeChanged = currentContext.split(':')[0] !== props.mode
  if (!groups.length) {
    selectedGroupId.value = 0
    currentContext = ''
    return
  }
  if (modeChanged || !groups.some(group => group.id === selectedGroupId.value)) {
    selectedGroupId.value = restoreSelectedGroupId(localStorage, props.userId, props.mode, groups) ?? groups[0]?.id ?? 0
  }
  const group = selectedGroup.value
  if (!group) return
  saveSelectedGroupId(localStorage, props.userId, props.mode, group.id)
  const context = `${props.mode}:${group.id}`
  if (context === currentContext) return
  currentContext = context
  selectedProtocol.value = protocolOptions.value[0] ?? (props.mode === 'image' ? 'openai' : 'seedance')
  references.value = []
  formNotice.value = ''
  syncModel()
  resetParameters()
}

function syncModel(): void {
  selectedModel.value = models.value[0]?.id ?? ''
  syncVideoProtocol()
}

function resetParameters(): void {
  imageSizeOpenAI.value = '1024x1024'
  imageAspectRatio.value = '1:1'
  imageResolution.value = '1K'
  imageQuality.value = 'auto'
  imageCount.value = 1
  videoResolution.value = ''
  videoRatio.value = ratioOptions.value[0] ?? '16:9'
  const duration = props.mode === 'video' && selectedProtocol.value !== 'openai'
    ? defaultVideoDuration(selectedProtocol.value, selectedModel.value, videoCapability.value)
    : undefined
  videoDuration.value = duration === undefined ? '' : String(duration)
}

function isProtocol(value: string): value is CreationProtocol {
  return ['openai', 'gemini', 'seedance', 'minimax_v1', 'minimax_v2', 'grok'].includes(value)
}

function changeProtocol(value: string): void {
  if (!isProtocol(value) || !protocolOptions.value.includes(value)) return
  selectedProtocol.value = value
  references.value = []
  formNotice.value = ''
  syncModel()
  resetParameters()
}

function maskKey(value: string): string {
  if (value.length <= 10) return '••••••••'
  return `${value.slice(0, 5)}…${value.slice(-4)}`
}

function normalizeImageCount(): number {
  const value = Math.floor(Number(imageCount.value))
  const count = selectedProtocol.value === 'gemini' || !Number.isFinite(value) ? 1 : Math.max(1, value)
  imageCount.value = count
  return count
}

function stepImageCount(direction: 1 | -1): void {
  imageCount.value = Math.max(1, normalizeImageCount() + direction)
}

function normalizeDuration(): void {
  if (props.mode !== 'video' || selectedProtocol.value === 'openai') return
  const duration = clampVideoDuration(videoDuration.value, selectedProtocol.value, selectedModel.value, videoCapability.value)
  videoDuration.value = duration === undefined ? '' : String(duration)
}

function setDuration(value: number | null): void {
  videoDuration.value = value === null ? '' : String(value)
}

/**
 * 步进时长：留空时从协议默认值（没有则从区间端点）起步；
 * 之后沿方向找到下一个合法取值，兼容只允许离散档位的模型（如 Veo 3.1 的 4/6/8 秒）。
 */
function stepDuration(direction: 1 | -1): void {
  const protocol = selectedProtocol.value
  if (props.mode !== 'video' || protocol === 'openai') return
  const model = selectedModel.value
  const capability = videoCapability.value
  const range = durationRange.value
  const current = clampVideoDuration(videoDuration.value, protocol, model, capability)
  if (current === undefined) {
    const start = defaultVideoDuration(protocol, model, capability) ?? (direction > 0 ? range.min : range.max) ?? 5
    setDuration(clampVideoDuration(start, protocol, model, capability) ?? start)
    return
  }
  const limit = direction > 0 ? range.max ?? current + 60 : range.min ?? 1
  for (let candidate = current + direction; direction > 0 ? candidate <= limit : candidate >= limit; candidate += direction) {
    const next = clampVideoDuration(candidate, protocol, model, capability)
    if (next !== undefined && next !== current) {
      setDuration(next)
      return
    }
  }
  setDuration(current)
}

function updateReferences(images: CreationReferenceImage[]): void {
  references.value = images
  enforceVeoDuration()
}

function setRequiredDuration(seconds: number): void {
  videoDuration.value = String(seconds)
  formNotice.value = t('creation.composer.veoDurationAdjusted')
}

function enforceVeoDuration(): void {
  if (props.mode !== 'video' || selectedProtocol.value !== 'gemini' || !selectedModel.value.toLowerCase().includes('3.1')) return
  const needsEightSeconds = videoResolution.value.toLowerCase() === '1080p' || references.value.some(image => image.role === 'reference_image')
  if (needsEightSeconds && videoDuration.value !== '8') setRequiredDuration(8)
}

function startCooldown(): void {
  cooldown.value = true
  clearTimeout(cooldownTimer)
  cooldownTimer = setTimeout(() => { cooldown.value = false }, 1000)
}

function submit(): void {
  formError.value = ''
  const group = selectedGroup.value
  const apiKey = selectedKey.value
  const model = selectedModel.value.trim()
  const text = prompt.value.trim()
  if (props.atCapacity || cooldown.value) return
  if (!group || !apiKey || !model || !text) {
    formError.value = t('creation.composer.required')
    return
  }
  const count = normalizeImageCount()
  if (props.mode === 'image') {
    const protocol = selectedProtocol.value
    if (protocol !== 'openai' && protocol !== 'gemini') return
    const common = {
      apiKey: apiKey.key,
      protocol,
      platform: group.platform,
      model,
      prompt: text,
      n: protocol === 'gemini' ? 1 : count,
      referenceImages: references.value.length ? references.value : undefined,
    }
    const submitAsync = submitsAsync.value
    let input: ImageGenerationInput
    if (protocol === 'gemini') input = { ...common, protocol, aspectRatio: imageAspectRatio.value, imageSize: imageResolution.value }
    else if (imageParamStyle.value === 'grok') input = { ...common, protocol, aspectRatio: imageAspectRatio.value, imageSize: imageResolution.value, preferAsync: submitAsync }
    else input = { ...common, protocol, size: imageSizeOpenAI.value, quality: imageQuality.value, preferAsync: submitAsync }
    startCooldown()
    emit('submit', { kind: 'image', groupId: group.id, groupName: group.name, apiKey: apiKey.key, input })
    return
  }
  if (selectedProtocol.value === 'openai') return
  enforceVeoDuration()
  const duration = clampVideoDuration(videoDuration.value, selectedProtocol.value, selectedModel.value, videoCapability.value)
  startCooldown()
  emit('submit', {
    kind: 'video', groupId: group.id, groupName: group.name, apiKey: apiKey.key,
    input: {
      apiKey: apiKey.key, protocol: selectedProtocol.value, model, prompt: text,
      duration, resolution: videoResolution.value || undefined, ratio: videoRatio.value || undefined,
      referenceImages: references.value.length ? references.value : undefined,
    },
  })
}

watch([() => props.mode, selectedGroupId, availableGroups], syncGroupContext, { immediate: true })
watch(availableKeys, keys => {
  if (!keys.some(key => key.id === selectedKeyId.value)) selectedKeyId.value = keys[0]?.id ?? 0
}, { immediate: true })
watch([videoResolution, () => props.mode], enforceVeoDuration)
onBeforeUnmount(() => clearTimeout(cooldownTimer))
</script>
