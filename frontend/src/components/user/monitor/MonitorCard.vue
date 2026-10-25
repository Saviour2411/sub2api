<template>
  <button type="button" class="monitor-compact-card" @click="emit('click')">
    <div class="monitor-compact-heading">
      <div class="min-w-0 flex-1">
        <div class="monitor-compact-name">{{ item.name }}</div>
        <div class="monitor-compact-model">
          <span>{{ formatMonitorModel(item.primary_model) }}</span>
          <span v-if="item.group_name" class="monitor-compact-group">{{
            item.group_name
          }}</span>
        </div>
      </div>
      <span
        class="monitor-compact-status"
        :class="statusBadgeClass(displayStatus)"
        >{{ statusLabel(displayStatus) }}</span
      >
    </div>
    <div class="monitor-compact-metrics">
      <div>
        <small>{{ t('monitorCommon.dialogLatency') }}</small
        ><strong>{{
          formatLatency(item.stale ? null : item.primary_latency_ms)
        }}</strong
        ><em>ms</em>
      </div>
      <div>
        <small>{{ t('monitorCommon.endpointPing') }}</small
        ><strong>{{
          formatLatency(item.stale ? null : item.primary_ping_latency_ms)
        }}</strong
        ><em>ms</em>
      </div>
      <div class="monitor-compact-availability">
        <small>{{ availabilityLabel }}</small
        ><strong :style="availabilityStyle">{{ availabilityText }}</strong
        ><em v-if="availabilityValue !== null">%</em>
      </div>
    </div>
    <MonitorQuotaView v-if="quotaVisible" :snapshot="item.latest_quota" />
    <MonitorTimeline
      :buckets="item.timeline"
      :stale="item.stale"
      :countdown-seconds="countdownSeconds"
      compact
    />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import {
  hslForPct,
  useChannelMonitorFormat
} from '@/composables/useChannelMonitorFormat'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import MonitorTimeline from './MonitorTimeline.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  sampleCount?: number
  countdownSeconds: number
}>()
const emit = defineEmits<{ (e: 'click'): void }>()
const { t } = useI18n()
const { statusLabel, statusBadgeClass, formatLatency, formatMonitorModel } =
  useChannelMonitorFormat()
const displayStatus = computed(() =>
  props.item.stale ? 'stale' : props.item.primary_status
)
const quotaVisible = computed(
  () =>
    !props.item.stale &&
    isChannelMonitorQuotaVisible() &&
    !!props.item.latest_quota
)
const availabilityLabel = computed(
  () =>
    `${t('monitorCommon.availabilityPrefix')} · ${t(`channelStatus.windowTab.${props.window}`)}`
)
const availabilityText = computed(() =>
  props.availabilityValue === null || !Number.isFinite(props.availabilityValue)
    ? t('monitorCommon.noSamples')
    : props.availabilityValue.toFixed(2)
)
const availabilityStyle = computed(() => ({
  color: hslForPct(props.availabilityValue) || 'rgb(156 163 175)'
}))
</script>

<style scoped>
.monitor-compact-card {
  width: 100%;
  min-width: 0;
  min-height: 146px;
  padding: 13px 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 13px;
  text-align: left;
  border: 1px solid #e1e7ed;
  border-radius: 8px;
  background: #fff;
  color: #20313a;
  transition: border-color 0.15s;
}
.monitor-compact-card:hover {
  border-color: #168477;
}
.monitor-compact-card:focus-visible {
  outline: 2px solid #168477;
  outline-offset: 3px;
}
.monitor-compact-heading {
  display: flex;
  gap: 9px;
  align-items: flex-start;
}
.monitor-compact-name {
  font-size: 13px;
  font-weight: 600;
  line-height: 20px;
  overflow-wrap: anywhere;
}
.monitor-compact-model {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 8px;
  font-size: 11px;
  line-height: 16px;
  color: #73808a;
  overflow-wrap: anywhere;
}
.monitor-compact-group {
  padding: 0 4px;
  border-radius: 3px;
  background: #f3f6f7;
}
.monitor-compact-status {
  flex-shrink: 0;
  font-size: 11px;
  padding: 3px 7px;
  border-radius: 4px;
}
.monitor-compact-metrics {
  display: grid;
  grid-template-columns: 1fr 0.82fr 1fr;
  gap: 8px;
  margin-top: auto;
}
.monitor-compact-metrics > div {
  min-width: 0;
}
.monitor-compact-metrics > div + div {
  border-left: 1px solid #e1e7ed;
  padding-left: 10px;
}
.monitor-compact-metrics small {
  display: block;
  font-size: 11px;
  color: #73808a;
  line-height: 16px;
  margin-bottom: 3px;
}
.monitor-compact-metrics strong {
  font-size: 17px;
  line-height: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.monitor-compact-metrics em {
  font-size: 11px;
  font-style: normal;
  color: #73808a;
  margin-left: 3px;
}
.monitor-compact-availability {
  text-align: right;
}
:global(.dark .monitor-compact-card) {
  background: #242c2f;
  border-color: #394346;
  color: #e2e8e6;
}
:global(.dark .monitor-compact-metrics > div + div) {
  border-color: #394346;
}
:global(.dark .monitor-compact-model),
:global(.dark .monitor-compact-metrics small),
:global(.dark .monitor-compact-metrics em) {
  color: #a0adb6;
}
:global(.dark .monitor-compact-group) {
  background: #2c3639;
}
</style>
