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

  it.each([
    [10, 5, 10.5],
    [49.99, 5, 52.49],
    [50, 8, 54],
    [199.99, 8, 215.99],
    [200, 10, 220],
  ])('uses the production tier boundary for %s', (amount, bonusRate, creditedAmount) => {
    const quote = calculateBonusQuote(amount, [
      { min_amount: 10, max_amount: 50, bonus_rate: 5 },
      { min_amount: 50, max_amount: 200, bonus_rate: 8 },
      { min_amount: 200, max_amount: null, bonus_rate: 10 },
    ])

    expect(quote.bonusRate).toBe(bonusRate)
    expect(quote.creditedAmount).toBe(creditedAmount)
  })

  it('keeps payment and credited amounts equal below the first bonus tier', () => {
    expect(calculateBonusQuote(9.99, [
      { min_amount: 10, max_amount: null, bonus_rate: 5 },
    ])).toEqual({
      baseAmount: 9.99,
      bonusAmount: 0,
      bonusRate: 0,
      creditedAmount: 9.99,
    })
  })
})
