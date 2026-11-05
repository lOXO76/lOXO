<template>
  <div class="space-y-6">
    <p class="text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.api.intro') }}</p>

    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.api.commonTitle') }}</h3>
      <ul class="mt-2 list-disc space-y-1.5 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.api.commonBase') }}</li>
        <li>{{ t('rewardIntegration.api.commonEnvelope') }}</li>
        <li>{{ t('rewardIntegration.api.commonError') }}</li>
        <li>{{ t('rewardIntegration.api.commonContentType') }}</li>
        <li>{{ t('rewardIntegration.api.commonLimit') }}</li>
      </ul>
    </div>

    <div class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.api.pingTitle') }}</h3>
        <div class="mt-2 flex flex-wrap items-center gap-2 font-mono text-xs">
          <span class="rounded bg-emerald-100 px-1.5 py-0.5 font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">POST</span>
          <span class="text-gray-700 dark:text-dark-200">/api/v1/reward-open/ping</span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300">{{ t('rewardIntegration.api.signed') }}</span>
        </div>
        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.api.pingDesc') }}</p>
      </div>
      <DocsCodeBlock label="ping.request.json" :code="pingRequest" />
      <DocsCodeBlock label="ping.response.json" :code="pingResponse" />
      <DocsTable :headers="responseHeaders">
        <tr v-for="field in pingResponseFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <ul class="list-disc space-y-1 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.api.pingNote1') }}</li>
        <li>{{ t('rewardIntegration.api.pingNote2') }}</li>
      </ul>
    </div>

    <div class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.api.ticketsTitle') }}</h3>
        <div class="mt-2 flex flex-wrap items-center gap-2 font-mono text-xs">
          <span class="rounded bg-emerald-100 px-1.5 py-0.5 font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">POST</span>
          <span class="text-gray-700 dark:text-dark-200">/api/v1/reward-open/tickets/query</span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300">{{ t('rewardIntegration.api.signed') }}</span>
        </div>
        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.api.ticketsDesc') }}</p>
      </div>
      <DocsCodeBlock label="tickets.request.json" :code="ticketsRequest" />
      <DocsTable :headers="requestHeaders">
        <tr v-for="field in ticketsRequestFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.required ? t('rewardIntegration.api.required') : t('rewardIntegration.api.optional') }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <DocsCodeBlock label="tickets.response.json" :code="ticketsResponse" />
      <DocsTable :headers="responseHeaders">
        <tr v-for="field in ticketsResponseFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <ul class="list-disc space-y-1 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.api.ticketsNote1') }}</li>
        <li>{{ t('rewardIntegration.api.ticketsNote2') }}</li>
      </ul>
    </div>

    <div class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.api.eventsTitle') }}</h3>
        <div class="mt-2 flex flex-wrap items-center gap-2 font-mono text-xs">
          <span class="rounded bg-emerald-100 px-1.5 py-0.5 font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">POST</span>
          <span class="text-gray-700 dark:text-dark-200">/api/v1/reward-open/events</span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300">{{ t('rewardIntegration.api.signed') }}</span>
        </div>
        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.api.eventsDesc') }}</p>
      </div>
      <DocsTable :headers="requestHeaders">
        <tr v-for="field in eventsRequestFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.required ? t('rewardIntegration.api.required') : t('rewardIntegration.api.optional') }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <DocsCodeBlock label="events.register.json" :code="registerRequest" />
      <DocsCodeBlock label="events.recharge.json" :code="rechargeRequest" />
      <DocsCodeBlock label="events.response.json" :code="eventsResponse" />
      <DocsTable :headers="responseHeaders">
        <tr v-for="field in eventsResponseFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('rewardIntegration.api.eventsResultTitle') }}</h4>
        <DocsTable class="mt-2" :headers="resultHeaders">
          <tr v-for="item in eventResults" :key="item.result">
            <td>{{ item.result }}</td>
            <td>{{ item.desc }}</td>
          </tr>
        </DocsTable>
      </div>
      <ul class="list-disc space-y-1 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.api.eventsNote1') }}</li>
        <li>{{ t('rewardIntegration.api.eventsNote2') }}</li>
        <li>{{ t('rewardIntegration.api.eventsNote3') }}</li>
      </ul>
    </div>

    <div class="space-y-3 border-t border-gray-200 pt-5 dark:border-dark-700">
      <div>
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.api.heartbeatTitle') }}</h3>
        <div class="mt-2 flex flex-wrap items-center gap-2 font-mono text-xs">
          <span class="rounded bg-emerald-100 px-1.5 py-0.5 font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">POST</span>
          <span class="text-gray-700 dark:text-dark-200">/api/v1/reward-open/sdk/heartbeat</span>
          <span class="rounded bg-gray-100 px-1.5 py-0.5 text-gray-500 dark:bg-dark-800 dark:text-dark-300">{{ t('rewardIntegration.api.unsigned') }}</span>
        </div>
        <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-dark-200">{{ t('rewardIntegration.api.heartbeatDesc') }}</p>
      </div>
      <DocsCodeBlock label="heartbeat.request.txt" :code="heartbeatRequest" />
      <DocsTable :headers="requestHeaders">
        <tr v-for="field in heartbeatRequestFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.required ? t('rewardIntegration.api.required') : t('rewardIntegration.api.optional') }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <DocsCodeBlock label="heartbeat.response.json" :code="heartbeatResponse" code-test-id="heartbeat-response" />
      <DocsTable :headers="responseHeaders">
        <tr v-for="field in heartbeatResponseFields" :key="field.name">
          <td>{{ field.name }}</td>
          <td class="font-mono text-xs">{{ field.type }}</td>
          <td>{{ field.desc }}</td>
        </tr>
      </DocsTable>
      <ul class="list-disc space-y-1 pl-5 text-sm leading-6 text-gray-600 dark:text-dark-200">
        <li>{{ t('rewardIntegration.api.heartbeatNote1') }}</li>
        <li>{{ t('rewardIntegration.api.heartbeatNote2') }}</li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'
