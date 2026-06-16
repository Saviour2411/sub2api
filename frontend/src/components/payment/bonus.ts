import type { PaymentBonusRule } from '@/types/payment'

export interface PaymentBonusQuote {
  baseAmount: number
  bonusAmount: number
  bonusRate: number
  creditedAmount: number
}

export function normalizeBonusRules(rules: PaymentBonusRule[] | null | undefined): PaymentBonusRule[] {
  return [...(rules || [])]
    .filter(rule => Number(rule.min_amount) >= 0 && Number(rule.bonus_rate) >= 0)
    .sort((a, b) => Number(a.min_amount) - Number(b.min_amount))
}

export function calculateBonusQuote(amount: number, rules: PaymentBonusRule[] | null | undefined, legacyMultiplier = 1): PaymentBonusQuote {
  const baseAmount = roundMoney(Number(amount) || 0)
  const normalizedRules = normalizeBonusRules(rules)
  if (normalizedRules.length === 0) {
    const multiplier = Number(legacyMultiplier) > 0 ? Number(legacyMultiplier) : 1
    const creditedAmount = roundMoney(baseAmount * multiplier)
    return {
      baseAmount,
      bonusAmount: roundMoney(creditedAmount - baseAmount),
      bonusRate: roundMoney((multiplier - 1) * 100),
      creditedAmount,
    }
  }

  const matched = normalizedRules.find(rule => {
    const min = Number(rule.min_amount) || 0
    const max = rule.max_amount == null ? null : Number(rule.max_amount)
    return baseAmount >= min && (max == null || baseAmount < max)
  })
  if (!matched) {
    return { baseAmount, bonusAmount: 0, bonusRate: 0, creditedAmount: baseAmount }
  }

  const bonusRate = Number(matched.bonus_rate) || 0
  const bonusAmount = roundMoney(baseAmount * bonusRate / 100)
  return {
    baseAmount,
    bonusAmount,
    bonusRate,
    creditedAmount: roundMoney(baseAmount + bonusAmount),
  }
}

export function roundMoney(value: number): number {
  return Math.round((Number(value) || 0) * 100) / 100
}
