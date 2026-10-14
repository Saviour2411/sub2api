<template>
  <AppLayout quiet>
    <div class="quality-page">
      <header class="quality-heading">
        <div>
          <div class="quality-title">
            <Icon name="beaker" size="lg" />
            <h1>降智检测</h1>
          </div>
          <p>最近 24 小时 · 糖果与鹈鹕检测记录</p>
        </div>
        <div class="quality-heading-actions">
          <span class="quality-muted">{{ refreshing ? '更新中' : '每 15 秒更新' }}</span
          ><button
            class="quality-icon-button"
            title="刷新"
            aria-label="刷新检测结果"
            :disabled="refreshing"
            @click="refresh"
          >
            <Icon name="refresh" size="sm" />
          </button>
        </div>
      </header>
      <div v-if="disabled" class="quality-empty">
        <Icon name="beaker" size="lg" />
        <h2>降智检测已关闭</h2>
      </div>
      <template v-else>
        <nav v-if="groups.length" class="quality-group-tabs" aria-label="检测分组">
          <button
            v-for="group in groups"
            :key="group.group_id"
            :aria-pressed="selectedGroup === group.group_id"
            :class="{ active: selectedGroup === group.group_id }"
            @click="selectGroup(group.group_id)"
          >
            <span class="quality-dot" :data-state="group.status"></span>{{ group.name
            }}<span class="quality-tab-state">{{
              qualityLabels[group.status] || group.status
            }}</span>
          </button>
        </nav>
        <div v-if="currentGroup" class="quality-group-context">
          <span
            ><span class="quality-dot" :data-state="currentGroup.status"></span
            >{{ qualityLabels[currentGroup.status] }}</span
          >
          <span>{{ currentGroup.models.length }} 个模型</span
          ><span>{{ currentGroup.methods.map(methodName).join(' + ') }}</span>
          <span>最近检测 {{ qualityTime(currentGroup.last_started_at) }}</span
          ><span
            >下次检测
            {{
              currentGroup.running ? '本轮完成后安排' : qualityTime(currentGroup.next_run_at)
            }}</span
          >
        </div>
        <p v-if="currentGroup?.pause_reason" class="quality-alert">
          {{ currentGroup.pause_reason }}
        </p>
        <div v-if="groups.length" class="quality-gallery-toolbar">
          <h2>
            检测画廊 <span class="quality-muted">{{ total }} 条</span>
          </h2>
          <div class="quality-filters">
            <select v-model="model" class="input" aria-label="筛选模型">
              <option value="">全部模型</option>
              <option v-for="id in currentGroup?.models || []" :key="id" :value="id">
                {{ id }}
              </option>
            </select>
            <select v-model="method" class="input" aria-label="筛选检测方式">
              <option value="">全部方式</option>
              <option value="pelican">鹈鹕</option>
              <option value="candy">糖果</option>
            </select>
          </div>
        </div>
        <div v-if="groups.length" class="quality-result-tabs" aria-label="筛选检测结果">
          <button
            v-for="value in ['', 'normal', 'degraded', 'failed', 'running', 'unknown']"
            :key="value"
            :aria-pressed="outcome === value"
            :class="{ active: outcome === value }"
            @click="outcome = value"
          >
            {{ value ? qualityLabels[value] : '全部结果' }}
          </button>
        </div>
        <p v-if="error" class="quality-error" role="alert">
          {{ error }}<button class="quality-inline-button" @click="refresh">重试</button>
        </p>
        <div v-if="loading" class="quality-empty" role="status">正在加载检测记录</div>
        <div v-else-if="!runs.length" class="quality-empty">
          <Icon name="beaker" size="lg" />
          <h2>{{ groups.length ? '暂无符合条件的检测记录' : '暂无检测分组' }}</h2>
          <p>{{ currentGroup?.pause_reason || '检测完成后，结果将在这里展示' }}</p>
        </div>
        <div v-else class="quality-gallery">
          <article v-for="run in runs" :key="run.id" :data-quality-id="run.id" class="quality-card">
            <button
              class="quality-card-open"
              :aria-label="`查看 ${run.model} ${qualityTime(run.started_at)} 的检测详情`"
              @click="openDetail(run)"
            >
              <div class="quality-card-art">
                <QualityPreview v-if="pelican(run)?.has_html && !detailID" :run-id="run.id" />
                <div v-else-if="!pelican(run)" class="quality-candy-art">
                  <Icon name="beaker" size="lg" /><span>糖果测试</span
                  ><strong>{{
                    candy(run)?.answer || qualityLabels[candy(run)?.status || 'pending']
                  }}</strong>
                </div>
                <div v-else class="quality-art-empty">
                  <Icon
                    :name="run.outcome === 'running' ? 'refresh' : 'beaker'"
                    size="lg"
                  /><span>{{
                    run.outcome === 'running'
                      ? '作品生成中'
                      : detailID
                        ? '作品预览已暂停'
                        : '未生成作品'
                  }}</span>
                </div>
                <span class="quality-art-badge quality-badge" :data-state="run.outcome">{{
                  qualityLabels[run.outcome]
                }}</span>
              </div>
              <div class="quality-card-body">
                <div class="quality-model-line">
                  <strong>{{ run.model }}</strong
                  ><span>{{ run.reasoning_effort }}</span>
                </div>
                <div class="quality-test-pills">
                  <span
                    v-for="test in run.tests"
                    :key="test.benchmark"
                    class="quality-badge"
                    :data-state="qualityTestState(test)"
                    >{{ methodName(test.benchmark) }} ·
                    {{ qualityLabels[qualityTestState(test)] }}</span
                  >
                </div>
                <p class="quality-answer-excerpt">
                  {{
                    candy(run)?.answer ||
                    candy(run)?.reason ||
                    pelican(run)?.reason ||
                    '等待检测结果'
                  }}
                </p>
              </div>
              <footer>
                <span>{{ qualityTime(run.started_at) }}</span
                ><span>{{ duration(run) }}<Icon name="arrowRight" size="xs" /></span>
              </footer>
            </button>
          </article>
        </div>
        <footer v-if="groups.length" class="quality-gallery-footer">
          <span>按检测开始时间倒序 · 每页 12 条</span>
          <div>
            <button
              class="quality-icon-button"
              title="上一页"
              aria-label="上一页"
              :disabled="page <= 1"
              @click="page--"
            >
              <Icon name="chevronLeft" size="sm" /></button
            ><span>{{ page }} / {{ pages }}</span
            ><button
              class="quality-icon-button"
              title="下一页"
              aria-label="下一页"
              :disabled="page >= pages"
              @click="page++"
            >
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
        </footer>
        <p class="quality-footnote">单次测试仅反映本次输出，不代表模型身份或整体能力。</p>
      </template>
      <BaseDialog
        :show="!!detailID"
        :title="detail ? `${detail.model} · 检测详情` : '检测详情'"
        width="extra-wide"
        @close="closeDetail"
      >
        <div v-if="detailLoading" class="quality-empty" role="status">正在加载详情</div>
        <div v-else-if="detailError" class="quality-empty">{{ detailError }}</div>
        <div v-else-if="detail" class="quality-detail-context">
          <div class="quality-detail-meta">
            <span>{{ detail.group_name }}</span
            ><span>{{ qualityTime(detail.started_at) }}</span
            ><span>推理程度 {{ detail.reasoning_effort }}</span
            ><span class="quality-badge" :data-state="detail.outcome">{{
              qualityLabels[detail.outcome]
            }}</span>
          </div>
          <div class="quality-detail-layout" :class="{ 'quality-detail-candy': !pelican(detail) }">
            <div v-if="pelican(detail)" class="quality-detail-art">
              <QualityPreview
                v-if="pelican(detail)?.html"
                :key="detail.id"
                :run-id="detail.id"
                :html="pelican(detail)?.html"
                interactive
              />
              <div v-else class="quality-art-empty">
                {{ qualityLabels[qualityTestState(pelican(detail)!)] }}
              </div>
              <h3>
                鹈鹕检测
                <span class="quality-badge" :data-state="qualityTestState(pelican(detail)!)">{{
                  qualityLabels[qualityTestState(pelican(detail)!)]
                }}</span>
              </h3>
              <p>{{ pelican(detail)?.reason || '等待判定' }}</p>
            </div>
            <section v-if="candy(detail)" class="quality-detail-answer">
              <h3>
                糖果答案
                <span class="quality-badge" :data-state="qualityTestState(candy(detail)!)">{{
                  qualityLabels[qualityTestState(candy(detail)!)]
                }}</span>
              </h3>
              <pre>{{ candy(detail)?.answer || '暂无答案' }}</pre>
              <p>{{ candy(detail)?.reason }}</p>
            </section>
          </div>
          <div class="quality-execution">
            <h3>执行明细</h3>
            <div v-for="test in detail.tests" :key="test.benchmark">
              <strong>{{ methodName(test.benchmark) }}</strong
              ><span>{{ qualityLabels[test.status] }}</span
              ><span>{{ (test.duration_ms / 1000).toFixed(1) }} 秒</span
              ><span>输入 {{ test.input_tokens ?? '—' }}</span
              ><span>输出 {{ test.output_tokens ?? '—' }}</span
              ><span>推理 {{ test.reasoning_tokens ?? '—' }}</span>
            </div>
          </div>
        </div>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityPreview from '@/components/model-quality/QualityPreview.vue'
