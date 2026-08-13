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
