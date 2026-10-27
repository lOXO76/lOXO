<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-7xl space-y-6">
      <!-- 页头 -->
      <header class="max-w-2xl">
        <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">
          {{ t('clientImport.title') }}
        </h1>
        <p class="mt-1.5 text-sm leading-6 text-gray-500 dark:text-dark-400">
          {{ t('clientImport.description') }}
        </p>
      </header>

      <!-- 加载中 -->
      <div v-if="keysLoading" class="card flex items-center justify-center py-24" data-test="client-import-loading">
        <div class="text-center">
          <div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">{{ t('clientImport.loading') }}</p>
        </div>
      </div>

      <div v-else class="grid items-start gap-6 lg:grid-cols-[19rem_minmax(0,1fr)]">
        <!-- 左栏：分组、密钥、端点 -->
        <aside class="space-y-4 lg:sticky lg:top-20">
          <section class="card space-y-4 p-5">
            <div>
              <label for="import-key" class="input-label">{{ t('clientImport.key') }}</label>
              <select
                id="import-key"
                v-model="selectedKeyId"
                class="input"
                :disabled="keysLoading"
                data-test="client-import-key"
              >
                <option :value="null">{{ t('clientImport.keyPlaceholder') }}</option>
                <option v-for="key in keys" :key="key.id" :value="key.id">
                  {{ key.name }} · {{ key.group?.name || t('clientImport.noGroup') }}
                </option>
              </select>

              <p v-if="selectedKey" class="mt-2 flex items-center gap-1.5 text-xs text-gray-500 dark:text-dark-400">
                <Icon name="key" size="xs" class="flex-shrink-0" />
                <span class="truncate font-mono">{{ maskApiKey(selectedKey.key) }}</span>
              </p>

              <div
                v-if="!keysLoading && keys.length === 0"
                class="mt-2.5 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300"
              >
                {{ t('clientImport.keyMissing') }}
                <router-link to="/keys" class="ml-1 font-medium underline underline-offset-2">
                  {{ t('clientImport.createKey') }}
                </router-link>
              </div>

              <p
                v-else-if="selectedKey && !selectedGroup"
                class="mt-2.5 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300"
                data-test="client-import-no-group"
              >
                {{ t('clientImport.noGroupHint') }}
              </p>
            </div>
          </section>

          <!-- 端点摘要：让用户在选客户端前就知道能连什么 -->
          <section v-if="endpointDisplay.length" class="card p-5">
            <div class="flex items-center justify-between">
              <h2 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('clientImport.endpoints.title') }}
              </h2>
              <span class="text-[11px] text-gray-400 dark:text-dark-500">
                {{ platformDisplayName(selectedGroup?.platform || '') }}
              </span>
            </div>
            <dl class="mt-3 space-y-2">
              <div v-for="row in endpointDisplay" :key="row.label" class="min-w-0">
                <dt class="text-[11px] uppercase tracking-wide text-gray-400 dark:text-dark-500">{{ row.label }}</dt>
                <dd class="mt-0.5 flex items-center gap-1.5">
                  <code class="min-w-0 flex-1 truncate font-mono text-xs text-gray-700 dark:text-gray-300">{{ row.value }}</code>
                  <button
                    type="button"
                    class="flex-shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                    @click="copyText(row.value)"
                  >
                    <Icon name="copy" size="xs" />
                  </button>
                </dd>
              </div>
            </dl>
          </section>
        </aside>

        <!-- 右栏 -->
        <div class="min-w-0 space-y-5">
          <!-- 客户端选择 -->
          <section class="card p-5">
            <div class="mb-4 flex items-baseline justify-between">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('clientImport.client') }}</h2>
              <span v-if="selectedGroup" class="text-xs text-gray-400 dark:text-dark-500">
                {{ platformDisplayName(selectedGroup.platform) }}
              </span>
            </div>

            <p v-if="!selectedGroup" class="rounded-xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400">
              {{ t('clientImport.emptyHint') }}
            </p>

            <div v-else class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-3">
              <button
                v-for="client in clientCards"
                :key="client.definition.id"
                type="button"
                :disabled="!client.allowed"
                :data-test="`client-card-${client.definition.id}`"
                :aria-pressed="selectedClientId === client.definition.id"
                :class="[
                  'group relative flex flex-col gap-2 rounded-xl border p-3.5 text-left transition-all duration-200',
                  selectedClientId === client.definition.id
                    ? 'border-primary-500 bg-primary-50/70 ring-1 ring-primary-500/30 dark:border-primary-500 dark:bg-primary-950/30'
                    : 'border-gray-200 bg-white hover:border-primary-300 hover:bg-gray-50/70 dark:border-dark-700 dark:bg-dark-800/40 dark:hover:border-primary-700 dark:hover:bg-dark-800/70',
                  !client.allowed && 'cursor-not-allowed opacity-55 hover:border-gray-200 hover:bg-white dark:hover:border-dark-700 dark:hover:bg-dark-800/40'
                ]"
                @click="selectClient(client.definition.id)"
              >
                <span class="flex w-full items-start gap-2.5">
                  <span
                    :class="[
                      'flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg transition-colors',
                      selectedClientId === client.definition.id
                        ? 'bg-primary-500 text-white'
                        : 'bg-gray-100 text-gray-500 group-hover:bg-white dark:bg-dark-700 dark:text-gray-400 dark:group-hover:bg-dark-600'
                    ]"
                  >
                    <Icon :name="clientIcon(client.definition.id)" size="sm" />
                  </span>

                  <span class="min-w-0 flex-1">
                    <span class="flex items-center gap-1.5">
                      <span class="truncate text-sm font-medium text-gray-900 dark:text-white">
                        {{ t(`clientImport.clients.${client.definition.id}`) }}
                      </span>
                    </span>
                    <span
                      :class="[
                        'mt-1 inline-flex rounded px-1.5 py-0.5 text-[10px] font-medium',
                        modeBadgeClass(client.definition.mode)
                      ]"
                    >
                      {{ t(`clientImport.clientHint.${client.definition.mode}`) }}
                    </span>
                  </span>
                </span>

                <span class="line-clamp-2 text-xs leading-5 text-gray-500 dark:text-dark-400">
                  {{ t(`clientImport.clientDescriptions.${client.definition.id}`) }}
                </span>

                <span
                  v-if="!client.allowed"
                  class="mt-0.5 flex items-start gap-1 text-[11px] leading-4 text-amber-600 dark:text-amber-400"
                  :data-test="`client-blocked-${client.definition.id}`"
                >
                  <Icon name="lock" size="xs" class="mt-0.5 flex-shrink-0" />
                  <span>{{ t(`clientImport.disabled.${client.reason}`) }}</span>
                </span>
              </button>
            </div>
          </section>

          <!-- 配置面板 -->
          <section
            v-if="activeClient && selectedKey"
            class="card overflow-hidden"
            data-test="client-import-panel"
          >
            <!-- 面板头 -->
            <div class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700">
              <span class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-primary-500 text-white">
                <Icon :name="clientIcon(activeClient.definition.id)" size="sm" />
              </span>
              <div class="min-w-0">
                <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-white">
                  {{ t(`clientImport.clients.${activeClient.definition.id}`) }}
                </h2>
                <p class="truncate text-xs text-gray-500 dark:text-dark-400">
                  {{ selectedKey.name }} · {{ selectedGroup?.name }}
                </p>
              </div>
            </div>

            <!-- CC Switch 目标应用 -->
            <div
              v-if="activeClient.definition.id === 'cc-switch'"
              class="space-y-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700"
              data-test="cc-switch-apps"
            >
              <div>
                <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('clientImport.ccSwitch.title') }}</h3>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t('clientImport.ccSwitch.description') }}</p>
              </div>
              <div class="flex flex-wrap gap-2" role="radiogroup" :aria-label="t('clientImport.ccSwitch.title')">
                <button
                  v-for="app in ccSwitchApps"
                  :key="app"
                  type="button"
                  role="radio"
                  :aria-checked="ccSwitchApp === app"
                  :data-test="`cc-switch-app-${app}`"
                  :class="[
                    'rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors',
                    ccSwitchApp === app
                      ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-950/40 dark:text-primary-300'
                      : 'border-gray-200 text-gray-600 hover:border-primary-300 hover:text-gray-900 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-700 dark:hover:text-white'
                  ]"
                  @click="ccSwitchApp = app"
                >
                  {{ ccSwitchAppLabel(app) }}
                </button>
              </div>
            </div>

            <!-- 模型选择 -->
            <div
              v-if="activeClient.definition.modelMode !== 'none'"
              class="space-y-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700"
            >
              <div class="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('clientImport.model.title') }}</h3>
                  <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">
                    {{ activeClient.definition.modelMode === 'multi'
                      ? t('clientImport.model.description')
                      : t('clientImport.model.singleDescription') }}
                  </p>
                </div>
                <div class="flex items-center gap-2">
                  <span v-if="selectedModels.length && activeClient.definition.modelMode === 'multi'" class="text-xs text-gray-500 dark:text-dark-400">
                    {{ t('clientImport.model.selectedCount', { count: selectedModels.length }) }}
                  </span>
                  <button type="button" class="btn btn-secondary btn-sm" :disabled="modelsLoading" @click="loadModels()">
                    <Icon name="refresh" size="xs" :class="modelsLoading ? 'animate-spin' : ''" />
                    {{ modelsLoading ? t('clientImport.model.fetching') : t('clientImport.model.fetch') }}
                  </button>
                  <button
                    v-if="selectedModels.length || defaultModel"
                    type="button"
                    class="text-xs text-gray-500 underline underline-offset-2 transition-colors hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200"
                    @click="clearModels"
                  >
                    {{ t('clientImport.model.clear') }}
                  </button>
                </div>
              </div>

              <p v-if="modelsError" class="rounded-lg bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">
                {{ t('clientImport.model.fetchFailed') }}
              </p>

              <!-- multi：候选列表 + 默认模型 -->
              <template v-if="activeClient.definition.modelMode === 'multi'">
                <div v-if="allModelOptions.length" class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
                  <div class="flex items-center gap-2 border-b border-gray-100 bg-gray-50/70 px-3 py-2 dark:border-dark-700 dark:bg-dark-800/50">
                    <Icon name="search" size="xs" class="text-gray-400" />
                    <input
                      v-model="modelSearch"
                      type="text"
                      :placeholder="t('clientImport.model.search')"
                      class="min-w-0 flex-1 bg-transparent text-xs text-gray-700 outline-none placeholder:text-gray-400 dark:text-gray-200 dark:placeholder:text-dark-500"
                    />
                  </div>

                  <div class="max-h-56 overflow-y-auto">
                    <label
                      v-for="model in filteredModelOptions"
                      :key="model.id"
                      class="flex cursor-pointer items-center gap-2.5 border-b border-gray-50 px-3 py-2 last:border-b-0 hover:bg-gray-50 dark:border-dark-800 dark:hover:bg-dark-800/60"
                    >
                      <input
                        type="checkbox"
                        class="checkbox"
                        :value="model.id"
                        :checked="selectedModels.includes(model.id)"
                        @change="toggleModel(model.id, ($event.target as HTMLInputElement).checked)"
                      />
                      <span class="min-w-0 flex-1 truncate font-mono text-xs text-gray-700 dark:text-gray-300">{{ model.id }}</span>
                      <span v-if="model.name && model.name !== model.id" class="truncate text-[11px] text-gray-400 dark:text-dark-500">{{ model.name }}</span>
                    </label>

                    <p v-if="!filteredModelOptions.length" class="px-3 py-6 text-center text-xs text-gray-400 dark:text-dark-500">
                      {{ t('clientImport.model.empty') }}
                    </p>
                  </div>
                </div>

                <div class="grid gap-3 sm:grid-cols-2">
                  <div>
                    <label for="default-model" class="input-label">{{ t('clientImport.model.default') }}</label>
                    <select id="default-model" v-model="defaultModel" class="input" data-test="client-import-default-model">
                      <option value="">{{ t('clientImport.model.defaultPlaceholder') }}</option>
                      <option v-for="model in selectedModels" :key="model" :value="model">{{ model }}</option>
                    </select>
                  </div>
                  <div>
                    <label for="manual-model" class="input-label">{{ t('clientImport.model.manual') }}</label>
                    <input
                      id="manual-model"
                      v-model="manualModelInput"
                      class="input font-mono text-sm"
                      :placeholder="t('clientImport.model.manualPlaceholder')"
                      data-test="client-import-manual-model"
                      @keydown.enter.prevent="addManualModel()"
                    />
                  </div>
                </div>
              </template>

              <!-- single：一个模型 -->
              <div v-else class="max-w-md">
                <label for="single-model" class="input-label">{{ t('clientImport.model.default') }}</label>
                <select
                  v-if="allModelOptions.length"
                  id="single-model"
                  v-model="defaultModel"
                  class="input"
                  data-test="client-import-single-model"
                >
                  <option value="">{{ t('clientImport.model.defaultPlaceholder') }}</option>
                  <option v-for="model in allModelOptions" :key="model.id" :value="model.id">{{ model.id }}</option>
                </select>
                <input
                  v-else
                  id="single-model"
                  v-model="defaultModel"
                  class="input font-mono text-sm"
                  :placeholder="t('clientImport.model.manualPlaceholder')"
                  data-test="client-import-single-model-manual"
                />
              </div>

              <!-- 手动添加的模型 -->
              <div v-if="manualModels.length" class="flex flex-wrap gap-1.5">
                <span
                  v-for="model in manualModels"
                  :key="model"
                  class="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-1 font-mono text-xs text-gray-700 dark:bg-dark-700 dark:text-gray-300"
                >
                  {{ model }}
                  <button type="button" class="text-gray-400 transition-colors hover:text-red-500" @click="removeManualModel(model)">
                    <Icon name="x" size="xs" />
                  </button>
                </span>
              </div>
            </div>

            <!-- 输出 -->
            <div class="space-y-4 px-5 py-4">
              <!-- 一键导入 -->
              <div v-if="result.deeplink" class="flex flex-wrap items-center gap-3">
                <a :href="result.deeplink" class="btn btn-primary btn-sm inline-flex" data-test="client-import-deeplink">
                  <Icon name="externalLink" size="sm" />
                  {{ t('clientImport.result.importNow') }}
                </a>
                <p class="flex items-center gap-1.5 text-xs text-amber-600 dark:text-amber-400">
                  <Icon name="exclamationTriangle" size="xs" class="flex-shrink-0" />
                  {{ t('clientImport.result.deeplinkWarning') }}
                </p>
              </div>

              <!-- 一键命令 -->
              <div v-if="result.commands?.length" class="space-y-2.5" data-test="client-import-commands">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <h3 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                    {{ t('clientImport.result.commandTitle') }}
                  </h3>
                  <div class="flex gap-1.5" role="tablist">
                    <button
                      v-for="command in result.commands"
                      :key="command.shell"
                      type="button"
                      role="tab"
                      :aria-selected="activeCommand?.shell === command.shell"
                      :data-test="`client-import-shell-${command.shell}`"
                      :class="[
                        'rounded-lg px-2.5 py-1 text-xs transition-colors',
                        activeCommand?.shell === command.shell
                          ? 'bg-gray-900 text-gray-100 dark:bg-dark-700 dark:text-white'
                          : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
                      ]"
                      @click="activeShell = command.shell"
                    >
                      {{ t(`clientImport.result.shells.${command.shell}`) }}
                    </button>
                  </div>
                </div>

                <p class="flex items-start gap-1.5 text-xs leading-5 text-amber-600 dark:text-amber-400">
                  <Icon name="exclamationTriangle" size="xs" class="mt-0.5 flex-shrink-0" />
                  {{ t('clientImport.result.commandWarning') }}
                </p>

                <div class="overflow-hidden rounded-xl bg-gray-900 ring-1 ring-black/5 dark:bg-dark-950">
                  <div class="flex items-center justify-between border-b border-white/10 px-4 py-2">
                    <span class="text-xs text-gray-400">{{ t(`clientImport.result.shellHints.${activeCommand?.shell ?? 'bash'}`) }}</span>
                    <button
                      type="button"
                      class="rounded-md px-2 py-1 text-xs text-gray-300 transition-colors hover:bg-white/10 hover:text-white"
                      data-test="client-import-copy-command"
                      @click="copyText(activeCommand?.content ?? '')"
                    >
                      {{ t('clientImport.result.copyCommand') }}
                    </button>
                  </div>
                  <pre class="max-h-72 overflow-auto p-4 text-xs leading-5 text-gray-100"><code v-text="activeCommand?.content"></code></pre>
                </div>
              </div>

              <!-- 文件预览 -->
              <div v-if="result.files.length" class="space-y-2.5">
                <div class="flex flex-wrap gap-1.5">
                  <button
                    v-for="(file, index) in result.files"
                    :key="file.path"
                    type="button"
                    :class="[
                      'rounded-lg px-2.5 py-1.5 font-mono text-xs transition-colors',
                      activeFileIndex === index
                        ? 'bg-gray-900 text-gray-100 dark:bg-dark-700 dark:text-white'
                        : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700'
                    ]"
                    @click="activeFileIndex = index"
                  >
                    {{ file.path }}
                  </button>
                </div>

                <p v-if="activeFile?.hint" class="flex items-start gap-1.5 text-xs leading-5 text-amber-600 dark:text-amber-400">
                  <Icon name="exclamationCircle" size="xs" class="mt-0.5 flex-shrink-0" />
                  {{ activeFile.hint }}
                </p>

                <div class="overflow-hidden rounded-xl bg-gray-900 ring-1 ring-black/5 dark:bg-dark-950">
                  <div class="flex items-center justify-between border-b border-white/10 px-4 py-2">
                    <span class="min-w-0 truncate font-mono text-xs text-gray-400">{{ activeFile?.path }}</span>
                    <div class="flex flex-shrink-0 items-center gap-1.5">
                      <button type="button" class="rounded-md px-2 py-1 text-xs text-gray-300 transition-colors hover:bg-white/10 hover:text-white" @click="copyText(activeFile?.content ?? '')">
                        {{ t('clientImport.result.copy') }}
                      </button>
                      <button type="button" class="rounded-md px-2 py-1 text-xs text-gray-300 transition-colors hover:bg-white/10 hover:text-white" @click="downloadFile(activeFile)">
                        {{ t('clientImport.result.download') }}
                      </button>
                    </div>
                  </div>
                  <pre class="max-h-80 overflow-auto p-4 text-xs leading-5 text-gray-100"><code v-text="activeFile?.content"></code></pre>
                </div>
              </div>

              <!-- 手动填写 -->
              <div v-if="result.manual.length" class="space-y-2">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('clientImport.result.manualTitle') }}
                </h3>
                <div class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
                  <div
                    v-for="field in result.manual"
                    :key="field.label"
                    class="flex items-center gap-3 border-b border-gray-100 px-3 py-2 last:border-b-0 dark:border-dark-700"
                  >
                    <span class="w-36 flex-shrink-0 text-[11px] text-gray-500 dark:text-dark-400">{{ field.label }}</span>
                    <span :class="['min-w-0 flex-1 truncate text-xs text-gray-800 dark:text-gray-200', field.mono && 'font-mono']">{{ field.value }}</span>
                    <button type="button" class="flex-shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400" @click="copyText(field.value)">
                      <Icon name="copy" size="xs" />
                    </button>
                  </div>
                </div>
              </div>

              <!-- 步骤 -->
              <div v-if="result.steps.length" class="space-y-2">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                  {{ t('clientImport.result.stepsTitle') }}
                </h3>
                <ol class="space-y-1.5">
                  <li
                    v-for="(step, index) in result.steps"
                    :key="index"
                    class="flex gap-2.5 text-xs leading-5 text-gray-600 dark:text-dark-300"
                  >
                    <span class="mt-0.5 flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full bg-gray-100 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-dark-300">
                      {{ index + 1 }}
                    </span>
                    <span class="min-w-0">{{ step.text }}</span>
                  </li>
                </ol>
              </div>
            </div>
          </section>

          <!-- 未选客户端时的引导 -->
          <section
            v-else-if="selectedGroup"
            class="card border border-dashed px-5 py-14 text-center shadow-none"
          >
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('clientImport.emptyHint') }}</p>
          </section>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { platformDisplayName } from '@/constants/platformCatalog'
