<template>
  <main class="min-h-screen bg-white text-gray-900 dark:bg-dark-950 dark:text-gray-100">
    <header class="border-b border-gray-200 dark:border-dark-700">
      <div class="mx-auto flex max-w-4xl items-center justify-between gap-4 px-5 py-5 sm:px-8">
        <div class="flex min-w-0 items-center gap-3"><img src="/logo.png" alt="" class="h-9 w-9 object-contain" /><h1 class="text-xl font-semibold">{{ t('balanceQuery.title') }}</h1></div>
        <button type="button" class="query-button" :title="t('balanceQuery.refresh')" :aria-label="t('balanceQuery.refresh')" :disabled="loading || retryRemaining > 0 || !validToken" @click="load(page)"><Icon name="refresh" size="md" :class="{ 'animate-spin': loading }" /></button>
      </div>
    </header>
    <div class="mx-auto max-w-4xl px-5 py-8 sm:px-8">
      <div v-if="loading" role="status" class="py-20 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="errorMessage" role="alert" class="border-l-4 border-amber-500 py-3 pl-4 text-sm text-gray-700 dark:text-gray-300">
        {{ errorMessage }}<span v-if="retryRemaining > 0"> {{ t('balanceQuery.retryAfter', { seconds: retryRemaining }) }}</span>
      </div>
      <template v-else-if="data">
        <section class="border-b border-gray-200 pb-8 dark:border-dark-700">
          <div class="mb-5 flex flex-wrap items-baseline justify-between gap-2">
            <h2 class="min-w-0 break-words text-base font-medium">{{ data.user.username || `#${data.user.id}` }}</h2>
            <span class="text-sm tabular-nums text-gray-500">ID {{ data.user.id }}</span>
          </div>
          <p class="text-sm text-gray-500">{{ t('balanceQuery.balance') }}</p>
          <p class="mt-2 break-all text-3xl font-semibold tabular-nums text-emerald-700 dark:text-emerald-400" data-test="query-balance">{{ money(data.user.balance) }}</p>
        </section>
        <section v-if="data.payment_methods.length" class="border-b border-gray-200 py-7 dark:border-dark-700">
          <h2 class="mb-4 text-base font-semibold">{{ t('balanceQuery.paymentMethods') }}</h2>
          <dl class="space-y-3">
            <div v-for="(method, index) in data.payment_methods" :key="index" class="grid min-w-0 grid-cols-[minmax(0,1fr)_32px] gap-2 sm:grid-cols-[minmax(100px,1fr)_minmax(0,3fr)_32px]">
              <dt class="min-w-0 break-words text-sm text-gray-500">{{ method.key }}</dt>
              <dd class="col-span-2 min-w-0 whitespace-pre-wrap break-all text-sm sm:col-span-1">{{ method.value }}</dd>
              <button type="button" class="query-button col-start-2 row-start-1 sm:col-start-3" :title="t('balanceQuery.copy')" :aria-label="t('balanceQuery.copy')" @click="copyValue(method.value)"><Icon name="copy" size="sm" /></button>
            </div>
          </dl>
          <p v-if="copyStatus" role="status" class="mt-3 text-xs text-gray-500">{{ copyStatus }}</p>
        </section>
        <section class="py-7">
          <h2 class="mb-4 text-base font-semibold">{{ t('balanceQuery.history') }}</h2>
          <p v-if="!data.records.length" class="py-8 text-center text-sm text-gray-500">{{ t('balanceQuery.empty') }}</p>
          <div v-else class="overflow-x-auto">
            <table class="w-full text-left text-sm">
              <thead class="border-y border-gray-200 text-xs text-gray-500 dark:border-dark-700"><tr><th class="py-3 pr-3 font-medium">{{ t('balanceQuery.time') }}</th><th class="px-2 py-3 text-right font-medium">{{ t('balanceQuery.amount') }}</th><th class="py-3 pl-3 font-medium">{{ t('balanceQuery.note') }}</th></tr></thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                <tr v-for="(record, index) in data.records" :key="index">
                  <td class="py-4 pr-3 text-xs text-gray-500">{{ formatDateTime(record.created_at) }}</td>
                  <td class="whitespace-nowrap px-2 py-4 text-right font-medium tabular-nums text-emerald-700 dark:text-emerald-400">+{{ money(record.amount) }}</td>
                  <td class="max-w-56 break-words py-4 pl-3" :class="record.type === 'temporary_credit' ? 'text-amber-700 dark:text-amber-400' : ''">{{ record.note }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="data.total > data.page_size" class="mt-4 flex items-center justify-end gap-3 text-sm">
            <button type="button" class="query-button" :disabled="page <= 1 || loading" :title="t('common.previous')" :aria-label="t('common.previous')" @click="load(page - 1)"><Icon name="chevronLeft" size="sm" /></button>
            <span class="tabular-nums text-gray-500">{{ page }} / {{ Math.ceil(data.total / data.page_size) }}</span>
            <button type="button" class="query-button" :disabled="page * data.page_size >= data.total || loading" :title="t('common.next')" :aria-label="t('common.next')" @click="load(page + 1)"><Icon name="chevronRight" size="sm" /></button>
          </div>
        </section>
      </template>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { queryPublicBalance, BalanceQueryError, type PublicBalanceQuery } from '@/api/balanceQuery'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const data = ref<PublicBalanceQuery | null>(null)
const loading = ref(false)
const errorMessage = ref('')
const copyStatus = ref('')
const page = ref(1)
const retryRemaining = ref(0)
const token = computed(() => route.hash.slice(1))
const validToken = computed(() => /^[A-Za-z0-9_-]{43}$/.test(token.value))
let controller: AbortController | null = null
let retryTimer: ReturnType<typeof setInterval> | undefined
let requestVersion = 0
const money = (amount: number) => `$${amount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 8 })}`

async function load(targetPage = 1) {
  const version = ++requestVersion
  controller?.abort()
  clearInterval(retryTimer)
  retryRemaining.value = 0
  data.value = null
  copyStatus.value = ''
  errorMessage.value = ''
  if (!validToken.value) { loading.value = false; errorMessage.value = t('balanceQuery.unavailable'); return }
  const currentController = new AbortController()
  controller = currentController
  loading.value = true
  const timeout = setTimeout(() => currentController.abort(), 15000)
  try {
    const result = await queryPublicBalance(token.value, targetPage, currentController.signal)
    if (version !== requestVersion) return
    data.value = result
    page.value = result.page
  } catch (error) {
    if (version !== requestVersion) return
    const status = error instanceof BalanceQueryError ? error.status : 0
    errorMessage.value = t(status === 404 ? 'balanceQuery.unavailable' : status === 429 ? 'balanceQuery.rateLimited' : 'balanceQuery.serviceUnavailable')
    if (error instanceof BalanceQueryError && status === 429) {
      retryRemaining.value = Math.min(3600, Math.max(1, error.retryAfter || 60))
      retryTimer = setInterval(() => { if (--retryRemaining.value <= 0) clearInterval(retryTimer) }, 1000)
    }
  } finally { clearTimeout(timeout); if (version === requestVersion) loading.value = false }
}
async function copyValue(value: string) {
  try { await navigator.clipboard.writeText(value); copyStatus.value = t('balanceQuery.copied') }
  catch { copyStatus.value = t('balanceQuery.copyFailed') }
}

const metaRestore: Array<() => void> = []
for (const [name, content] of [['robots', 'noindex, nofollow, noarchive'], ['referrer', 'no-referrer']]) {
  const existing = document.querySelector<HTMLMetaElement>(`meta[name="${name}"]`)
  const element = existing || document.createElement('meta')
  const previous = element.content
  element.name = name
  element.content = content
  if (!existing) document.head.append(element)
  metaRestore.push(() => { if (existing) element.content = previous; else element.remove() })
}
watch(token, () => { void load(1) }, { immediate: true })
onBeforeUnmount(() => { requestVersion++; controller?.abort(); clearInterval(retryTimer); metaRestore.forEach(restore => restore()) })
</script>

<style scoped>
.query-button { display: inline-flex; width: 32px; height: 32px; flex-shrink: 0; align-items: center; justify-content: center; border-radius: 4px; }
.query-button:hover:not(:disabled) { background: rgb(148 163 184 / 15%); }
.query-button:disabled { opacity: .4; cursor: not-allowed; }
</style>
