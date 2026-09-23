import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MonitorCard from '../MonitorCard.vue'
import MonitorMetricPair from '../MonitorMetricPair.vue'
import type { UserMonitorView } from '@/api/channelMonitor'

vi.mock('@/utils/featureFlags', () => ({ isChannelMonitorQuotaVisible: () => false }))

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: { n: number }) => params ? `${key}:${params.n}` : key }),
}))

const item: UserMonitorView = {
  id: 1, name: '测试渠道', provider: 'openai', group_name: '', primary_model: 'model',
  primary_status: 'operational', primary_latency_ms: 2350, primary_ping_latency_ms: 100,
  availability_7d: null, samples_7d: 0, stale: true,
  last_checked_at: '2026-09-16T06:03:29Z', extra_models: [], timeline: [],
}

describe('渠道状态的新鲜度与无样本展示', () => {
  it('过期渠道不再展示正常或旧延迟，无样本不渲染成零百分比', () => {
    const wrapper = mount(MonitorCard, {
      props: { item, window: '7d', availabilityValue: null, sampleCount: 0, countdownSeconds: 60 },
      global: { stubs: { ProviderIcon: true, MonitorMetricPair: true, MonitorQuotaView: true } },
    })
    expect(wrapper.text()).toContain('monitorCommon.status.stale')
    expect(wrapper.text()).toContain('monitorCommon.noSamples')
    expect(wrapper.text()).toContain('monitorCommon.sampleCount:0')
    expect(wrapper.text()).not.toContain('0.00')
    expect(wrapper.text()).not.toContain('monitorCommon.status.operational')
    expect(wrapper.text()).not.toContain('monitorCommon.now')
    expect(wrapper.get('time').attributes('datetime')).toBe(item.last_checked_at)
    expect(wrapper.findComponent(MonitorMetricPair).props('primaryValue')).toBe('monitorCommon.latencyEmpty')
  })

  it('真实零成功率仍展示 0.00%，而非暂无数据', () => {
    const wrapper = mount(MonitorCard, {
      props: { item: { ...item, stale: false, primary_status: 'failed' }, window: '7d', availabilityValue: 0, sampleCount: 5, countdownSeconds: 60 },
      global: { stubs: { ProviderIcon: true, MonitorMetricPair: true, MonitorQuotaView: true } },
    })
    expect(wrapper.text()).toContain('0.00')
    expect(wrapper.text()).toContain('monitorCommon.sampleCount:5')
    expect(wrapper.text()).toContain('monitorCommon.status.failed')
  })
})