import { maskApiKey } from '@/utils/maskApiKey'
import { extractApiErrorMessage } from '@/utils/apiError'
import { resolveEndpoints, siteRoot } from '@/utils/clientImport/endpoints'
import { fetchGroupModels, type GroupModel } from '@/utils/clientImport/models'
import {
  CLIENT_DEFINITIONS,
  ccSwitchAppLabel,
  evaluateClient,
  findClientDefinition,
  listCcSwitchApps,
  type ClientAvailability
} from '@/utils/clientImport/registry'
import type { ClientImportId, ImportMode } from '@/utils/clientImport/types'
import type { CcSwitchApp } from '@/utils/ccswitchImport'
import type { ApiKey } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const keysLoading = ref(true)
const keys = ref<ApiKey[]>([])

const selectedKeyId = ref<number | null>(null)
const selectedClientId = ref<ClientImportId | null>(null)

const modelOptions = ref<GroupModel[]>([])
const manualModels = ref<string[]>([])
const selectedModels = ref<string[]>([])
const defaultModel = ref('')
const manualModelInput = ref('')
const modelSearch = ref('')
const modelsLoading = ref(false)
const modelsError = ref(false)

const activeFileIndex = ref(0)

/**
 * 一键命令默认按访问者系统选 shell：Windows 给 PowerShell，其余给 bash。
 * 不能用宽松的 /win/：macOS 的 "darwin" 也含 win。
 */
