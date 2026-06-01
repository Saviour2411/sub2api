<template>
  <AppLayout>
    <div class="mx-auto max-w-[1120px] space-y-6">
      <section class="card overflow-hidden border border-primary-100/80 bg-white/95 p-6 dark:border-primary-900/40 dark:bg-dark-900/70">
        <div class="flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase tracking-[0.18em] text-primary-500">
              {{ t('dailyCheckin.kicker') }}
            </p>
            <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ t('dailyCheckin.title') }}
            </h1>
            <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-gray-400">
              {{ statusText }}
            </p>
          </div>
          <div class="grid grid-cols-2 gap-3 sm:min-w-[340px]">
            <div class="rounded-lg border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700 dark:bg-dark-900/60">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dailyCheckin.balance') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ formatCurrency(authStore.user?.balance || 0) }}</p>
            </div>
            <div class="rounded-lg border border-gray-100 bg-gray-50/80 p-4 dark:border-dark-700 dark:bg-dark-900/60">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('dailyCheckin.factor') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ factorLabel }}</p>
            </div>
          </div>
        </div>
      </section>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <section class="card border border-gray-100 bg-white/95 p-6 dark:border-dark-700 dark:bg-dark-900/70">
          <div class="flex flex-col items-center gap-6">
            <div class="relative h-[min(76vw,430px)] w-[min(76vw,430px)]">
              <div class="absolute left-1/2 top-0 z-10 h-10 w-8 -translate-x-1/2 rounded-b-full bg-primary-600 shadow-lg dark:bg-primary-400"></div>
              <div
                class="checkin-wheel h-full w-full rounded-full border-[10px] border-white shadow-2xl shadow-primary-900/10 ring-1 ring-primary-100 transition-transform duration-[2600ms] ease-out dark:border-dark-800 dark:ring-primary-900/50"
                :style="{ background: wheelBackground, transform: `rotate(${wheelRotation}deg)` }"
              >
                <div class="absolute inset-[18%] rounded-full bg-white/90 shadow-inner dark:bg-dark-900/90"></div>
              </div>
              <button
                type="button"
                class="absolute left-1/2 top-1/2 z-20 flex h-28 w-28 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-primary-200 bg-primary-600 px-4 text-center text-sm font-semibold text-white shadow-xl shadow-primary-700/25 transition hover:bg-primary-700 disabled:cursor-not-allowed disabled:bg-gray-400 dark:border-primary-400/40"
                :disabled="spinDisabled"
                @click="spin"
              >
                {{ spinButtonText }}
              </button>
            </div>

            <div v-if="lastResult" class="w-full rounded-lg border border-primary-100 bg-primary-50/80 p-4 text-center dark:border-primary-900/50 dark:bg-primary-950/30">
              <p class="text-sm text-primary-700 dark:text-primary-200">{{ t('dailyCheckin.result') }}</p>
              <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ rewardLabel(lastResult) }}</p>
            </div>
          </div>
        </section>

        <aside class="space-y-6">
          <section class="card border border-gray-100 bg-white/95 p-5 dark:border-dark-700 dark:bg-dark-900/70">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('dailyCheckin.prizePool') }}</h2>
            <div class="mt-4 space-y-3">
              <div
                v-for="prize in effectivePrizes"
                :key="prize.id"
                class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 bg-gray-50/80 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/50"
              >
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ prize.name }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ prizeTypeLabel(prize.type) }}</p>
                </div>
                <span class="shrink-0 text-sm font-semibold text-primary-600 dark:text-primary-300">{{ probabilityLabel(prize.effective_probability_bps) }}</span>
              </div>
            </div>
          </section>

          <section class="card border border-gray-100 bg-white/95 p-5 dark:border-dark-700 dark:bg-dark-900/70">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('dailyCheckin.recent') }}</h2>
            <div class="mt-4 space-y-3">
              <div v-if="!recentRecords.length" class="rounded-lg border border-dashed border-gray-200 p-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
                {{ t('dailyCheckin.empty') }}
              </div>
              <div
                v-for="record in recentRecords"
                :key="record.id"
                class="rounded-lg border border-gray-100 bg-gray-50/80 px-3 py-2 dark:border-dark-700 dark:bg-dark-900/50"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ rewardLabel(record) }}</span>
                  <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(record.checked_in_at) }}</span>
                </div>
              </div>
            </div>
          </section>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { redeemAPI, type DailyCheckinRecord, type DailyCheckinReward, type DailyCheckinStatus } from '@/api'
