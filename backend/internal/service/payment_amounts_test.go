package service

import "testing"

func TestCalculateBonusQuoteWithConfiguredRules(t *testing.T) {
	max500 := 500.0
	rules := []PaymentBonusRule{
		{MinAmount: 100, MaxAmount: &max500, BonusRate: 5},
		{MinAmount: 500, BonusRate: 10},
	}

	tests := []struct {
		name           string
		amount         float64
		wantBonus      float64
		wantRate       float64
		wantCredited   float64
		wantRuleMin    float64
		wantRuleExists bool
	}{
		{name: "below first tier has no bonus", amount: 99.99, wantBonus: 0, wantRate: 0, wantCredited: 99.99},
		{name: "min is inclusive", amount: 100, wantBonus: 5, wantRate: 5, wantCredited: 105, wantRuleMin: 100, wantRuleExists: true},
		{name: "max is exclusive", amount: 500, wantBonus: 50, wantRate: 10, wantCredited: 550, wantRuleMin: 500, wantRuleExists: true},
		{name: "rounds bonus to cents", amount: 123.45, wantBonus: 6.17, wantRate: 5, wantCredited: 129.62, wantRuleMin: 100, wantRuleExists: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateBonusQuote(tt.amount, rules, 9)
			if got.BaseAmount != tt.amount {
				t.Fatalf("BaseAmount = %v, want %v", got.BaseAmount, tt.amount)
			}
			if got.BonusAmount != tt.wantBonus {
				t.Fatalf("BonusAmount = %v, want %v", got.BonusAmount, tt.wantBonus)
			}
			if got.BonusRate != tt.wantRate {
				t.Fatalf("BonusRate = %v, want %v", got.BonusRate, tt.wantRate)
			}
			if got.CreditedAmount != tt.wantCredited {
				t.Fatalf("CreditedAmount = %v, want %v", got.CreditedAmount, tt.wantCredited)
			}
			if tt.wantRuleExists {
				if got.Rule == nil || got.Rule.MinAmount != tt.wantRuleMin {
					t.Fatalf("Rule = %#v, want min %v", got.Rule, tt.wantRuleMin)
				}
			} else if got.Rule != nil {
				t.Fatalf("Rule = %#v, want nil", got.Rule)
			}
		})
	}
}

func TestCalculateBonusQuoteLegacyMultiplierFallback(t *testing.T) {
	got := calculateBonusQuote(100, nil, 1.2)
	if got.BaseAmount != 100 {
		t.Fatalf("BaseAmount = %v, want 100", got.BaseAmount)
	}
	if got.BonusAmount != 20 {
		t.Fatalf("BonusAmount = %v, want 20", got.BonusAmount)
	}
	if got.BonusRate != 20 {
		t.Fatalf("BonusRate = %v, want 20", got.BonusRate)
	}
	if got.CreditedAmount != 120 {
		t.Fatalf("CreditedAmount = %v, want 120", got.CreditedAmount)
	}
}

func TestPaymentBonusRulesValidation(t *testing.T) {
	max200 := 200.0
	max300 := 300.0

	if _, err := normalizePaymentBonusRules([]PaymentBonusRule{
		{MinAmount: 100, MaxAmount: &max300, BonusRate: 5},
		{MinAmount: 200, BonusRate: 10},
	}); err == nil {
		t.Fatal("expected overlapping ranges to fail")
	}

	normalized, err := normalizePaymentBonusRules([]PaymentBonusRule{
		{MinAmount: 300, BonusRate: 8},
		{MinAmount: 100, MaxAmount: &max200, BonusRate: 5},
	})
	if err != nil {
		t.Fatalf("normalizePaymentBonusRules returned error: %v", err)
	}
	if len(normalized) != 2 || normalized[0].MinAmount != 100 || normalized[1].MinAmount != 300 {
		t.Fatalf("normalized order = %#v", normalized)
	}
}

func TestFormatAndParsePaymentBonusRules(t *testing.T) {
	max500 := 500.0
	raw, err := formatPaymentBonusRules([]PaymentBonusRule{
		{MinAmount: 100, MaxAmount: &max500, BonusRate: 5},
		{MinAmount: 500, BonusRate: 10},
	})
	if err != nil {
		t.Fatalf("formatPaymentBonusRules returned error: %v", err)
	}
	if raw == "" {
		t.Fatal("formatPaymentBonusRules returned empty string")
	}

	parsed := parsePaymentBonusRules(raw)
	if len(parsed) != 2 {
		t.Fatalf("parsePaymentBonusRules len = %d, want 2", len(parsed))
	}
	if parsed[0].MaxAmount == nil || *parsed[0].MaxAmount != 500 {
		t.Fatalf("parsed first MaxAmount = %#v, want 500", parsed[0].MaxAmount)
	}
	if parsed[1].MaxAmount != nil {
		t.Fatalf("parsed second MaxAmount = %#v, want nil", parsed[1].MaxAmount)
	}
}
