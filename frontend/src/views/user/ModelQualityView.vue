<template>
  <AppLayout quiet>
    <div class="quality-page quality-compact-page">
      <div v-if="disabled" class="quality-empty">
        <Icon name="beaker" size="lg" />
        <h2>降智检测已关闭或当前记录不可访问</h2>
      </div>
      <template v-else>
        <div class="quality-topbar">
          <nav
            v-if="groups.length"
            class="quality-group-tabs"
            aria-label="检测分组"
          >
            <button
              v-for="group in groups"
              :key="group.group_id"
              :aria-pressed="selectedGroup === group.group_id"
              :class="{ active: selectedGroup === group.group_id }"
              @click="selectGroup(group.group_id)"
            >
              <span class="quality-dot" :data-state="group.status"></span
              >{{ group.name }}
            </button>
          </nav>
          <select
            v-if="groups.length"
            class="input quality-mobile-groups"
            aria-label="选择检测分组"
            :value="selectedGroup"
            @change="
              selectGroup(Number(($event.target as HTMLSelectElement).value))
            "
          >
            <option
              v-for="group in groups"
              :key="group.group_id"
              :value="group.group_id"
            >
              {{ group.name }}
            </option>
          </select>
          <div class="quality-heading-actions">
            <span class="quality-muted">{{
              refreshing ? '更新中' : '每 15 秒更新'
            }}</span>
            <button
              class="quality-icon-button"
              title="刷新"
              aria-label="刷新检测结果"
              :disabled="refreshing"
              @click="refresh"
            >
              <Icon name="refresh" size="sm" />
            </button>
          </div>
        </div>
        <div v-if="currentGroup" class="quality-group-context">
          <span>最近 24 小时</span
          ><span>{{ currentGroup.models.length }} 个模型</span>
          <span>{{
            currentGroup.methods.map(qualityMethodName).join(' / ')
          }}</span>
          <span
            >下次检测
            {{
              currentGroup.running
                ? '本轮完成后安排'
                : qualityTime(currentGroup.next_run_at)
            }}</span
          >
          <span class="quality-record-count">共 {{ total }} 条记录</span>
        </div>
        <p v-if="currentGroup?.pause_reason" class="quality-alert">
          {{ currentGroup.pause_reason }}
        </p>
        <div v-if="groups.length" class="quality-filterbar">
          <div class="quality-result-tabs" aria-label="筛选检测结果">
            <button
              v-for="value in [
                '',
                'normal',
                'degraded',
                'failed',
                'running',
                'unknown'
              ]"
              :key="value"
              :aria-pressed="filters.outcome === value"
              :class="{ active: filters.outcome === value }"
              @click="updateFilters({ outcome: value })"
            >
              {{ value ? qualityLabels[value] : '全部结果' }}
            </button>
          </div>
          <div class="quality-filters">
            <select
              class="input"
              aria-label="筛选模型"
              :value="filters.model"
              @change="
                updateFilters({
                  model: ($event.target as HTMLSelectElement).value
                })
              "
            >
              <option value="">全部模型</option>
              <option
                v-for="id in currentGroup?.models || []"
                :key="id"
                :value="id"
              >
                {{ id }}
              </option>
            </select>
            <select
              class="input"
              aria-label="筛选检测方式"
              :value="filters.method"
              @change="
                updateFilters({
                  method: ($event.target as HTMLSelectElement).value
                })
              "
            >
              <option value="">全部方式</option>
              <option value="pelican">鹈鹕</option>
              <option value="candy">糖果</option>
            </select>
          </div>
        </div>
        <p v-if="error" class="quality-error" role="alert">
          {{ error
          }}<button class="quality-inline-button" @click="refresh">重试</button>
        </p>
        <div v-if="loading" class="quality-empty" role="status">
          正在加载检测记录
        </div>
        <div v-else-if="!runs.length" class="quality-empty">
          <Icon name="beaker" size="lg" />
          <h2>
            {{ groups.length ? '暂无符合条件的检测记录' : '暂无检测分组' }}
          </h2>
          <p>
            {{ currentGroup?.pause_reason || '检测完成后，结果将在这里展示' }}
          </p>
        </div>
        <div v-else class="quality-gallery">
          <article
            v-for="run in runs"
            :key="run.id"
            :data-quality-id="run.id"
            class="quality-card"
          >
            <button
              class="quality-card-open"
              :aria-label="`查看 ${run.model} ${qualityTime(run.started_at)} 的检测详情`"
              @click="openDetail(run)"
            >
              <div class="quality-card-art">
                <QualityPreview
                  v-if="pelican(run)?.has_html"
                  :run-id="run.id"
                  :expires-at="run.expires_at"
                  :version="qualityRunVersion(run)"
                />
                <div v-else-if="!pelican(run)" class="quality-candy-art">
                  <Icon name="beaker" size="lg" />
                  <strong
                    >糖果{{
                      candy(run) ? qualityTestLabel(candy(run)!) : '等待检测'
                    }}</strong
                  ><span>单项检测</span>
                </div>
                <div v-else class="quality-art-empty">
                  <Icon
                    :name="
                      run.outcome === 'running'
                        ? 'refresh'
                        : 'exclamationCircle'
                    "
                    size="lg"
                  /><span>{{
                    run.outcome === 'running' ? '作品生成中' : '未生成作品'
                  }}</span>
                </div>
                <span
                  class="quality-art-badge quality-badge"
                  :data-state="run.outcome"
                  >{{ qualityLabels[run.outcome] }}</span
                >
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
                    >{{ qualityMethodName(test.benchmark) }} ·
                    {{ qualityTestLabel(test) }}</span
                  >
                </div>
              </div>
              <footer>
                <span>{{ qualityTime(run.started_at) }}</span
                ><span
                  >{{ qualityDuration(run) }}<Icon name="arrowRight" size="xs"
                /></span>
              </footer>
            </button>
          </article>
        </div>
        <footer v-if="groups.length" class="quality-gallery-footer">
          <span>按检测时间倒序 · 每页 12 条</span>
          <div>
            <button
              class="quality-icon-button"
              title="上一页"
              aria-label="上一页"
              :disabled="filters.page <= 1"
              @click="updateFilters({ page: filters.page - 1 }, false)"
            >
              <Icon name="chevronLeft" size="sm" />
            </button>
            <span>{{ filters.page }} / {{ pages }}</span>
            <button
              class="quality-icon-button"
              title="下一页"
              aria-label="下一页"
              :disabled="filters.page >= pages"
              @click="updateFilters({ page: filters.page + 1 }, false)"
            >
              <Icon name="chevronRight" size="sm" />
            </button>
          </div>
        </footer>
        <p class="quality-footnote">
          单次测试仅反映本次输出，不代表模型身份或整体能力。
        </p>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import QualityPreview from '@/components/model-quality/QualityPreview.vue'
