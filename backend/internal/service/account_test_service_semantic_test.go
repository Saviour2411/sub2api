package service

import "testing"

func testSemanticSettingService(t *testing.T, platform string, pattern string, customMessage string) *SettingService {
	t.Helper()
	svc := &SettingService{}
	svc.refreshCachedSettings(&SystemSettings{
		SemanticErrorDetectionEnabled: true,
		SemanticErrorMatchMaxChars:    4096,
		SemanticErrorRules: []SemanticErrorRule{
			{
				Enabled:       true,
				Name:          "test-rule",
				Platforms:     []string{platform},
				MatchType:     "contains",
				Pattern:       pattern,
				CustomMessage: customMessage,
				Priority:      1,
			},
		},
	})
	t.Cleanup(func() {
		svc.refreshCachedSettings(&SystemSettings{})
	})
	return svc
}
