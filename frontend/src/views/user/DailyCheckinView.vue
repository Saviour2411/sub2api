<template>
  <AppLayout>
    <div class="mx-auto max-w-[1180px] space-y-6">
      <section class="checkin-hero overflow-hidden rounded-lg border border-amber-200/60 bg-white p-6 shadow-sm dark:border-amber-500/20 dark:bg-dark-900">
        <div class="relative z-10 flex flex-col gap-6 lg:flex-row lg:items-center lg:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-semibold uppercase text-amber-600 dark:text-amber-300">
              {{ t('dailyCheckin.kicker') }}
            </p>
            <h1 class="mt-2 text-2xl font-semibold text-gray-950 dark:text-white">
              {{ t('dailyCheckin.title') }}
            </h1>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-gray-600 dark:text-dark-300">
              {{ statusText }}
            </p>
          </div>
          <div
            class="grid gap-3 sm:min-w-[340px]"
            :class="showPoolCard ? 'grid-cols-2' : 'grid-cols-1'"
          >
            <div class="rounded-lg border border-white/70 bg-white/80 p-4 shadow-sm dark:border-white/10 dark:bg-dark-950/55">
              <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('dailyCheckin.balance') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-950 dark:text-white">
                {{ formatCurrency(authStore.user?.balance || 0) }}
              </p>
            </div>
            <div
              v-if="showPoolCard"
              class="rounded-lg border border-white/70 bg-white/80 p-4 shadow-sm dark:border-white/10 dark:bg-dark-950/55"
            >
              <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('dailyCheckin.factor') }}</p>
              <p class="mt-1 text-lg font-semibold text-gray-950 dark:text-white">{{ t('dailyCheckin.linuxdoPool') }}</p>
            </div>
          </div>
        </div>
      </section>

      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
        <section class="rounded-lg border border-gray-100 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-col items-center gap-6">
            <div class="wheel-stage">
              <div class="wheel-pointer">
                <span></span>
              </div>
              <div class="wheel-orbit">
                <div
                  class="checkin-wheel"
                  :class="{ 'is-spinning': spinning }"
                  :style="wheelStyle"
                >
                  <div
                    v-for="(prize, index) in wheelPrizes"
                    :key="prize.id"
                    class="wheel-label"
                    :style="labelStyle(index)"
                  >
                    <span>{{ prize.name }}</span>
                  </div>
                  <div class="wheel-hub">
                    <span class="wheel-hub-ring"></span>
                  </div>
                </div>
                <div class="wheel-glass"></div>
              </div>
              <button
                type="button"
                class="spin-button"
                :disabled="spinDisabled"
                @click="spin"
              >
                <Icon v-if="spinning" name="refresh" size="sm" :stroke-width="2" />
                <Icon v-else name="sparkles" size="sm" :stroke-width="2" />
                <span>{{ spinButtonText }}</span>
              </button>
            </div>

            <div
              v-if="lastResult"
              class="w-full rounded-lg border border-amber-200 bg-amber-50/80 p-4 text-center shadow-sm dark:border-amber-500/20 dark:bg-amber-500/10"
            >
              <p class="text-sm font-medium text-amber-700 dark:text-amber-200">{{ t('dailyCheckin.result') }}</p>
              <p class="mt-1 text-xl font-semibold text-gray-950 dark:text-white">{{ rewardLabel(lastResult) }}</p>
            </div>
          </div>
        </section>

        <aside class="space-y-6">
          <section class="rounded-lg border border-gray-100 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <div class="flex items-center gap-2">
              <span class="flex h-9 w-9 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-500/10 dark:text-amber-200">
                <Icon name="gift" size="sm" :stroke-width="2" />
              </span>
              <h2 class="text-base font-semibold text-gray-950 dark:text-white">{{ t('dailyCheckin.prizePool') }}</h2>
            </div>
            <div class="mt-4 space-y-3">
              <div
                v-for="prize in wheelPrizes"
                :key="prize.id"
                class="flex items-center justify-between gap-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-950/45"
              >
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-gray-950 dark:text-white">{{ prize.name }}</p>
                  <p class="text-xs text-gray-500 dark:text-dark-300">{{ prizeDescription(prize) }}</p>
                </div>
                <span class="h-2.5 w-2.5 shrink-0 rounded-full" :style="{ backgroundColor: prizeColor(prize) }"></span>
              </div>
              <div v-if="!wheelPrizes.length" class="rounded-lg border border-dashed border-gray-200 p-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-300">
                {{ t('dailyCheckin.empty') }}
              </div>
            </div>
          </section>

          <section class="rounded-lg border border-gray-100 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <h2 class="text-base font-semibold text-gray-950 dark:text-white">{{ t('dailyCheckin.recent') }}</h2>
            <div class="mt-4 space-y-3">
              <div v-if="!recentRecords.length" class="rounded-lg border border-dashed border-gray-200 p-4 text-sm text-gray-500 dark:border-dark-700 dark:text-dark-300">
                {{ t('dailyCheckin.empty') }}
              </div>
              <div
                v-for="record in recentRecords"
                :key="record.id"
                class="rounded-lg border border-gray-100 bg-gray-50 px-3 py-2.5 dark:border-dark-700 dark:bg-dark-950/45"
              >
                <div class="flex items-center justify-between gap-3">
                  <span class="truncate text-sm font-medium text-gray-950 dark:text-white">{{ rewardLabel(record) }}</span>
                  <span class="shrink-0 text-xs text-gray-500 dark:text-dark-300">{{ formatDate(record.checked_in_at) }}</span>
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
import Icon from '@/components/icons/Icon.vue'
import { redeemAPI, type DailyCheckinPrize, type DailyCheckinRecord, type DailyCheckinReward, type DailyCheckinStatus } from '@/api'
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

