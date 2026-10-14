<template>
  <div ref="host" class="quality-preview" :class="{ 'quality-preview-interactive': interactive }">
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
    <div v-if="failed || !content" class="quality-preview-placeholder" role="status">
      <Icon :name="failed ? 'exclamationCircle' : 'refresh'" size="lg" />
      <span>{{ failed ? '作品已过期或暂不可用' : '作品加载中' }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { buildApiUrl } from '@/api/url'
import { modelQualityAPI } from '@/api/modelQuality'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ runId: number; html?: string; interactive?: boolean }>()
const host = ref<HTMLElement>()
const frame = ref<HTMLIFrameElement>()
const visible = ref(false)
const content = ref('')
const failed = ref(false)
const frameVersion = ref(0)
const dimensions = ref({ width: 1000, height: 625, scale: 1 })
const frameStyle = computed(() => ({
  width: `${dimensions.value.width}px`,
  height: `${dimensions.value.height}px`,
  transform: `scale(${dimensions.value.scale})`,
  transformOrigin: 'top left'
}))
let observer: IntersectionObserver | undefined
let resizeObserver: ResizeObserver | undefined
let controller: AbortController | undefined
let inViewport = false
let ready = false
let generation = 0

function send() {
  if (ready && content.value && visible.value)
    frame.value?.contentWindow?.postMessage({ type: 'quality-artwork', html: content.value }, '*')
}
function receive(event: MessageEvent) {
  if (event.source !== frame.value?.contentWindow || event.data?.type !== 'quality-ready') return
  ready = true
  send()
}
async function activate() {
  const current = ++generation
  frameVersion.value++
  controller?.abort()
  ready = false
  content.value = ''
  failed.value = false
  visible.value = inViewport && document.visibilityState !== 'hidden'
  if (!visible.value) return
  if (props.html) {
    content.value = props.html
    send()
    return
  }
  controller = new AbortController()
  try {
    const run = await modelQualityAPI.detail(props.runId, controller.signal)
    if (current !== generation) return
    content.value = run.tests.find((test) => test.benchmark === 'pelican')?.html || ''
    failed.value = !content.value
    send()
  } catch {
    if (current === generation) failed.value = true
  }
}
watch(() => [props.runId, props.html], activate)
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
  document.addEventListener('visibilitychange', activate)
  observer = new IntersectionObserver(
    (entries) => {
      const next = entries[0]?.isIntersecting ?? false
      if (next !== inViewport) {
        inViewport = next
        void activate()
      }
    },
    { threshold: 0.05 }
  )
  if (host.value) observer.observe(host.value)
})
onBeforeUnmount(() => {
  generation++
  controller?.abort()
  observer?.disconnect()
  resizeObserver?.disconnect()
  window.removeEventListener('message', receive)
  document.removeEventListener('visibilitychange', activate)
})
</script>
