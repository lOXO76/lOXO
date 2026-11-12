<template>
  <div v-if="rule.maxImages > 0" class="cs-field">
    <div class="cs-field-head">
      <span id="creation-reference-label" class="cs-field-label">{{ t('creation.composer.referenceImages') }}</span>
      <span class="cs-field-counter">{{ drafts.length ? `${drafts.length} / ${rule.maxImages}` : t('creation.composer.referenceLimit', { count: rule.maxImages }) }}</span>
    </div>

    <!-- 新上传图片的用途（首帧 / 尾帧 / 参考图） -->
    <template v-if="rule.roles.length > 1">
      <span id="creation-upload-role-label" class="sr-only">{{ t('creation.composer.role') }}</span>
      <CreationChipGroup
        name="creation-upload-role"
        :model-value="uploadRole"
        :options="roleOptions"
        labelledby="creation-upload-role-label"
        @update:model-value="setUploadRole"
      />
    </template>

    <ul :class="drafts.length ? 'cs-thumb-list' : undefined" aria-labelledby="creation-reference-label">
      <li v-for="draft in drafts" :key="draft.id" class="min-w-0">
        <div class="cs-thumb" :title="draft.file.name">
          <img :src="draft.previewUrl" :alt="draft.file.name" />
          <span v-if="rule.roles.length === 1 && draft.role !== 'reference_image'" class="cs-thumb-label">{{ t(`creation.composer.roles.${draft.role}`) }}</span>
          <button type="button" class="cs-thumb-remove" :aria-label="t('creation.composer.removeImage', { name: draft.file.name })" @click="removeImage(draft.id)">
            <X aria-hidden="true" />
          </button>
        </div>
        <template v-if="rule.roles.length > 1">
          <label :for="`creation-image-role-${draft.id}`" class="sr-only">{{ t('creation.composer.role') }}</label>
          <select :id="`creation-image-role-${draft.id}`" :value="draft.role" class="cs-thumb-role" @change="changeRole(draft, $event)">
            <option v-for="role in rule.roles" :key="role" :value="role">{{ t(`creation.composer.roles.${role}`) }}</option>
          </select>
        </template>
      </li>
      <!-- 达到数量上限后只保留缩略图，上传入口隐藏 -->
      <li v-if="drafts.length < rule.maxImages">
        <label
          class="cs-dropzone"
          :class="{ 'cs-dropzone--tile': drafts.length > 0 }"
          :data-dragover="dragover ? 'true' : undefined"
          @dragover.prevent="dragover = true"
          @dragleave="dragover = false"
          @drop.prevent="onDrop"
        >
          <input id="creation-reference-files" type="file" accept="image/png,image/jpeg,image/webp" :multiple="rule.maxImages > 1" class="sr-only" @change="addFiles" />
          <Upload class="cs-dropzone-icon" aria-hidden="true" />
          <span :class="drafts.length ? undefined : 'cs-dropzone-title'">{{ t('creation.composer.addImages') }}</span>
          <template v-if="!drafts.length">
            <span class="cs-dropzone-hint">{{ t('creation.composer.dragHint') }}</span>
            <span class="cs-dropzone-hint">{{ t('creation.composer.imageTypes', { size: MAX_FILE_MB }) }}</span>
          </template>
        </label>
      </li>
    </ul>

    <p v-if="isVeo31" class="cs-field-warning">
      <TriangleAlert aria-hidden="true" />
      <span>{{ t('creation.composer.veoReferenceHint') }}</span>
    </p>
    <p v-if="error" role="alert" class="cs-field-error">
      <CircleX aria-hidden="true" />
      <span>{{ error }}</span>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleX, TriangleAlert, Upload, X } from '@lucide/vue'
import CreationChipGroup from './CreationChipGroup.vue'
import type { CreationImageRole, CreationProtocol, CreationReferenceImage } from '@/features/creation/types'
import type { CreationChipOption, ReferenceImageDraft } from './types'

const MAX_FILE_MB = 10
const MAX_FILE_BYTES = MAX_FILE_MB * 1024 * 1024
const allowedTypes = new Set(['image/png', 'image/jpeg', 'image/webp'])

const props = withDefaults(defineProps<{
  kind: 'image' | 'video'
  protocol: CreationProtocol
  model: string
  platform: string
  editSupported?: boolean
}>(), { editSupported: false })
const emit = defineEmits<{
  change: [images: CreationReferenceImage[]]
  requireDuration: [seconds: number]
}>()
const { t } = useI18n()
const drafts = ref<ReferenceImageDraft[]>([])
const uploadRole = ref<CreationImageRole>('reference_image')
const error = ref('')
const dragover = ref(false)
let nextId = 0

