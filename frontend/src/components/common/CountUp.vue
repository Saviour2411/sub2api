<template>
  <span ref="rootEl" class="count-up tabular-nums" v-bind="$attrs">
    <span v-if="prefix" class="count-up-prefix">{{ prefix }}</span>
    <span class="count-up-value">{{ display }}</span>
    <span v-if="suffix" class="count-up-suffix">{{ suffix }}</span>
  </span>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

interface Props {
  /** Target numeric value */
  value: number | string | undefined | null
  /** Decimals to display (default 0) */
  decimals?: number
  /** Animation duration in ms (default 900) */
  duration?: number
  /** Prefix text (e.g. "$") */
  prefix?: string
  /** Suffix text (e.g. "%") */
  suffix?: string
  /** Group thousands with comma (default true) */
  group?: boolean
  /** Wait until the element scrolls into view (default true) */
  triggerOnView?: boolean
  /** Re-run animation when value changes (default true) */
  reanimateOnChange?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  decimals: 0,
  duration: 900,
  prefix: '',
  suffix: '',
  group: true,
  triggerOnView: true,
  reanimateOnChange: true
})

defineOptions({ inheritAttrs: false })

const rootEl = ref<HTMLElement | null>(null)
const display = ref<string>('')
let rafId = 0
let started = false
let observer: IntersectionObserver | null = null

function toNumber(v: number | string | null | undefined): number {
  if (v === null || v === undefined || v === '') return 0
  const n = typeof v === 'number' ? v : parseFloat(v)
  return Number.isFinite(n) ? n : 0
}

function reduced() {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
}

function format(n: number) {
  const fixed = n.toFixed(props.decimals)
  if (!props.group) return fixed
  const [intPart, decPart] = fixed.split('.')
  const grouped = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return decPart === undefined ? grouped : `${grouped}.${decPart}`
}

function easeOutCubic(t: number) {
  return 1 - Math.pow(1 - t, 3)
}

function snapTo(n: number) {
  display.value = format(n)
}

function animate(from: number, to: number) {
  if (rafId) cancelAnimationFrame(rafId)
  if (reduced() || props.duration <= 0) {
    snapTo(to)
    return
  }
  const start = performance.now()
  const delta = to - from
  const tick = (now: number) => {
    const t = Math.min(1, (now - start) / props.duration)
    const eased = easeOutCubic(t)
    snapTo(from + delta * eased)
    if (t < 1) {
      rafId = requestAnimationFrame(tick)
    } else {
      rafId = 0
    }
  }
  rafId = requestAnimationFrame(tick)
}

function start(fromZero = false) {
  const target = toNumber(props.value)
  const from = fromZero ? 0 : toNumber(display.value.replace(/[^0-9.\-]/g, ''))
  animate(from, target)
}

function ensureStarted() {
  if (started) return
  started = true
  start(true)
}

onMounted(() => {
  // Initial display: snap to current value so SSR / first paint isn't blank
  snapTo(toNumber(props.value))

  if (!props.triggerOnView) {
    ensureStarted()
    return
  }

  if (typeof IntersectionObserver === 'undefined') {
    ensureStarted()
    return
  }

  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          ensureStarted()
          observer?.disconnect()
          observer = null
          break
        }
      }
    },
    { threshold: 0.2 }
  )
  if (rootEl.value) observer.observe(rootEl.value)
})

watch(
  () => props.value,
  () => {
    if (!started) {
      snapTo(toNumber(props.value))
      return
    }
    if (props.reanimateOnChange) start(false)
    else snapTo(toNumber(props.value))
  }
)

onBeforeUnmount(() => {
  if (rafId) cancelAnimationFrame(rafId)
  observer?.disconnect()
  observer = null
})
</script>

<style scoped>
.count-up {
  font-variant-numeric: tabular-nums;
  display: inline-flex;
  align-items: baseline;
  gap: 0.05em;
}

.count-up-value {
  font-feature-settings: 'tnum' 1, 'lnum' 1;
}
</style>
