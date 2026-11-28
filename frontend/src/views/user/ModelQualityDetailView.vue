<template>
  <AppLayout>
    <div class="quality-page quality-compact-page quality-detail-page">
      <button class="quality-back" @click="back">
        <Icon name="arrowLeft" size="sm" />返回检测画廊
      </button>
      <div v-if="loading" class="quality-empty" role="status">正在加载详情</div>
      <div v-else-if="error" class="quality-empty" role="alert">
        <h2>{{ error }}</h2>
        <button class="quality-inline-button" @click="refresh(true)">重试</button>
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
              {{ detail.protocol || '旧版记录' }} · 推理强度
              {{ detail.reasoning_effort || '默认' }}
            </p>
          </div>
          <span class="quality-muted">检测耗时 {{ qualityDuration(detail) }}</span>
        </header>
        <nav class="quality-result-tabs quality-detail-tabs" aria-label="检测题目">
          <button
            v-for="test in detail.tests"
            :key="test.id || test.benchmark"
            :class="{ active: activeTest === test }"
            :aria-pressed="activeTest === test"
            @click="selected = test.id || test.benchmark"
          >
            <span>{{ test.name || qualityMethodName(test.benchmark) }}</span>
            <span class="quality-badge" :data-state="qualityTestState(test)">{{
              qualityTestLabel(test)
            }}</span>
          </button>
        </nav>
        <template v-if="activeTest">
          <div
            class="quality-detail-layout"
            :class="{ 'quality-detail-candy': !activeTest.has_html }"
          >
            <section v-if="activeTest.has_html" class="quality-detail-art">
              <h3>
                {{ activeTest.name || '作品预览'
                }}<button
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
                  :key="activeTest.id || activeTest.benchmark"
                  :run-id="detail.id"
                  :test-id="activeTest.id || activeTest.benchmark"
                  :html="activeTest.html"
                  :expires-at="detail.expires_at"
                  :version="qualityRunVersion(detail)"
                  interactive
                />
              </div>
            </section>
            <section class="quality-detail-answer">
              <h3>
                {{
                  (activeTest.name || qualityMethodName(activeTest.benchmark)) +
                  ' · ' +
                  (activeTest.has_html ? '作品源码' : '模型回答')
                }}<button
                  class="quality-icon-button quality-section-action"
                  :disabled="!activeTest.answer && !activeTest.generation?.answer"
                  title="复制回答"
                  aria-label="复制回答"
                  @click="copyToClipboard(activeTest.answer || activeTest.generation?.answer || '')"
                >
                  <Icon :name="copied ? 'check' : 'copy'" size="sm" />
                </button>
              </h3>
              <pre v-if="activeAnswer">{{ activeAnswer }}</pre>
              <div
                v-else
                class="quality-answer-empty"
                role="status"
                :data-state="activeTest.status"
              >
                <Icon
                  :name="
                    ['pending', 'generating'].includes(activeTest.status)
                      ? 'clock'
                      : 'exclamationTriangle'
                  "
                  size="md"
                />
                <span>{{
                  ['pending', 'generating'].includes(activeTest.status)
                    ? activeTest.status === 'pending'
                      ? '等待生成'
                      : '正在生成，等待模型返回正文'
                    : '本题未取得有效回答'
                }}</span>
              </div>
              <p>
                <span class="quality-badge" :data-state="qualityTestState(activeTest)">{{
                  qualityTestLabel(activeTest)
                }}</span>
                {{ activeTest.reason || '等待判定' }}
              </p>
            </section>
          </div>
          <section
            v-if="activeTest.judgment && activeTest.judgment.status !== 'pending'"
            class="quality-judgment"
          >
            <div class="quality-heading">
              <h3>判题结果 · {{ activeTest.judge_model || detail.model }}</h3>
              <button
                v-if="
                  auth.isAdmin &&
                  activeTest.rule_kind === 'model' &&
                  activeTest.generation?.status === 'completed'
                "
                class="btn btn-secondary"
                :disabled="
                  rejudging.includes(activeTest.id || '') ||
                  ['pending', 'generating', 'ready_judge', 'judging'].includes(activeTest.status) ||
                  (activeTest.previous_judgments?.length || 0) >= 3
                "
                @click="rejudge(activeTest)"
              >
                <Icon name="refresh" size="sm" />仅重新判题
              </button>
            </div>
            <pre>{{
              activeTest.judgment.answer ||
              activeTest.judgment.reason ||
              qualityLabels[activeTest.judgment.status]
            }}</pre>
          </section>
        </template>
        <section class="quality-execution">
          <h3>执行信息</h3>
          <template v-for="test in detail.tests" :key="test.id || test.benchmark"
            ><div
              v-for="phase in test.generation
                ? [
                    { label: '生成', ...test.generation },
                    ...(test.previous_judgments || []).map((p, i) => ({
                      label: '历史判题 ' + (i + 1),
                      ...p
                    })),
                    ...(test.judgment ? [{ label: '判题', ...test.judgment }] : [])
                  ]
                : [{ label: '旧版检测', ...test }]"
              :key="phase.label"
            >
              <strong
                >{{ test.name || qualityMethodName(test.benchmark) }} · {{ phase.label }}</strong
              ><span>{{ qualityLabels[phase.status] || phase.status }}</span
              ><span>{{ phaseDuration(phase) }}</span
              ><span>输入 {{ phase.input_tokens ?? '—' }}</span
              ><span>输出 {{ phase.output_tokens ?? '—' }}</span
              ><span>推理 {{ phase.reasoning_tokens ?? '—' }}</span
              ><span>{{ qualityTime(phase.finished_at) }}</span>
            </div></template
          >
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
  type QualityRun,
  type QualityTest
} from '@/api/modelQuality'
import { useAuthStore } from '@/stores/auth'
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
const auth = useAuthStore()
const rejudging = ref<string[]>([])
const id = computed(() =>
  /^\d+$/.test(String(route.params.id)) ? Number(route.params.id) : String(route.params.id)
)
const detail = ref<QualityRun>(),
  error = ref(''),
  loading = ref(true),
  artwork = ref<HTMLElement>()
