//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// Composite 分组的公开别名经 BillingModelSource 覆盖为计费模型后，可能出现两类错计：
// 任意别名查无价格而静默计为零，或含家族词的别名被价格表模糊匹配到错误价格。
// compositeBillableModel 要求别名必须有显式渠道定价才参与计费，否则使用实际转发模型。
func TestCompositeBillableModel(t *testing.T) {
	svc := &GatewayService{billingService: NewBillingService(&config.Config{}, nil)}
	apiKey := &APIKey{}
	ctx := context.Background()

	// 别名无渠道定价时，即使含家族词也使用具体模型。
	require.Equal(t, "claude-opus-4-7",
		svc.compositeBillableModel(ctx, apiKey, "all/claude", "claude-opus-4-7"))
	require.Equal(t, "claude-sonnet-4",
		svc.compositeBillableModel(ctx, apiKey, "team/best", "claude-sonnet-4"))

	// 未发生来源覆盖时，计费模型已经是具体模型，应保持原值。
	require.Equal(t, "claude-sonnet-4",
		svc.compositeBillableModel(ctx, apiKey, "claude-sonnet-4", "claude-sonnet-4"))

	// 具体模型缺失时保持原值，由后续严格计费路径报告定价错误。
	require.Equal(t, "all/claude",
		svc.compositeBillableModel(ctx, apiKey, "all/claude", ""))
}
