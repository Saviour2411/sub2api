<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <!-- 固定区域：操作按钮 -->
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <!-- 固定区域：搜索和过滤器 -->
    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <!-- 滚动区域：表格 -->
    <div class="layout-section-scrollable">
      <div
        ref="tableScrollContainerRef"
        class="card mecha-panel table-scroll-container"
        @wheel="handleTableWheel"
      >
        <slot name="table" />
      </div>
    </div>

    <!-- 固定区域：分页器 -->
    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)
const tableScrollContainerRef = ref<HTMLElement | null>(null)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

const handleTableWheel = (event: WheelEvent) => {
  if (isMobile.value) return
  if (Math.abs(event.deltaY) <= Math.abs(event.deltaX)) return

  const tableWrapper = tableScrollContainerRef.value?.querySelector<HTMLElement>('.table-wrapper')
  if (!tableWrapper) return

  const consumeWheel = () => {
    event.preventDefault()
    event.stopPropagation()
  }

  const canScrollVertically = tableWrapper.scrollHeight > tableWrapper.clientHeight + 1
  if (!canScrollVertically) {
    consumeWheel()
    return
  }

  const deltaY = event.deltaMode === WheelEvent.DOM_DELTA_LINE
    ? event.deltaY * 16
    : event.deltaMode === WheelEvent.DOM_DELTA_PAGE
      ? event.deltaY * tableWrapper.clientHeight
      : event.deltaY
  const maxScrollTop = Math.max(0, tableWrapper.scrollHeight - tableWrapper.clientHeight)
  const nextScrollTop = Math.min(Math.max(tableWrapper.scrollTop + deltaY, 0), maxScrollTop)

  consumeWheel()
  tableWrapper.scrollTop = nextScrollTop
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
/* 桌面端：Flexbox 布局 */
.table-page-layout {
  @apply flex flex-col gap-6;
  height: calc(100dvh - 64px - 4rem); /* 减去 header + lg:p-8 的上下padding */
  max-height: calc(100dvh - 64px - 4rem);
  overflow: hidden;
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply flex-1 min-h-0 flex flex-col;
}

/* 表格滚动容器 - 增强版表体滚动方案 */
.table-scroll-container {
  @apply flex h-full flex-col overflow-hidden border border-slate-200/95 bg-white/95 shadow-sm dark:border-primary-400/25 dark:bg-[#07111d]/95;
  border-radius: 0;
  clip-path: polygon(14px 0, 100% 0, 100% calc(100% - 16px), calc(100% - 16px) 100%, 0 100%, 0 14px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    inset 0 -1px 0 rgba(75, 181, 255, 0.1),
    0 16px 42px rgba(8, 47, 88, 0.1);
}

:global(.dark) .table-scroll-container {
  background:
    linear-gradient(135deg, rgba(7, 16, 28, 0.96), rgba(3, 8, 15, 0.94)),
    linear-gradient(90deg, rgba(75, 181, 255, 0.08), transparent 44%, rgba(255, 111, 56, 0.06));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    inset 0 -1px 0 rgba(75, 181, 255, 0.14),
    0 24px 60px rgba(0, 0, 0, 0.42);
}

.table-scroll-container :deep(.table-wrapper) {
  @apply flex-1 overflow-x-auto overflow-y-auto;
  /* 确保横向滚动条显示在最底部 */
  scrollbar-gutter: stable;
  overscroll-behavior-y: contain;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content; /* 关键：确保表格宽度根据内容撑开，从而触发横向滚动 */
  display: table; /* 使用标准 table 布局以支持 sticky 列 */
}

.table-scroll-container :deep(thead) {
  @apply bg-slate-50/90 backdrop-blur-sm dark:bg-[#07111d]/95;
}

.table-scroll-container :deep(tbody) {
  /* 保持默认 table-row-group 显示，不使用 block */
}

.table-scroll-container :deep(th) {
  @apply border-b border-slate-200 px-5 py-4 text-left text-sm font-bold text-slate-600 dark:border-primary-400/25 dark:text-primary-100;
  box-shadow: inset 0 -1px 0 rgba(75, 181, 255, 0.1);
}

.table-scroll-container :deep(td) {
  @apply border-b border-slate-100/95 px-5 py-4 text-sm text-slate-700 dark:border-white/5 dark:text-slate-300;
}

.table-scroll-container :deep(tbody tr) {
  @apply transition-colors duration-150;
}

.table-scroll-container :deep(tbody tr:hover) {
  @apply bg-primary-50/70 dark:bg-primary-500/10;
  box-shadow: inset 3px 0 0 rgba(75, 181, 255, 0.72);
}

/* 移动端：恢复正常滚动 */
.table-page-layout.mobile-mode {
  height: auto;
  max-height: none;
  overflow: visible;
}

.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible border-none bg-transparent shadow-none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply flex-none min-h-fit;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(.table-wrapper) {
  @apply overflow-visible;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
