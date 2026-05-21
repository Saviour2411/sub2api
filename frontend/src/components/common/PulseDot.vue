<template>
  <span
    class="pulse-dot"
    :class="toneClass"
    :title="title"
    :aria-label="ariaLabel || title"
    role="status"
  >
    <span class="pulse-dot-ring" aria-hidden="true"></span>
    <span class="pulse-dot-ring pulse-dot-ring-2" aria-hidden="true"></span>
    <span class="pulse-dot-core" aria-hidden="true"></span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

type Tone = 'primary' | 'success' | 'warning' | 'danger' | 'neutral'

interface Props {
  tone?: Tone
  title?: string
  ariaLabel?: string
}

const props = withDefaults(defineProps<Props>(), {
  tone: 'success',
  title: 'LIVE',
  ariaLabel: ''
})

const toneClass = computed(() => {
  const map: Record<Tone, string> = {
    primary: 'text-primary-500 dark:text-primary-300',
    success: 'text-emerald-500 dark:text-emerald-300',
    warning: 'text-amber-500 dark:text-amber-300',
    danger: 'text-rose-500 dark:text-rose-300',
    neutral: 'text-slate-400 dark:text-slate-500'
  }
  return map[props.tone]
})
</script>
