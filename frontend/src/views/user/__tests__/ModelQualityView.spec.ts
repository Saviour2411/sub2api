import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ModelQualityView from '../ModelQualityView.vue'
import type { QualityRun } from '@/api/modelQuality'

const mocks = vi.hoisted(() => ({
  groups: vi.fn(),
  runs: vi.fn(),
  detail: vi.fn(),
  fetchSettings: vi.fn()
}))
vi.mock('@/api/modelQuality', async (original) => ({
  ...(await original<typeof import('@/api/modelQuality')>()),
  modelQualityAPI: mocks
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ fetchPublicSettings: mocks.fetchSettings })
}))
const run: QualityRun = {
  id: 9,
  group_id: 7,
  group_name: '测试分组',
  model: 'model-a',
  reasoning_effort: 'low',
  outcome: 'failed',
  started_at: '2026-10-15T01:00:00Z',
  finished_at: '2026-10-15T01:01:00Z',
  expires_at: '2099-01-01T00:00:00Z',
  tests: [
    {
      benchmark: 'pelican',
      status: 'completed',
      verdict: 'normal',
      reason: '动画完整',
      has_html: true,
      duration_ms: 1000,
      finished_at: null
    },
    {
      benchmark: 'candy',
      status: 'failed',
      verdict: 'unknown',
      reason: '检测超时',
      answer: '<script>danger()</script>',
      has_html: false,
      duration_ms: 1000,
      finished_at: null
    }
  ]
}
const mounted: ReturnType<typeof mount>[] = []
function view() {
  const wrapper = mount(ModelQualityView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        QualityPreview: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show" role="dialog"><slot /></div>' }
      }
    }
  })
  mounted.push(wrapper)
  return wrapper
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  mocks.groups.mockResolvedValue([
    {
      group_id: 7,
      name: '测试分组',
      models: ['model-a', 'model-b'],
      methods: ['candy', 'pelican'],
      status: 'failed',
      running: false,
      last_started_at: null,
      next_run_at: null
    },
    {
      group_id: 8,
      name: '另一分组',
      models: ['model-c'],
      methods: ['candy'],
      status: 'paused',
      pause_reason: '待绑定密钥',
      running: false
    }
  ])
  mocks.runs.mockResolvedValue({ items: [run], total: 25, page: 1, page_size: 12, pages: 3 })
  mocks.detail.mockResolvedValue({
    ...run,
    tests: run.tests.map((test) => ({
      ...test,
      html: test.benchmark === 'pelican' ? '<canvas></canvas>' : undefined
    }))
  })
})
afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount())
  vi.useRealTimers()
})
describe('降智检测画廊', () => {
  it('分别展示测试状态，糖果答案按纯文本展示，点击后才读取详情', async () => {
    const wrapper = view()
    await flushPromises()
    expect(wrapper.text()).toContain('鹈鹕 · 正常')
    expect(wrapper.text()).toContain('糖果 · 检测失败')
    expect(wrapper.text()).toContain('<script>danger()</script>')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(mocks.detail).not.toHaveBeenCalled()
    await wrapper.get('.quality-card-open').trigger('click')
    await flushPromises()
    expect(mocks.detail).toHaveBeenCalledWith(9, expect.any(AbortSignal))
    expect(wrapper.get('[role="dialog"]').text()).toContain('执行明细')
  })
  it('切换筛选和分组重置分页，定时刷新保留筛选', async () => {
    const wrapper = view()
    await flushPromises()
    await wrapper.get('[aria-label="下一页"]').trigger('click')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0].page).toBe(2)
    await wrapper.get('[aria-label="筛选模型"]').setValue('model-b')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({ page: 1, model: 'model-b' })
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({ page: 1, model: 'model-b' })
    await wrapper.findAll('.quality-group-tabs button')[1].trigger('click')
    await flushPromises()
    expect(mocks.runs.mock.lastCall?.[0]).toMatchObject({ group_id: 8, page: 1, model: '' })
    expect(wrapper.text()).toContain('待绑定密钥')
  })
  it('关闭功能后清空作品和弹窗并刷新功能开关', async () => {
    const wrapper = view()
    await flushPromises()
    mocks.groups.mockRejectedValue({ status: 404 })
    await wrapper.get('[aria-label="刷新检测结果"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('降智检测已关闭')
    expect(wrapper.find('.quality-card').exists()).toBe(false)
    expect(mocks.fetchSettings).toHaveBeenCalledWith(true)
  })
  it('展示空记录状态', async () => {
    mocks.runs.mockResolvedValue({ items: [], total: 0, pages: 1 })
    const wrapper = view()
    await flushPromises()
    expect(wrapper.text()).toContain('暂无符合条件的检测记录')
  })
})
