<template>
  <div
    ref="host"
    class="quality-preview"
    :class="{ 'quality-preview-interactive': interactive }"
  >
    <iframe
      v-if="visible && !failed"
      :key="frameVersion"
      ref="frame"
      :src="buildApiUrl('/model-quality/preview')"
      :style="frameStyle"
      sandbox="allow-scripts"
      referrerpolicy="no-referrer"
      title="鹈鹕作品动画"
    />
    <div
      v-if="failed || !content || !ready"
      class="quality-preview-placeholder"
      role="status"
    >
      <Icon :name="failed ? 'exclamationCircle' : 'refresh'" size="lg" />
      <span>{{ failed ? '作品已过期或暂不可用' : '作品加载中' }}</span>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { buildApiUrl } from '@/api/url'
import { useModelQualityStore } from '@/stores/modelQuality'
import {
  isQualityAbort,
  scheduleQualityExpiry
} from '@/utils/modelQualityLoader'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  runId: number
  html?: string
  expiresAt?: string
  version?: string
  interactive?: boolean
}>()
const { loader } = useModelQualityStore()
const host = ref<HTMLElement>()
const frame = ref<HTMLIFrameElement>()
const visible = ref(false),
  content = ref(''),
  failed = ref(false),
  ready = ref(false)
const frameVersion = ref(0)
const dimensions = ref({ width: 1000, height: 625, scale: 1 })
const frameStyle = computed(() => ({
  width: `${dimensions.value.width}px`,
  height: `${dimensions.value.height}px`,
  transform: `scale(${dimensions.value.scale})`,
  transformOrigin: 'top left'
}))
let visibilityObserver: IntersectionObserver | undefined,
  preloadObserver: IntersectionObserver | undefined
let resizeObserver: ResizeObserver | undefined,
  controller: AbortController | undefined
let cancelExpiry: (() => void) | undefined
let inViewport = false,
  nearViewport = false,
  blocked = false,
  disposed = false,
  delivered = false
let generation = 0
let contentExpires = 0

function reset(block = false) {
  generation++
  controller?.abort()
  controller = undefined
  cancelExpiry?.()
  content.value = ''
  ready.value = false
  delivered = false
  visible.value = false
  failed.value = block
  blocked = block
  contentExpires = 0
  frameVersion.value++
}
function setExpiry(value?: string) {
  cancelExpiry?.()
  contentExpires = value ? Date.parse(value) : 0
  if (!contentExpires) return true
  if (contentExpires <= Date.now()) {
    loader.invalidate(props.runId, 'expired')
    return false
  }
  cancelExpiry = scheduleQualityExpiry(value!, () =>
    loader.invalidate(props.runId, 'expired')
  )
  return true
}
function send() {
  if (
    ready.value &&
    content.value &&
    visible.value &&
    !delivered &&
    (!contentExpires || contentExpires > Date.now())
  ) {
    frame.value?.contentWindow?.postMessage(
      { type: 'quality-artwork', html: content.value },
      '*'
    )
    delivered = true
  }
}
function receive(event: MessageEvent) {
  if (
    event.source !== frame.value?.contentWindow ||
    event.data?.type !== 'quality-ready'
  )
    return
  ready.value = true
  send()
}
async function ensureContent() {
  if (blocked || disposed || document.hidden || (!nearViewport && !inViewport))
    return
  if (content.value) {
    send()
    return
  }
  if (controller) {
    if (inViewport) loader.prioritize(props.runId)
    return
  }
  if (!setExpiry(props.expiresAt)) return
  failed.value = false
  if (props.html) {
    if (props.html.length > 5242880) {
      failed.value = true
      return
    }
    if (visible.value) content.value = props.html
    send()
    return
  }
  const current = generation
  const request = new AbortController()
  controller = request
  try {
    const run = await loader.load(props.runId, {
      signal: request.signal,
      priority: props.interactive
        ? 'detail'
        : inViewport
          ? 'visible'
          : 'prefetch'
    })
    if (current !== generation || disposed || request.signal.aborted) return
    if (!setExpiry(run.expires_at)) return
    const html =
      run.tests.find((test) => test.benchmark === 'pelican')?.html || ''
    if (!html || html.length > 5242880) {
      failed.value = true
      return
    }
    if (visible.value) content.value = html
    send()
  } catch (error) {
    if (current === generation && !isQualityAbort(error)) failed.value = true
  } finally {
    if (controller === request) controller = undefined
  }
}
function updateVisibility() {
  const next = inViewport && !document.hidden && !blocked
  if (next !== visible.value) {
    ready.value = false
    delivered = false
    visible.value = next
    if (!next) content.value = ''
  }
  if (document.hidden || (!nearViewport && !inViewport)) {
    controller?.abort()
    controller = undefined
  } else void ensureContent()
}
const unsubscribe = loader.subscribe((id, reason) => {
  if (id !== null && id !== props.runId) return
  reset(reason !== 'changed')
  if (reason === 'changed') updateVisibility()
})
watch(
  () => [props.runId, props.version, props.html, props.expiresAt],
  () => {
    reset()
    updateVisibility()
  }
)
onMounted(() => {
  resizeObserver = new ResizeObserver((entries) => {
    const rect = entries[0]?.contentRect
    if (!rect?.width || !rect.height) return
    const width = Math.max(1000, rect.width)
    dimensions.value = {
      width,
      height: (width * rect.height) / rect.width,
      scale: rect.width / width
    }
  })
  if (host.value) resizeObserver.observe(host.value)
  window.addEventListener('message', receive)
  document.addEventListener('visibilitychange', updateVisibility)
  visibilityObserver = new IntersectionObserver(
    (entries) => {
      inViewport = entries[0]?.isIntersecting ?? false
      updateVisibility()
    },
    { threshold: 0.05 }
  )
  preloadObserver = new IntersectionObserver(
    (entries) => {
      nearViewport = entries[0]?.isIntersecting ?? false
      updateVisibility()
    },
    { rootMargin: '400px 0px', threshold: 0 }
  )
  if (host.value) {
    visibilityObserver.observe(host.value)
    preloadObserver.observe(host.value)
  }
})
onBeforeUnmount(() => {
  disposed = true
  reset()
  unsubscribe()
  visibilityObserver?.disconnect()
  preloadObserver?.disconnect()
  resizeObserver?.disconnect()
  window.removeEventListener('message', receive)
  document.removeEventListener('visibilitychange', updateVisibility)
})
</script>
