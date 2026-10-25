<template>
  <article
    data-test="image-group-success-rate-card"
    class="image-success-compact"
  >
    <span class="image-success-icon"><Icon name="chart" size="sm" /></span>
    <div class="min-w-0 flex-1">
      <h3>{{ item.group_name }}</h3>
      <p>{{ t('channelStatus.imageSuccessRate.groupLabel') }}</p>
    </div>
    <div class="image-success-value">
      <strong :class="rateClass">{{ formattedRate }}<small>%</small></strong>
      <p>{{ t('channelStatus.imageSuccessRate.successRate') }}</p>
    </div>
  </article>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ImageGroupSuccessRateItem } from '@/api/channelMonitor'
import Icon from '@/components/icons/Icon.vue'
const props = defineProps<{ item: ImageGroupSuccessRateItem }>()
const { t } = useI18n()
const normalizedRate = computed(() => {
  const rate = Number(props.item.success_rate)
  return Number.isFinite(rate) ? Math.min(100, Math.max(0, rate)) : 100
})
const formattedRate = computed(() => normalizedRate.value.toFixed(2))
const rateClass = computed(() =>
  normalizedRate.value >= 99
    ? 'text-emerald-600 dark:text-emerald-300'
    : normalizedRate.value >= 90
      ? 'text-amber-600 dark:text-amber-300'
      : 'text-red-600 dark:text-red-300'
)
</script>
<style scoped>
.image-success-compact {
  min-height: 74px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 13px 14px;
  border: 1px solid #e1e7ed;
  border-radius: 8px;
  background: #fff;
  color: #20313a;
}
.image-success-icon {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 6px;
  color: #cd5283;
  background: #fff0f6;
}
h3 {
  font-size: 13px;
  font-weight: 600;
  overflow-wrap: anywhere;
}
p {
  font-size: 11px;
  color: #73808a;
  margin-top: 2px;
}
.image-success-value {
  text-align: right;
  flex-shrink: 0;
}
strong {
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
small {
  font-size: 14px;
}
:global(.dark .image-success-compact) {
  background: #242c2f;
  border-color: #394346;
  color: #e2e8e6;
}
:global(.dark .image-success-compact p) {
  color: #a0adb6;
}
:global(.dark .image-success-icon) {
  background: #432a36;
  color: #f2a2c5;
}
</style>
