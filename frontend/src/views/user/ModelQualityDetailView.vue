<template>
  <AppLayout quiet>
    <div class="quality-page quality-compact-page quality-detail-page">
      <button class="quality-back" @click="back">
        <Icon name="arrowLeft" size="sm" />返回检测画廊
      </button>
      <div v-if="loading" class="quality-empty" role="status">正在加载详情</div>
      <div v-else-if="error" class="quality-empty" role="alert">
        <h2>{{ error }}</h2>
        <button class="quality-inline-button" @click="refresh(true)">
          重试
        </button>
      </div>
      <template v-else-if="detail">
        <header class="quality-detail-heading">
          <div>
            <h1>
              {{ detail.model
              }}<span class="quality-badge" :data-state="detail.outcome">{{
                qualityLabels[detail.outcome]
              }}</span>
            </h1>
            <p>
              {{ detail.group_name }} · {{ qualityTime(detail.started_at) }} ·
              推理程度 {{ detail.reasoning_effort }}
            </p>
          </div>
          <span class="quality-muted"
            >检测耗时 {{ qualityDuration(detail) }}</span
          >
        </header>
        <div
          class="quality-detail-layout"
          :class="{ 'quality-detail-candy': !pelican }"
        >
          <section v-if="pelican" class="quality-detail-art">
            <h3>
              鹈鹕作品<span
                class="quality-badge"
                :data-state="qualityTestState(pelican)"
                >{{ qualityTestLabel(pelican) }}</span
              >
              <button
                v-if="pelican.html"
                class="quality-icon-button quality-section-action"
                :disabled="!canFullscreen"
                title="全屏查看作品"
                aria-label="全屏查看作品"
                @click="fullscreen"
              >
                <Icon name="externalLink" size="sm" />
              </button>
            </h3>
            <div ref="artwork" class="quality-detail-artwork">
              <QualityPreview
                v-if="pelican.html"
                :key="detail.id"
                :run-id="detail.id"
                :html="pelican.html"
                :expires-at="detail.expires_at"
                :version="qualityRunVersion(detail)"
                interactive
              />
              <div v-else class="quality-art-empty">
                {{ qualityTestLabel(pelican) }}
              </div>
            </div>
            <p>{{ pelican.reason || '等待判定' }}</p>
          </section>
          <section v-if="candy" class="quality-detail-answer">
            <h3>
              糖果回答<span
                class="quality-badge"
                :data-state="qualityTestState(candy)"
                >{{ qualityTestLabel(candy) }}</span
              >
              <button
                class="quality-icon-button quality-section-action"
                :disabled="!candy.answer"
                title="复制回答"
                aria-label="复制回答"
                @click="copyToClipboard(candy.answer || '')"
              >
                <Icon :name="copied ? 'check' : 'copy'" size="sm" />
              </button>
            </h3>
            <pre>{{ candy.answer || '暂无答案' }}</pre>
            <p>{{ candy.reason || '等待判定' }}</p>
          </section>
        </div>
        <section class="quality-execution">
          <h3>执行信息</h3>
          <div v-for="test in detail.tests" :key="test.benchmark">
            <strong>{{ qualityMethodName(test.benchmark) }}</strong>
            <span>{{ qualityTestLabel(test) }}</span
            ><span>{{ (test.duration_ms / 1000).toFixed(1) }} 秒</span>
            <span>输入 {{ test.input_tokens ?? '—' }}</span
            ><span>输出 {{ test.output_tokens ?? '—' }}</span>
            <span>推理 {{ test.reasoning_tokens ?? '—' }}</span
            ><span>完成时间 {{ qualityTime(test.finished_at) }}</span>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityPreview from '@/components/model-quality/QualityPreview.vue'
import {
  modelQualityAPI,
  qualityLabels,
  qualityTestState,
  qualityTime,
  type QualityRun
} from '@/api/modelQuality'
import { useAppStore } from '@/stores/app'
import { useModelQualityStore } from '@/stores/modelQuality'
import { useClipboard } from '@/composables/useClipboard'
import {
  isQualityAbort,
  qualityRequestStatus,
  qualityRunVersion,
  scheduleQualityExpiry
} from '@/utils/modelQualityLoader'
import {
  qualityFilters,
  qualityFilterQuery,
  qualityMethodName,
  qualityTestLabel,
  qualityDuration
} from '@/utils/modelQualityNavigation'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/model-quality.css'

