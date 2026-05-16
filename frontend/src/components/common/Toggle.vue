<template>
  <button
    type="button"
    @click="toggle"
    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border transition-all duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:ring-offset-2 dark:focus:ring-offset-[#0f1724]"
    :class="[modelValue ? 'border-primary-300 bg-gradient-to-r from-primary-400 to-primary-700 shadow-glow' : 'border-slate-300 bg-slate-200 dark:border-slate-600 dark:bg-slate-700']"
    role="switch"
    :aria-checked="modelValue"
  >
    <span
      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
      :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
    />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>
