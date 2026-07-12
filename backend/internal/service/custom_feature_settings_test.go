//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type customFeatureSettingsRepoStub struct {
	values  map[string]string
	updates map[string]string
}

func (s *customFeatureSettingsRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *customFeatureSettingsRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *customFeatureSettingsRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *customFeatureSettingsRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *customFeatureSettingsRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	s.updates = make(map[string]string, len(values))
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range values {
		s.updates[key] = value
		s.values[key] = value
	}
	return nil
}

func (s *customFeatureSettingsRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *customFeatureSettingsRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type customFeatureGroupReaderStub struct {
	groups map[int64]*Group
}

func (s *customFeatureGroupReaderStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func TestSettingService_GetCustomFeatureSettings_读取生产形态配置(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyModelMarketplaceEnabled:          "true",
		SettingKeyModelMarketplaceIntro:            "  模型说明  ",
		SettingKeyModelMarketplaceGroupIDs:         `[3,0,2,3,-1]`,
		SettingKeyDailyCheckinEnabled:              "true",
		SettingKeyDailyCheckinMode:                 "range",
		SettingKeyDailyCheckinAmount:               "1",
		SettingKeyDailyCheckinMin:                  "0.1",
		SettingKeyDailyCheckinMax:                  "0.3",
		SettingKeyDailyCheckinPrizes:               `[ {"id":"none","name":"谢谢参与","type":"none","probability_bps":500,"enabled":true}, {"id":"balance","name":"余额","type":"balance","probability_bps":9500,"enabled":true,"balance_mode":"fixed","amount":0.1} ]`,
		SettingKeyDailyCheckinUnpaidFullDays:       "7",
		SettingKeyDailyCheckinUnpaidDecayRules:     `[{"after_days":30,"factor_bps":500},{"after_days":7,"factor_bps":5000},{"after_days":14,"factor_bps":2000}]`,
		SettingKeyDailyCheckinLinuxDoExemptEnabled: "true",
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetCustomFeatureSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ModelMarketplace.Enabled)
	require.Equal(t, "模型说明", settings.ModelMarketplace.Intro)
	require.Equal(t, []int64{3, 2}, settings.ModelMarketplace.GroupIDs)
	require.True(t, settings.DailyCheckin.Enabled)
	require.Len(t, settings.DailyCheckin.Prizes, 2)
	require.Equal(t, 7, settings.DailyCheckin.UnpaidDecayRules[0].AfterDays)
	require.Equal(t, 500, settings.DailyCheckin.UnpaidDecayRules[2].FactorBps)
	require.True(t, settings.DailyCheckin.LinuxDoExemptEnabled)
}

func TestSettingService_GetCustomFeatureSettings_旧版奖励转换为奖项(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyDailyCheckinMode:   "range",
		SettingKeyDailyCheckinMin:    "1.2",
		SettingKeyDailyCheckinMax:    "2.4",
		SettingKeyDailyCheckinAmount: "1",
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetCustomFeatureSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ModelMarketplace.Enabled)
	require.Empty(t, settings.ModelMarketplace.GroupIDs)
	require.False(t, settings.DailyCheckin.Enabled)
	require.Len(t, settings.DailyCheckin.Prizes, 1)
	require.Equal(t, "range", settings.DailyCheckin.Prizes[0].BalanceMode)
	require.Equal(t, 1.2, settings.DailyCheckin.Prizes[0].MinAmount)
	require.Equal(t, 2.4, settings.DailyCheckin.Prizes[0].MaxAmount)
	require.Equal(t, DefaultDailyCheckinDecayRules(), settings.DailyCheckin.UnpaidDecayRules)
}

func TestSettingService_UpdateModelMarketplaceSettings_仅写专属键并刷新缓存(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{
		2: {ID: 2, Status: StatusActive},
	}})
	updated := 0
	svc.SetOnUpdateCallback(func() { updated++ })

	settings, err := svc.UpdateModelMarketplaceSettings(context.Background(), ModelMarketplaceSettings{
		Enabled:  true,
		Intro:    "  hello  ",
		GroupIDs: []int64{2, 2, 0, -1},
	})
	require.NoError(t, err)
	require.Equal(t, "hello", settings.Intro)
	require.Equal(t, []int64{2}, settings.GroupIDs)
	require.Len(t, repo.updates, 3)
	require.Equal(t, "[2]", repo.updates[SettingKeyModelMarketplaceGroupIDs])
	require.Equal(t, 1, updated)
}