import { useAppStore, useAuthStore } from '@/stores'
import { formatCurrency } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const status = ref<DailyCheckinStatus | null>(null)
const loading = ref(false)
const spinning = ref(false)
const wheelRotation = ref(0)
const lastResult = ref<DailyCheckinReward | DailyCheckinRecord | null>(null)

const colors = ['#2563eb', '#10b981', '#f59e0b', '#ef4444', '#14b8a6', '#8b5cf6', '#64748b', '#f97316']

const effectivePrizes = computed(() => status.value?.prizes?.filter((p) => p.effective_probability_bps > 0) ?? [])
const recentRecords = computed(() => status.value?.recent_records ?? [])
const spinDisabled = computed(() => loading.value || spinning.value || !status.value?.enabled || status.value?.checked_in_today)
const spinButtonText = computed(() => {
  if (loading.value) return t('dailyCheckin.loading')
  if (spinning.value) return t('dailyCheckin.spinning')
  if (status.value?.checked_in_today) return t('dailyCheckin.done')
  return t('dailyCheckin.spin')
})
const factorLabel = computed(() => probabilityLabel(status.value?.decay?.factor_bps ?? 10000))
const statusText = computed(() => {
  if (!status.value) return t('dailyCheckin.loading')
  if (!status.value.enabled) return t('dailyCheckin.disabled')
  if (status.value.checked_in_today) return t('dailyCheckin.doneHint')
  if (status.value.decay?.exempt_reason === 'linuxdo') return t('dailyCheckin.linuxdoExempt')
  if (status.value.decay?.paid) return t('dailyCheckin.paid')
  return t('dailyCheckin.ready')
})
const wheelBackground = computed(() => {
  const prizes = effectivePrizes.value
  if (!prizes.length) return '#e5e7eb'
  let cursor = 0
  const stops: string[] = []
  prizes.forEach((prize, index) => {
    const start = cursor
    const end = cursor + prize.effective_probability_bps / 100
    const color = colors[index % colors.length]
    stops.push(`${color} ${start}% ${end}%`)
    cursor = end
  })
  return `conic-gradient(${stops.join(', ')})`
})

async function loadStatus() {
  loading.value = true
  try {
    status.value = await redeemAPI.getDailyCheckinStatus()
    lastResult.value = status.value.today_result || null
  } catch (error: any) {
    appStore.showError(t('dailyCheckin.loadFailed') + ': ' + (error?.message || t('common.unknownError')))
  } finally {
    loading.value = false
  }
}

async function spin() {
  if (spinDisabled.value) return
  spinning.value = true
  wheelRotation.value += 1440 + Math.floor(Math.random() * 360)
  try {
    const result = await redeemAPI.dailyCheckin()
    lastResult.value = result.prize || null
    appStore.showSuccess(t('dailyCheckin.success', { reward: rewardLabel(result.prize) }))
    await authStore.refreshUser()
    setTimeout(() => {
      loadStatus()
      spinning.value = false
    }, 900)
  } catch (error: any) {
    spinning.value = false
    if (Number(error?.status || 0) === 409) {
      appStore.showInfo(t('dailyCheckin.doneHint'))
      await loadStatus()
      return
    }
    appStore.showError(t('dailyCheckin.failed') + ': ' + (error?.message || t('common.unknownError')))
  }
}

function probabilityLabel(bps: number | undefined): string {
  return `${((Number(bps || 0)) / 100).toFixed(2)}%`
}

function prizeTypeLabel(type: string): string {
  if (type === 'concurrency') return t('dailyCheckin.types.concurrency')
  if (type === 'subscription') return t('dailyCheckin.types.subscription')
  if (type === 'none') return t('dailyCheckin.types.none')
  return t('dailyCheckin.types.balance')
}

function rewardLabel(reward: DailyCheckinReward | DailyCheckinRecord | null | undefined): string {
  if (!reward) return ''
  if (reward.type === 'balance') return `${reward.prize_name} ${formatCurrency(reward.amount || 0)}`
  if (reward.type === 'concurrency') return `${reward.prize_name} +${reward.concurrency || 0}`
  if (reward.type === 'subscription') return `${reward.prize_name} ${reward.validity_days || 0}${t('dailyCheckin.days')}`
  return reward.prize_name || t('dailyCheckin.types.none')
}

function formatDate(value: string): string {
  if (!value) return ''
  return new Date(value).toLocaleDateString()
}

onMounted(loadStatus)
</script>

<style scoped>
.checkin-wheel {
  position: relative;
}
</style>
