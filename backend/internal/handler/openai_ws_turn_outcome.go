package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// recordOpenAIWSTurnOutcomeError 按 WS turn 结算一次明确的上游失败，并把
// 停调度结果反馈给连接生命周期，使后续 turn 不再绕过调度状态。
func recordOpenAIWSTurnOutcomeError(
	ctx context.Context,
	recorder FailoverOutcomeRecorder,
	accountID int64,
	outcomeErr *service.UpstreamOutcomeError,
	clientDisconnected bool,
) (recognized bool, blocked bool) {
	if outcomeErr == nil {
		return false, false
	}
	if recorder == nil || accountID <= 0 || outcomeErr.ClientDisconnect || clientDisconnected || contextOutcomeCanceled(ctx) {
		return true, false
	}
	failoverErr := &service.UpstreamFailoverError{
		StatusCode:   outcomeErr.StatusCode,
		ResponseBody: outcomeErr.ResponseBody,
	}
	if strictScheduler, ok := any(recorder).(FailoverStrictScheduler); ok && strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr) {
		return true, true
	}
	_, blocked = recorder.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
	return true, blocked
}