type CommandShell = 'bash' | 'powershell'
const isWindowsVisitor = typeof navigator !== 'undefined' &&
  (/^win/i.test(navigator.platform || '') || /\bWindows\b/.test(navigator.userAgent || ''))
const activeShell = ref<CommandShell>(isWindowsVisitor ? 'powershell' : 'bash')
let modelsController: AbortController | null = null

const selectedKey = computed(() => keys.value.find((key) => key.id === selectedKeyId.value) ?? null)

/**
 * 分组直接取自密钥：密钥列表接口已经带回完整的 group（含 platform 与
 * claude_code_only），不必再让用户单独选一次分组。
 */
const selectedGroup = computed(() => selectedKey.value?.group ?? null)

/** CC Switch 的目标应用：可选项由分组平台与 claude_code_only 决定 */
const ccSwitchApp = ref<CcSwitchApp | null>(null)
const ccSwitchApps = computed((): CcSwitchApp[] =>
  selectedGroup.value ? listCcSwitchApps(selectedGroup.value) : []
)

/** 分组与密钥都就绪时才能真正生成配置 */
const activeClient = computed(() => {
  if (!selectedGroup.value || !selectedKey.value || !selectedClientId.value) return null
  const definition = findClientDefinition(selectedClientId.value)
  if (!definition) return null
  const availability = evaluateClient(definition, selectedGroup.value)
  if (!availability.allowed) return null
  return { definition, availability }
})

