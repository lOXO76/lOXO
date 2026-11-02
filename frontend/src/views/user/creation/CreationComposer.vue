<template>
  <form class="card space-y-5 p-5" :aria-busy="busy" @submit.prevent="submit">
    <div>
      <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">{{ t('creation.composer.title') }}</h2>
    </div>

    <div>
      <label for="creation-group" class="input-label">{{ t('creation.composer.group') }}</label>
      <select id="creation-group" v-model.number="selectedGroupId" class="input">
        <option v-for="group in availableGroups" :key="group.id" :value="group.id">{{ group.name }} · {{ group.platform }}</option>
      </select>
      <p class="input-hint">{{ t('creation.composer.groupHint') }}</p>
      <div v-if="selectedGroup" class="mt-2 flex flex-wrap gap-2 text-xs text-gray-500 dark:text-gray-400">
        <span class="rounded bg-gray-100 px-2 py-1 dark:bg-dark-800">{{ t('creation.composer.platform', { value: selectedGroup.platform }) }}</span>
      </div>
      <p v-if="selectedGroup?.description" class="mt-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{{ selectedGroup.description }}</p>
    </div>

    <div>
      <div class="flex items-center justify-between gap-2">
        <label for="creation-api-key" class="input-label">{{ t('creation.composer.apiKey') }}</label>
        <button type="button" class="text-xs text-primary-600 hover:underline dark:text-primary-400" :disabled="loadingKeys" @click="emit('reloadKeys')">{{ t('creation.composer.reloadKeys') }}</button>
      </div>
      <select id="creation-api-key" v-model.number="selectedKeyId" class="input" :disabled="loadingKeys || availableKeys.length === 0">
        <option :value="0">{{ t('creation.composer.selectKey') }}</option>
        <option v-for="apiKey in availableKeys" :key="apiKey.id" :value="apiKey.id">{{ apiKey.name }} · {{ maskKey(apiKey.key) }}</option>
      </select>
      <p class="input-hint">{{ t('creation.composer.apiKeyHint') }}</p>
      <div v-if="selectedKey && selectedGroup" class="mt-2 rounded-lg bg-gray-50 p-3 text-xs leading-5 text-gray-600 dark:bg-dark-900/70 dark:text-gray-300">
        <p>{{ t('creation.composer.keyBound', { group: selectedGroup.name }) }}</p>
        <p>{{ t('creation.composer.keySource', { name: selectedKey.name }) }}</p>
      </div>
      <div v-if="!loadingKeys && availableKeys.length === 0" class="mt-3 rounded-lg border border-dashed border-gray-300 p-3 dark:border-dark-600">
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('creation.composer.noKey') }}</p>
        <button type="button" class="btn btn-secondary btn-sm mt-2 w-full" :disabled="creatingKey || !selectedGroup" @click="selectedGroup && emit('createKey', selectedGroup)">
          <Icon name="plus" size="sm" class="mr-1" />{{ creatingKey ? t('creation.composer.creatingKey') : t('creation.composer.createKey') }}
        </button>
      </div>
      <div v-if="keyError" role="alert" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ keyError }}</div>
    </div>

    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
      <div v-if="protocolOptions.length > 1">
        <label for="creation-protocol" class="input-label">{{ t('creation.composer.protocol') }}</label>
        <select id="creation-protocol" :value="selectedProtocol" class="input" @change="changeProtocol">
          <option v-for="protocol in protocolOptions" :key="protocol" :value="protocol">{{ protocol }}</option>
        </select>
      </div>
      <div>
        <label for="creation-model" class="input-label">{{ t('creation.composer.model') }}</label>
        <select id="creation-model" v-model="selectedModel" class="input" :disabled="!models.length">
          <option v-if="!models.length" value="">{{ t('creation.composer.noModels') }}</option>
          <option v-for="model in models" :key="model.id" :value="model.id">{{ model.id }}</option>
        </select>
        <p v-if="!models.length" class="input-hint">{{ t('creation.composer.noModelsHint') }}</p>
        <div v-else-if="priceEntries.length" class="mt-2 flex flex-wrap items-center gap-1.5 text-xs" data-testid="creation-model-price">
          <span class="text-gray-500 dark:text-gray-400">{{ t('creation.composer.price') }}</span>
          <span v-for="entry in priceEntries" :key="entry.label || 'flat'" class="inline-flex items-center gap-1 rounded bg-gray-100 px-2 py-1 font-mono text-gray-800 dark:bg-dark-800 dark:text-gray-200">
            <span v-if="entry.label" class="font-sans text-gray-400 dark:text-gray-500">{{ entry.label }}</span>{{ entry.price }}<span class="font-sans text-gray-400 dark:text-gray-500">{{ priceUnit }}</span>
          </span>
        </div>
        <p v-else class="input-hint">{{ t('creation.composer.noPrice') }}</p>
      </div>
    </div>

    <div>
      <label for="creation-prompt" class="input-label">{{ t('creation.composer.prompt') }}</label>
      <textarea id="creation-prompt" v-model="prompt" rows="5" class="input min-h-28 resize-y leading-6" :placeholder="mode === 'image' ? t('creation.composer.promptImagePlaceholder') : t('creation.composer.promptVideoPlaceholder')" />
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

    <div v-if="mode === 'image'" class="grid gap-4 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
      <div v-if="imageParamStyle === 'openai'">
        <label for="creation-image-size" class="input-label">{{ t('creation.composer.size') }}</label>
        <select id="creation-image-size" v-model="imageSizeOpenAI" class="input"><option v-for="size in openAISizes" :key="size" :value="size">{{ size }}</option></select>
      </div>
      <div v-else>
        <label for="creation-image-ratio" class="input-label">{{ t('creation.composer.aspectRatio') }}</label>
        <select id="creation-image-ratio" v-model="imageAspectRatio" class="input"><option v-for="ratio in imageRatioOptions" :key="ratio" :value="ratio">{{ ratio }}</option></select>
      </div>
      <div v-if="imageParamStyle === 'openai'">
        <label for="creation-image-quality" class="input-label">{{ t('creation.composer.quality') }}</label>
        <select id="creation-image-quality" v-model="imageQuality" class="input"><option v-for="quality in imageQualities" :key="quality" :value="quality">{{ quality }}</option></select>
      </div>
      <div v-else>
        <label for="creation-image-resolution" class="input-label">{{ t('creation.composer.imageSize') }}</label>
        <select id="creation-image-resolution" v-model="imageResolution" class="input"><option v-for="size in imageResolutionOptions" :key="size" :value="size">{{ size }}</option></select>
      </div>
      <div>
        <label for="creation-image-count" class="input-label">{{ t('creation.composer.count') }}</label>
        <input id="creation-image-count" v-model.number="imageCount" type="number" min="1" step="1" class="input" :disabled="selectedProtocol === 'gemini'" @blur="normalizeImageCount" />
      </div>
      <label v-if="selectedProtocol === 'openai' && imageCapability?.async_supported" class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-700 sm:col-span-2 xl:col-span-1 2xl:col-span-2">
        <input v-model="preferAsync" type="checkbox" class="mt-1 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
        <span><span class="block text-gray-800 dark:text-gray-200">{{ t('creation.composer.async') }}</span><span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('creation.composer.asyncHint') }}</span></span>
      </label>
    </div>

    <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
      <div>
        <label for="creation-video-duration" class="input-label">{{ t('creation.composer.duration') }}</label>
        <input id="creation-video-duration" v-model="videoDuration" type="number" step="1" :min="durationRange.min ?? undefined" :max="durationRange.max ?? undefined" class="input" :placeholder="t('creation.composer.durationAuto')" @blur="normalizeDuration" />
      </div>
      <div>
        <label for="creation-video-resolution" class="input-label">{{ t('creation.composer.resolution') }}</label>
        <select id="creation-video-resolution" v-model="videoResolution" class="input">
          <option value="">{{ t('creation.composer.resolutionAuto') }}</option>
          <option v-for="resolution in resolutionOptions" :key="resolution" :value="resolution">{{ resolution }}</option>
        </select>
      </div>
      <div>
        <label for="creation-video-ratio" class="input-label">{{ t('creation.composer.ratio') }}</label>
        <select id="creation-video-ratio" v-model="videoRatio" class="input"><option v-for="ratio in ratioOptions" :key="ratio" :value="ratio">{{ ratio }}</option></select>
      </div>
    </div>

    <p v-if="formNotice" role="status" class="text-xs text-amber-700 dark:text-amber-300">{{ formNotice }}</p>
    <p v-if="formError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ formError }}</p>
    <button type="submit" class="btn btn-primary w-full justify-center" :disabled="busy || !canSubmit">
      <Icon :name="busy ? 'refresh' : 'sparkles'" size="sm" class="mr-2" :class="busy ? 'animate-spin' : ''" />
      {{ busy ? t('creation.composer.busy') : mode === 'image' ? t('creation.composer.submitImage') : t('creation.composer.submitVideo') }}
    </button>
    <p v-if="busy" class="text-center text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('creation.composer.busyHint') }}</p>
  </form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ReferenceImagePicker from './ReferenceImagePicker.vue'
