//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type paymentRechargeBonusRateRepoStub struct {
	UserGroupRateRepository
	rates map[int64]float64
	err   error
	calls int
}

func (s *paymentRechargeBonusRateRepoStub) GetByUserID(context.Context, int64) (map[int64]float64, error) {
	s.calls++
	return s.rates, s.err
}

func newPaymentRechargeBonusPolicyService(enabled bool, repo UserGroupRateRepository) *PaymentService {
	settingRepo := &customFeatureSettingsRepoStub{values: map[string]string{
		SettingKeyGatewayDisableRechargeBonusForCustomRateUsers: "false",
	}}
	if enabled {
		settingRepo.values[SettingKeyGatewayDisableRechargeBonusForCustomRateUsers] = "true"
	}
	svc := &PaymentService{}
	svc.SetRechargeBonusPolicyDependencies(NewSettingService(settingRepo, &config.Config{}), repo)
	return svc
}

func TestPaymentService_IsBalanceRechargeBonusDisabled(t *testing.T) {
	t.Run("开关关闭时不查询专属倍率", func(t *testing.T) {
		repo := &paymentRechargeBonusRateRepoStub{rates: map[int64]float64{2: 0.8}}
		svc := newPaymentRechargeBonusPolicyService(false, repo)

		require.False(t, svc.IsBalanceRechargeBonusDisabled(context.Background(), 11))
		require.Zero(t, repo.calls)
	})

	t.Run("任意专属倍率禁用返利", func(t *testing.T) {
		repo := &paymentRechargeBonusRateRepoStub{rates: map[int64]float64{2: 0.8}}
		svc := newPaymentRechargeBonusPolicyService(true, repo)

		require.True(t, svc.IsBalanceRechargeBonusDisabled(context.Background(), 11))
		require.Equal(t, 1, repo.calls)
	})

	t.Run("没有倍率或仅有RPM配置时保留返利", func(t *testing.T) {
		repo := &paymentRechargeBonusRateRepoStub{rates: map[int64]float64{}}
		svc := newPaymentRechargeBonusPolicyService(true, repo)

		require.False(t, svc.IsBalanceRechargeBonusDisabled(context.Background(), 11))
	})

	t.Run("查询失败时保守禁用返利", func(t *testing.T) {
		repo := &paymentRechargeBonusRateRepoStub{err: errors.New("database unavailable")}
		svc := newPaymentRechargeBonusPolicyService(true, repo)

		require.True(t, svc.IsBalanceRechargeBonusDisabled(context.Background(), 11))
	})

	t.Run("开关开启但仓储缺失时保守禁用返利", func(t *testing.T) {
		svc := newPaymentRechargeBonusPolicyService(true, nil)

		require.True(t, svc.IsBalanceRechargeBonusDisabled(context.Background(), 11))
	})
}
