<template>
  <AppLayout fullscreen>
    <div class="relative h-full min-h-0 overflow-hidden bg-white dark:bg-[#11100f]">
      <iframe
        :key="frameKey"
        ref="frameRef"
        :src="frameSrc"
        :title="t('canvas.title')"
        class="h-full w-full border-0"
        sandbox="allow-scripts allow-same-origin allow-downloads allow-forms"
        allow="clipboard-read; clipboard-write"
        @load="handleFrameLoad"
        @error="handleFrameError"
      />

      <div
        v-if="loading || errorMessage"
        class="absolute inset-0 flex items-center justify-center bg-white/95 dark:bg-[#11100f]/95"
      >
        <div class="flex flex-col items-center gap-4 text-center">
          <div v-if="loading && !errorMessage" class="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500"></div>
          <p class="text-sm text-gray-600 dark:text-gray-300">
            {{ errorMessage || t('canvas.loading') }}
          </p>
          <button
            v-if="errorMessage"
            type="button"
            class="btn btn-secondary inline-flex items-center gap-2"
            @click="reloadFrame"
          >
            <Icon name="refresh" size="sm" />
            {{ t('canvas.reload') }}
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { getCanvasBootstrap, resolveCanvasCredential } from '@/api/canvas'
import { getLocale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isTrustedCanvasMessage } from '@/utils/canvasBridge'

type CanvasRequestMessage =
  | { type: 'canvas:ready' }
  | { type: 'canvas:resolve-credential'; request_id: string; group_id: number }

const { t } = useI18n()
const authStore = useAuthStore()
const frameRef = ref<HTMLIFrameElement | null>(null)
const frameKey = ref(0)
const loading = ref(true)
const errorMessage = ref('')
const frameSrc = computed(() => `/canvas-app/canvas?host=${frameKey.value}`)
let loadTimer: ReturnType<typeof setTimeout> | null = null
let documentObserver: MutationObserver | null = null

function currentContext() {
  return {
    locale: getLocale(),
    theme: document.documentElement.classList.contains('dark') ? 'dark' : 'light'
  } as const
}

function postToCanvas(message: unknown) {
  frameRef.value?.contentWindow?.postMessage(message, window.location.origin)
}

async function initializeCanvas() {
  try {
    const bootstrap = await getCanvasBootstrap()
    postToCanvas({ type: 'canvas:init', bootstrap, ...currentContext() })
    errorMessage.value = ''
    loading.value = false
    clearLoadTimer()
  } catch (error) {
    loading.value = false
    errorMessage.value = extractApiErrorMessage(error, t('canvas.loadFailed'))
  }
}

async function handleCanvasMessage(event: MessageEvent<CanvasRequestMessage>) {
  const target = frameRef.value?.contentWindow
  if (!isTrustedCanvasMessage(event, target || null)) return
  if (!event.data || typeof event.data !== 'object') return

  if (event.data.type === 'canvas:ready') {
    await initializeCanvas()
    return
  }

  if (event.data.type === 'canvas:resolve-credential') {
    const requestId = String(event.data.request_id || '')
    const groupId = Number(event.data.group_id)
    if (!requestId || !Number.isInteger(groupId) || groupId <= 0) return
    try {
      const credential = await resolveCanvasCredential(groupId)
      postToCanvas({ type: 'canvas:credential', request_id: requestId, credential })
    } catch (error) {
      postToCanvas({
        type: 'canvas:credential',
        request_id: requestId,
        error: extractApiErrorMessage(error, t('canvas.credentialFailed'))
      })
    }
  }
}

function syncContext() {
  postToCanvas({ type: 'canvas:context', ...currentContext() })
}

function clearLoadTimer() {
  if (loadTimer) clearTimeout(loadTimer)
  loadTimer = null
}

function startLoadTimer() {
  clearLoadTimer()
  loadTimer = setTimeout(() => {
    if (loading.value) {
      loading.value = false
      errorMessage.value = t('canvas.loadTimeout')
    }
  }, 15000)
}

function reloadFrame() {
  loading.value = true
  errorMessage.value = ''
  frameKey.value += 1
  startLoadTimer()
}

function handleFrameLoad() {
  // 等待子应用发送 canvas:ready，避免把静态资源加载完成误认为初始化完成。
}

function handleFrameError() {
  clearLoadTimer()
  loading.value = false
  errorMessage.value = t('canvas.loadFailed')
}

watch(
  () => authStore.user?.id,
  (next, previous) => {
    if (previous !== undefined && next !== previous) reloadFrame()
  }
)

onMounted(() => {
  window.addEventListener('message', handleCanvasMessage)
  documentObserver = new MutationObserver(syncContext)
  documentObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'lang'] })
  startLoadTimer()
})

onBeforeUnmount(() => {
  clearLoadTimer()
  window.removeEventListener('message', handleCanvasMessage)
  documentObserver?.disconnect()
  documentObserver = null
})
</script>
