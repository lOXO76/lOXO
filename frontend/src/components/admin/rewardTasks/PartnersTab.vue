<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center gap-2">
      <p class="mr-auto text-sm text-gray-500 dark:text-dark-400">
        {{ t('admin.rewardTasks.partners.partnerCount', { count: partners.length, tasks: totalTaskCount }) }}
      </p>
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="loadPartners">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        {{ t('admin.rewardTasks.refresh') }}
      </button>
      <button type="button" class="btn btn-primary" data-testid="partner-create" @click="openCreate">
        <Icon name="plus" size="sm" />
        {{ t('admin.rewardTasks.partners.create') }}
      </button>
    </div>

    <div class="card overflow-hidden !rounded-2xl !p-0">
      <DataTable :columns="columns" :data="partners" :loading="loading" row-key="id" :actions-count="3">
        <template #cell-name="{ row }">
          <div class="min-w-0 max-w-xs">
            <p class="truncate font-medium text-gray-900 dark:text-white" :title="row.name">{{ row.name }}</p>
            <p v-if="row.contact" class="mt-0.5 truncate text-xs text-gray-400 dark:text-dark-500">{{ row.contact }}</p>
          </div>
        </template>
        <template #cell-appKey="{ row }">
          <div class="flex items-center gap-1">
            <code class="max-w-[11rem] truncate rounded bg-gray-50 px-2 py-0.5 font-mono text-xs text-gray-700 dark:bg-dark-900 dark:text-gray-300" :title="row.app_key">
              {{ row.app_key }}
            </code>
            <button
              type="button"
              class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-200"
              :title="t('admin.rewardTasks.partners.copyKey')"
              :aria-label="t('admin.rewardTasks.partners.copyKey')"
              @click="copy(row.app_key)"
            >
              <Icon name="copy" size="sm" />
            </button>
          </div>
        </template>
        <template #cell-secret="{ row }">
          <span class="font-mono text-xs text-gray-500 dark:text-dark-400">****{{ row.secret_hint }}</span>
        </template>
        <template #cell-domains="{ row }">
          <div v-if="row.allowed_domains.length" class="flex max-w-xs flex-wrap gap-1">
            <span
              v-for="domain in row.allowed_domains"
              :key="domain"
              class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[11px] text-gray-600 dark:bg-dark-700 dark:text-gray-300"
            >
              {{ domain }}
            </span>
          </div>
          <span v-else class="text-xs text-gray-400 dark:text-dark-500">{{ t('admin.rewardTasks.partners.domainsEmpty') }}</span>
        </template>
        <template #cell-taskCount="{ row }">
          <span class="tabular-nums text-sm text-gray-700 dark:text-gray-300">{{ row.task_count }}</span>
        </template>
        <template #cell-lastEvent="{ row }">
          <span class="text-xs" :class="row.last_event_at ? 'text-gray-600 dark:text-dark-300' : 'text-gray-400 dark:text-dark-500'">
            {{ row.last_event_at ? formatDateTime(row.last_event_at) : t('admin.rewardTasks.partners.notIntegrated') }}
          </span>
        </template>
        <template #cell-enabled="{ row }">
          <span class="badge" :class="row.enabled ? 'badge-success' : 'badge-gray'">
            {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
          </span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex flex-wrap items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm" @click="copyIntegration(row)">
              {{ t('admin.rewardTasks.partners.copyIntegration') }}
            </button>
            <button type="button" class="btn btn-ghost btn-sm" @click="openEdit(row)">{{ t('common.edit') }}</button>
            <button type="button" class="btn btn-ghost btn-sm" @click="requestReset(row)">
              {{ t('admin.rewardTasks.partners.resetSecret') }}
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-sm text-red-600 dark:text-red-400"
              @click="requestDelete(row)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </template>
        <template #empty>
          <EmptyState
            :title="loadFailed ? t('admin.rewardTasks.partners.loadFailed') : t('admin.rewardTasks.partners.empty')"
            :description="loadFailed ? t('admin.rewardTasks.loadFailed') : t('admin.rewardTasks.partners.emptyDescription')"
            :action-text="loadFailed ? t('common.tryAgain') : t('admin.rewardTasks.partners.create')"
            @action="loadFailed ? loadPartners() : openCreate()"
          />
        </template>
      </DataTable>
    </div>

    <PartnerFormDialog :show="showForm" :partner="editingPartner" @close="closeForm" @saved="handleSaved" />
    <PartnerSecretDialog :show="showSecret" :partner="secretPartner" @close="closeSecret" />

    <ConfirmDialog
      :show="showReset"
      :title="t('admin.rewardTasks.partners.resetSecret')"
      :message="resetMessage"
      :confirm-text="t('admin.rewardTasks.partners.resetSecret')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmReset"
      @cancel="showReset = false"
    />
    <ConfirmDialog
      :show="showDelete"
      :title="t('common.delete')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDelete = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import PartnerFormDialog from './PartnerFormDialog.vue'
import PartnerSecretDialog from './PartnerSecretDialog.vue'
import type { Column } from '@/components/common/types'
import { adminAPI } from '@/api/admin'
import type { AdminRewardTaskPartner } from '@/api/admin/rewardTasks'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const emit = defineEmits<{
  // 站点变化后通知页面刷新概览（站点数与任务下拉）
  (e: 'changed'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const partners = ref<AdminRewardTaskPartner[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const showForm = ref(false)
const editingPartner = ref<AdminRewardTaskPartner | null>(null)
const showSecret = ref(false)
const secretPartner = ref<AdminRewardTaskPartner | null>(null)
const showReset = ref(false)
const resettingPartner = ref<AdminRewardTaskPartner | null>(null)
const resetLoading = ref(false)
const showDelete = ref(false)
const deletingPartner = ref<AdminRewardTaskPartner | null>(null)
const deleteLoading = ref(false)
let requestSequence = 0

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.rewardTasks.partners.columns.name'), sortable: false },
  { key: 'appKey', label: t('admin.rewardTasks.partners.columns.appKey'), sortable: false },
  { key: 'secret', label: t('admin.rewardTasks.partners.columns.secret'), sortable: false },
  { key: 'domains', label: t('admin.rewardTasks.partners.columns.domains'), sortable: false },
  { key: 'taskCount', label: t('admin.rewardTasks.partners.columns.taskCount'), sortable: false },
  { key: 'lastEvent', label: t('admin.rewardTasks.partners.columns.lastEvent'), sortable: false },
  { key: 'enabled', label: t('admin.rewardTasks.partners.columns.enabled'), sortable: false },
  { key: 'actions', label: t('admin.rewardTasks.partners.columns.actions'), sortable: false },
])
const totalTaskCount = computed(() => partners.value.reduce((sum, partner) => sum + (partner.task_count ?? 0), 0))
const resetMessage = computed(() => t('admin.rewardTasks.partners.resetConfirm', { name: resettingPartner.value?.name ?? '' }))
const deleteMessage = computed(() => t('admin.rewardTasks.partners.deleteConfirm', { name: deletingPartner.value?.name ?? '' }))

async function loadPartners() {
  const sequence = ++requestSequence
  loading.value = true
  try {
    const result = await adminAPI.rewardTasks.listPartners()
    if (sequence !== requestSequence) return
    partners.value = result ?? []
    loadFailed.value = false
  } catch (error) {
    if (sequence !== requestSequence) return
    loadFailed.value = true
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.partners.loadFailed')))
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

function openCreate() {
  editingPartner.value = null
  showForm.value = true
}

function openEdit(partner: AdminRewardTaskPartner) {
  editingPartner.value = partner
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingPartner.value = null
}

function handleSaved(saved: AdminRewardTaskPartner) {
  void loadPartners()
  emit('changed')
  // 创建响应带一次性明文密钥：立刻展示，关闭后无法再次查看
  if (saved?.app_secret) {
    secretPartner.value = saved
    showSecret.value = true
  }
}

function closeSecret() {
  showSecret.value = false
  secretPartner.value = null
}

function copy(text: string) {
  void copyToClipboard(text)
}

// 复制对接信息：文档地址 + app_key，直接发给合作方
function copyIntegration(partner: AdminRewardTaskPartner) {
  const origin = typeof window === 'undefined' ? '' : window.location.origin
  const text = [
    `${t('admin.rewardTasks.partners.docsLineLabel')}: ${origin}/docs/reward-integration`,
    `app_key: ${partner.app_key}`,
  ].join('\n')
  void copyToClipboard(text, t('admin.rewardTasks.partners.copyIntegrationSuccess'))
}

function requestReset(partner: AdminRewardTaskPartner) {
  resettingPartner.value = partner
  showReset.value = true
}

async function confirmReset() {
  if (resetLoading.value || !resettingPartner.value) return
  resetLoading.value = true
  try {
    const result = await adminAPI.rewardTasks.resetPartnerSecret(resettingPartner.value.id)
    appStore.showSuccess(t('admin.rewardTasks.partners.resetSuccess'))
    showReset.value = false
    resettingPartner.value = null
    void loadPartners()
    if (result?.app_secret) {
      secretPartner.value = result
      showSecret.value = true
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.partners.resetFailed')))
  } finally {
    resetLoading.value = false
  }
}

function requestDelete(partner: AdminRewardTaskPartner) {
  deletingPartner.value = partner
  showDelete.value = true
}

async function confirmDelete() {
  if (deleteLoading.value || !deletingPartner.value) return
  deleteLoading.value = true
  try {
    await adminAPI.rewardTasks.deletePartner(deletingPartner.value.id)
    appStore.showSuccess(t('admin.rewardTasks.partners.deleteSuccess'))
    showDelete.value = false
    deletingPartner.value = null
    void loadPartners()
    emit('changed')
  } catch (error) {
    // 仍绑定任务时后端返回 REWARD_PARTNER_IN_USE（409），message 直接展示
    appStore.showError(extractApiErrorMessage(error, t('admin.rewardTasks.partners.deleteFailed')))
  } finally {
    deleteLoading.value = false
  }
}

onMounted(() => {
  void loadPartners()
})

onBeforeUnmount(() => {
  requestSequence += 1
})
</script>