import {
  modelQualityAPI,
  qualityLabels,
  qualityTestState,
  qualityTime,
  type QualityFilters,
  type QualityGroup,
  type QualityPage,
  type QualityRun
} from '@/api/modelQuality'
import { useAppStore } from '@/stores/app'
import { useModelQualityStore } from '@/stores/modelQuality'
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

const app = useAppStore(),
  session = useModelQualityStore()
const route = useRoute(),
  router = useRouter()
const filters = computed(() => qualityFilters(route.query))
const selectedGroup = computed(() => filters.value.group_id)
const filterKey = computed(() => JSON.stringify(filters.value))
const groups = ref<QualityGroup[]>([]),
  runs = ref<QualityRun[]>([])
const total = ref(0),
  pages = ref(1),
  loading = ref(true),
  refreshing = ref(false),
  error = ref(''),
  disabled = ref(false)
const currentGroup = computed(() =>
  groups.value.find((group) => group.group_id === selectedGroup.value)
)
const candy = (run: QualityRun) =>
  run.tests.find((test) => test.benchmark === 'candy')
const pelican = (run: QualityRun) =>
  run.tests.find((test) => test.benchmark === 'pelican')
let disposed = false,
  ready = false,
  listGeneration = 0,
  restored = false
let listController: AbortController | undefined,
  refreshController: AbortController | undefined
let timer: ReturnType<typeof setInterval> | undefined,
  cancelExpiry: (() => void) | undefined
let result: QualityPage | undefined

