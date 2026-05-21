<template>
  <div
    ref="rootEl"
    class="stat-card stat-card-mecha scan-host"
    :class="{ 'tilt-surface tilt-glow': !disableTilt }"
  >
    <div :class="['stat-icon', iconClass]">
      <component v-if="icon" :is="icon" class="h-6 w-6" aria-hidden="true" />
    </div>
    <div class="min-w-0 flex-1">
      <div class="flex items-center justify-between gap-2">
        <p class="stat-label truncate">{{ title }}</p>
        <PulseDot v-if="live" :tone="pulseTone" :title="liveTitle" />
      </div>
      <div class="mt-1 flex items-baseline gap-2">
        <CountUp
          v-if="numericValue !== null && !props.formatValue"
          class="stat-value"
          :value="numericValue"
          :decimals="decimals"
          :duration="900"
        />
        <p v-else class="stat-value" :title="String(formattedValue)">{{ formattedValue }}</p>
        <span v-if="change !== undefined" :class="['stat-trend', trendClass]">
          <Icon
            v-if="changeType !== 'neutral'"
            name="arrowUp"
            size="xs"
            :class="changeType === 'down' && 'rotate-180'"
          />
          {{ formattedChange }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import CountUp from './CountUp.vue'
import PulseDot from './PulseDot.vue'
import { useTilt } from '@/composables/useTilt'

type ChangeType = 'up' | 'down' | 'neutral'
type IconVariant = 'primary' | 'success' | 'warning' | 'danger'
type PulseTone = 'primary' | 'success' | 'warning' | 'danger' | 'neutral'

interface Props {
  title: string
  value: number | string
  icon?: Component
  iconVariant?: IconVariant
  change?: number
  changeType?: ChangeType
  formatValue?: (value: number | string) => string
  /** Show animated pulse dot (string title or true for default LIVE) */
  live?: boolean | string
  /** Pulse tone (default success) */
  pulseTone?: PulseTone
  /** Number of decimal places when value is numeric (default 0) */
  decimals?: number
  /** Disable mouse-driven 3D tilt (default false) */
  disableTilt?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  changeType: 'neutral',
  iconVariant: 'primary',
  live: false,
  pulseTone: 'success',
  decimals: 0,
  disableTilt: false
})

const rootEl = ref<HTMLElement | null>(null)
if (!props.disableTilt) {
  useTilt(rootEl, { max: 5, lift: 4, spotlight: true })
}

const liveTitle = computed<string>(() => {
  if (typeof props.live === 'string') return props.live
  return 'LIVE'
})

const numericValue = computed<number | null>(() => {
  if (typeof props.value === 'number' && Number.isFinite(props.value)) return props.value
  if (typeof props.value === 'string') {
    const parsed = parseFloat(props.value.replace(/[, $%]/g, ''))
    if (Number.isFinite(parsed) && /^[\d,.\-+\s$%]+$/.test(props.value)) return parsed
  }
  return null
})

const formattedValue = computed(() => {
  if (props.formatValue) {
    return props.formatValue(props.value)
  }
  if (typeof props.value === 'number') {
    return props.value.toLocaleString()
  }
  return props.value
})

const formattedChange = computed(() => {
  if (props.change === undefined) return ''
  const absChange = Math.abs(props.change)
  return `${absChange}%`
})

const iconClass = computed(() => {
  const classes: Record<IconVariant, string> = {
    primary: 'stat-icon-primary',
    success: 'stat-icon-success',
    warning: 'stat-icon-warning',
    danger: 'stat-icon-danger'
  }
  return classes[props.iconVariant]
})

const trendClass = computed(() => {
  const classes: Record<ChangeType, string> = {
    up: 'stat-trend-up',
    down: 'stat-trend-down',
    neutral: 'text-gray-500 dark:text-dark-400'
  }
  return classes[props.changeType]
})
</script>

<style scoped>
.stat-card-mecha {
  position: relative;
  transition: transform 320ms ease, box-shadow 320ms ease;
  will-change: transform;
}

.stat-card-mecha::before {
  content: '';
  position: absolute;
  inset: -1px;
  pointer-events: none;
  background: linear-gradient(
    135deg,
    rgba(75, 181, 255, 0.42),
    transparent 30%,
    transparent 70%,
    rgba(255, 111, 56, 0.32)
  );
  opacity: 0;
  transition: opacity 320ms ease;
  mix-blend-mode: screen;
}

.stat-card-mecha.is-tilting::before {
  opacity: 0.6;
}
</style>
