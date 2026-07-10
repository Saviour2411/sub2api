//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

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