const selected = ref('')
const activeTest = computed(
  () =>
    detail.value?.tests.find((t) => (t.id || t.benchmark) === selected.value) ||
    detail.value?.tests[0]
)
const activeAnswer = computed(
  () => activeTest.value?.answer || activeTest.value?.generation?.answer || ''
)
function phaseDuration(phase: { status: string; duration_ms: number; started_at?: string }) {
  if (phase.status === 'running') {
    if (!phase.started_at) return '进行中'
    return (
      Math.max(0, (Date.now() - Date.parse(phase.started_at)) / 1000).toFixed(0) + ' 秒 · 进行中'
    )
  }
  if (phase.status === 'pending') return '等待开始'
  return (phase.duration_ms / 1000).toFixed(1) + ' 秒'
}
const canFullscreen = typeof document.documentElement.requestFullscreen === 'function'
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
    clearDetail(reason === 'expired' ? '作品已过期或暂不可用' : '检测记录已失效，请重新加载')
})
async function refresh(force = false) {
  if (refreshing || document.hidden || disposed) return
  if (!/^(?:[1-9][0-9]*|v1-[1-9][0-9]*|v2-[0-9a-f-]{36})$/.test(String(id.value))) {
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
    if (disposed || generation !== requestGeneration || scope !== session.generation) return
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
    if (disposed || generation !== requestGeneration || scope !== session.generation) return
    if (!groups.some((group) => group.group_id === value.group_id)) {
      session.loader.invalidate(id.value, 'expired')
      return
    }
    if (Date.parse(value.expires_at) <= Date.now()) {
      session.loader.invalidate(id.value, 'expired')
      return
    }
    detail.value = value
    if (!value.tests.some((test) => (test.id || test.benchmark) === selected.value)) {
      const requested = value.tests.find((test) => test.benchmark === route.query.method)
      const answered = value.tests.find((test) => test.answer || test.generation?.answer)
      const first = requested || answered || value.tests[0]
      selected.value = first?.id || first?.benchmark || ''
    }
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
async function rejudge(test: QualityTest) {
  if (!test.id) return
  rejudging.value.push(test.id)
  try {
    await modelQualityAPI.rejudge(test.id)
    session.loader.invalidate(id.value, 'changed')
    await refresh(true)
    app.showSuccess('已加入重新判题队列')
  } catch (e) {
    app.showError(extractApiErrorMessage(e, '无法重新判题'))
  } finally {
    rejudging.value = rejudging.value.filter((v) => v !== test.id)
  }
}
function back() {
  const filters = qualityFilters(route.query)
  if (!filters.group_id && detail.value) filters.group_id = detail.value.group_id
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

<style scoped>
.quality-detail-tabs {
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.quality-judgment {
  padding: 24px 0;
}
.quality-judgment pre {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.quality-detail-answer pre {
  max-height: 640px;
  overflow: auto;
}
</style>
