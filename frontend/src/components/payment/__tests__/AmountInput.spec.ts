import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AmountInput from '../AmountInput.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

describe('AmountInput bonus badges', () => {
  it('renders each configured bonus in the quick amount top-right corner', () => {
    const labels: Record<number, string> = { 10: '+5%', 50: '+8%', 200: '+10%' }
    const wrapper = mount(AmountInput, {
      props: {
        amounts: [10, 50, 200],
        modelValue: null,
        bonusLabel: (amount: number) => labels[amount] || '',
      },
    })

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(3)
    expect(buttons.map(button => button.text())).toEqual(['10+5%', '50+8%', '200+10%'])

    for (const button of buttons) {
      const badge = button.find('span.absolute')
      expect(badge.exists()).toBe(true)
      expect(badge.classes()).toContain('right-1.5')
      expect(badge.classes()).toContain('top-1.5')
    }
  })
})