func TestSettingService_UpdateModelMarketplaceSettings_拒绝停用分组(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{
		2: {ID: 2, Status: StatusDisabled},
	}})

	_, err := svc.UpdateModelMarketplaceSettings(context.Background(), ModelMarketplaceSettings{GroupIDs: []int64{2}})
	require.Error(t, err)
	require.Empty(t, repo.updates)
}

func TestSettingService_UpdateDailyCheckinSettings_规范化并仅写专属键(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{
		11: {ID: 11, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
	}})
	updated := 0
	svc.SetOnUpdateCallback(func() { updated++ })

	settings, err := svc.UpdateDailyCheckinSettings(context.Background(), DailyCheckinSettings{
		Enabled: true,
		Prizes: []DailyCheckinPrizeConfig{
			{ID: "balance", Name: "余额", Type: DailyCheckinPrizeTypeBalance, ProbabilityBps: 9000, Enabled: true, BalanceMode: "fixed", Amount: 0.1},
			{ID: "subscription", Name: "订阅", Type: DailyCheckinPrizeTypeSubscription, ProbabilityBps: 1000, Enabled: true, GroupID: 11, ValidityDays: 7},
		},
		UnpaidFullDays: 7,
		UnpaidDecayRules: []DailyCheckinDecayRule{
			{AfterDays: 30, FactorBps: 500},
			{AfterDays: 7, FactorBps: 5000},
		},
		LinuxDoExemptEnabled: true,
	})
	require.NoError(t, err)
	require.Len(t, repo.updates, 5)
	require.NotContains(t, repo.updates, SettingKeyDailyCheckinMode)
	require.Equal(t, 7, settings.UnpaidDecayRules[0].AfterDays)
	require.Equal(t, 30, settings.UnpaidDecayRules[1].AfterDays)
	require.Equal(t, 1, updated)

	var stored []DailyCheckinDecayRule
	require.NoError(t, json.Unmarshal([]byte(repo.updates[SettingKeyDailyCheckinUnpaidDecayRules]), &stored))
	require.Equal(t, settings.UnpaidDecayRules, stored)
}

func TestSettingService_UpdateDailyCheckinSettings_拒绝非法概率和衰减(t *testing.T) {
	svc := NewSettingService(&customFeatureSettingsRepoStub{}, &config.Config{})

	_, err := svc.UpdateDailyCheckinSettings(context.Background(), DailyCheckinSettings{
		Enabled: true,
		Prizes: []DailyCheckinPrizeConfig{
			{ID: "none", Name: "谢谢参与", Type: DailyCheckinPrizeTypeNone, ProbabilityBps: 9999, Enabled: true},
		},
		UnpaidFullDays: 7,
	})
	require.Error(t, err)

	_, err = svc.UpdateDailyCheckinSettings(context.Background(), DailyCheckinSettings{
		Enabled: true,
		Prizes: []DailyCheckinPrizeConfig{
			{ID: "none", Name: "谢谢参与", Type: DailyCheckinPrizeTypeNone, ProbabilityBps: 11000, Enabled: true},
		},
		UnpaidFullDays: 7,
	})
	require.Error(t, err)

	_, err = svc.UpdateDailyCheckinSettings(context.Background(), DailyCheckinSettings{
		Enabled:        false,
		UnpaidFullDays: 3651,
	})
	require.ErrorIs(t, err, ErrDailyCheckinDecayInvalid)
}

func TestDailyCheckinService_Config_使用独立配置读取(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyDailyCheckinEnabled:              "true",
		SettingKeyDailyCheckinPrizes:               `[{"id":"none","name":"谢谢参与","type":"none","probability_bps":10000,"enabled":true}]`,
		SettingKeyDailyCheckinUnpaidFullDays:       "9",
		SettingKeyDailyCheckinUnpaidDecayRules:     `[{"after_days":10,"factor_bps":3000}]`,
		SettingKeyDailyCheckinLinuxDoExemptEnabled: "true",
	}}
	settingService := NewSettingService(repo, &config.Config{})
	dailyService := &DailyCheckinService{settingService: settingService}

	cfg, err := dailyService.config(context.Background())
	require.NoError(t, err)
	require.True(t, cfg.Enabled)
	require.Equal(t, 9, cfg.UnpaidFullDays)
	require.Equal(t, 3000, cfg.UnpaidDecayRules[0].FactorBps)
	require.True(t, cfg.LinuxDoExemptEnabled)
}

