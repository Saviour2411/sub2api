import { describe, expect, it } from 'vitest'

import { calculateBonusQuote } from '../bonus'

describe('payment bonus quote', () => {
  it('uses configured amount ranges before the legacy multiplier', () => {
    const quote = calculateBonusQuote(150, [
      { min_amount: 0, max_amount: 100, bonus_rate: 5 },
      { min_amount: 100, max_amount: null, bonus_rate: 10 },
    ], 2)

    expect(quote).toEqual({
      baseAmount: 150,
      bonusAmount: 15,
      bonusRate: 10,
      creditedAmount: 165,
    })
  })

  it('falls back to the legacy multiplier when no rules are configured', () => {
    expect(calculateBonusQuote(50, [], 1.2)).toEqual({
      baseAmount: 50,
      bonusAmount: 10,
      bonusRate: 20,
      creditedAmount: 60,
    })
  })
})
