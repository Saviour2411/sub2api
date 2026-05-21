<template>
  <div class="scanline-sweep" :data-orientation="orientation" aria-hidden="true">
    <span
      :class="orientation === 'horizontal' ? 'scan-beam' : 'scan-beam-y'"
      :style="style"
    ></span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  /** Cycle duration in seconds (default 4) */
  duration?: number
  /** Animation start delay in seconds (default 0) */
  delay?: number
  /** Sweep direction (default 'horizontal') */
  orientation?: 'horizontal' | 'vertical'
}

const props = withDefaults(defineProps<Props>(), {
  duration: 4,
  delay: 0,
  orientation: 'horizontal'
})

const style = computed(() => ({
  '--scan-duration': `${props.duration}s`,
  '--scan-delay': `${props.delay}s`
}))
</script>

<style scoped>
.scanline-sweep {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}
</style>
