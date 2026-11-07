<template>
  <BaseDialog :show="show" :title="t('tickets.create.title')" width="normal" @close="close">
    <form id="ticket-create-form" class="space-y-5" @submit.prevent="submit">
      <div>
        <label class="input-label mb-1.5 block" for="ticket-create-subject">
          {{ t('tickets.create.subject') }}
          <span class="text-red-500">*</span>
        </label>
        <input
          id="ticket-create-subject"
          :value="subject"
          type="text"
          class="input w-full"
          :placeholder="t('tickets.create.subjectPlaceholder')"
          required
          data-testid="ticket-subject"
          @input="onSubjectInput"
        />
        <p class="mt-1.5 text-right text-xs text-gray-400 dark:text-dark-500">
          {{ t('tickets.create.counter', { current: subjectLength, max: maxSubjectLength }) }}
        </p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="input-label mb-1.5 block" :for="categoryId">{{ t('tickets.create.category') }}</label>
          <Select
            :id="categoryId"
            v-model="category"
            :options="categoryOptions"
            data-testid="ticket-category"
          />
        </div>
        <div>
          <label class="input-label mb-1.5 block" :for="priorityId">{{ t('tickets.create.priority') }}</label>
          <Select
            :id="priorityId"
            v-model="priority"
            :options="priorityOptions"
            data-testid="ticket-priority"
          />
        </div>
      </div>

      <div>
        <label class="input-label mb-1.5 block" for="ticket-create-body">
          {{ t('tickets.create.body') }}
          <span class="text-red-500">*</span>
        </label>
        <textarea
          id="ticket-create-body"
          :value="body"
          rows="7"
          class="input w-full min-h-[80px] resize-y"
          :placeholder="t('tickets.create.bodyPlaceholder')"
          required
          data-testid="ticket-body"
          @input="onBodyInput"
        ></textarea>
        <p class="mt-1.5 flex items-center justify-between gap-3 text-xs text-gray-400 dark:text-dark-500">
          <span>{{ t('tickets.create.openLimitHint', { max: maxOpenTickets }) }}</span>
          <span class="tabular-nums">{{ t('tickets.create.counter', { current: bodyLength, max: maxBodyLength }) }}</span>
        </p>
      </div>

      <p v-if="errorMessage" class="rounded-xl bg-red-50 px-3 py-2 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-300" role="alert">
        {{ errorMessage }}
      </p>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="close">
          {{ t('tickets.common.cancel') }}
        </button>
        <button
          type="submit"
          form="ticket-create-form"
          class="btn btn-primary"
          :disabled="submitting || !canSubmit"
          data-testid="ticket-submit"
        >
          {{ submitting ? t('tickets.create.submitting') : t('tickets.create.submit') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { createTicket } from '@/api/tickets'
import { resolveTicketErrorMessage } from '@/utils/ticketErrors'
import {
  countTicketChars,
  isBlankTicketText,
  isTicketTextWithinLimit,
  truncateTicketText,
} from '@/utils/ticketText'
import type { TicketCategory, TicketDetail, TicketPriority } from '@/types/tickets'

const props = defineProps<{
  show: boolean
  categories: TicketCategory[]
  priorities: TicketPriority[]
  maxSubjectLength: number
  maxBodyLength: number
  maxOpenTickets: number
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created', detail: TicketDetail): void
}>()

const { t } = useI18n()

const categoryId = 'ticket-create-category'
const priorityId = 'ticket-create-priority'

const subject = ref('')
const body = ref('')
const category = ref<string | number | boolean | null>('general')
const priority = ref<string | number | boolean | null>('normal')
const submitting = ref(false)
const errorMessage = ref('')

const categoryOptions = computed<SelectOption[]>(() =>
  props.categories.map(value => ({ value, label: t(`tickets.category.${value}`) })),
)
const priorityOptions = computed<SelectOption[]>(() =>
  props.priorities.map(value => ({ value, label: t(`tickets.priority.${value}`) })),
)

const canSubmit = computed(
  () => !isBlankTicketText(subject.value) && !isBlankTicketText(body.value),
)

// 后端按 code point 计数，这里同步用 rune 计数展示与截断。
const subjectLength = computed(() => countTicketChars(subject.value))
const bodyLength = computed(() => countTicketChars(body.value))

function reset(): void {
  subject.value = ''
  body.value = ''
  category.value = 'general'
  priority.value = 'normal'
  errorMessage.value = ''
  submitting.value = false
}

// 不依赖原生 maxlength（它按 UTF-16 code unit 计数），而是按 code point 截断后
// 回写 DOM，保证 emoji 不会被切成半个代理对。
function onSubjectInput(event: Event): void {
  const el = event.target as HTMLInputElement
  const next = truncateTicketText(el.value, props.maxSubjectLength)
  subject.value = next
  if (el.value !== next) el.value = next
}

function onBodyInput(event: Event): void {
  const el = event.target as HTMLTextAreaElement
  const next = truncateTicketText(el.value, props.maxBodyLength)
  body.value = next
  if (el.value !== next) el.value = next
}

function close(): void {
  if (submitting.value) return
  reset()
  emit('close')
}

watch(
  () => props.show,
  (show) => {
    if (show) reset()
  },
)

async function submit(): Promise<void> {
  if (submitting.value) return
  if (!subject.value.trim()) {
    errorMessage.value = t('tickets.create.errors.subjectRequired')
    return
  }
  if (!body.value.trim()) {
    errorMessage.value = t('tickets.create.errors.bodyRequired')
    return
  }
  if (
    !isTicketTextWithinLimit(subject.value, props.maxSubjectLength) ||
    !isTicketTextWithinLimit(body.value, props.maxBodyLength)
  ) {
    errorMessage.value = t('tickets.create.errors.tooLong', {
      max: isTicketTextWithinLimit(subject.value, props.maxSubjectLength)
        ? props.maxBodyLength
        : props.maxSubjectLength,
    })
    return
  }

  submitting.value = true
  errorMessage.value = ''
  try {
    const detail = await createTicket({
      subject: subject.value.trim(),
      category: (category.value as TicketCategory) ?? 'general',
      priority: (priority.value as TicketPriority) ?? 'normal',
      body: body.value,
    })
    reset()
    emit('created', detail)
  } catch (error) {
    errorMessage.value = resolveTicketErrorMessage(error, t)
  } finally {
    submitting.value = false
  }
}
</script>