func TestSettingService_GetGatewaySettings_UsesDefaultsAndStoredValues(t *testing.T) {
	serviceWithDefaults := NewSettingService(&customFeatureSettingsRepoStub{}, &config.Config{})
	defaults, err := serviceWithDefaults.GetGatewaySettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, defaults.DefaultPoolModeRetryCount)
	require.Equal(t, []int{401, 403, 429, 502, 503, 504}, defaults.DefaultPoolModeRetryStatusCodes)
	require.Equal(t, []int{5, 10, 15, 30, 60}, defaults.AutoManagedProbeBackoffMinutes)
	require.Equal(t, 60, defaults.FirstTokenTimeoutSeconds)
	require.Equal(t, 3, defaults.FirstTokenTimeoutConsecutiveThreshold)
	require.Equal(t, []int{502, 503, 504}, defaults.UpstreamErrorStatusCodes)
	require.Equal(t, 10, defaults.UpstreamErrorConsecutiveThreshold)
	require.True(t, defaults.ImageGroupSuccessRateVisible)
	require.Equal(t, int64(1), defaults.FailurePolicyRevision)

	repo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayDefaultPoolModeRetryCount:             "4",
		SettingKeyGatewayDefaultPoolModeRetryStatusCodes:       `[504,429,429]`,
		SettingKeyGatewayAutoManagedProbeBackoffMinutes:        `[2,8,8,30]`,
		SettingKeyGatewayFirstTokenTimeoutSeconds:              "0",
		SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold: "4",
		SettingKeyGatewayUpstreamErrorStatusCodes:              `[504,502,502]`,
		SettingKeyGatewayUpstreamErrorConsecutiveThreshold:     "7",
		SettingKeyGatewayImageGroupSuccessRateVisible:          "false",
		SettingKeyGatewayFailurePolicyRevision:                 "7",
	}}
	svc := NewSettingService(repo, &config.Config{})
	settings, err := svc.GetGatewaySettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 4, settings.DefaultPoolModeRetryCount)
	require.Equal(t, []int{429, 504}, settings.DefaultPoolModeRetryStatusCodes)
	require.Equal(t, []int{2, 8, 8, 30}, settings.AutoManagedProbeBackoffMinutes)
	require.Zero(t, settings.FirstTokenTimeoutSeconds)
	require.Equal(t, 4, settings.FirstTokenTimeoutConsecutiveThreshold)
	require.Equal(t, []int{502, 504}, settings.UpstreamErrorStatusCodes)
	require.Equal(t, 7, settings.UpstreamErrorConsecutiveThreshold)
	require.False(t, settings.ImageGroupSuccessRateVisible)
	require.Equal(t, int64(7), settings.FailurePolicyRevision)
}

