//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGateway首Token分组配置保存读取与缓存隔离(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{46: {ID: 46, Status: StatusActive}}})
	settings := scopedFirstTokenSettings()
	settings.FirstTokenTimeoutGroupIDs = []int64{46, 46}
	saved, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, []int64{46}, saved.FirstTokenTimeoutGroupIDs)
	require.Equal(t, "selected_groups", repo.updates[SettingKeyGatewayFirstTokenTimeoutScope])
	require.Equal(t, "[46]", repo.updates[SettingKeyGatewayFirstTokenTimeoutGroupIDs])
	saved.FirstTokenTimeoutGroupIDs[0] = 1
	read, err := svc.GetCustomFeatureSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int64{46}, read.Gateway.FirstTokenTimeoutGroupIDs)
	require.Equal(t, FirstTokenTimeoutScopeSelectedGroups, read.Gateway.FirstTokenTimeoutScope)
	require.Equal(t, []int64{46}, svc.GetGatewayRuntime(context.Background()).FirstTokenTimeoutGroupIDs)
}

func TestGateway首Token分组验证与旧客户端兼容(t *testing.T) {
	repo := &customFeatureSettingsRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	settings := scopedFirstTokenSettings()
	saved, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	settings.FirstTokenTimeoutScope = ""
	settings.FirstTokenTimeoutGroupIDs = nil
	legacy, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, saved.FirstTokenTimeoutScope, legacy.FirstTokenTimeoutScope)
	require.Equal(t, saved.FirstTokenTimeoutGroupIDs, legacy.FirstTokenTimeoutGroupIDs)
	for _, scope := range []string{"selected_groups", "invalid"} {
		settings.FirstTokenTimeoutScope = scope
		settings.FirstTokenTimeoutGroupIDs = nil
		_, err := svc.UpdateGatewaySettings(context.Background(), settings)
		require.ErrorIs(t, err, ErrGatewaySettingsInvalid)
	}
	settings = scopedFirstTokenSettings()
	svc.SetDefaultSubscriptionGroupReader(&customFeatureGroupReaderStub{groups: map[int64]*Group{46: {ID: 46, Status: StatusDisabled}}})
	_, err = svc.UpdateGatewaySettings(context.Background(), settings)
	require.ErrorIs(t, err, ErrCustomFeatureGroupInvalid)
}

func TestGateway首Token分组变更提升策略版本且顺序无关(t *testing.T) {
	svc := NewSettingService(&customFeatureSettingsRepoStub{}, &config.Config{})
	settings := scopedFirstTokenSettings()
	first, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	settings.FirstTokenTimeoutGroupIDs = []int64{46, 26}
	second, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	require.Greater(t, second.FailurePolicyRevision, first.FailurePolicyRevision)
	settings.FirstTokenTimeoutGroupIDs = []int64{26, 46}
	third, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, second.FailurePolicyRevision, third.FailurePolicyRevision)
	settings.FirstTokenTimeoutScope = FirstTokenTimeoutScopeAll
	last, err := svc.UpdateGatewaySettings(context.Background(), settings)
	require.NoError(t, err)
	require.Greater(t, last.FailurePolicyRevision, third.FailurePolicyRevision)
}