const isVeo31 = computed(() => props.kind === 'video' && props.protocol === 'gemini' && props.model.toLowerCase().includes('3.1'))
const rule = computed(() => {
  if (props.kind === 'image') {
    if (!props.editSupported) return { maxImages: 0, roles: [] as CreationImageRole[], noMix: false }
    const gemini = props.protocol === 'gemini'
    const pro = props.model.toLowerCase().includes('pro-image')
    return {
      maxImages: gemini ? (pro ? 14 : 3) : props.platform === 'grok' ? 3 : 4,
      roles: ['reference_image'] as CreationImageRole[],
      noMix: false,
    }
  }
  switch (props.protocol) {
    case 'seedance':
      return { maxImages: 4, roles: ['first_frame', 'last_frame', 'reference_image'] as CreationImageRole[], noMix: false }
    case 'gemini':
      return isVeo31.value
        ? { maxImages: 3, roles: ['first_frame', 'reference_image'] as CreationImageRole[], noMix: true }
        : { maxImages: 1, roles: ['first_frame'] as CreationImageRole[], noMix: false }
    case 'minimax_v2':
      return { maxImages: 9, roles: ['first_frame', 'last_frame', 'reference_image'] as CreationImageRole[], noMix: true }
    case 'minimax_v1':
      return { maxImages: 3, roles: ['first_frame', 'last_frame', 'reference_image'] as CreationImageRole[], noMix: false }
    case 'grok':
      return { maxImages: 3, roles: ['first_frame', 'last_frame', 'reference_image'] as CreationImageRole[], noMix: false }
    default:
      return { maxImages: 0, roles: [] as CreationImageRole[], noMix: false }
  }
})

const roleOptions = computed<CreationChipOption[]>(() => rule.value.roles.map(role => ({
  value: role, label: t(`creation.composer.roles.${role}`),
})))

function setUploadRole(value: string): void {
  const role = parseRole(value)
  if (role && rule.value.roles.includes(role)) uploadRole.value = role
}

function roleFamily(role: CreationImageRole): 'frame' | 'reference' {
  return role === 'reference_image' ? 'reference' : 'frame'
}

function canUseRole(role: CreationImageRole, exceptId?: number): boolean {
  if (!rule.value.roles.includes(role)) return false
  if (role !== 'reference_image' && drafts.value.some(draft => draft.id !== exceptId && draft.role === role)) {
    error.value = t('creation.composer.duplicateFrame', { role: t(`creation.composer.roles.${role}`) })
    return false
  }
  if (!rule.value.noMix) return true
  const family = roleFamily(role)
  const compatible = drafts.value.every(draft => draft.id === exceptId || roleFamily(draft.role) === family)
  if (!compatible) error.value = t('creation.composer.noMix')
  return compatible
}

function syncImages(): void {
  emit('change', drafts.value.map(draft => ({ file: draft.file, role: draft.role })))
  if (isVeo31.value && drafts.value.some(draft => draft.role === 'reference_image')) emit('requireDuration', 8)
}

function clearImages(): void {
  for (const draft of drafts.value) URL.revokeObjectURL(draft.previewUrl)
  drafts.value = []
  error.value = ''
  syncImages()
}

function addFiles(event: Event): void {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  acceptFiles(files)
}

function onDrop(event: DragEvent): void {
  dragover.value = false
  acceptFiles(Array.from(event.dataTransfer?.files ?? []))
}

function acceptFiles(files: File[]): void {
  error.value = ''
  if (!files.length || !canUseRole(uploadRole.value)) return
  const slots = Math.max(0, rule.value.maxImages - drafts.value.length)
  if (files.length > slots) error.value = t('creation.composer.tooManyFiles', { count: rule.value.maxImages })
  for (const file of files.slice(0, slots)) {
    if (!allowedTypes.has(file.type)) {
      error.value = t('creation.composer.invalidFileType')
      continue
    }
    if (file.size === 0) {
      error.value = t('creation.composer.emptyFile')
      continue
    }
    if (file.size > MAX_FILE_BYTES) {
      error.value = t('creation.composer.fileTooLarge', { name: file.name, size: MAX_FILE_MB })
      continue
    }
    drafts.value.push({ id: ++nextId, file, previewUrl: URL.createObjectURL(file), role: uploadRole.value })
  }
  syncImages()
}

function removeImage(id: number): void {
  const index = drafts.value.findIndex(draft => draft.id === id)
  if (index < 0) return
  const [removed] = drafts.value.splice(index, 1)
  if (removed) URL.revokeObjectURL(removed.previewUrl)
  error.value = ''
  syncImages()
}

function parseRole(value: string): CreationImageRole | null {
  return value === 'first_frame' || value === 'last_frame' || value === 'reference_image' ? value : null
}

function changeRole(draft: ReferenceImageDraft, event: Event): void {
  const target = event.target as HTMLSelectElement
  const role = parseRole(target.value)
  if (!role || !canUseRole(role, draft.id)) {
    target.value = draft.role
    return
  }
  draft.role = role
  error.value = ''
  syncImages()
}

watch(rule, value => {
  uploadRole.value = value.roles[0] ?? 'reference_image'
}, { immediate: true })
watch(() => [props.kind, props.protocol, props.model, props.platform, props.editSupported], clearImages)
onMounted(syncImages)
onBeforeUnmount(() => {
  for (const draft of drafts.value) URL.revokeObjectURL(draft.previewUrl)
})
</script>