func TestSettingService_UpdateGatewaySettings_NormalizesCachesAndReschedules(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	planRepo := &scheduledTestPlanRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetScheduledTestPlanRepository(planRepo)

	updated, err := svc.UpdateGatewaySettings(context.Background(), GatewaySettings{
		DefaultPoolModeRetryCount:             2,
		DefaultPoolModeRetryStatusCodes:       []int{503, 429, 503},
		AutoManagedProbeBackoffMinutes:        []int{1, 3, 10},
		FirstTokenTimeoutSeconds:              25,
		FirstTokenTimeoutConsecutiveThreshold: 4,
		UpstreamErrorStatusCodes:              []int{504, 502, 504},
		UpstreamErrorConsecutiveThreshold:     8,
		ImageGroupSuccessRateVisible:          false,
	})
	require.NoError(t, err)
	require.Equal(t, []int{429, 503}, updated.DefaultPoolModeRetryStatusCodes)
	require.Equal(t, "2", repo.updates[SettingKeyGatewayDefaultPoolModeRetryCount])
	require.Equal(t, `[429,503]`, repo.updates[SettingKeyGatewayDefaultPoolModeRetryStatusCodes])
	require.Equal(t, `[1,3,10]`, repo.updates[SettingKeyGatewayAutoManagedProbeBackoffMinutes])
	require.Equal(t, "4", repo.updates[SettingKeyGatewayFirstTokenTimeoutConsecutiveThreshold])
	require.Equal(t, `[502,504]`, repo.updates[SettingKeyGatewayUpstreamErrorStatusCodes])
	require.Equal(t, "8", repo.updates[SettingKeyGatewayUpstreamErrorConsecutiveThreshold])
	require.Equal(t, "2", repo.updates[SettingKeyGatewayFailurePolicyRevision])
	require.Equal(t, int64(2), updated.FailurePolicyRevision)
	require.Equal(t, []time.Duration{time.Minute, 3 * time.Minute, 10 * time.Minute}, planRepo.rescheduledSteps)
	require.False(t, planRepo.rescheduledAt.IsZero())

	runtime := svc.GetGatewayRuntime(context.Background())
	require.Equal(t, 25, runtime.FirstTokenTimeoutSeconds)
	require.Equal(t, []int{502, 504}, runtime.UpstreamErrorStatusCodes)
	require.False(t, runtime.ImageGroupSuccessRateVisible)

	updated.DefaultPoolModeRetryStatusCodes[0] = 500
	updated.UpstreamErrorStatusCodes[0] = 500
	require.Equal(t, []int{429, 503}, svc.GetGatewayRuntime(context.Background()).DefaultPoolModeRetryStatusCodes)
	require.Equal(t, []int{502, 504}, svc.GetGatewayRuntime(context.Background()).UpstreamErrorStatusCodes)

	unchangedPolicyInput := cloneGatewaySettings(*updated)
	unchangedPolicyInput.DefaultPoolModeRetryStatusCodes = []int{429, 503}
	unchangedPolicyInput.UpstreamErrorStatusCodes = []int{502, 504}
	unchangedPolicyInput.ImageGroupSuccessRateVisible = true
	unchangedPolicyInput.AutoManagedProbeBackoffMinutes = []int{2, 4, 12}
	unchangedPolicy, err := svc.UpdateGatewaySettings(context.Background(), unchangedPolicyInput)
	require.NoError(t, err)
	require.Equal(t, int64(2), unchangedPolicy.FailurePolicyRevision)

	changedPolicyInput := cloneGatewaySettings(*unchangedPolicy)
	changedPolicyInput.UpstreamErrorConsecutiveThreshold = 9
	changedPolicy, err := svc.UpdateGatewaySettings(context.Background(), changedPolicyInput)
	require.NoError(t, err)
	require.Equal(t, int64(3), changedPolicy.FailurePolicyRevision)
}

func TestSettingService_UpdateGatewaySettings_AcceptsEmptyRetryStatusCodes(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	updated, err := svc.UpdateGatewaySettings(context.Background(), GatewaySettings{
		DefaultPoolModeRetryCount:             0,
		DefaultPoolModeRetryStatusCodes:       []int{},
		AutoManagedProbeBackoffMinutes:        []int{5},
		FirstTokenTimeoutSeconds:              0,
		FirstTokenTimeoutConsecutiveThreshold: 1,
		UpstreamErrorStatusCodes:              []int{},
		UpstreamErrorConsecutiveThreshold:     1,
		ImageGroupSuccessRateVisible:          true,
	})
	require.NoError(t, err)
	require.Empty(t, updated.DefaultPoolModeRetryStatusCodes)
	require.NotNil(t, updated.DefaultPoolModeRetryStatusCodes)
	require.Equal(t, `[]`, repo.updates[SettingKeyGatewayDefaultPoolModeRetryStatusCodes])
	require.NotNil(t, updated.UpstreamErrorStatusCodes)
	require.Equal(t, `[]`, repo.updates[SettingKeyGatewayUpstreamErrorStatusCodes])
}