const WHEEL_START_DEG = -90
const WHEEL_SPIN_DURATION_MS = 3200
const WHEEL_SETTLE_BUFFER_MS = 100
const colors = ['#f59e0b', '#14b8a6', '#e11d48', '#2563eb', '#84cc16', '#a855f7', '#f97316', '#0f766e']

const wheelPrizes = computed(() => status.value?.prizes ?? [])
const recentRecords = computed(() => status.value?.recent_records ?? [])
const spinDisabled = computed(() => loading.value || spinning.value || !status.value?.enabled || status.value?.checked_in_today || wheelPrizes.value.length === 0)
const spinButtonText = computed(() => {
  if (loading.value) return t('dailyCheckin.loading')
  if (spinning.value) return t('dailyCheckin.spinning')
  if (status.value?.checked_in_today) return t('dailyCheckin.done')
  return t('dailyCheckin.spin')
})
const showPoolCard = computed(() => status.value?.decay?.exempt_reason === 'linuxdo')
const statusText = computed(() => {
  if (!status.value) return t('dailyCheckin.loading')
  if (!status.value.enabled) return t('dailyCheckin.disabled')
  if (status.value.checked_in_today) return t('dailyCheckin.doneHint')
  if (status.value.decay?.exempt_reason === 'linuxdo') return t('dailyCheckin.linuxdoExempt')
  return t('dailyCheckin.ready')
})
const wheelBackground = computed(() => {
  const prizes = wheelPrizes.value
  if (!prizes.length) return '#e5e7eb'
  const segment = 100 / prizes.length
  const stops = prizes.map((prize, index) => {
    const start = index * segment
    const end = (index + 1) * segment
    const color = prizeColor(prize)
    return `${color} ${start}% ${end}%`
  })
  return `conic-gradient(from ${WHEEL_START_DEG}deg, ${stops.join(', ')})`
})
const wheelStyle = computed(() => ({
  '--wheel-spin-duration': `${WHEEL_SPIN_DURATION_MS}ms`,
  background: wheelBackground.value,
  transform: `rotate(${wheelRotation.value}deg)`
}))

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
  lastResult.value = null
  const baseRotation = wheelRotation.value
  wheelRotation.value = baseRotation + 720
  try {
    const result = await redeemAPI.dailyCheckin()
    const prizes = result.prizes?.length ? result.prizes : wheelPrizes.value
    if (result.prizes?.length) {
      status.value = status.value ? { ...status.value, prizes: result.prizes } : status.value
    }
    wheelRotation.value = targetRotation(baseRotation, prizes, result.prize?.prize_id)
    await waitForWheelSettled()
    lastResult.value = result.prize || null
    appStore.showSuccess(t('dailyCheckin.success', { reward: rewardLabel(result.prize) }))
    await authStore.refreshUser()
    await loadStatus()
    spinning.value = false
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

function targetRotation(base: number, prizes: DailyCheckinPrize[], prizeID: string | undefined): number {
  if (!prizes.length) return base + 1440
  const index = Math.max(0, prizes.findIndex((prize) => prize.id === prizeID))
  const segmentCenter = segmentCenterAngle(index, prizes.length)
  const currentTurns = Math.ceil(base / 360)
  return currentTurns * 360 + 1440 - segmentCenter
}

function labelStyle(index: number) {
  const count = Math.max(1, wheelPrizes.value.length)
  const angle = segmentCenterAngle(index, count)
  return {
    '--label-angle': `${angle}deg`,
    '--label-counter-angle': `${-angle}deg`,
    transform: `rotate(${angle}deg)`
  }
}

function waitForWheelSettled(): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, WHEEL_SPIN_DURATION_MS + WHEEL_SETTLE_BUFFER_MS)
  })
}