import DocsTable from './DocsTable.vue'

interface ApiField {
  name: string
  type: string
  desc: string
  required?: boolean
}

const { t } = useI18n()

const requestHeaders = computed(() => [
  t('rewardIntegration.api.fieldColName'),
  t('rewardIntegration.api.fieldColType'),
  t('rewardIntegration.api.fieldColRequired'),
  t('rewardIntegration.api.fieldColDesc'),
])

const responseHeaders = computed(() => [
  t('rewardIntegration.api.fieldColName'),
  t('rewardIntegration.api.fieldColType'),
  t('rewardIntegration.api.fieldColDesc'),
])

const resultHeaders = computed(() => [
  t('rewardIntegration.api.fieldColValue'),
  t('rewardIntegration.api.fieldColDesc'),
])

const pingRequest = `{}`

const pingResponse = JSON.stringify({
  code: 0,
  message: 'success',
  data: {
    partner_name: 'Example Site',
    app_key: 'rk_0123456789abcdef01234567',
    server_time: 1730790000,
  },
}, null, 2)

const pingResponseFields = computed<ApiField[]>(() => [
  { name: 'partner_name', type: 'string', desc: t('rewardIntegration.api.pingFieldPartnerName') },
  { name: 'app_key', type: 'string', desc: t('rewardIntegration.api.pingFieldAppKey') },
  { name: 'server_time', type: 'number', desc: t('rewardIntegration.api.pingFieldServerTime') },
])

const ticketsRequest = JSON.stringify({
  ticket: 'rt_0123456789abcdef0123456789abcdef01234567',
}, null, 2)

const ticketsRequestFields = computed<ApiField[]>(() => [
  { name: 'ticket', type: 'string', required: true, desc: t('rewardIntegration.api.fieldTicketDesc') },
])

const ticketsResponse = JSON.stringify({
  code: 0,
  message: 'success',
  data: {
    ticket: 'rt_0123456789abcdef0123456789abcdef01234567',
    task_id: 1,
    task_title: '累计充值满 100 元',
    task_type: 'recharge',
    period: 'monthly',
    period_key: '2026-11',
    status: 'in_progress',
    expires_at: '2026-11-30T23:59:59+08:00',
    expired: false,
    external_user_id: '',
    browse_seconds: 0,
    progress_seconds: 0,
    min_recharge_amount: 100,
    recharge_total: 30,
  },
}, null, 2)

const ticketsResponseFields = computed<ApiField[]>(() => [
  { name: 'ticket', type: 'string', desc: t('rewardIntegration.api.ticketsFieldTicket') },
  { name: 'task_id', type: 'number', desc: t('rewardIntegration.api.ticketsFieldTaskId') },
  { name: 'task_title', type: 'string', desc: t('rewardIntegration.api.ticketsFieldTaskTitle') },
  { name: 'task_type', type: 'string', desc: t('rewardIntegration.api.ticketsFieldTaskType') },
  { name: 'period', type: 'string', desc: t('rewardIntegration.api.ticketsFieldPeriod') },
  { name: 'period_key', type: 'string', desc: t('rewardIntegration.api.ticketsFieldPeriodKey') },
  { name: 'status', type: 'string', desc: t('rewardIntegration.api.ticketsFieldStatus') },
  { name: 'expires_at', type: 'string', desc: t('rewardIntegration.api.ticketsFieldExpiresAt') },
  { name: 'expired', type: 'boolean', desc: t('rewardIntegration.api.ticketsFieldExpired') },
  { name: 'external_user_id', type: 'string', desc: t('rewardIntegration.api.ticketsFieldExternalUserId') },
  { name: 'browse_seconds', type: 'number', desc: t('rewardIntegration.api.ticketsFieldBrowseSeconds') },
  { name: 'progress_seconds', type: 'number', desc: t('rewardIntegration.api.ticketsFieldProgressSeconds') },
  { name: 'min_recharge_amount', type: 'number', desc: t('rewardIntegration.api.ticketsFieldMinRechargeAmount') },
  { name: 'recharge_total', type: 'number', desc: t('rewardIntegration.api.ticketsFieldRechargeTotal') },
])

