import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ModelQualityView from '../ModelQualityView.vue'
import ModelQualityDetailView from '../ModelQualityDetailView.vue'
import { qualityRun, qualityGroups, deferred } from '@/__tests__/fixtures/modelQuality'
import { useModelQualityStore } from '@/stores/modelQuality'
import type { QualityGroup, QualityPage } from '@/api/modelQuality'

const mocks = vi.hoisted(() => ({
  groups: vi.fn(),
  runs: vi.fn(),
  detail: vi.fn(),
  rejudge: vi.fn(),
  fetchSettings: vi.fn(),
  copy: vi.fn()
}))
vi.mock('@/api/modelQuality', async (original) => ({
  ...(await original<typeof import('@/api/modelQuality')>()),
  modelQualityAPI: mocks
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 1 },
    isAuthenticated: true,
    isAdmin: true
  })
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: mocks.fetchSettings,
    showSuccess: vi.fn(),
    showError: vi.fn(),
    cachedPublicSettings: { model_quality_enabled: true }
  })
}))
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copied: false, copyToClipboard: mocks.copy })
}))
let wrapper: ReturnType<typeof mount>
let router: ReturnType<typeof createRouter>
async function view(path = '/model-quality') {
  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/model-quality', component: ModelQualityView },
      { path: '/model-quality/runs/:id', component: ModelQualityDetailView }
    ]
  })
  await router.push(path)
  wrapper = mount(
    { template: '<router-view />' },
    {
      global: {
        plugins: [router],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          QualityPreview: true
        }
      }
    }
  )
  await flushPromises()
  return wrapper
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  setActivePinia(createPinia())
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  mocks.groups.mockResolvedValue(qualityGroups)
  mocks.runs.mockResolvedValue({
    items: [qualityRun()],
    total: 25,
    page: 1,
    page_size: 12,
    pages: 3
  })
  mocks.detail.mockImplementation(async (id: number) => qualityRun(id))
})
afterEach(() => {
  wrapper?.unmount()
  useModelQualityStore().loader.clear()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

describe('紧凑画廊与独立详情', () => {
  it('普通结果没有账号区域，管理员私有结果按生成和判题展示', async () => {
    await view()
    expect(wrapper.find('.quality-account-summary').exists()).toBe(false)
    const run = qualityRun()
    run.tests[0].rule_kind = 'model'
    run.account_hits = [
      {
        test_id: 'art',
        phase: 'generation',
        account_id: 740,
        account_name: '生成渠道',
        source: 'usage'
      },
      {
        test_id: 'art',
        phase: 'judgment',
        account_id: 741,
        account_name: '判题渠道',
        source: 'usage'
      }
    ]
    mocks.runs.mockResolvedValue({
      items: [run],
      total: 1,
      page: 1,
      page_size: 12,
      pages: 1
    })
    await wrapper.get('[aria-label="刷新检测结果"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.quality-account-summary').text()).toContain('生成账号：生成渠道（#740）')
    expect(wrapper.get('.quality-account-summary').text()).toContain('判题账号：判题渠道（#741）')
  })
  it('详情按题目和判题历史分别对应账号，缺失关联不猜测', async () => {
    const run = qualityRun()
    const phase = { status: 'completed', duration_ms: 1000, attempts: 1 }
    run.tests[0] = {
      ...run.tests[0],
      id: 'art',
      generation: phase,
      judgment: phase,
      previous_judgments: [phase, phase]
    }
    run.account_hits = [
      {
        test_id: 'art',
        phase: 'generation',
        account_id: 740,
        account_name: '生成渠道',
        source: 'usage'
      },
      {
        test_id: 'art',
        phase: 'judgment',
        account_id: 741,
        account_name: '当前判题',
        source: 'usage'
      },
      {
        test_id: 'art',
        phase: 'previous_judgment',
        judgment_index: 1,
        account_id: 742,
        account_name: '历史判题',
        source: 'error'
      }
    ]
    mocks.detail.mockResolvedValue(run)
    await view('/model-quality/runs/9')
    const rows = wrapper.findAll('.quality-execution > div')
    expect(rows[0].text()).toContain('生成渠道（#740）')
    expect(rows[1].text()).toContain('历史判题（#742）')
    expect(rows[2].text()).toContain('命中账号：未记录')
    expect(rows[3].text()).toContain('当前判题（#741）')
    expect(rows[4].text()).toContain('命中账号：未记录')
  })
  it('生成中的动画与已通过糖果题分别展示，默认选中已有答案', async () => {
    const run = qualityRun()
    run.outcome = 'running'
    run.tests = [
      {
        benchmark: 'art',
        id: 'art',
        name: '动画题',
        status: 'generating',
        verdict: 'unknown',
        reason: '',
        has_html: false,
        duration_ms: 0,
        finished_at: null,
        generation: { status: 'running', duration_ms: 0, attempts: 1 }
      },
      {
        benchmark: 'candy',
        id: 'candy',
        name: '糖果题',
        status: 'completed',
        verdict: 'normal',
        reason: '答案与预期相等',
        answer: '21',
        has_html: false,
        duration_ms: 12003,
        finished_at: new Date().toISOString(),
        generation: {
          status: 'completed',
          answer: '21',
          duration_ms: 12003,
          attempts: 1
        }
      }
    ]
    mocks.detail.mockResolvedValue(run)
    await view('/model-quality/runs/9')
    expect(wrapper.get('.quality-detail-answer h3').text()).toContain('糖果题 · 模型回答')
    expect(wrapper.get('.quality-detail-answer pre').text()).toBe('21')
    await wrapper.findAll('[aria-label="检测题目"] button')[0].trigger('click')
    expect(wrapper.get('.quality-detail-answer h3').text()).toContain('动画题 · 模型回答')
    expect(wrapper.get('.quality-answer-empty').text()).toContain('正在生成，等待模型返回正文')
    expect(wrapper.find('.quality-detail-answer pre').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('暂无答案')
    expect(wrapper.get('.quality-execution').text()).toContain('进行中')
    expect(wrapper.get('.quality-execution').text()).not.toContain('0.0 秒')
  })
  it('列表只有判定结果，完整答案与原因只在独立详情按纯文本展示', async () => {
    await view()
    expect(wrapper.text()).toContain('鹈鹕 · 通过')
    expect(wrapper.text()).toContain('糖果 · 通过')
    expect(wrapper.text()).not.toContain('danger()')
    expect(wrapper.text()).not.toContain('回答正确')
    expect(mocks.detail).not.toHaveBeenCalled()
    await wrapper.get('.quality-card-open').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/model-quality/runs/9')
    await wrapper.findAll('[aria-label="检测题目"] button')[1].trigger('click')
    expect(wrapper.get('pre').text()).toContain('<script>danger()</script>')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain('回答正确')
    expect(wrapper.text()).toContain('执行信息')
    await wrapper.get('[aria-label="复制回答"]').trigger('click')
    expect(mocks.copy).toHaveBeenCalledWith(qualityRun().tests[1].answer)
  })
  it('全部筛选写入查询参数，详情往返恢复页码及滚动且复用正文', async () => {
    await view('/model-quality?group_id=7&model=model-b&method=candy&outcome=normal&page=2')
    vi.spyOn(window, 'scrollY', 'get').mockReturnValue(480)
    await useModelQualityStore().loader.load(9)
    await wrapper.get('.quality-card-open').trigger('click')
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.query).toEqual({
      group_id: '7',
      model: 'model-b',
      method: 'candy',
      outcome: 'normal',
      page: '2'
    })
    await wrapper.get('.quality-back').trigger('click')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({
      group_id: 7,
      page: 2,
      model: 'model-b',
      method: 'candy',
      outcome: 'normal'
    })
    expect(window.scrollTo).toHaveBeenCalledWith({
      left: 0,
      top: 480,
      behavior: 'instant'
    })
    await wrapper.get('.quality-card-open').trigger('click')
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledTimes(1)
  })
  it('筛选和分组切换重置分页，后台刷新保留筛选', async () => {
    await view()
    await wrapper.get('[aria-label="下一页"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.page).toBe('2')
    await wrapper.get('[aria-label="筛选模型"]').setValue('model-b')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({
      page: 1,
      model: 'model-b'
    })
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({
      page: 1,
      model: 'model-b'
    })
    await wrapper.findAll('.quality-group-tabs button')[1].trigger('click')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({
      group_id: 8,
      page: 1,
      model: ''
    })
    expect(wrapper.text()).toContain('待绑定密钥')
  })
  it.each(['failed', 'running', 'unknown'])('保留独立状态%s', async (state) => {
    const run = qualityRun()
    run.outcome = state
    run.tests[1] = {
      ...run.tests[1],
      status: state === 'unknown' ? 'completed' : state,
      verdict: state
    }
    mocks.runs.mockResolvedValue({ items: [run], total: 1, pages: 1 })
    await view()
    const expected = {
      failed: '检测失败',
      running: '检测中',
      unknown: '无法判定'
    }[state]
    expect(wrapper.text()).toContain(`糖果 · ${expected}`)
  })
  it('功能关闭后清除列表及缓存，旧列表响应不能回填', async () => {
    await view()
    const pending = deferred<QualityPage>()
    mocks.runs.mockReturnValue(pending.promise)
    await wrapper.get('[aria-label="筛选模型"]').setValue('model-b')
    await flushPromises()
    mocks.groups.mockRejectedValue({ status: 404 })
    await wrapper.get('[aria-label="刷新检测结果"]').trigger('click')
    await flushPromises()
    pending.resolve({
      items: [qualityRun()],
      total: 1,
      page: 1,
      pages: 1,
      page_size: 12
    })
    await flushPromises()
    expect(wrapper.text()).toContain('降智检测已关闭')
    expect(wrapper.find('.quality-card').exists()).toBe(false)
    expect(mocks.fetchSettings).toHaveBeenCalledWith(true)
  })
  it('初次默认分组切换期间保持加载状态，空记录再展示空态', async () => {
    const pending = deferred<QualityPage>()
    mocks.runs.mockReturnValue(pending.promise)
    await view()
    expect(wrapper.text()).toContain('正在加载检测记录')
    expect(wrapper.text()).not.toContain('暂无符合条件')
    pending.resolve({ items: [], total: 0, pages: 1, page: 1, page_size: 12 })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无符合条件的检测记录')
  })
  it('直接打开详情，返回所属分组画廊', async () => {
    mocks.detail.mockResolvedValue(qualityRun(9, { group_id: 8 }))
    await view('/model-quality/runs/9')
    await wrapper.get('.quality-back').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query.group_id).toBe('8')
  })
  it('管理员已完成详情刷新迟到的账号关联，撤权响应不保留账号', async () => {
    const run = qualityRun(9, { account_hits: [] })
    run.tests[0].id = 'art'
    run.tests[0].generation = { status: 'completed', duration_ms: 100, attempts: 1 }
    mocks.detail
      .mockResolvedValueOnce(run)
      .mockResolvedValueOnce({
        ...run,
        account_hits: [
          {
            test_id: 'art',
            phase: 'generation',
            account_id: 740,
            account_name: '迟到的账号关联',
            source: 'usage'
          }
        ]
      })
      .mockResolvedValueOnce({ ...run, account_hits: undefined })
    await view('/model-quality/runs/9')
    expect(wrapper.text()).toContain('未记录')
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(wrapper.text()).toContain('迟到的账号关联')
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledTimes(3)
    expect(wrapper.text()).not.toContain('迟到的账号关联')
    expect(wrapper.find('.quality-account-hit').exists()).toBe(false)
  })
  it('进行中详情十五秒刷新，到期后立即卸载作品及答案', async () => {
    mocks.detail.mockResolvedValue(
      qualityRun(9, {
        outcome: 'running',
        finished_at: null,
        expires_at: new Date(Date.now() + 16000).toISOString()
      })
    )
    await view('/model-quality/runs/9')
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(1000)
    await flushPromises()
    expect(wrapper.find('pre').exists()).toBe(false)
    expect(wrapper.find('quality-preview-stub').exists()).toBe(false)
    expect(wrapper.text()).toContain('作品已过期')
  })
  it('切换详情时旧请求结束不解除新请求锁', async () => {
    const first = deferred<QualityGroup[]>(),
      second = deferred<QualityGroup[]>()
    mocks.groups.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    await view('/model-quality/runs/9')
    await router.push('/model-quality/runs/10')
    await flushPromises()
    first.resolve(qualityGroups)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(15000)
    expect(mocks.groups).toHaveBeenCalledTimes(2)
    second.resolve(qualityGroups)
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledTimes(1)
    expect(mocks.detail).toHaveBeenCalledWith(10, expect.any(AbortSignal))
  })
  it('详情功能关闭后即使仍在轮询也不显示旧答案', async () => {
    await view('/model-quality/runs/9')
    mocks.groups.mockRejectedValue({ status: 403 })
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(wrapper.find('pre').exists()).toBe(false)
    expect(wrapper.find('quality-preview-stub').exists()).toBe(false)
  })
})

it('UUID记录仅重判已存档作品，不再次发起生成', async () => {
  const id = 'v2-12345678-1234-4234-8234-123456789abc'
  const run = qualityRun(9, {
    id,
    tests: [
      {
        id: '12345678-1234-4234-8234-123456789def',
        benchmark: 'custom-art',
        name: '自定义动画',
        output_type: 'svg',
        rule_kind: 'model',
        status: 'failed',
        verdict: 'unknown',
        reason: '判题失败',
        has_html: true,
        html: '<svg/>',
        answer: '<svg/>',
        duration_ms: 100,
        finished_at: null,
        generation: {
          status: 'completed',
          answer: '<svg/>',
          attempts: 1,
          duration_ms: 80
        },
        judgment: {
          status: 'failed',
          reason: '网络中断',
          attempts: 1,
          duration_ms: 20
        }
      }
    ]
  })
  mocks.detail.mockResolvedValue(run)
  mocks.rejudge.mockResolvedValue({ queued: true })
  await view('/model-quality/runs/' + id)
  expect(wrapper.text()).toContain('自定义动画')
  await wrapper
    .findAll('button')
    .find((b) => b.text().includes('仅重新判题'))!
    .trigger('click')
  await flushPromises()
  expect(mocks.rejudge).toHaveBeenCalledWith(run.tests[0].id)
  expect(mocks.detail).toHaveBeenCalledTimes(2)
})