interface ClientCard {
  definition: (typeof CLIENT_DEFINITIONS)[number]
  allowed: boolean
  reason?: NonNullable<ClientAvailability['reason']>
}

/** 管理端「隐藏 CCS 导入按钮」开关同样作用于导入页的 CC Switch 卡片 */
const hideCcSwitch = computed(() => Boolean(appStore.cachedPublicSettings?.hide_ccs_import_button))

const clientCards = computed((): ClientCard[] => {
  const group = selectedGroup.value
  if (!group) return []
  return CLIENT_DEFINITIONS.filter(
    (definition) => !(definition.id === 'cc-switch' && hideCcSwitch.value)
  ).map((definition) => {
    const availability = evaluateClient(definition, group)
    return { definition, allowed: availability.allowed, reason: availability.reason }
  })
})

const baseUrl = computed(() => {
  const configured = appStore.cachedPublicSettings?.api_base_url
  return siteRoot(configured || window.location.origin)
})

const siteName = computed(() => appStore.cachedPublicSettings?.site_name?.trim() || 'Sub2API')

/** 左栏端点摘要：让用户先看清这个分组对外暴露哪些协议 */
const endpointDisplay = computed((): Array<{ label: string; value: string }> => {
  const group = selectedGroup.value
  if (!group) return []
  const endpoints = resolveEndpoints(baseUrl.value, group.platform)
  const rows: Array<{ label: string; value: string }> = []
  if (endpoints.anthropic) rows.push({ label: 'Anthropic', value: endpoints.anthropic })
  if (endpoints.chat) rows.push({ label: 'Chat', value: endpoints.chat })
  if (endpoints.responses) rows.push({ label: 'Responses', value: endpoints.responses })
  if (endpoints.gemini) rows.push({ label: 'Gemini', value: endpoints.gemini })
  return rows
})

