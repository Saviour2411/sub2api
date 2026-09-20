package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationEngineSwitchPreservesCustomSettings(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModePreBlock
	cfg.CyberPolicyExcludeFromBanCount = true
	cfg.LocalAuditEnabled = true
	cfg.LocalAuditMaxStorageGB = 2.5
	cfg.LocalAuditScenePolicy = ContentModerationLocalAuditSceneProgrammingOnly
	cfg.LocalAuditExcludeImage = true
	cfg.LocalAuditClientPatterns = []string{"codex_", "cursor"}
	cfg.LocalAuditToolPatterns = []string{"apply_patch", "read_file"}
	cfg.LocalAuditMaxCaptureConcurrency = 17
	cfg.ModelFilter = ContentModerationModelFilter{Type: ContentModerationModelFilterInclude, Models: []string{"business-model"}}
	cfg.APIKeys = []string{"test-openai-key"}
	cfg.TypeSafe = moderationEngineDefaults(ContentModerationEngineTypeSafe)
	cfg.TypeSafe.APIKeys = []string{"test-typesafe-key"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &contentModerationTestSettingRepo{values: map[string]string{SettingKeyContentModerationConfig: string(raw)}}
	svc := &ContentModerationService{settingRepo: repo}
	var expected map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &expected))
	assertCustom := func(value any) {
		t.Helper()
		data, err := json.Marshal(value)
		require.NoError(t, err)
		var actual map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(data, &actual))
		for _, field := range []string{
			"enabled", "mode", "cyber_policy_exclude_from_ban_count", "model_filter",
			"local_audit_enabled", "local_audit_max_storage_gb", "local_audit_scene_policy",
			"local_audit_exclude_image_generation", "local_audit_programming_client_patterns",
			"local_audit_programming_tool_patterns", "local_audit_max_capture_concurrency",
		} {
			require.JSONEq(t, string(expected[field]), string(actual[field]), field)
		}
	}
	for _, engine := range []string{ContentModerationEngineTypeSafe, ContentModerationEngineOpenAI} {
		t.Run(engine, func(t *testing.T) {
			view, err := svc.UpdateConfig(context.Background(), UpdateContentModerationConfigInput{Engine: &engine})
			require.NoError(t, err)
			require.Equal(t, engine, view.Engine)
			assertCustom(view)
			for _, profile := range view.EngineConfigs {
				assertCustom(profile)
			}
			var stored ContentModerationConfig
			require.NoError(t, json.Unmarshal([]byte(repo.values[SettingKeyContentModerationConfig]), &stored))
			assertCustom(&stored)
			require.Equal(t, []string{"test-openai-key"}, stored.APIKeys)
			require.Equal(t, []string{"test-typesafe-key"}, stored.TypeSafe.APIKeys)
			loaded, err := svc.GetConfig(context.Background())
			require.NoError(t, err)
			assertCustom(loaded)
		})
	}
}

func TestContentModerationEngineSnapshotClonesCustomPatterns(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.LocalAuditClientPatterns = []string{"codex_"}
	cfg.LocalAuditToolPatterns = []string{"apply_patch"}
	cfg.TypeSafe = moderationEngineDefaults(ContentModerationEngineTypeSafe)
	cfg.TypeSafe.APIKeys = []string{"test-typesafe-key"}
	proxyID := int64(7)
	cfg.TypeSafe.ProxyID = &proxyID
	cloned := cloneContentModerationConfig(cfg)
	cloned.LocalAuditClientPatterns[0] = "changed"
	cloned.LocalAuditToolPatterns[0] = "changed"
	cloned.TypeSafe.APIKeys[0] = "changed"
	cloned.TypeSafe.Thresholds["sexual"] = 0.99
	*cloned.TypeSafe.ProxyID = 9
	require.Equal(t, []string{"codex_"}, cfg.LocalAuditClientPatterns)
	require.Equal(t, []string{"apply_patch"}, cfg.LocalAuditToolPatterns)
	require.Equal(t, []string{"test-typesafe-key"}, cfg.TypeSafe.APIKeys)
	require.Equal(t, 0.65, cfg.TypeSafe.Thresholds["sexual"])
	require.Equal(t, int64(7), *cfg.TypeSafe.ProxyID)
}
