<template>
  <div class="space-y-4 rounded-xl border border-gray-200 bg-gray-50/60 p-4 dark:border-dark-700 dark:bg-dark-800/40">
    <div>
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('rewardIntegration.signature.calcTitle') }}</h3>
      <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-dark-300">{{ t('rewardIntegration.signature.calcDesc') }}</p>
    </div>

    <div class="grid gap-3 sm:grid-cols-2">
      <div class="sm:col-span-2">
        <label for="reward-calc-secret" class="input-label">{{ t('rewardIntegration.signature.calcSecretLabel') }}</label>
        <input
          id="reward-calc-secret"
          v-model="secret"
          type="text"
          autocomplete="off"
          spellcheck="false"
          class="input font-mono"
          placeholder="rs_..."
        />
        <p class="input-hint">{{ t('rewardIntegration.signature.calcSecretHint') }}</p>
      </div>

      <div>
        <label for="reward-calc-timestamp" class="input-label">{{ t('rewardIntegration.signature.calcTimestampLabel') }}</label>
        <div class="flex gap-2">
          <input
            id="reward-calc-timestamp"
            v-model="timestamp"
            type="text"
            inputmode="numeric"
            autocomplete="off"
            spellcheck="false"
            class="input font-mono"
          />
          <button type="button" class="shrink-0 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-600 transition hover:bg-gray-100 dark:border-dark-600 dark:bg-dark-900 dark:text-dark-200 dark:hover:bg-dark-700" @click="fillTimestamp">
            {{ t('rewardIntegration.signature.calcGenerateTimestamp') }}
          </button>
        </div>
      </div>

      <div>
        <label for="reward-calc-nonce" class="input-label">{{ t('rewardIntegration.signature.calcNonceLabel') }}</label>
        <div class="flex gap-2">
          <input
            id="reward-calc-nonce"
            v-model="nonce"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="input font-mono"
          />
          <button type="button" class="shrink-0 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-600 transition hover:bg-gray-100 dark:border-dark-600 dark:bg-dark-900 dark:text-dark-200 dark:hover:bg-dark-700" @click="fillNonce">
            {{ t('rewardIntegration.signature.calcGenerateNonce') }}
          </button>
        </div>
      </div>

      <div class="sm:col-span-2">
        <label for="reward-calc-body" class="input-label">{{ t('rewardIntegration.signature.calcBodyLabel') }}</label>
        <textarea
          id="reward-calc-body"
          v-model="body"
          rows="4"
          spellcheck="false"
          class="input font-mono text-xs"
        ></textarea>
      </div>
    </div>

    <DocsCodeBlock
      :label="t('rewardIntegration.signature.calcSigningString')"
      :code="signingString"
      code-test-id="signing-string"
    />

    <div>
      <div class="mb-1.5 text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('rewardIntegration.signature.calcSignature') }}</div>
      <DocsCodeBlock
        :label="t('rewardIntegration.signature.calcSignature')"
        :code="signatureDisplay"
        code-test-id="signature-hex"
      />
      <p v-if="status === 'idle'" class="input-hint">{{ t('rewardIntegration.signature.calcIdle') }}</p>
      <p v-else-if="status === 'computing'" class="input-hint">{{ t('rewardIntegration.signature.calcComputing') }}</p>
      <p v-else-if="status === 'unsupported'" class="input-error-text">{{ t('rewardIntegration.signature.calcUnsupported') }}</p>
      <p v-else-if="status === 'failed'" class="input-error-text">{{ t('rewardIntegration.signature.calcFailed') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DocsCodeBlock from './DocsCodeBlock.vue'

type CalcStatus = 'idle' | 'computing' | 'ready' | 'unsupported' | 'failed'

const { t } = useI18n()

const encoder = new TextEncoder()

const secret = ref('')
const timestamp = ref('')
const nonce = ref('')
const body = ref('{"event_id":"evt_20261105_0001","event_type":"register","ticket":"rt_0123456789abcdef0123456789abcdef01234567","external_user_id":"u_1001"}')

const status = ref<CalcStatus>('idle')
const signature = ref('')

const signingString = computed(() => `${timestamp.value}\n${nonce.value}\n${body.value}`)

const signatureDisplay = computed(() => (signature.value ? signature.value : t('rewardIntegration.signature.calcSignatureEmpty')))

// 每次输入变化都自增，避免慢请求把旧结果写回输入框
let requestSeq = 0

function toHex(buffer: ArrayBuffer): string {
  return Array.from(new Uint8Array(buffer))
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join('')
}

async function computeSignature(): Promise<void> {
  const seq = ++requestSeq
  signature.value = ''

  if (!secret.value) {
    status.value = 'idle'
    return
  }

  const subtle = globalThis.crypto?.subtle
  if (!subtle) {
    status.value = 'unsupported'
    return
  }

  status.value = 'computing'
  try {
    const key = await subtle.importKey(
      'raw',
      encoder.encode(secret.value),
      { name: 'HMAC', hash: 'SHA-256' },
      false,
      ['sign'],
    )
    const digest = await subtle.sign('HMAC', key, encoder.encode(signingString.value))
    if (seq !== requestSeq) return
    signature.value = toHex(digest)
    status.value = 'ready'
  } catch {
    if (seq !== requestSeq) return
    status.value = 'failed'
  }
}

function fillTimestamp(): void {
  timestamp.value = String(Math.floor(Date.now() / 1000))
}

function fillNonce(): void {
  const bytes = new Uint8Array(16)
  if (globalThis.crypto?.getRandomValues) {
    globalThis.crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256)
  }
  nonce.value = toHex(bytes.buffer)
}

watch([secret, timestamp, nonce, body], () => {
  void computeSignature()
})

onMounted(() => {
  fillTimestamp()
  fillNonce()
})
</script>
