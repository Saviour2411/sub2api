import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

import UsageStatsCards from '../UsageStatsCards.vue'

const messages: Record<string, string> = {
  'usage.totalRequests': 'Total Requests',
  'usage.inSelectedRange': 'in selected range',
  'usage.totalTokens': 'Total Tokens',
  'usage.in': 'In',
  'usage.out': 'Out',
  'usage.cacheTotal': 'Cache',
  'usage.cacheBreakdown': 'Cache Token Breakdown',
  'usage.cacheCreationTokensLabel': 'Cache Creation',
  'usage.cacheReadTokensLabel': 'Cache Read',
  'usage.totalCost': 'Total Cost',
  'usage.accountCost': 'Cost',
  'usage.standardCost': 'Standard',
  'usage.avgDuration': 'Avg Duration',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const stats = {
  total_requests: 1,
  total_input_tokens: 100,
  total_output_tokens: 50,
  total_cache_tokens: 34,
  total_cache_creation_tokens: 12,
  total_cache_read_tokens: 22,
  total_tokens: 184,
  total_cost: 0.001,
  total_actual_cost: 0.001,
  total_account_cost: 0.001,
  average_duration_ms: 250,
}

enableAutoUnmount(afterEach)
afterEach(() => { document.body.innerHTML = '' })

describe('UsageStatsCards', () => {
  it('悬停时在卡片外展示完整缓存明细', async () => {
    const wrapper = mount(UsageStatsCards, {
      attachTo: document.body,
      props: { stats },
      global: { stubs: { Icon: true } },
    })
    expect(wrapper.text()).toContain('Cache: 34')
    const trigger = wrapper.getComponent(HelpTooltip)
    await trigger.trigger('mouseenter')
    await nextTick()
    const tooltip = document.body.querySelector<HTMLElement>('[role="tooltip"]')!
    expect(tooltip.parentElement).toBe(document.body)
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('Cache Token Breakdown')
    expect(tooltip.textContent).toContain('Cache Creation')
    expect(tooltip.textContent).toContain('12')
    expect(tooltip.textContent).toContain('Cache Read')
    expect(tooltip.textContent).toContain('22')
    expect(wrapper.element.contains(tooltip)).toBe(false)
  })

  it('隐藏时不占布局空间，键盘聚焦也能打开并关闭明细', async () => {
    const wrapper = mount(UsageStatsCards, {
      attachTo: document.body,
      props: { stats },
      global: { stubs: { Icon: true } },
    })
    const tooltip = document.body.querySelector<HTMLElement>('[role="tooltip"]')!
    expect(tooltip.style.display).toBe('none')
    const trigger = wrapper.getComponent(HelpTooltip)
    await trigger.trigger('focusin')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    await trigger.trigger('focusout')
    expect(tooltip.style.display).toBe('none')
  })
})
