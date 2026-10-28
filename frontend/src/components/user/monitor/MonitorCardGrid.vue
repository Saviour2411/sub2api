<template>
  <div>
    <div v-if="loading && !hasVisibleItems" class="monitor-platform-grid">
      <div
        v-for="i in 6"
        :key="i"
        class="h-[146px] animate-pulse rounded-lg bg-gray-100 dark:bg-dark-800"
      ></div>
    </div>
    <EmptyState
      v-else-if="!hasVisibleItems"
      :title="t('channelStatus.empty.title')"
      :description="t('channelStatus.empty.description')"
    />
    <template v-else>
      <section
        v-for="group in platformGroups"
        :key="group.provider"
        class="monitor-platform-section"
        :data-provider="group.provider"
      >
        <header class="monitor-platform-heading">
          <ProviderIcon :provider="group.provider" :size="18" :class="group.colorClass" />
          <h2 :class="group.colorClass">{{ providerLabel(group.provider) }}</h2>
          <span class="monitor-platform-rule"></span>
        </header>
        <div class="monitor-platform-grid">
          <MonitorCard
            v-for="item in group.items"
            :key="item.id"
            :item="item"
            :window="window"
            :availability-value="resolveAvailability(item)"
            :countdown-seconds="countdownSeconds"
            @click="emit('cardClick', item)"
          />
        </div>
      </section>
      <section
        v-if="visibleImageSuccessRateItems.length"
        class="monitor-platform-section"
        data-provider="image"
      >
        <header class="monitor-platform-heading">
          <Icon name="chart" size="sm" />
          <h2>Image</h2>
          <span class="monitor-platform-rule"></span>
        </header>
        <div class="monitor-platform-grid">
          <ImageGroupSuccessRateCard
            v-for="item in visibleImageSuccessRateItems"
            :key="item.group_id"
            :item="item"
          />
        </div>
      </section>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  UserMonitorView,
  UserMonitorDetail,
  ImageGroupSuccessRates
} from '@/api/channelMonitor'
import { PROVIDERS } from '@/constants/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { platformTextClass } from '@/utils/platformColors'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderIcon from './ProviderIcon.vue'
import MonitorCard from './MonitorCard.vue'
import ImageGroupSuccessRateCard from './ImageGroupSuccessRateCard.vue'
const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
  imageGroupSuccessRates: ImageGroupSuccessRates
}>()
const emit = defineEmits<{ (e: 'cardClick', item: UserMonitorView): void }>()
const { t } = useI18n()
const { providerLabel } = useChannelMonitorFormat()
const visibleImageSuccessRateItems = computed(() =>
  props.imageGroupSuccessRates.visible ? props.imageGroupSuccessRates.items : []
)
const hasVisibleItems = computed(
  () => props.items.length > 0 || visibleImageSuccessRateItems.value.length > 0
)
const platformGroups = computed(() => {
  const grouped = new Map<string, UserMonitorView[]>()
  for (const item of props.items) {
    if (!grouped.has(item.provider)) grouped.set(item.provider, [])
    grouped.get(item.provider)!.push(item)
  }
  const order = [
    ...PROVIDERS,
    ...[...grouped.keys()].filter(
      (provider) => !PROVIDERS.some((p) => p === provider)
    )
  ]
  return order
    .filter((provider) => grouped.has(provider))
    .map((provider) => ({
      provider,
      items: grouped.get(provider)!,
      colorClass: PROVIDERS.some((p) => p === provider) ? platformTextClass(provider) : ''
    }))
})
function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') return item.availability_7d ?? null
  const primary = props.detailCache[item.id]?.models.find(
    (model) => model.model === item.primary_model
  )
  return (
    (props.window === '15d'
      ? primary?.availability_15d
      : primary?.availability_30d) ?? null
  )
}
</script>
<style scoped>
.monitor-platform-section {
  margin-bottom: 19px;
}
.monitor-platform-heading {
  display: flex;
  align-items: center;
  gap: 9px;
  min-height: 24px;
  margin-bottom: 10px;
  color: #20313a;
}
.monitor-platform-heading h2 {
  font-size: 14px;
  font-weight: 600;
}
.monitor-platform-rule {
  height: 1px;
  flex: 1;
  background: #e1e7ed;
  margin-left: 4px;
}
.monitor-platform-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr));
  gap: 12px;
}
:global(.dark .monitor-platform-heading) {
  color: #e2e8e6;
}
:global(.dark .monitor-platform-rule) {
  background: #394346;
}
</style>