/** 候选模型 = 远程列表 + 手动添加，去重且保持顺序 */
const allModelOptions = computed((): GroupModel[] => {
  const seen = new Set<string>()
  const merged: GroupModel[] = []
  for (const model of [...modelOptions.value, ...manualModels.value.map((id) => ({ id }))]) {
    if (seen.has(model.id)) continue
    seen.add(model.id)
    merged.push(model)
  }
  return merged
})

const filteredModelOptions = computed((): GroupModel[] => {
  const query = modelSearch.value.trim().toLowerCase()
  if (!query) return allModelOptions.value
  return allModelOptions.value.filter(
    (model) =>
      model.id.toLowerCase().includes(query) ||
      (model.name ?? '').toLowerCase().includes(query)
  )
})

const activeCommand = computed(() => {
  const commands = result.value.commands ?? []
  return commands.find((command) => command.shell === activeShell.value) ?? commands[0]
})

const activeFile = computed(() => result.value.files[activeFileIndex.value] ?? result.value.files[0])

const result = computed(() => {
  if (!activeClient.value || !selectedKey.value || !selectedGroup.value) {
    return { files: [], manual: [], steps: [] } as ReturnType<(typeof CLIENT_DEFINITIONS)[number]['generate']>
  }
  const models = activeClient.value.definition.modelMode === 'multi'
    ? selectedModels.value
    : defaultModel.value
      ? [defaultModel.value]
      : []
  return activeClient.value.definition.generate({
    baseUrl: baseUrl.value,
    apiKey: selectedKey.value.key,
    siteName: siteName.value,
    models,
    defaultModel: defaultModel.value,
    platform: selectedGroup.value.platform,
    keyId: selectedKey.value.id,
    keyName: selectedKey.value.name,
    ccSwitchApp: ccSwitchApp.value ?? undefined
  })
})