import type { ApiKey } from '@/types'
import type { CreationGroup, CreationKind, CreationProtocol, CreationReferenceImage, ImageGenerationInput } from '@/features/creation/types'
import {
  clampVideoDuration,
  creationPriceEntries,
  defaultVideoDuration,
  defaultVideoRatios,
  defaultVideoResolutions,
  filterCreationGroups,
  filterCreationModels,
  restoreSelectedGroupId,
  saveSelectedGroupId,
  videoDurationRange,
} from './viewModel'
import type { CreationSubmission } from './types'

const props = defineProps<{
  mode: CreationKind
  groups: CreationGroup[]
  apiKeys: ApiKey[]
  userId: number | string
  loadingKeys: boolean
  creatingKey: boolean
  keyError: string
  busy: boolean
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
const imageCount = ref(1)
const preferAsync = ref(false)
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
const models = computed(() => filterCreationModels(selectedGroup.value, props.mode, selectedProtocol.value))
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
}

function resetParameters(): void {
  imageSizeOpenAI.value = '1024x1024'
  imageAspectRatio.value = '1:1'
  imageResolution.value = '1K'
  imageQuality.value = 'auto'
  imageCount.value = 1
  preferAsync.value = false
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

function changeProtocol(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
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

function normalizeImageCount(): void {
  const value = Math.floor(Number(imageCount.value))
  imageCount.value = Number.isFinite(value) ? Math.max(1, value) : 1
  if (selectedProtocol.value === 'gemini') imageCount.value = 1
}

function normalizeDuration(): void {
  if (props.mode !== 'video' || selectedProtocol.value === 'openai') return
  const duration = clampVideoDuration(videoDuration.value, selectedProtocol.value, selectedModel.value, videoCapability.value)
  videoDuration.value = duration === undefined ? '' : String(duration)
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

function submit(): void {
  formError.value = ''
  const group = selectedGroup.value
  const apiKey = selectedKey.value
  const model = selectedModel.value.trim()
  const text = prompt.value.trim()
  if (props.busy) return
  if (!group || !apiKey || !model || !text) {
    formError.value = t('creation.composer.required')
    return
  }
  normalizeImageCount()
  if (props.mode === 'image') {
    const protocol = selectedProtocol.value
    if (protocol !== 'openai' && protocol !== 'gemini') return
    const common = {
      apiKey: apiKey.key,
      protocol,
      platform: group.platform,
      model,
      prompt: text,
      n: protocol === 'gemini' ? 1 : imageCount.value,
      referenceImages: references.value.length ? references.value : undefined,
    }
    const submitAsync = Boolean(imageCapability.value?.async_supported && preferAsync.value)
    let input: ImageGenerationInput
    if (protocol === 'gemini') input = { ...common, protocol, aspectRatio: imageAspectRatio.value, imageSize: imageResolution.value }
    else if (imageParamStyle.value === 'grok') input = { ...common, protocol, aspectRatio: imageAspectRatio.value, imageSize: imageResolution.value, preferAsync: submitAsync }
    else input = { ...common, protocol, size: imageSizeOpenAI.value, quality: imageQuality.value, preferAsync: submitAsync }
    emit('submit', { kind: 'image', groupId: group.id, groupName: group.name, apiKey: apiKey.key, input })
    return
  }
  if (selectedProtocol.value === 'openai') return
  enforceVeoDuration()
  const duration = clampVideoDuration(videoDuration.value, selectedProtocol.value, selectedModel.value, videoCapability.value)
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
</script>
