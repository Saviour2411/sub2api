package service

import "testing"

func TestAccountSupportsOpenAIImageSizeTier(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra: map[string]any{
			"openai_image_size_tiers": []any{"1K", "4K"},
		},
	}

	if !account.SupportsOpenAIImageSizeTier("1K") {
		t.Fatal("expected 1K to be supported")
	}
	if account.SupportsOpenAIImageSizeTier("2K") {
		t.Fatal("expected 2K to be rejected")
	}
	if !account.SupportsOpenAIImageSizeTier("4096x4096") {
		t.Fatal("expected 4096x4096 to normalize to 4K")
	}
}

func TestAccountSupportsOpenAIImageSizeTier_UnsetOrInvalidKeepsLegacyBehavior(t *testing.T) {
	for _, account := range []*Account{
		{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_image_size_tiers": []any{}}},
		{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"openai_image_size_tiers": []any{"bad"}}},
	} {
		if !account.SupportsOpenAIImageSizeTier("2K") {
			t.Fatalf("expected unset/empty/invalid tiers to keep legacy behavior: %#v", account.Extra)
		}
	}
}