function segmentCenterAngle(index: number, count: number): number {
  const segment = 360 / Math.max(1, count)
  return WHEEL_START_DEG + index * segment + segment / 2
}

function prizeColor(prize: DailyCheckinPrize): string {
  const index = wheelPrizes.value.findIndex((item) => item.id === prize.id)
  return colors[Math.max(0, index) % colors.length]
}

function prizeDescription(prize: DailyCheckinPrize): string {
  if (prize.type === 'balance') {
    if (prize.balance_mode === 'range') {
      return `${prizeTypeLabel(prize.type)} ${formatPrizeCurrency(prize.min_amount)} - ${formatPrizeCurrency(prize.max_amount)}`
    }
    return `${prizeTypeLabel(prize.type)} ${formatPrizeCurrency(prize.amount)}`
  }
  if (prize.type === 'concurrency') return `${prizeTypeLabel(prize.type)} +${prize.concurrency || 0}`
  if (prize.type === 'subscription') return `${prizeTypeLabel(prize.type)} ${prize.validity_days || 0}${t('dailyCheckin.days')}`
  return prizeTypeLabel(prize.type)
}

function prizeTypeLabel(type: string): string {
  if (type === 'concurrency') return t('dailyCheckin.types.concurrency')
  if (type === 'subscription') return t('dailyCheckin.types.subscription')
  if (type === 'none') return t('dailyCheckin.types.none')
  return t('dailyCheckin.types.balance')
}

function rewardLabel(reward: DailyCheckinReward | DailyCheckinRecord | null | undefined): string {
  if (!reward) return ''
  if (reward.type === 'balance') return `${reward.prize_name} ${formatPrizeCurrency(reward.amount)}`
  if (reward.type === 'concurrency') return `${reward.prize_name} +${reward.concurrency || 0}`
  if (reward.type === 'subscription') return `${reward.prize_name} ${reward.validity_days || 0}${t('dailyCheckin.days')}`
  return reward.prize_name || t('dailyCheckin.types.none')
}

function formatPrizeCurrency(amount: number | null | undefined): string {
  return formatCurrency(amount || 0).replace('US$', '$')
}

function formatDate(value: string): string {
  if (!value) return ''
  return new Date(value).toLocaleDateString()
}

onMounted(loadStatus)
</script>

<style scoped>
.checkin-hero {
  position: relative;
  background:
    linear-gradient(135deg, rgba(255, 251, 235, 0.94), rgba(255, 255, 255, 0.96) 42%, rgba(236, 254, 255, 0.9)),
    radial-gradient(circle at 84% 10%, rgba(245, 158, 11, 0.22), transparent 30%);
}

.dark .checkin-hero {
  background:
    linear-gradient(135deg, rgba(28, 25, 23, 0.96), rgba(15, 23, 42, 0.94) 48%, rgba(19, 78, 74, 0.62)),
    radial-gradient(circle at 84% 10%, rgba(245, 158, 11, 0.18), transparent 30%);
}

.wheel-stage {
  position: relative;
  width: min(82vw, 470px);
  aspect-ratio: 1;
  display: grid;
  place-items: center;
}

