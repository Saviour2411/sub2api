package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// openAIUpstreamOutcomeTracker 保证一次用户请求内每个账号只结算一次最终结果。
// 池内重试期间不调用该追踪器，只有同账号重试耗尽或明确成功后才结算。
type openAIUpstreamOutcomeTracker struct {
	mu       sync.Mutex
	recorded map[int64]openAIUpstreamOutcome
}

type openAIUpstreamOutcome uint8

const (
	openAIUpstreamOutcomeFailure openAIUpstreamOutcome = iota + 1
	openAIUpstreamOutcomeSuccess
)

func newOpenAIUpstreamOutcomeTracker() *openAIUpstreamOutcomeTracker {
	return &openAIUpstreamOutcomeTracker{recorded: make(map[int64]openAIUpstreamOutcome)}
}

func (t *openAIUpstreamOutcomeTracker) recordFailure(
	ctx context.Context,
	recorder FailoverOutcomeRecorder,
	accountID int64,
	failoverErr *service.UpstreamFailoverError,
) (managed bool, blocked bool) {
	if recorder == nil || failoverErr == nil || accountID <= 0 || contextOutcomeCanceled(ctx) {
		return false, false
	}
	if !t.markFailure(accountID) {
		return false, false
	}
	if strictScheduler, ok := any(recorder).(FailoverStrictScheduler); ok && strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr) {
		return false, true
	}
	return recorder.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
}

func (t *openAIUpstreamOutcomeTracker) recordSuccess(
	ctx context.Context,
	recorder FailoverSuccessOutcomeRecorder,
	accountID int64,
	clientDisconnected bool,
) {
	if recorder == nil || accountID <= 0 || clientDisconnected || contextOutcomeCanceled(ctx) {
		return
	}
	if !t.markSuccess(accountID) {
		return
	}
	recorder.RecordUpstreamSuccessOutcome(ctx, accountID)
}

// recordOutcomeError 只结算明确的非 failover 上游结果，不改变原有响应和切号语义。
func (t *openAIUpstreamOutcomeTracker) recordOutcomeError(
	ctx context.Context,
	recorder FailoverOutcomeRecorder,
	accountID int64,
	err error,
	clientDisconnected bool,
) bool {
	var outcomeErr *service.UpstreamOutcomeError
	if !errors.As(err, &outcomeErr) {
		return false
	}
	if outcomeErr == nil || outcomeErr.ClientDisconnect || clientDisconnected || contextOutcomeCanceled(ctx) {
		return true
	}
	t.recordFailure(ctx, recorder, accountID, &service.UpstreamFailoverError{
		StatusCode:   outcomeErr.StatusCode,
		ResponseBody: outcomeErr.ResponseBody,
	})
	return true
}

func (t *openAIUpstreamOutcomeTracker) markFailure(accountID int64) bool {
	if t == nil || accountID <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.recorded == nil {
		t.recorded = make(map[int64]openAIUpstreamOutcome)
	}
	if _, exists := t.recorded[accountID]; exists {
		return false
	}
	t.recorded[accountID] = openAIUpstreamOutcomeFailure
	return true
}

func (t *openAIUpstreamOutcomeTracker) markSuccess(accountID int64) bool {
	if t == nil || accountID <= 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.recorded == nil {
		t.recorded = make(map[int64]openAIUpstreamOutcome)
	}
	if t.recorded[accountID] == openAIUpstreamOutcomeSuccess {
		return false
	}
	// 若调用方随后确认同账号内部重试成功，以最终成功结果修正先前失败计数。
	t.recorded[accountID] = openAIUpstreamOutcomeSuccess
	return true
}

// WebSocket 每个 turn 都是一条独立用户请求，不能复用连接级去重状态。
func recordOpenAIUpstreamTurnSuccess(
	ctx context.Context,
	recorder FailoverSuccessOutcomeRecorder,
	accountID int64,
	clientDisconnected bool,
) {
	if recorder == nil || accountID <= 0 || clientDisconnected || contextOutcomeCanceled(ctx) {
		return
	}
	recorder.RecordUpstreamSuccessOutcome(ctx, accountID)
}

// recordOpenAIUpstreamTurnOutcomeError 按 WebSocket turn 独立结算失败，不能复用连接级去重状态。
func recordOpenAIUpstreamTurnOutcomeError(
	ctx context.Context,
	recorder FailoverOutcomeRecorder,
	accountID int64,
	outcomeErr *service.UpstreamOutcomeError,
	clientDisconnected bool,
) bool {
	if outcomeErr == nil {
		return false
	}
	if recorder == nil || accountID <= 0 || outcomeErr.ClientDisconnect || clientDisconnected || contextOutcomeCanceled(ctx) {
		return true
	}
	failoverErr := &service.UpstreamFailoverError{
		StatusCode:   outcomeErr.StatusCode,
		ResponseBody: outcomeErr.ResponseBody,
	}
	if strictScheduler, ok := any(recorder).(FailoverStrictScheduler); ok && strictScheduler.HandleUpstreamFailoverError(ctx, accountID, failoverErr) {
		return true
	}
	recorder.RecordUpstreamFailureOutcome(ctx, accountID, failoverErr)
	return true
}

func contextOutcomeCanceled(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}

func openAIImageUpstreamOutcomeError(err *service.OpenAIImagesUpstreamError) *service.UpstreamFailoverError {
	if err == nil {
		return nil
	}
	statusCode := err.StatusCode
	if statusCode <= 0 {
		statusCode = http.StatusBadGateway
	}
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    err.ErrorType,
			"code":    err.Code,
			"message": err.Message,
		},
	})
	return &service.UpstreamFailoverError{
		StatusCode:   statusCode,
		ResponseBody: body,
	}
}