func TestSettingService_UpdateGatewaySettings_RejectsInvalidValues(t *testing.T) {
	valid := GatewaySettings{
		DefaultPoolModeRetryCount:             1,
		DefaultPoolModeRetryStatusCodes:       []int{429},
		AutoManagedProbeBackoffMinutes:        []int{5, 10},
		FirstTokenTimeoutSeconds:              60,
		FirstTokenTimeoutConsecutiveThreshold: 3,
		UpstreamErrorStatusCodes:              []int{502, 503, 504},
		UpstreamErrorConsecutiveThreshold:     10,
		ImageGroupSuccessRateVisible:          true,
	}
	tests := []struct {
		name   string
		mutate func(*GatewaySettings)
	}{
		{name: "retry count", mutate: func(v *GatewaySettings) { v.DefaultPoolModeRetryCount = 11 }},
		{name: "status code", mutate: func(v *GatewaySettings) { v.DefaultPoolModeRetryStatusCodes = []int{99} }},
		{name: "empty backoff", mutate: func(v *GatewaySettings) { v.AutoManagedProbeBackoffMinutes = nil }},
		{name: "decreasing backoff", mutate: func(v *GatewaySettings) { v.AutoManagedProbeBackoffMinutes = []int{10, 5} }},
		{name: "backoff range", mutate: func(v *GatewaySettings) { v.AutoManagedProbeBackoffMinutes = []int{1441} }},
		{name: "timeout", mutate: func(v *GatewaySettings) { v.FirstTokenTimeoutSeconds = 601 }},
		{name: "首 Token 阈值为零", mutate: func(v *GatewaySettings) { v.FirstTokenTimeoutConsecutiveThreshold = 0 }},
		{name: "首 Token 阈值过大", mutate: func(v *GatewaySettings) { v.FirstTokenTimeoutConsecutiveThreshold = 101 }},
		{name: "上游状态码越界", mutate: func(v *GatewaySettings) { v.UpstreamErrorStatusCodes = []int{600} }},
		{name: "上游错误阈值为零", mutate: func(v *GatewaySettings) { v.UpstreamErrorConsecutiveThreshold = 0 }},
		{name: "上游错误阈值过大", mutate: func(v *GatewaySettings) { v.UpstreamErrorConsecutiveThreshold = 101 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := cloneGatewaySettings(valid)
			tt.mutate(&input)
			repo := &customFeatureSettingsRepoStub{}
			_, err := NewSettingService(repo, &config.Config{}).UpdateGatewaySettings(context.Background(), input)
			require.ErrorIs(t, err, ErrGatewaySettingsInvalid)
			require.Empty(t, repo.updates)
		})
	}
}

func TestApplyGatewayPoolModeDefaults_CoversAPIKeyAndBedrock(t *testing.T) {
	settings := GatewaySettings{
		DefaultPoolModeRetryCount:       2,
		DefaultPoolModeRetryStatusCodes: []int{429, 503},
	}
	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeBedrock} {
		credentials := ApplyGatewayPoolModeDefaults(accountType, map[string]any{}, settings)
		require.Equal(t, true, credentials["pool_mode"])
		require.Equal(t, 2, credentials["pool_mode_retry_count"])
		require.Equal(t, []int{429, 503}, credentials["pool_mode_retry_status_codes"])
	}

	explicitFalse := map[string]any{"pool_mode": false}
	require.Equal(t, explicitFalse, ApplyGatewayPoolModeDefaults(AccountTypeAPIKey, explicitFalse, settings))
	require.NotContains(t, explicitFalse, "pool_mode_retry_count")

	explicitValues := map[string]any{
		"pool_mode":                    true,
		"pool_mode_retry_count":        7,
		"pool_mode_retry_status_codes": []int{502},
	}
	result := ApplyGatewayPoolModeDefaults(AccountTypeBedrock, explicitValues, settings)
	require.Equal(t, 7, result["pool_mode_retry_count"])
	require.Equal(t, []int{502}, result["pool_mode_retry_status_codes"])

	oauth := map[string]any{}
	require.Equal(t, oauth, ApplyGatewayPoolModeDefaults(AccountTypeOAuth, oauth, settings))
	require.Empty(t, oauth)
}

func TestAdminService_CreateAccount_AppliesGatewayPoolDefaults(t *testing.T) {
	settingRepo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayDefaultPoolModeRetryCount:       "2",
		SettingKeyGatewayDefaultPoolModeRetryStatusCodes: `[429,504]`,
	}}
	accountRepo := &mockAccountRepoForGemini{}
	svc := &adminServiceImpl{
		accountRepo:    accountRepo,
		settingService: NewSettingService(settingRepo, &config.Config{}),
	}

	created, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "bedrock",
		Platform:             PlatformAnthropic,
		Type:                 AccountTypeBedrock,
		Credentials:          map[string]any{"access_key_id": "test"},
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.Same(t, accountRepo.createdAccount, created)
	require.Equal(t, true, created.Credentials["pool_mode"])
	require.Equal(t, 2, created.Credentials["pool_mode_retry_count"])
	require.Equal(t, []int{429, 504}, created.Credentials["pool_mode_retry_status_codes"])

	accountRepo.createdAccount = nil
	created, err = svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "explicit false",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"pool_mode": false},
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.Equal(t, false, created.Credentials["pool_mode"])
	require.NotContains(t, created.Credentials, "pool_mode_retry_count")
}
