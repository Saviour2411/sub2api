//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayServiceRecordUsage_ResponseModelIsDiagnosticOnly(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
	requestedModel := "claude-opus-4.8"
	responseModel := "claude-sonnet-4"
	wantCost, err := svc.billingService.CalculateCost(requestedModel, UsageTokens{InputTokens: 100, OutputTokens: 50}, 1.1)
	require.NoError(t, err)

	err = svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID:             "gateway_response_model_diagnostic",
			Usage:                 ClaudeUsage{InputTokens: 100, OutputTokens: 50},
			Model:                 requestedModel,
			UpstreamResponseModel: responseModel,
			Duration:              time.Second,
		},
		APIKey:  &APIKey{ID: 501, Quota: 100},
		User:    &User{ID: 601},
		Account: &Account{ID: 701},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID:          9,
			OriginalModel:      requestedModel,
			ChannelMappedModel: requestedModel,
			BillingModelSource: BillingModelSourceResponse,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, wantCost.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, wantCost.ActualCost, userRepo.lastAmount, 1e-12)
	require.Equal(t, requestedModel, usageRepo.lastLog.Model)
	require.Equal(t, requestedModel, usageRepo.lastLog.RequestedModel)
	require.NotNil(t, usageRepo.lastLog.UpstreamResponseModel)
	require.Equal(t, responseModel, *usageRepo.lastLog.UpstreamResponseModel)
	require.NotNil(t, usageRepo.lastLog.UpstreamModelMismatch)
	require.True(t, *usageRepo.lastLog.UpstreamModelMismatch)
}