/** 图标名必须收窄成字面量，Icon 组件的 name 是 keyof 联合类型 */
type ClientIconName = 'terminal' | 'chat' | 'chatBubble' | 'sparkles' | 'cpu' | 'bolt' | 'swap'

const clientIcon = (id: ClientImportId): ClientIconName => {
  switch (id) {
    case 'cherry-studio':
      return 'chat'
    case 'chatbox':
      return 'chatBubble'
    case 'magpie':
      return 'sparkles'
    case 'workbuddy':
      return 'cpu'
    case 'deepseek-hermes':
      return 'bolt'
    case 'cc-switch':
      return 'swap'
    default:
      return 'terminal'
  }
}

/** 导入方式的色彩语义：深链与命令=可一键、文件=需下载、手动=仅参考 */
const modeBadgeClass = (mode: ImportMode): string => {
  switch (mode) {
    case 'deeplink':
    case 'command':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
    case 'file':
      return 'bg-primary-50 text-primary-700 dark:bg-primary-950/40 dark:text-primary-300'
    default:
      return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300'
  }
}

async function loadKeys() {
  keysLoading.value = true
  try {
    // 一次拉全部密钥：分组信息随密钥返回，用户只需挑一个密钥。
    const response = await keysAPI.list(1, 200)
    keys.value = response.items.filter((key) => key.status === 'active' && key.group)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('clientImport.loadFailed')))
  } finally {
    keysLoading.value = false
  }
}