function updateFilters(change: Partial<QualityFilters>, resetPage = true) {
  return router.replace({
    path: '/model-quality',
    query: qualityFilterQuery({
      ...filters.value,
      ...(resetPage ? { page: 1 } : {}),
      ...change
    })
  })
}
function selectGroup(group_id: number) {
  void updateFilters({ group_id, model: '', method: '', outcome: '' })
}
function anchor() {
  const element = [
    ...document.querySelectorAll<HTMLElement>('[data-quality-id]')
  ].find((el) => el.getBoundingClientRect().top >= 64)
  return {
    anchorId: element ? Number(element.dataset.qualityId) : undefined,
    anchorTop: element?.getBoundingClientRect().top,
    scrollY: window.scrollY
  }
}
async function restorePosition(position: {
  anchorId?: number
  anchorTop?: number
  scrollY: number
}) {
  await nextTick()
  if (disposed) return
  const element = position.anchorId
    ? document.querySelector<HTMLElement>(
        `[data-quality-id="${position.anchorId}"]`
      )
    : null
  window.scrollTo({
    left: 0,
    top: element && position.anchorTop !== undefined
      ? window.scrollY +
          element.getBoundingClientRect().top -
          position.anchorTop
      : position.scrollY,
    // 避免全站平滑滚动让后续刷新捕获恢复过程中的中间位置。
    behavior: 'instant'
  })
}
function applyResult(value: QualityPage) {
  result = value
  session.loader.sync(value.items)
  runs.value = value.items.filter(
    (run) => Date.parse(run.expires_at) > Date.now()
  )
  total.value = value.total
  pages.value = Math.max(1, value.pages)
  cancelExpiry?.()
  if (runs.value.length)
    cancelExpiry = scheduleQualityExpiry(
      new Date(
        Math.min(...runs.value.map((run) => Date.parse(run.expires_at)))
      ).toISOString(),
      () => {
        for (const run of runs.value)
          if (Date.parse(run.expires_at) <= Date.now())
            session.loader.invalidate(run.id, 'expired')
        runs.value = runs.value.filter(
          (run) => Date.parse(run.expires_at) > Date.now()
        )
        if (!document.hidden) void loadRuns(true)
      }
    )
}
async function loadRuns(quiet = false) {
  if (!selectedGroup.value || disposed || !ready) return
  const generation = ++listGeneration,
    scope = session.generation
  listController?.abort()
  const controller = new AbortController()
  listController = controller
  if (!quiet) loading.value = true
  const position = quiet ? anchor() : undefined
  try {
    const value = await modelQualityAPI.runs(filters.value, controller.signal)
    if (
      disposed ||
      generation !== listGeneration ||
      scope !== session.generation
    )
      return
    if (filters.value.page > Math.max(1, value.pages)) {
      await updateFilters({ page: Math.max(1, value.pages) }, false)
      return
    }
    applyResult(value)
    error.value = ''
    loading.value = false
    if (position) await restorePosition(position)
  } catch (e) {
    if (
      disposed ||
      generation !== listGeneration ||
      scope !== session.generation ||
      isQualityAbort(e)
    )
      return
    if ([401, 403, 404].includes(qualityRequestStatus(e) || 0))
      session.loader.clear('denied')
    else error.value = extractApiErrorMessage(e, '检测记录加载失败')
  } finally {
    if (generation === listGeneration) loading.value = false
  }
}
async function refresh() {
  if (refreshing.value || document.hidden || disposed) return
  refreshing.value = true
  const scope = session.generation
  refreshController = new AbortController()
  try {
    const value = await modelQualityAPI.groups(refreshController.signal)
    if (disposed || scope !== session.generation) return
    groups.value = value
    session.loader.setGroups(value.map((group) => group.group_id))
    disabled.value = false
    ready = true
    if (!value.length) {
      runs.value = []
      total.value = 0
      loading.value = false
      return
    }
    if (!value.some((group) => group.group_id === selectedGroup.value)) {
      selectGroup(value[0].group_id)
      return
    }
    const saved = session.gallery
    if (!restored && saved?.key === filterKey.value) {
      restored = true
      applyResult(saved.result)
      loading.value = false
      await restorePosition(saved)
    }
    await loadRuns(runs.value.length > 0)
  } catch (e) {
    if (disposed || scope !== session.generation || isQualityAbort(e)) return
    if ([401, 403, 404].includes(qualityRequestStatus(e) || 0)) {
      session.loader.clear('denied')
      void app.fetchPublicSettings(true)
    } else {
      error.value = extractApiErrorMessage(e, '检测状态加载失败')
      loading.value = false
    }
  } finally {
    refreshing.value = false
  }
}
function openDetail(run: QualityRun) {
  if (result)
    session.gallery = {
      key: filterKey.value,
      groups: groups.value,
      result,
      ...anchor()
    }
  void router.push({
    path: `/model-quality/runs/${run.id}`,
    query: qualityFilterQuery(filters.value)
  })
}
watch(filterKey, () => {
  if (ready && route.path === '/model-quality') void loadRuns()
})
watch(
  () => session.generation,
  () => {
    listGeneration++
    listController?.abort()
    refreshController?.abort()
    cancelExpiry?.()
    runs.value = []
    groups.value = []
    result = undefined
    disabled.value = true
    loading.value = false
    ready = false
  },
  { flush: 'sync' }
)
onMounted(() => {
  void refresh()
  timer = setInterval(refresh, 15000)
  document.addEventListener('visibilitychange', refresh)
})
onBeforeUnmount(() => {
  disposed = true
  listGeneration++
  listController?.abort()
  refreshController?.abort()
  clearInterval(timer)
  cancelExpiry?.()
  document.removeEventListener('visibilitychange', refresh)
})
</script>