import {
  modelQualityAPI,
  qualityLabels,
  qualityTestState,
  qualityTime,
  type QualityGroup,
  type QualityRun
} from '@/api/modelQuality'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/model-quality.css'

const app = useAppStore()
const groups = ref<QualityGroup[]>([]),
  runs = ref<QualityRun[]>([])
const selectedGroup = ref(0),
  model = ref(''),
  method = ref(''),
  outcome = ref(''),
  page = ref(1),
  total = ref(0),
  pages = ref(1)
const loading = ref(true),
  refreshing = ref(false),
  error = ref(''),
  disabled = ref(false)
const detailID = ref(0),
  detail = ref<QualityRun>(),
  detailLoading = ref(false),
  detailError = ref('')
const currentGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroup.value)
)
let timer: ReturnType<typeof setInterval> | undefined
let listController: AbortController | undefined,
  detailController: AbortController | undefined,
  refreshController: AbortController | undefined
let listGeneration = 0,
  disposed = false
const methodName = (value: string) => (value === 'pelican' ? '鹈鹕' : '糖果')
const candy = (run: QualityRun) => run.tests.find((test) => test.benchmark === 'candy')
const pelican = (run: QualityRun) => run.tests.find((test) => test.benchmark === 'pelican')
function duration(run: QualityRun) {
  return run.finished_at
    ? `${Math.max(0, (new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000).toFixed(0)} 秒`
    : '进行中'
}
function selectGroup(id: number) {
  selectedGroup.value = id
  model.value = ''
  method.value = ''
  outcome.value = ''
  page.value = 1
}
async function loadRuns(quiet = false) {
  const generation = ++listGeneration
  listController?.abort()
  if (!selectedGroup.value) {
    loading.value = false
    return
  }
  listController = new AbortController()
  if (!quiet) loading.value = true
  const anchor = quiet
    ? [...document.querySelectorAll<HTMLElement>('[data-quality-id]')].find(
        (el) => el.getBoundingClientRect().top >= 0
      )
    : undefined
  const anchorTop = anchor?.getBoundingClientRect().top
  try {
    const result = await modelQualityAPI.runs(
      {
        group_id: selectedGroup.value,
        model: model.value,
        method: method.value,
        outcome: outcome.value,
        page: page.value
      },
      listController.signal
    )
    if (generation !== listGeneration || disposed) return
    runs.value = result.items
    total.value = result.total
    pages.value = result.pages
    error.value = ''
    if (page.value > result.pages) {
      page.value = result.pages
      return
    }
    await nextTick()
    if (anchor?.isConnected && anchorTop !== undefined) {
      let parent = anchor.parentElement
      while (parent && !/(auto|scroll)/.test(getComputedStyle(parent).overflowY))
        parent = parent.parentElement
      if (parent) parent.scrollTop += anchor.getBoundingClientRect().top - anchorTop
      else window.scrollBy(0, anchor.getBoundingClientRect().top - anchorTop)
    }
  } catch (e) {
    if (generation === listGeneration && !disposed)
      error.value = extractApiErrorMessage(e, '检测记录加载失败')
  } finally {
    if (generation === listGeneration) loading.value = false
  }
}
async function refresh() {
  if (refreshing.value || document.hidden || disposed) return
  refreshing.value = true
  refreshController = new AbortController()
  try {
    const result = await modelQualityAPI.groups(refreshController.signal)
    if (disposed) return
    groups.value = result
    if (!result.some((group) => group.group_id === selectedGroup.value)) {
      selectGroup(result[0]?.group_id || 0)
      if (!selectedGroup.value) {
        runs.value = []
        total.value = 0
        loading.value = false
      }
    } else await loadRuns(true)
    if (detailID.value && detail.value?.outcome === 'running')
      await fetchDetail(detailID.value, true)
    if (detail.value && Date.parse(detail.value.expires_at) <= Date.now()) {
      detail.value = undefined
      detailError.value = '作品已超过 24 小时保留期限'
    }
  } catch (e) {
    const status =
      (e as { status?: number; response?: { status?: number } }).status ||
      (e as { response?: { status?: number } }).response?.status
    if (status === 404) {
      disabled.value = true
      groups.value = []
      runs.value = []
      closeDetail()
      void app.fetchPublicSettings(true)
    } else error.value = extractApiErrorMessage(e, '检测状态加载失败')
  } finally {
    refreshing.value = false
    loading.value = false
  }
}
async function fetchDetail(id: number, quiet = false) {
  detailController?.abort()
  detailController = new AbortController()
  if (!quiet) detailLoading.value = true
  try {
    const result = await modelQualityAPI.detail(id, detailController.signal)
    if (detailID.value === id) {
      detail.value = result
      detailError.value = ''
    }
  } catch (e) {
    if (detailID.value === id) detailError.value = extractApiErrorMessage(e, '作品已过期或暂不可用')
  } finally {
    if (detailID.value === id) detailLoading.value = false
  }
}
function openDetail(run: QualityRun) {
  detailID.value = run.id
  detail.value = undefined
  detailError.value = ''
  void fetchDetail(run.id)
}
function closeDetail() {
  detailID.value = 0
  detailController?.abort()
  detail.value = undefined
}
watch([selectedGroup, model, method, outcome], () => {
  page.value = 1
  void loadRuns()
})
watch(page, () => {
  void loadRuns()
})
onMounted(() => {
  void refresh()
  timer = setInterval(refresh, 15000)
  document.addEventListener('visibilitychange', refresh)
})
onBeforeUnmount(() => {
  disposed = true
  listGeneration++
  clearInterval(timer)
  listController?.abort()
  detailController?.abort()
  refreshController?.abort()
  document.removeEventListener('visibilitychange', refresh)
})
</script>