async function loadModels() {
  const key = selectedKey.value
  const group = selectedGroup.value
  if (!key || !group) return

  modelsController?.abort()
  modelsController = new AbortController()
  modelsLoading.value = true
  modelsError.value = false
  try {
    modelOptions.value = await fetchGroupModels(baseUrl.value, key.key, group.platform, modelsController.signal)
    // 默认模型失效时回退到第一个可用模型
    if (!defaultModel.value || !allModelOptions.value.some((model) => model.id === defaultModel.value)) {
      defaultModel.value = allModelOptions.value[0]?.id ?? ''
    }
  } catch (error) {
    if ((error as Error)?.name === 'AbortError') return
    modelsError.value = true
    modelOptions.value = []
  } finally {
    modelsLoading.value = false
  }
}

function resetModels() {
  modelsController?.abort()
  modelOptions.value = []
  manualModels.value = []
  selectedModels.value = []
  defaultModel.value = ''
  manualModelInput.value = ''
  modelSearch.value = ''
  modelsLoading.value = false
  modelsError.value = false
}

function selectClient(id: ClientImportId) {
  const definition = findClientDefinition(id)
  const group = selectedGroup.value
  if (!definition || !group) return
  if (!evaluateClient(definition, group).allowed) return

  selectedClientId.value = id
  activeFileIndex.value = 0
  resetModels()
  // CC Switch 默认选第一个可用应用（按分组平台筛过），其余客户端清空
  ccSwitchApp.value = id === 'cc-switch' ? (ccSwitchApps.value[0] ?? null) : null

  // 切换客户端后立刻拉一次模型，省掉一次点击
  if (definition.modelMode === 'multi') {
    void loadModels()
  }
}

