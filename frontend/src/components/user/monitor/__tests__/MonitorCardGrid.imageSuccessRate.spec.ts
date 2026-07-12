import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import MonitorCardGrid from '../MonitorCardGrid.vue'
import type { ImageGroupSuccessRates } from '@/api/channelMonitor'

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

function mountGrid(imageGroupSuccessRates: ImageGroupSuccessRates) {
  return mount(MonitorCardGrid, {
    props: {
      items: [],
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
      },
    },
  })
}

describe('MonitorCardGrid Image 分组成功率', () => {
  it('在网格末尾展示组名、成功率和最后成功时间', () => {
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
    expect(card.text()).toContain('formatted:2026-07-12T01:02:03Z')
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
})
