//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorBodyOverrideValidation(t *testing.T) {
	tests := []struct {
		name string
		mode string
		body map[string]any
		err  error
	}{
		{name: "默认允许空请求体", mode: MonitorBodyOverrideModeOff},
		{name: "合并允许未填写", mode: MonitorBodyOverrideModeMerge},
		{name: "合并允许空对象", mode: MonitorBodyOverrideModeMerge, body: map[string]any{}},
		{name: "合并允许自定义字段", mode: MonitorBodyOverrideModeMerge, body: map[string]any{"max_tokens": 20}},
		{name: "覆盖拒绝未填写", mode: MonitorBodyOverrideModeReplace, err: ErrChannelMonitorTemplateBodyRequired},
		{name: "覆盖拒绝空对象", mode: MonitorBodyOverrideModeReplace, body: map[string]any{}, err: ErrChannelMonitorTemplateBodyRequired},
		{name: "覆盖保留协议字段校验", mode: MonitorBodyOverrideModeReplace, body: map[string]any{"max_tokens": 20}, err: ErrChannelMonitorInvalidRequestBody},
		{name: "拒绝未知模式", mode: "invalid", err: ErrChannelMonitorTemplateInvalidBodyMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBodyModeForProtocol(MonitorProviderOpenAI, MonitorAPIModeChatCompletions, tt.mode, tt.body)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChannelMonitorBodyOverrideEmptyMergeCreate(t *testing.T) {
	for _, body := range []map[string]any{nil, {}} {
		repo := &duplicateChannelMonitorRepoStub{}
		svc := NewChannelMonitorService(repo, &duplicateChannelMonitorEncryptor{})
		monitor, err := svc.Create(context.Background(), ChannelMonitorCreateParams{
			Name: "空合并监控", Provider: MonitorProviderAnthropic,
			Endpoint: "https://example.com", APIKey: "test-key", PrimaryModel: "claude-test",
			IntervalSeconds: 60, BodyOverrideMode: MonitorBodyOverrideModeMerge, BodyOverride: body,
		})
		require.NoError(t, err)
		require.Len(t, repo.created, 1)
		require.Equal(t, MonitorBodyOverrideModeMerge, monitor.BodyOverrideMode)
		require.Empty(t, monitor.BodyOverride)
		require.NoError(t, validateTemplateCreateParams(ChannelMonitorRequestTemplateCreateParams{
			Name: "空合并模板", Provider: MonitorProviderAnthropic,
			BodyOverrideMode: MonitorBodyOverrideModeMerge, BodyOverride: body,
		}))
	}
}

func TestChannelMonitorBodyOverrideEmptyMergeUpdate(t *testing.T) {
	mode := MonitorBodyOverrideModeMerge
	emptyBody := map[string]any{}
	for _, initialBody := range []map[string]any{nil, {"system": "旧配置"}} {
		monitor := &ChannelMonitor{
			Provider: MonitorProviderAnthropic, Endpoint: "https://example.com",
			PrimaryModel: "claude-test", IntervalSeconds: 60,
			BodyOverrideMode: MonitorBodyOverrideModeOff, BodyOverride: initialBody,
		}
		require.NoError(t, applyMonitorUpdate(monitor, ChannelMonitorUpdateParams{
			BodyOverrideMode: &mode, BodyOverride: &emptyBody,
		}))
		require.Equal(t, mode, monitor.BodyOverrideMode)
		require.Empty(t, monitor.BodyOverride)

		template := &ChannelMonitorRequestTemplate{
			Provider: MonitorProviderAnthropic, BodyOverrideMode: MonitorBodyOverrideModeOff, BodyOverride: initialBody,
		}
		require.NoError(t, applyTemplateUpdate(template, ChannelMonitorRequestTemplateUpdateParams{
			BodyOverrideMode: &mode, BodyOverride: &emptyBody,
		}))
		require.Equal(t, mode, template.BodyOverrideMode)
		require.Empty(t, template.BodyOverride)
	}
}

func TestChannelMonitorBodyOverrideEmptyMergeKeepsDefaultRequest(t *testing.T) {
	for _, protocol := range []struct{ provider, apiMode string }{
		{MonitorProviderAnthropic, MonitorAPIModeChatCompletions},
		{MonitorProviderOpenAI, MonitorAPIModeChatCompletions},
		{MonitorProviderOpenAI, MonitorAPIModeResponses},
		{MonitorProviderGemini, MonitorAPIModeChatCompletions},
	} {
		t.Run(protocol.provider+"/"+protocol.apiMode, func(t *testing.T) {
			adapter, apiMode, ok := providerAdapterFor(protocol.provider, protocol.apiMode)
			require.True(t, ok)
			defaultBody, err := adapter.buildBody("test-model", "请计算 1+1")
			require.NoError(t, err)
			for _, body := range []map[string]any{nil, {}} {
				actual, err := buildRequestBody(adapter, protocol.provider, apiMode, "test-model", "请计算 1+1", &CheckOptions{
					BodyOverrideMode: MonitorBodyOverrideModeMerge, BodyOverride: body,
				})
				require.NoError(t, err)
				require.JSONEq(t, string(defaultBody), string(actual))
			}
		})
	}
}
