package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCanAcceptOpenAIPartialImageResult(t *testing.T) {
	result := &service.OpenAIForwardResult{ImageCount: 1}

	require.True(t, canAcceptOpenAIPartialImageResult(result, errors.New("本地收尾失败")))
	require.False(t, canAcceptOpenAIPartialImageResult(result, service.NewUpstreamOutcomeError(
		http.StatusServiceUnavailable,
		[]byte(`{"type":"response.failed"}`),
		errors.New("上游返回失败结果"),
	)))
	require.False(t, canAcceptOpenAIPartialImageResult(&service.OpenAIForwardResult{}, errors.New("失败")))
	require.False(t, canAcceptOpenAIPartialImageResult(result, nil))
}
