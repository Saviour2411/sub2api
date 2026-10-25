import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import MonitorCardGrid from '../MonitorCardGrid.vue'
import type { ImageGroupSuccessRates, UserMonitorView } from '@/api/channelMonitor'
import MonitorCard from '../MonitorCard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => `formatted:${value}`,
}))

function mountGrid(imageGroupSuccessRates: ImageGroupSuccessRates, items: UserMonitorView[] = []) {
  return mount(MonitorCardGrid, {
    props: {
      items,
      window: '7d',
      countdownSeconds: 30,
      loading: false,
      detailCache: {},
      imageGroupSuccessRates,
    },
    global: {
      stubs: {
        EmptyState: { template: '<div data-test="empty-state" />' },
        MonitorCard: true,
        Icon: true,
        ProviderIcon: true,
      },
    },
  })
}

describe('MonitorCardGrid Image 分组成功率', () => {
  it('独立展示 Image 分组名称和成功率，隐藏最后成功时间', () => {
    const wrapper = mountGrid({
      visible: true,
      items: [
        {
          group_id: 7,
          group_name: 'Premium Image',
          success_rate: 92.345,
          last_success_at: '2026-07-12T01:02:03Z',
        },
      ],
    })

    const card = wrapper.get('[data-test="image-group-success-rate-card"]')
    expect(card.text()).toContain('Premium Image')
    expect(card.text()).toContain('92.34%')
    expect(card.text()).not.toContain('2026-07-12')
    expect(wrapper.get('[data-provider="image"]').text()).toContain('Image')
    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(false)
  })

  it('配置关闭时不展示成功率卡片', () => {
    const wrapper = mountGrid({
      visible: false,
      items: [
        {
          group_id: 7,
          group_name: 'Premium Image',
          success_rate: 100,
          last_success_at: null,
        },
      ],
    })

    expect(wrapper.find('[data-test="image-group-success-rate-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="empty-state"]').exists()).toBe(true)
  })
  it('按现有平台顺序分区，保留区内顺序并追加未知平台', () => {
    const item = (id: number, provider: string) => ({
      id, provider, name: `渠道${id}`, group_name: '分组', primary_model: 'model',
      primary_status: 'operational', primary_latency_ms: 1, primary_ping_latency_ms: 1,
      availability_7d: 100, timeline: [], extra_models: []
    } as UserMonitorView)
    const wrapper = mountGrid({ visible: false, items: [] }, [item(1, 'new-provider'), item(2, 'grok'), item(3, 'openai'), item(4, 'grok')])
    expect(wrapper.findAll('[data-provider]').map(section => section.attributes('data-provider'))).toEqual(['openai', 'grok', 'new-provider'])
    expect(wrapper.findAllComponents(MonitorCard).map(card => card.props('item').id)).toEqual([3, 2, 4, 1])
  })
  it('长周期缺少详情样本时保持无样本，真实零成功率仍保留', async () => {
    const item = { id: 1, provider: 'openai', primary_model: 'model', timeline: [] } as unknown as UserMonitorView
    const wrapper = mountGrid({ visible: false, items: [] }, [item])
    await wrapper.setProps({ window: '15d' })
    expect(wrapper.findComponent(MonitorCard).props('availabilityValue')).toBeNull()
    await wrapper.setProps({ detailCache: { 1: { models: [{ model: 'model', availability_15d: 0 }] } } })
    expect(wrapper.findComponent(MonitorCard).props('availabilityValue')).toBe(0)
  })
})