function toggleModel(id: string, checked: boolean) {
  if (checked) {
    if (!selectedModels.value.includes(id)) selectedModels.value = [...selectedModels.value, id]
    if (!defaultModel.value) defaultModel.value = id
  } else {
    selectedModels.value = selectedModels.value.filter((model) => model !== id)
    if (defaultModel.value === id) defaultModel.value = selectedModels.value[0] ?? ''
  }
}

function addManualModel() {
  const id = manualModelInput.value.trim()
  if (!id) return
  if (!manualModels.value.includes(id)) manualModels.value = [...manualModels.value, id]
  const definition = activeClient.value?.definition
  if (definition?.modelMode === 'multi') {
    if (!selectedModels.value.includes(id)) selectedModels.value = [...selectedModels.value, id]
    if (!defaultModel.value) defaultModel.value = id
  } else {
    defaultModel.value = id
  }
  manualModelInput.value = ''
}

function removeManualModel(id: string) {
  manualModels.value = manualModels.value.filter((model) => model !== id)
  if (selectedModels.value.includes(id)) toggleModel(id, false)
  if (defaultModel.value === id) defaultModel.value = ''
}

function clearModels() {
  selectedModels.value = []
  defaultModel.value = ''
}

async function copyText(text: string) {
  await copyToClipboard(text, t('clientImport.result.copied'))
}

function downloadFile(file: { path: string; content: string } | undefined) {
  if (!file) return
  // 配置路径取 basename（~/.codex/config.toml → config.toml）；
  // 终端标签这类非路径的展示文案不能按 / 切，否则会截成 "Linux)"。
  const isPath = file.path.startsWith('~') || file.path.startsWith('%') || file.path.includes('\\')
  const basename = isPath ? file.path.split(/[\\/]/).pop() : file.path
  const name = (basename || 'config.txt').replace(/[^\w.-]+/g, '-').replace(/^-+|-+$/g, '') || 'config.txt'
  saveAs(new Blob([file.content], { type: 'text/plain;charset=utf-8' }), name)
}

watch(selectedKeyId, () => {
  // 换密钥等于换分组，之前选的客户端可能不再可用，一并重置。
  selectedClientId.value = null
  ccSwitchApp.value = null
  resetModels()
  void loadModels()
})

onMounted(loadKeys)
</script>
