<template>
  <div v-if="rule.maxImages > 0" class="space-y-3">
    <div class="flex items-center justify-between gap-3">
      <label for="creation-reference-files" class="input-label mb-0">{{ t('creation.composer.referenceImages') }}</label>
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('creation.composer.referenceLimit', { count: rule.maxImages }) }}</span>
    </div>
    <div v-if="rule.roles.length > 1">
      <label for="creation-upload-role" class="sr-only">{{ t('creation.composer.role') }}</label>
      <select id="creation-upload-role" v-model="uploadRole" class="input">
        <option v-for="role in rule.roles" :key="role" :value="role">{{ t(`creation.composer.roles.${role}`) }}</option>
      </select>
    </div>
    <label for="creation-reference-files" class="btn btn-secondary flex cursor-pointer justify-center" :class="drafts.length >= rule.maxImages ? 'pointer-events-none opacity-50' : ''">
      <Icon name="upload" size="sm" class="mr-2" />{{ t('creation.composer.addImages') }}
    </label>
    <input id="creation-reference-files" type="file" accept="image/png,image/jpeg,image/webp" :multiple="rule.maxImages > 1" :disabled="drafts.length >= rule.maxImages" class="sr-only" @change="addFiles" />
    <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ t('creation.composer.imageTypes', { size: MAX_FILE_MB }) }}</p>
    <p v-if="isVeo31" class="text-xs leading-5 text-amber-700 dark:text-amber-300">{{ t('creation.composer.veoReferenceHint') }}</p>
    <p v-if="error" role="alert" class="text-xs leading-5 text-red-600 dark:text-red-400">{{ error }}</p>

    <ul v-if="drafts.length" class="grid grid-cols-2 gap-3 sm:grid-cols-3" role="list">
      <li v-for="draft in drafts" :key="draft.id" class="min-w-0 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
        <div class="relative aspect-square bg-gray-100 dark:bg-dark-800">
          <img :src="draft.previewUrl" :alt="draft.file.name" class="h-full w-full object-cover" />
          <button type="button" class="absolute right-1 top-1 rounded-full bg-gray-950/70 p-1.5 text-white hover:bg-gray-950" :aria-label="t('creation.composer.removeImage', { name: draft.file.name })" @click="removeImage(draft.id)"><Icon name="x" size="xs" /></button>
        </div>
        <div class="p-2">
          <p class="truncate text-xs text-gray-600 dark:text-gray-300" :title="draft.file.name">{{ draft.file.name }}</p>
          <template v-if="rule.roles.length > 1">
            <label :for="`creation-image-role-${draft.id}`" class="sr-only">{{ t('creation.composer.role') }}</label>
            <select :id="`creation-image-role-${draft.id}`" :value="draft.role" class="input mt-2 h-8 px-2 py-1 text-xs" @change="changeRole(draft, $event)">
              <option v-for="role in rule.roles" :key="role" :value="role">{{ t(`creation.composer.roles.${role}`) }}</option>
            </select>
          </template>
        </div>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CreationImageRole, CreationProtocol, CreationReferenceImage } from '@/features/creation/types'
import type { ReferenceImageDraft } from './types'

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
