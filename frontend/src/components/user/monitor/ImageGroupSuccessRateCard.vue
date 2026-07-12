<template>
  <article
    data-test="image-group-success-rate-card"
    class="flex min-h-[280px] w-full flex-col rounded-lg border border-gray-200/80 bg-white/70 p-5 shadow-card backdrop-blur-xl dark:border-dark-700/70 dark:bg-dark-800/60"
  >
    <div class="flex items-start gap-3">
      <span class="grid h-9 w-9 flex-shrink-0 place-items-center rounded-lg bg-rose-50 text-rose-600 ring-1 ring-rose-100 dark:bg-rose-900/20 dark:text-rose-300 dark:ring-rose-800/50">
        <Icon name="chart" size="sm" />
      </span>
      <div class="min-w-0 flex-1">
        <h3 class="truncate text-base font-semibold text-gray-900 dark:text-gray-100">
          {{ item.group_name }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('channelStatus.imageSuccessRate.groupLabel') }}
        </p>
      </div>
    </div>

    <div class="flex flex-1 flex-col items-center justify-center py-6 text-center">
      <div class="text-4xl font-semibold tabular-nums" :class="rateClass">
        {{ formattedRate }}%
      </div>
      <p class="mt-2 text-sm font-medium text-gray-600 dark:text-gray-300">
        {{ t('channelStatus.imageSuccessRate.successRate') }}
      </p>
    </div>

    <div class="border-t border-gray-100 pt-4 dark:border-dark-700/60">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('channelStatus.imageSuccessRate.lastSuccess') }}
      </p>
      <p class="mt-1 truncate text-sm font-medium text-gray-800 dark:text-gray-200" :title="lastSuccessText">
        {{ lastSuccessText }}
      </p>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageGroupSuccessRateItem } from '@/api/channelMonitor'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  item: ImageGroupSuccessRateItem
}>()

const { t } = useI18n()

const normalizedRate = computed(() => {
  const rate = Number(props.item.success_rate)
  if (!Number.isFinite(rate)) return 100
  return Math.min(100, Math.max(0, rate))
})

const formattedRate = computed(() => normalizedRate.value.toFixed(2))
const rateClass = computed(() => {
  if (normalizedRate.value >= 99) return 'text-emerald-600 dark:text-emerald-300'
  if (normalizedRate.value >= 90) return 'text-amber-600 dark:text-amber-300'
  return 'text-red-600 dark:text-red-300'
})
const lastSuccessText = computed(() =>
  props.item.last_success_at
    ? formatDateTime(props.item.last_success_at)
    : t('channelStatus.imageSuccessRate.noSuccessfulRequest')
)
</script>
