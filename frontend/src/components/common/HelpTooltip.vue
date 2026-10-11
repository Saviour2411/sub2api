<script setup lang="ts">
import { inject, onBeforeUnmount, onMounted, provide, ref, useId, useTemplateRef, nextTick, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'

const props = withDefaults(defineProps<{
  content?: string
  trigger?: 'hover' | 'click'
  widthClass?: string
  disabled?: boolean
}>(), {
  trigger: 'hover',
  widthClass: 'w-64',
})

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
const contentRef = useTemplateRef<HTMLElement>('content')
const tooltipId = useId()
const tooltipDepth = inject('sub2api-help-tooltip-depth', 0)
provide('sub2api-help-tooltip-depth', tooltipDepth + 1)
const tooltipStyle = ref({ top: '0px', left: '0px' })
const placement = ref<'top' | 'bottom'>('top')
const maxHeight = ref('calc(100vh - 16px)')
const arrowLeft = ref('50%')

function openTooltip() {
  if (props.disabled) return
  show.value = true
  nextTick(updatePosition)
}

function closeTooltip() {
  show.value = false
}

function onEnter() {
  if (props.trigger !== 'hover') return
  openTooltip()
}

function isInside(container: HTMLElement | null, target: EventTarget | null): boolean {
  return target instanceof Node && !!container?.contains(target)
}

// 悬停模式下指针在触发图标与提示框之间往返时保持打开，便于选中提示里的文字。
function onLeave(event: MouseEvent) {
  if (props.trigger !== 'hover') return
  if (isInside(tooltipRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onTooltipLeave(event: MouseEvent) {
  if (props.trigger !== 'hover') return
  if (isInside(triggerRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onClick(event: MouseEvent) {
  if (props.trigger !== 'click') return
  event.stopPropagation()
  if (show.value) {
    closeTooltip()
    return
  }
  openTooltip()
}

function onDocumentClick(event: MouseEvent) {
  if (props.trigger !== 'click' || !show.value) return
  const target = event.target as Node | null
  if (!target) return
  if (triggerRef.value?.contains(target) || tooltipRef.value?.contains(target)) return
  closeTooltip()
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeTooltip()
  }
}

function onFocusOut(event: FocusEvent) {
  if (props.trigger !== 'hover') return
  if (isInside(triggerRef.value, event.relatedTarget) || isInside(tooltipRef.value, event.relatedTarget)) return
  closeTooltip()
}

function onViewportChange() {
  if (!show.value) return
  updatePosition()
}

function updatePosition() {
  const el = triggerRef.value
  const tooltip = tooltipRef.value
  const content = contentRef.value
  if (!show.value || !el || !tooltip || !content) return
  const rect = el.getBoundingClientRect()
  const width = tooltip.getBoundingClientRect().width
  const viewportWidth = document.documentElement.clientWidth || window.innerWidth
  const viewportHeight = window.innerHeight
  const padding = 8
  const gap = 8
  const spaceAbove = Math.max(0, rect.top - gap - padding)
  const spaceBelow = Math.max(0, viewportHeight - rect.bottom - gap - padding)
  const openAbove = content.scrollHeight <= spaceAbove || spaceAbove >= spaceBelow
  const availableHeight = openAbove ? spaceAbove : spaceBelow
  const height = Math.min(content.scrollHeight, availableHeight)
  const center = rect.left + rect.width / 2
  const left = Math.max(padding, Math.min(center - width / 2, viewportWidth - width - padding))

  // fixed 定位使用视口坐标；浮层脱离卡片裁切，并根据剩余空间翻转、收窄和滚动。
  placement.value = openAbove ? 'top' : 'bottom'
  maxHeight.value = `${availableHeight}px`
  arrowLeft.value = `${Math.max(12, Math.min(center - left, width - 12))}px`
  tooltipStyle.value = {
    top: `${Math.max(padding, openAbove ? rect.top - gap - height : rect.bottom + gap)}px`,
    left: `${left}px`,
  }
}

useResizeObserver(tooltipRef, onViewportChange)
watch(() => props.disabled, disabled => {
  if (disabled) closeTooltip()
})

onMounted(() => {
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onDocumentKeydown)
  window.addEventListener('resize', onViewportChange)
  window.addEventListener('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onDocumentKeydown)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})
</script>

<template>
  <span
    ref="trigger"
    class="group relative ml-1 inline-flex items-center align-middle"
    :tabindex="props.trigger === 'hover' ? 0 : undefined"
    :aria-describedby="show ? tooltipId : undefined"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
    @click="onClick"
    @focusin="onEnter"
    @focusout="onFocusOut"
  >
    <!-- 触发图标 -->
    <slot name="trigger">
      <svg
        class="h-4 w-4 cursor-help text-gray-400 transition-colors hover:text-primary-600 dark:text-gray-500 dark:hover:text-primary-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    </slot>

    <!-- 挂载到页面根节点，避免被卡片、弹窗和表格的裁切区域遮挡。 -->
    <Teleport to="body">
      <!-- 透明桥接区域覆盖图标与提示框之间的空隙，便于移入提示框选中文字。 -->
      <div
        ref="tooltip"
        :id="tooltipId"
        v-show="show"
        role="tooltip"
        :class="[
          'fixed z-[99999] max-w-[calc(100vw-16px)] rounded-lg bg-gray-900 text-left text-xs font-normal leading-relaxed text-white shadow-xl ring-1 ring-white/10 selection:bg-primary-200 selection:text-gray-900 before:absolute before:inset-x-0 before:h-3 dark:bg-gray-800 dark:selection:bg-primary-200 dark:selection:text-gray-900',
          placement === 'top' ? 'before:top-full' : 'before:bottom-full',
          props.widthClass,
        ]"
        :style="[tooltipStyle, { zIndex: 99999 + tooltipDepth }]"
        @mouseleave="onTooltipLeave"
        @focusout="onFocusOut"
      >
        <div ref="content" class="overflow-y-auto whitespace-normal break-words p-3" :class="{ 'pr-8': props.trigger === 'click' }" :style="{ maxHeight }">
          <button
            v-if="props.trigger === 'click'"
            type="button"
            class="absolute right-1.5 top-1.5 rounded p-1 text-gray-300 transition-colors hover:bg-white/10 hover:text-white"
            aria-label="Close"
            @click.stop="closeTooltip"
          >
            <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <slot>{{ content }}</slot>
        </div>
        <div
          class="pointer-events-none absolute h-2 w-2 -translate-x-1/2 rotate-45 bg-gray-900 dark:bg-gray-800"
          :class="placement === 'top' ? '-bottom-1' : '-top-1'"
          :style="{ left: arrowLeft }"
        ></div>
      </div>
    </Teleport>
  </span>
</template>
