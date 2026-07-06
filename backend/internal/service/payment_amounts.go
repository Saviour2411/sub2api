package service

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

const defaultBalanceRechargeMultiplier = 1.0

type PaymentBonusRule struct {
	MinAmount float64  `json:"min_amount"`
	MaxAmount *float64 `json:"max_amount,omitempty"`
	BonusRate float64  `json:"bonus_rate"`
}

type PaymentBonusQuote struct {
	BaseAmount     float64           `json:"base_amount"`
	BonusAmount    float64           `json:"bonus_amount"`
	BonusRate      float64           `json:"bonus_rate"`
	CreditedAmount float64           `json:"credited_amount"`
	Rule           *PaymentBonusRule `json:"rule,omitempty"`
}

func normalizeBalanceRechargeMultiplier(multiplier float64) float64 {
	if math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return defaultBalanceRechargeMultiplier
	}
	return multiplier
}

// normalizeSubscriptionUSDToCNYRate 将非法值归一为 0（换算关闭）。
// 与余额倍率不同，0 是合法状态：表示订阅保持 price 直付的存量行为。
func normalizeSubscriptionUSDToCNYRate(rate float64) float64 {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return 0
	}
	return rate
}

func calculateCreditedBalance(paymentAmount, multiplier float64) float64 {
	return decimal.NewFromFloat(paymentAmount).
		Mul(decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(multiplier))).
		Round(2).
		InexactFloat64()
}

func normalizePaymentBonusRules(rules []PaymentBonusRule) ([]PaymentBonusRule, error) {
	out := make([]PaymentBonusRule, 0, len(rules))
	for _, rule := range rules {
		if math.IsNaN(rule.MinAmount) || math.IsInf(rule.MinAmount, 0) || rule.MinAmount < 0 {
			return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule min_amount must be non-negative")
		}
		if math.IsNaN(rule.BonusRate) || math.IsInf(rule.BonusRate, 0) || rule.BonusRate < 0 || rule.BonusRate > 1000 {
			return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule bonus_rate must be between 0 and 1000")
		}
		if !decimal.NewFromFloat(rule.MinAmount).Equal(decimal.NewFromFloat(rule.MinAmount).Round(2)) ||
			!decimal.NewFromFloat(rule.BonusRate).Equal(decimal.NewFromFloat(rule.BonusRate).Round(2)) {
			return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule amounts and rates allow at most 2 decimal places")
		}
		if rule.MaxAmount != nil {
			maxAmount := *rule.MaxAmount
			if math.IsNaN(maxAmount) || math.IsInf(maxAmount, 0) || maxAmount <= rule.MinAmount {
				return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule max_amount must be greater than min_amount")
			}
			if !decimal.NewFromFloat(maxAmount).Equal(decimal.NewFromFloat(maxAmount).Round(2)) {
				return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule max_amount allows at most 2 decimal places")
			}
			rule.MaxAmount = &maxAmount
		}
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].MinAmount < out[j].MinAmount
	})
	for i := 1; i < len(out); i++ {
		prev := out[i-1]
		curr := out[i]
		if prev.MaxAmount == nil || curr.MinAmount < *prev.MaxAmount {
			return nil, infraerrors.BadRequest("INVALID_PAYMENT_BONUS_RULES", "bonus rule ranges must not overlap")
		}
	}
	return out, nil
}

func parsePaymentBonusRules(raw string) []PaymentBonusRule {
	if raw == "" {
		return nil
	}
	var rules []PaymentBonusRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil
	}
	normalized, err := normalizePaymentBonusRules(rules)
	if err != nil {
		return nil
	}
	return normalized
}

func formatPaymentBonusRules(rules []PaymentBonusRule) (string, error) {
	normalized, err := normalizePaymentBonusRules(rules)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "", nil
	}
	b, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func calculateBonusQuote(paymentAmount float64, rules []PaymentBonusRule, legacyMultiplier float64) PaymentBonusQuote {
	baseAmount := decimal.NewFromFloat(paymentAmount).Round(2).InexactFloat64()
	quote := PaymentBonusQuote{
		BaseAmount:     baseAmount,
		CreditedAmount: baseAmount,
	}
	if len(rules) == 0 {
		creditedAmount := calculateCreditedBalance(baseAmount, legacyMultiplier)
		return PaymentBonusQuote{
			BaseAmount:     baseAmount,
			BonusAmount:    decimal.NewFromFloat(creditedAmount).Sub(decimal.NewFromFloat(baseAmount)).Round(2).InexactFloat64(),
			BonusRate:      decimal.NewFromFloat(normalizeBalanceRechargeMultiplier(legacyMultiplier)).Sub(decimal.NewFromInt(1)).Mul(decimal.NewFromInt(100)).Round(2).InexactFloat64(),
			CreditedAmount: creditedAmount,
		}
	}
	for _, rule := range rules {
		if baseAmount < rule.MinAmount {
			continue
		}
		if rule.MaxAmount != nil && baseAmount >= *rule.MaxAmount {
			continue
		}
		bonusAmount := decimal.NewFromFloat(baseAmount).
			Mul(decimal.NewFromFloat(rule.BonusRate)).
			Div(decimal.NewFromInt(100)).
			Round(2).
			InexactFloat64()
		ruleCopy := rule
		return PaymentBonusQuote{
			BaseAmount:     baseAmount,
			BonusAmount:    bonusAmount,
			BonusRate:      rule.BonusRate,
			CreditedAmount: decimal.NewFromFloat(baseAmount).Add(decimal.NewFromFloat(bonusAmount)).Round(2).InexactFloat64(),
			Rule:           &ruleCopy,
		}
	}
	return quote
}

func calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount float64, currency string) float64 {
	if orderAmount <= 0 || payAmount <= 0 || refundAmount <= 0 {
		return 0
	}
	fractionDigits := int32(payment.CurrencyMaxFractionDigits(currency))
	if math.Abs(refundAmount-orderAmount) <= paymentAmountToleranceForCurrency(currency) {
		return decimal.NewFromFloat(payAmount).Round(fractionDigits).InexactFloat64()
	}
	return decimal.NewFromFloat(payAmount).
		Mul(decimal.NewFromFloat(refundAmount)).
		Div(decimal.NewFromFloat(orderAmount)).
		Round(fractionDigits).
		InexactFloat64()
}