func TestOpenAIGatewayServiceRecordUsage_ResponseModelIsDiagnosticOnly(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
	requestedModel := "gpt-5.5"
	responseModel := "gpt-5.4-nano"
	wantCost, err := svc.billingService.CalculateCost(requestedModel, UsageTokens{InputTokens: 20, OutputTokens: 10}, 1.1)
	require.NoError(t, err)

	err = svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{
			RequestID:             "openai_response_model_diagnostic",
			Model:                 requestedModel,
			UpstreamModel:         requestedModel,
			UpstreamResponseModel: responseModel,
			Usage:                 OpenAIUsage{InputTokens: 20, OutputTokens: 10},
			Duration:              time.Second,
		},
		APIKey:  &APIKey{ID: 10},
		User:    &User{ID: 20},
		Account: &Account{ID: 30},
		ChannelUsageFields: ChannelUsageFields{
			ChannelID:          9,
			OriginalModel:      requestedModel,
			ChannelMappedModel: requestedModel,
			BillingModelSource: BillingModelSourceResponse,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.InDelta(t, wantCost.ActualCost, usageRepo.lastLog.ActualCost, 1e-12)
	require.InDelta(t, wantCost.ActualCost, userRepo.lastAmount, 1e-12)
	require.Equal(t, requestedModel, usageRepo.lastLog.Model)
	require.NotNil(t, usageRepo.lastLog.UpstreamResponseModel)
	require.Equal(t, responseModel, *usageRepo.lastLog.UpstreamResponseModel)
	require.NotNil(t, usageRepo.lastLog.UpstreamModelMismatch)
	require.True(t, *usageRepo.lastLog.UpstreamModelMismatch)
}

func TestGrokResponseAuditDefaultSourceKeepsCost(t *testing.T) {
	tokens := UsageTokens{InputTokens: 20, OutputTokens: 10}
	record := func(t *testing.T, source, responseModel string) float64 {
		t.Helper()
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID:             "grok_audit_default_" + source + "_" + responseModel,
				Model:                 "grok-4.7",
				UpstreamModel:         "grok-4.7",
				UpstreamResponseModel: responseModel,
				Usage:                 OpenAIUsage{InputTokens: tokens.InputTokens, OutputTokens: tokens.OutputTokens},
				Duration:              time.Second,
			},
			APIKey:  &APIKey{ID: 10},
			User:    &User{ID: 20},
			Account: &Account{ID: 30, Platform: PlatformGrok},
			ChannelUsageFields: ChannelUsageFields{
				ChannelID:          9,
				OriginalModel:      "grok-4.7",
				ChannelMappedModel: "grok-4.7",
				BillingModelSource: source,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		if responseModel == "" {
			require.Nil(t, usageRepo.lastLog.UpstreamResponseModel)
		} else {
			require.NotNil(t, usageRepo.lastLog.UpstreamResponseModel)
			require.Equal(t, responseModel, *usageRepo.lastLog.UpstreamResponseModel)
		}
		return usageRepo.lastLog.ActualCost
	}

	for _, source := range []string{"", BillingModelSourceChannelMapped} {
		t.Run(source, func(t *testing.T) {
			baseline := record(t, source, "")
			withAlias := record(t, source, "grok-4.7-build")
			withCheaper := record(t, source, "grok-4.3")
			withFaster := record(t, source, "grok-4.7-fast")
			require.Greater(t, baseline, 0.0)
			require.InDelta(t, baseline, withAlias, 1e-12)
			require.InDelta(t, baseline, withCheaper, 1e-12)
			require.InDelta(t, baseline, withFaster, 1e-12)
		})
	}
}

func TestGrokResponseAuditResponseModelRejectsUnknownHigherAndConflict(t *testing.T) {
	tokens := UsageTokens{InputTokens: 20, OutputTokens: 10}
	record := func(t *testing.T, baseline, responseModel string, conflict bool) float64 {
		t.Helper()
		usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
		userRepo := &openAIRecordUsageUserRepoStub{}
		svc := newOpenAIRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{}, nil)
		err := svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
			Result: &OpenAIForwardResult{
				RequestID:                     "grok_audit_explicit_" + baseline + "_" + responseModel,
				Model:                         baseline,
				UpstreamModel:                 baseline,
				UpstreamResponseModel:         responseModel,
				UpstreamResponseModelConflict: conflict,
				Usage:                         OpenAIUsage{InputTokens: tokens.InputTokens, OutputTokens: tokens.OutputTokens},
				Duration:                      time.Second,
			},
			APIKey:  &APIKey{ID: 10},
			User:    &User{ID: 20},
			Account: &Account{ID: 30, Platform: PlatformGrok},
			ChannelUsageFields: ChannelUsageFields{
				ChannelID:          9,
				OriginalModel:      baseline,
				ChannelMappedModel: baseline,
				BillingModelSource: BillingModelSourceResponse,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, usageRepo.lastLog)
		return usageRepo.lastLog.ActualCost
	}

	t.Run("unknown price", func(t *testing.T) {
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		require.False(t, svc.billingService.HasIdentifiedTokenPricing("grok-4.5-build"))
		baselineCost, err := svc.billingService.CalculateCost("grok-4.3", tokens, 1)
		require.NoError(t, err)
		familyCost, err := svc.billingService.CalculateCost("grok-4.5-build", tokens, 1)
		require.NoError(t, err)
		require.NotEqual(t, baselineCost.TotalCost, familyCost.TotalCost)

		require.InDelta(t, record(t, "grok-4.3", "", false), record(t, "grok-4.3", "grok-4.5-build", false), 1e-12)
	})

	t.Run("higher price", func(t *testing.T) {
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		require.True(t, svc.billingService.HasIdentifiedTokenPricing("grok-4.7"))
		require.True(t, svc.billingService.HasIdentifiedTokenPricing("grok-4.7-fast"))
		baselineCost, err := svc.billingService.CalculateCost("grok-4.7", tokens, 1)
		require.NoError(t, err)
		fasterCost, err := svc.billingService.CalculateCost("grok-4.7-fast", tokens, 1)
		require.NoError(t, err)
		require.Greater(t, fasterCost.TotalCost, baselineCost.TotalCost)

		require.InDelta(t, record(t, "grok-4.7", "", false), record(t, "grok-4.7", "grok-4.7-fast", false), 1e-12)
	})

	t.Run("conflict", func(t *testing.T) {
		svc := newOpenAIRecordUsageServiceForTest(&openAIRecordUsageLogRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		require.True(t, svc.billingService.HasIdentifiedTokenPricing("grok-4.3"))
		baselineCost, err := svc.billingService.CalculateCost("grok-4.7", tokens, 1)
		require.NoError(t, err)
		cheaperCost, err := svc.billingService.CalculateCost("grok-4.3", tokens, 1)
		require.NoError(t, err)
		require.Less(t, cheaperCost.TotalCost, baselineCost.TotalCost)

		require.InDelta(t, record(t, "grok-4.7", "", false), record(t, "grok-4.7", "grok-4.3", true), 1e-12)
	})
}
