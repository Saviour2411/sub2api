<template>
  <div ref="anchorRef" class="relative inline-flex">
    <slot name="trigger" :open="open" :toggle="toggle" :close="close" />

    <Teleport to="body">
      <div
        v-if="open && panelPosition"
        ref="panelRef"
        class="column-settings-dropdown fixed z-[100000020] overflow-y-auto rounded-lg border border-gray-200 bg-white py-1 shadow-xl ring-1 ring-black/5 dark:border-dark-600 dark:bg-dark-800 dark:ring-white/10"
        :style="panelStyle"
        @click.stop
      >
        <slot :close="close" />
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  type CSSProperties,
} from 'vue'
import { getFloatingPanelPosition, type FloatingPanelPosition } from '@/utils/floatingPanel'

const props = withDefaults(defineProps<{
  width?: number
  maxHeight?: number
  viewportPadding?: number
  gap?: number
  minComfortableHeight?: number
}>(), {
  width: 192,
  maxHeight: 320,
  viewportPadding: 12,
  gap: 6,
  minComfortableHeight: 220,
})

const open = ref(false)
const anchorRef = ref<HTMLElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const panelPosition = ref<FloatingPanelPosition | null>(null)

const panelStyle = computed<CSSProperties>(() => {
  const position = panelPosition.value
  if (!position) return {}

  return {
    top: position.top == null ? 'auto' : `${position.top}px`,
    bottom: position.bottom == null ? 'auto' : `${position.bottom}px`,
    left: `${position.left}px`,
    width: `${position.width}px`,
    maxHeight: `${position.maxHeight}px`,
  }
})

const updatePosition = () => {
  const anchor = anchorRef.value
  if (!anchor) return

  const position = getFloatingPanelPosition(
    anchor.getBoundingClientRect(),
    document.documentElement.clientWidth || window.innerWidth,
    window.innerHeight,
    {
      viewportPadding: props.viewportPadding,
      gap: props.gap,
      maxWidth: props.width,
      minComfortableHeight: props.minComfortableHeight,
    },
  )

  panelPosition.value = {
    ...position,
    maxHeight: Math.min(props.maxHeight, position.maxHeight),
  }
}

const close = () => {
  open.value = false
  panelPosition.value = null
}

const toggle = async () => {
  if (open.value) {
    close()
    return
  }

  open.value = true
  updatePosition()
  await nextTick()
  updatePosition()
}

const handleDocumentClick = (event: MouseEvent) => {
  if (!open.value) return

  const target = event.target as Node
  if (anchorRef.value?.contains(target) || panelRef.value?.contains(target)) return
  close()
}

const handleDocumentKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') close()
}

const handleViewportChange = () => {
  if (open.value) updatePosition()
}

onMounted(() => {
  document.addEventListener('click', handleDocumentClick)
  document.addEventListener('keydown', handleDocumentKeydown)
  window.addEventListener('resize', handleViewportChange)
  window.addEventListener('scroll', handleViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleDocumentClick)
  document.removeEventListener('keydown', handleDocumentKeydown)
  window.removeEventListener('resize', handleViewportChange)
  window.removeEventListener('scroll', handleViewportChange, true)
})

defineExpose({ close, open, updatePosition })
</script>
