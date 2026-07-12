package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type openAIWSTurnOutcomeRecorderStub struct {
	blocked  bool
	statuses []int
}

func (s *openAIWSTurnOutcomeRecorderStub) RecordUpstreamFailureOutcome(_ context.Context, _ int64, err *service.UpstreamFailoverError) (bool, bool) {
	s.statuses = append(s.statuses, err.StatusCode)
	return true, s.blocked
}

func TestRecordOpenAIWSTurnOutcomeErrorReturnsBlockedState(t *testing.T) {
	recorder := &openAIWSTurnOutcomeRecorderStub{blocked: true}
	recognized, blocked := recordOpenAIWSTurnOutcomeError(
		context.Background(),
		recorder,
		42,
		service.NewUpstreamOutcomeError(http.StatusServiceUnavailable, nil, nil),
		false,
	)

	require.True(t, recognized)
	require.True(t, blocked)
	require.Equal(t, []int{http.StatusServiceUnavailable}, recorder.statuses)
}

func TestRecordOpenAIWSTurnOutcomeErrorSkipsDisconnectedClient(t *testing.T) {
	recorder := &openAIWSTurnOutcomeRecorderStub{blocked: true}
	recognized, blocked := recordOpenAIWSTurnOutcomeError(
		context.Background(),
		recorder,
		42,
		service.NewUpstreamOutcomeError(http.StatusServiceUnavailable, nil, nil),
		true,
	)

	require.True(t, recognized)
	require.False(t, blocked)
	require.Empty(t, recorder.statuses)
}
