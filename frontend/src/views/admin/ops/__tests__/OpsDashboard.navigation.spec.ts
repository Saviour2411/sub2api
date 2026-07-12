import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import OpsDashboard from '../OpsDashboard.vue'

const { opsAPI, routerReplace, settingsFetch } = vi.hoisted(() => ({
  opsAPI: {
    getAdvancedSettings: vi.fn(),
    getMetricThresholds: vi.fn(),
    getDashboardSnapshotV2: vi.fn(),
    getDashboardOverview: vi.fn(),
    getThroughputTrend: vi.fn(),
    getErrorTrend: vi.fn(),
    getLatencyHistogram: vi.fn(),
    getErrorDistribution: vi.fn(),
  },
  routerReplace: vi.fn(),
  settingsFetch: vi.fn(),
}))

vi.mock('@/api/admin/ops', () => ({ default: opsAPI, opsAPI }))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
  useAdminSettingsStore: () => ({
    opsMonitoringEnabled: true,
    opsQueryModeDefault: 'auto',
    fetch: settingsFetch,
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} }),
  useRouter: () => ({ replace: routerReplace }),
}))

vi.mock('@vueuse/core', () => ({
  useDebounceFn: (fn: (...args: unknown[]) => unknown) => fn,
  useIntervalFn: () => ({ pause: vi.fn(), resume: vi.fn() }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const DashboardHeaderStub = defineComponent({
  emits: ['openRequestDetails', 'openErrorDetails'],
  template: `
    <div>
      <button data-test="open-request-details" @click="$emit('openRequestDetails')">请求明细</button>
      <button data-test="open-error-details" @click="$emit('openErrorDetails', 'request')">错误明细</button>
    </div>
  `,
})

const ErrorDetailsStub = defineComponent({
  props: {
    show: { type: Boolean, default: false },
    closeOnEscape: { type: Boolean, default: true },
  },
  emits: ['update:show', 'openErrorDetail'],
  template: `
    <div v-if="show" data-test="error-details">
      <input data-test="error-details-filter" />
      <button data-test="error-details-row" @click="$emit('openErrorDetail', 101)">查看错误</button>
      <button data-test="close-error-details" @click="$emit('update:show', false)">关闭错误明细</button>
    </div>
  `,
})

const RequestDetailsStub = defineComponent({
  props: {
    modelValue: { type: Boolean, default: false },
    closeOnEscape: { type: Boolean, default: true },
  },
  emits: ['update:modelValue', 'openErrorDetail'],
  template: `
    <div v-if="modelValue" data-test="request-details">
      <input data-test="request-details-filter" />
      <button data-test="request-details-row" @click="$emit('openErrorDetail', 202)">查看错误</button>
      <button data-test="close-request-details" @click="$emit('update:modelValue', false)">关闭请求明细</button>
    </div>
  `,
})

const ErrorDetailStub = defineComponent({
  props: {
    show: { type: Boolean, default: false },
    errorId: { type: Number, default: null },
  },
  emits: ['update:show'],
  template: `
    <div v-if="show" data-test="error-detail">
      <span data-test="error-detail-id">{{ errorId }}</span>
      <button data-test="close-error-detail" @click="$emit('update:show', false)">关闭错误详情</button>
    </div>
  `,
})

function mountDashboard() {
  return mount(OpsDashboard, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: true,
        OpsDashboardSkeleton: true,
        OpsDashboardHeader: DashboardHeaderStub,
        OpsConcurrencyCard: true,
        OpsSwitchRateTrendChart: true,
        OpsThroughputTrendChart: true,
        OpsLatencyChart: true,
        OpsErrorDistributionChart: true,
        OpsErrorTrendChart: true,
        OpsOpenAITokenStatsCard: true,
        OpsAlertEventsCard: true,
        OpsSystemLogTable: true,
        OpsSettingsDialog: true,
        OpsAlertRulesCard: true,
        OpsErrorDetailsModal: ErrorDetailsStub,
        OpsRequestDetailsModal: RequestDetailsStub,
        OpsErrorDetailModal: ErrorDetailStub,
      },
    },
  })
}

describe('运维监控错误详情返回层级', () => {
  beforeEach(() => {
    Object.values(opsAPI).forEach((mock) => mock.mockReset())
    routerReplace.mockReset().mockResolvedValue(undefined)
    settingsFetch.mockReset().mockResolvedValue(undefined)
    opsAPI.getAdvancedSettings.mockResolvedValue({
      display_alert_events: false,
      display_openai_token_stats: false,
      auto_refresh_enabled: false,
      auto_refresh_interval_seconds: 30,
    })
    opsAPI.getMetricThresholds.mockResolvedValue(null)
    opsAPI.getDashboardSnapshotV2.mockResolvedValue({
      overview: null,
      throughput_trend: null,
      error_trend: null,
    })
    opsAPI.getDashboardOverview.mockResolvedValue(null)
    opsAPI.getThroughputTrend.mockResolvedValue(null)
    opsAPI.getErrorTrend.mockResolvedValue(null)
    opsAPI.getLatencyHistogram.mockResolvedValue(null)
    opsAPI.getErrorDistribution.mockResolvedValue(null)
  })

  afterEach(() => {
    document.body.classList.remove('modal-open')
  })

  it('从错误明细打开单条详情后返回错误明细并保留筛选状态', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    await wrapper.get('[data-test="open-error-details"]').trigger('click')
    await wrapper.get('[data-test="error-details-filter"]').setValue('gateway')
    await wrapper.get('[data-test="error-details-row"]').trigger('click')

    expect(wrapper.get('[data-test="error-detail-id"]').text()).toBe('101')
    expect(wrapper.findComponent(ErrorDetailsStub).props('closeOnEscape')).toBe(false)

    await wrapper.get('[data-test="close-error-detail"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-test="error-detail"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="error-details"]').exists()).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-test="error-details-filter"]').element.value).toBe('gateway')
  })

  it('从请求明细打开单条详情后返回请求明细并保留筛选状态', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    await wrapper.get('[data-test="open-request-details"]').trigger('click')
    await wrapper.get('[data-test="request-details-filter"]').setValue('gpt-5')
    await wrapper.get('[data-test="request-details-row"]').trigger('click')

    expect(wrapper.get('[data-test="error-detail-id"]').text()).toBe('202')
    expect(wrapper.findComponent(RequestDetailsStub).props('closeOnEscape')).toBe(false)

    await wrapper.get('[data-test="close-error-detail"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-test="error-detail"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="request-details"]').exists()).toBe(true)
    expect(wrapper.get<HTMLInputElement>('[data-test="request-details-filter"]').element.value).toBe('gpt-5')
  })

  it('直接打开单条详情后关闭时返回监控看板', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const vm = wrapper.vm as unknown as { openError: (id: number) => void }
    vm.openError(303)
    await nextTick()
    expect(wrapper.get('[data-test="error-detail-id"]').text()).toBe('303')

    await wrapper.get('[data-test="close-error-detail"]').trigger('click')
    await nextTick()

    expect(wrapper.find('[data-test="error-detail"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="error-details"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="request-details"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="open-error-details"]').exists()).toBe(true)
  })
})