const eventsRequestFields = computed<ApiField[]>(() => [
  { name: 'event_id', type: 'string', required: true, desc: t('rewardIntegration.api.fieldEventIdDesc') },
  { name: 'event_type', type: 'string', required: true, desc: t('rewardIntegration.api.fieldEventTypeDesc') },
  { name: 'ticket', type: 'string', required: true, desc: t('rewardIntegration.api.fieldEventTicketDesc') },
  { name: 'external_user_id', type: 'string', required: true, desc: t('rewardIntegration.api.fieldExternalUserIdDesc') },
  { name: 'amount', type: 'number | string', required: false, desc: t('rewardIntegration.api.fieldAmountDesc') },
  { name: 'order_id', type: 'string', required: false, desc: t('rewardIntegration.api.fieldOrderIdDesc') },
  { name: 'occurred_at', type: 'number', required: false, desc: t('rewardIntegration.api.fieldOccurredAtDesc') },
])

const registerRequest = JSON.stringify({
  event_id: 'evt_20261105_0001',
  event_type: 'register',
  ticket: 'rt_0123456789abcdef0123456789abcdef01234567',
  external_user_id: 'u_1001',
}, null, 2)

const rechargeRequest = JSON.stringify({
  event_id: 'evt_20261105_0002',
  event_type: 'recharge',
  ticket: 'rt_0123456789abcdef0123456789abcdef01234567',
  external_user_id: 'u_1001',
  amount: '100.00',
  order_id: 'ord_20261105_0001',
  occurred_at: 1730790000,
}, null, 2)

const eventsResponse = JSON.stringify({
  code: 0,
  message: 'success',
  data: {
    event_id: 'evt_20261105_0002',
    duplicate: false,
    result: 'completed',
    task_id: 1,
    task_type: 'recharge',
    status: 'completed',
    rewarded: true,
    progress: {
      recharge_total: 100,
      min_recharge_amount: 100,
      progress_seconds: 0,
      browse_seconds: 0,
    },
  },
}, null, 2)

const eventsResponseFields = computed<ApiField[]>(() => [
  { name: 'event_id', type: 'string', desc: t('rewardIntegration.api.eventsFieldEventId') },
  { name: 'duplicate', type: 'boolean', desc: t('rewardIntegration.api.eventsFieldDuplicate') },
  { name: 'result', type: 'string', desc: t('rewardIntegration.api.eventsFieldResult') },
  { name: 'task_id', type: 'number', desc: t('rewardIntegration.api.eventsFieldTaskId') },
  { name: 'task_type', type: 'string', desc: t('rewardIntegration.api.eventsFieldTaskType') },
  { name: 'status', type: 'string', desc: t('rewardIntegration.api.eventsFieldStatus') },
  { name: 'rewarded', type: 'boolean', desc: t('rewardIntegration.api.eventsFieldRewarded') },
  { name: 'progress', type: 'object', desc: t('rewardIntegration.api.eventsFieldProgress') },
])

const eventResults = computed(() => [
  { result: 'accepted', desc: t('rewardIntegration.api.resultAccepted') },
  { result: 'completed', desc: t('rewardIntegration.api.resultCompleted') },
  { result: 'ignored', desc: t('rewardIntegration.api.resultIgnored') },
  { result: 'rejected', desc: t('rewardIntegration.api.resultRejected') },
])

const heartbeatRequest = [
  'POST /api/v1/reward-open/sdk/heartbeat',
  'Content-Type: text/plain',
  '',
  '{"app_key":"rk_0123456789abcdef01234567","ticket":"rt_0123456789abcdef0123456789abcdef01234567"}',
].join('\n')

const heartbeatResponse = JSON.stringify({
  code: 0,
  message: 'success',
  data: {
    status: 'in_progress',
    progress_seconds: 12,
    browse_seconds: 30,
    completed: false,
    reward_amount: 0.5,
    heartbeat_interval: 1,
    idle_timeout_seconds: 15,
  },
}, null, 2)

const heartbeatRequestFields = computed<ApiField[]>(() => [
  { name: 'app_key', type: 'string', required: true, desc: t('rewardIntegration.api.fieldAppKeyDesc') },
  { name: 'ticket', type: 'string', required: true, desc: t('rewardIntegration.api.fieldSdkTicketDesc') },
])

const heartbeatResponseFields = computed<ApiField[]>(() => [
  { name: 'status', type: 'string', desc: t('rewardIntegration.api.heartbeatFieldStatus') },
  { name: 'progress_seconds', type: 'number', desc: t('rewardIntegration.api.heartbeatFieldProgressSeconds') },
  { name: 'browse_seconds', type: 'number', desc: t('rewardIntegration.api.heartbeatFieldBrowseSeconds') },
  { name: 'completed', type: 'boolean', desc: t('rewardIntegration.api.heartbeatFieldCompleted') },
  { name: 'reward_amount', type: 'number', desc: t('rewardIntegration.api.heartbeatFieldRewardAmount') },
  { name: 'heartbeat_interval', type: 'number', desc: t('rewardIntegration.api.heartbeatFieldHeartbeatInterval') },
  { name: 'idle_timeout_seconds', type: 'number', desc: t('rewardIntegration.api.heartbeatFieldIdleTimeout') },
])
</script>