const route = useRoute(),
  router = useRouter(),
  app = useAppStore(),
  session = useModelQualityStore()
const { copied, copyToClipboard } = useClipboard()
const id = computed(() => Number(route.params.id))
const detail = ref<QualityRun>(),
  error = ref(''),
  loading = ref(true),
  artwork = ref<HTMLElement>()
const candy = computed(() =>
  detail.value?.tests.find((test) => test.benchmark === 'candy')
)
const pelican = computed(() =>
  detail.value?.tests.find((test) => test.benchmark === 'pelican')
)
const canFullscreen =
  typeof document.documentElement.requestFullscreen === 'function'
let disposed = false,
  refreshing = false,
  requestGeneration = 0
let controller: AbortController | undefined,
  timer: ReturnType<typeof setInterval> | undefined,
  cancelExpiry: (() => void) | undefined

function clearDetail(message: string) {
  requestGeneration++
  controller?.abort()
  cancelExpiry?.()
  refreshing = false
  detail.value = undefined
  loading.value = false
  error.value = message
}
const unsubscribe = session.loader.subscribe((runId, reason) => {
  if (runId !== null && runId !== id.value) return
  if (reason !== 'changed')
    clearDetail(
      reason === 'expired'
        ? '作品已过期或暂不可用'
        : '检测记录已失效，请重新加载'
    )
})
async function refresh(force = false) {
  if (refreshing || document.hidden || disposed) return
  if (!Number.isSafeInteger(id.value) || id.value <= 0) {
    clearDetail('检测记录不存在')
    return
  }
  refreshing = true
  const generation = ++requestGeneration,
    scope = session.generation
  controller?.abort()
  const request = new AbortController()
  controller = request
  try {
    const groups = await modelQualityAPI.groups(request.signal)
    if (
      disposed ||
      generation !== requestGeneration ||
      scope !== session.generation
    )
      return
    session.loader.setGroups(groups.map((group) => group.group_id))
    if (!groups.length) {
      clearDetail('降智检测已关闭或当前记录不可访问')
      return
    }
    if (generation !== requestGeneration) return
    const value =
      !detail.value || force || detail.value.outcome === 'running'
        ? await session.loader.load(id.value, {
            priority: 'detail',
            signal: request.signal,
            force
          })
        : detail.value
    if (
      disposed ||
      generation !== requestGeneration ||
      scope !== session.generation
    )
      return
    if (!groups.some((group) => group.group_id === value.group_id)) {
      session.loader.invalidate(id.value, 'expired')
      return
    }
    if (Date.parse(value.expires_at) <= Date.now()) {
      session.loader.invalidate(id.value, 'expired')
      return
    }
    detail.value = value
    error.value = ''
    cancelExpiry?.()
    cancelExpiry = scheduleQualityExpiry(value.expires_at, () =>
      session.loader.invalidate(value.id, 'expired')
    )
  } catch (e) {
    if (
      disposed ||
      generation !== requestGeneration ||
      scope !== session.generation ||
      isQualityAbort(e)
    )
      return
    const status = qualityRequestStatus(e)
    if (status === 401 || status === 403 || status === 404) {
      session.loader.clear('denied')
      void app.fetchPublicSettings(true)
    }
    detail.value = undefined
    error.value = extractApiErrorMessage(e, '作品已过期或暂不可用')
  } finally {
    if (generation === requestGeneration) {
      refreshing = false
      loading.value = false
    }
  }
}
function back() {
  const filters = qualityFilters(route.query)
  if (!filters.group_id && detail.value)
    filters.group_id = detail.value.group_id
  void router.push({
    path: '/model-quality',
    query: qualityFilterQuery(filters)
  })
}
async function fullscreen() {
  try {
    await artwork.value?.requestFullscreen()
  } catch {
    app.showError('暂时无法全屏显示作品')
  }
}
const wake = () => {
  void refresh()
}
watch(id, () => {
  clearDetail('')
  loading.value = true
  refreshing = false
  void refresh()
})
onMounted(() => {
  void refresh()
  timer = setInterval(wake, 15000)
  document.addEventListener('visibilitychange', wake)
})
onBeforeUnmount(() => {
  disposed = true
  requestGeneration++
  controller?.abort()
  unsubscribe()
  clearInterval(timer)
  cancelExpiry?.()
  document.removeEventListener('visibilitychange', wake)
})
</script>