.wheel-orbit {
  position: absolute;
  inset: 6%;
  border-radius: 9999px;
  background:
    repeating-conic-gradient(from -90deg, rgba(255, 255, 255, 0.9) 0deg 2deg, transparent 2deg 12deg),
    linear-gradient(135deg, #fde68a, #f59e0b 32%, #0f766e 68%, #0f172a);
  box-shadow: 0 26px 64px rgba(15, 23, 42, 0.16), inset 0 0 0 10px rgba(255, 255, 255, 0.75);
  padding: 18px;
}

.dark .wheel-orbit {
  box-shadow: 0 28px 70px rgba(0, 0, 0, 0.36), inset 0 0 0 10px rgba(255, 255, 255, 0.08);
}

.checkin-wheel {
  position: relative;
  height: 100%;
  width: 100%;
  overflow: hidden;
  border-radius: 9999px;
  box-shadow: inset 0 0 0 8px rgba(255, 255, 255, 0.54), inset 0 0 26px rgba(15, 23, 42, 0.18);
  transition: transform 2.8s cubic-bezier(0.12, 0.68, 0.12, 1);
}

.checkin-wheel.is-spinning {
  transition-duration: var(--wheel-spin-duration, 3.2s);
}

.wheel-glass {
  pointer-events: none;
  position: absolute;
  inset: 18px;
  border-radius: 9999px;
  background:
    radial-gradient(circle at 36% 28%, rgba(255, 255, 255, 0.5), transparent 18%),
    linear-gradient(135deg, rgba(255, 255, 255, 0.28), transparent 42%, rgba(255, 255, 255, 0.16));
}

.wheel-hub {
  position: absolute;
  inset: 31%;
  display: grid;
  place-items: center;
  border-radius: 9999px;
  background: radial-gradient(circle, #fff7ed, #ffffff 62%, #fde68a);
  box-shadow: 0 16px 34px rgba(15, 23, 42, 0.18), inset 0 0 0 1px rgba(245, 158, 11, 0.28);
}

.dark .wheel-hub {
  background: radial-gradient(circle, #1f2937, #0f172a 62%, #78350f);
}

.wheel-hub-ring {
  height: 42%;
  width: 42%;
  border-radius: 9999px;
  background: linear-gradient(135deg, #f59e0b, #fef3c7 45%, #14b8a6);
  box-shadow: inset 0 0 0 6px rgba(255, 255, 255, 0.42);
}

.wheel-pointer {
  position: absolute;
  top: 0;
  left: 50%;
  z-index: 20;
  width: 68px;
  height: 74px;
  transform: translateX(-50%);
  display: grid;
  place-items: start center;
}

.wheel-pointer span {
  width: 42px;
  height: 60px;
  clip-path: polygon(50% 100%, 0 0, 100% 0);
  background: linear-gradient(180deg, #fff7ed, #f59e0b 45%, #b45309);
  filter: drop-shadow(0 8px 14px rgba(146, 64, 14, 0.35));
}

.wheel-label {
  position: absolute;
  inset: 0;
  transform-origin: center center;
  pointer-events: none;
}

.wheel-label span {
  position: absolute;
  left: 50%;
  top: 18%;
  display: -webkit-box;
  width: clamp(58px, 19%, 86px);
  max-height: 32px;
  overflow: hidden;
  transform: translate(-50%, -50%) rotate(var(--label-counter-angle));
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  border-radius: 9999px;
  background: rgba(255, 255, 255, 0.86);
  padding: 4px 7px;
  color: #111827;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.05;
  text-align: center;
  box-shadow: 0 6px 14px rgba(15, 23, 42, 0.16);
}

.dark .wheel-label span {
  background: rgba(15, 23, 42, 0.78);
  color: #f8fafc;
}

.spin-button {
  position: relative;
  z-index: 30;
  display: inline-flex;
  min-width: 118px;
  min-height: 118px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.72);
  background: linear-gradient(135deg, #111827, #0f766e 55%, #f59e0b);
  color: #fff;
  font-size: 14px;
  font-weight: 800;
  box-shadow: 0 18px 36px rgba(15, 23, 42, 0.28), inset 0 0 0 8px rgba(255, 255, 255, 0.12);
  transition: transform 0.18s ease, filter 0.18s ease, opacity 0.18s ease;
}

.spin-button:hover:not(:disabled) {
  transform: translateY(-1px) scale(1.02);
  filter: saturate(1.12);
}

.spin-button:disabled {
  cursor: not-allowed;
  opacity: 0.56;
  filter: grayscale(0.35);
}
</style>
