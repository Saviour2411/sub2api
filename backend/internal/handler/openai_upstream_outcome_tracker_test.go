package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type openAIOutcomeRecorderStub struct {
	failures      []int64
	failureStatus []int
	success       []int64
}

func (s *openAIOutcomeRecorderStub) RecordUpstreamFailureOutcome(_ context.Context, accountID int64, failoverErr *service.UpstreamFailoverError) (bool, bool) {
	s.failures = append(s.failures, accountID)
	s.failureStatus = append(s.failureStatus, failoverErr.StatusCode)
	return true, false
}

func (s *openAIOutcomeRecorderStub) RecordUpstreamSuccessOutcome(_ context.Context, accountID int64) {
	s.success = append(s.success, accountID)
}

func TestOpenAIUpstreamOutcomeTrackerEachAccountOnlyOnce(t *testing.T) {
	recorder := &openAIOutcomeRecorderStub{}
	tracker := newOpenAIUpstreamOutcomeTracker()
	err502 := &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway}

	managed, blocked := tracker.recordFailure(context.Background(), recorder, 11, err502)
	require.True(t, managed)
	require.False(t, blocked)
	tracker.recordFailure(context.Background(), recorder, 11, err502)
	tracker.recordSuccess(context.Background(), recorder, 11, false)
	tracker.recordSuccess(context.Background(), recorder, 11, false)
	tracker.recordFailure(context.Background(), recorder, 12, err502)

	require.Equal(t, []int64{11, 12}, recorder.failures)
	require.Equal(t, []int64{11}, recorder.success)
}

func TestOpenAIUpstreamOutcomeTrackerSkipsCanceledOrDisconnected(t *testing.T) {
	recorder := &openAIOutcomeRecorderStub{}
	tracker := newOpenAIUpstreamOutcomeTracker()
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	tracker.recordFailure(canceledCtx, recorder, 11, &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway})
	tracker.recordSuccess(context.Background(), recorder, 12, true)
	tracker.recordSuccess(canceledCtx, recorder, 13, false)

	require.Empty(t, recorder.failures)
	require.Empty(t, recorder.success)
}

func TestOpenAIUpstreamOutcomeTrackerRecordsTypedOutcomeWithoutFailover(t *testing.T) {
	recorder := &openAIOutcomeRecorderStub{}
	tracker := newOpenAIUpstreamOutcomeTracker()
	outcomeErr := service.NewUpstreamOutcomeError(
		http.StatusServiceUnavailable,
		[]byte(`{"error":{"message":"busy"}}`),
		context.DeadlineExceeded,
	)

	require.True(t, tracker.recordOutcomeError(context.Background(), recorder, 21, outcomeErr, false))
	require.Equal(t, []int64{21}, recorder.failures)
	require.Equal(t, []int{http.StatusServiceUnavailable}, recorder.failureStatus)

	disconnected := service.NewUpstreamOutcomeError(http.StatusBadGateway, nil, context.Canceled)
	disconnected.ClientDisconnect = true
	require.True(t, tracker.recordOutcomeError(context.Background(), recorder, 22, disconnected, false))
	require.Equal(t, []int64{21}, recorder.failures)
	require.False(t, tracker.recordOutcomeError(context.Background(), recorder, 23, context.Canceled, false))
}

func TestRecordOpenAIUpstreamTurnSuccessDoesNotDeduplicateTurns(t *testing.T) {
	recorder := &openAIOutcomeRecorderStub{}
	recordOpenAIUpstreamTurnSuccess(context.Background(), recorder, 11, false)
	recordOpenAIUpstreamTurnSuccess(context.Background(), recorder, 11, false)
	recordOpenAIUpstreamTurnSuccess(context.Background(), recorder, 11, true)

	require.Equal(t, []int64{11, 11}, recorder.success)
}

func TestRecordOpenAIUpstreamTurnOutcomeErrorDoesNotDeduplicateTurns(t *testing.T) {
	recorder := &openAIOutcomeRecorderStub{}
	outcomeErr := service.NewUpstreamOutcomeError(http.StatusGatewayTimeout, nil, context.DeadlineExceeded)

	require.True(t, recordOpenAIUpstreamTurnOutcomeError(context.Background(), recorder, 11, outcomeErr, false))
	require.True(t, recordOpenAIUpstreamTurnOutcomeError(context.Background(), recorder, 11, outcomeErr, false))
	require.True(t, recordOpenAIUpstreamTurnOutcomeError(context.Background(), recorder, 11, outcomeErr, true))

	require.Equal(t, []int64{11, 11}, recorder.failures)
	require.Equal(t, []int{http.StatusGatewayTimeout, http.StatusGatewayTimeout}, recorder.failureStatus)
}

func TestOpenAIImageUpstreamOutcomeErrorPreservesStatusAndReason(t *testing.T) {
	converted := openAIImageUpstreamOutcomeError(&service.OpenAIImagesUpstreamError{
		StatusCode: http.StatusServiceUnavailable,
		ErrorType:  "server_error",
		Code:       "overloaded",
		Message:    "upstream busy",
	})

	require.Equal(t, http.StatusServiceUnavailable, converted.StatusCode)
	require.JSONEq(t, `{"error":{"type":"server_error","code":"overloaded","message":"upstream busy"}}`, string(converted.ResponseBody))
}
