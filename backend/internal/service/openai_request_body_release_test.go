package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRequestBodyReleaseRunsOnceAfterFirstToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	released := 0
	SetOpenAIRequestBodyRelease(c, func() { released++ })

	attempt := &firstTokenAttempt{startedAt: time.Now()}
	attempt.state.Store(int32(firstTokenAttemptWaiting))
	attempt.firstTokenMs.Store(-1)
	attempt.setAcceptedCallback(func() { releaseOpenAIRequestBody(c) })

	require.Zero(t, released)
	attempt.markReceived()
	attempt.markReceived()
	require.Equal(t, 1, released)
}

func TestOpenAIRequestBodyReleaseCanBeCleared(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	released := false
	SetOpenAIRequestBodyRelease(c, func() { released = true })
	ClearOpenAIRequestBodyRelease(c)
	releaseOpenAIRequestBody(c)
	require.False(t, released)
}

func TestFirstTokenAcceptedCallbackRegisteredAfterReceiveStillRunsOnce(t *testing.T) {
	attempt := &firstTokenAttempt{startedAt: time.Now()}
	attempt.state.Store(int32(firstTokenAttemptWaiting))
	attempt.firstTokenMs.Store(-1)
	attempt.markReceived()

	var called atomic.Int32
	attempt.setAcceptedCallback(func() { called.Add(1) })
	attempt.setAcceptedCallback(func() { called.Add(1) })
	require.Equal(t, int32(1), called.Load())
}

func TestFirstTokenAcceptedCallbackConcurrentRegistration(t *testing.T) {
	for range 100 {
		attempt := &firstTokenAttempt{startedAt: time.Now()}
		attempt.state.Store(int32(firstTokenAttemptWaiting))
		attempt.firstTokenMs.Store(-1)

		var called atomic.Int32
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			attempt.setAcceptedCallback(func() { called.Add(1) })
		}()
		go func() {
			defer wg.Done()
			attempt.markReceived()
		}()
		wg.Wait()
		require.Equal(t, int32(1), called.Load())
	}
}

func TestStreamingResponseReleasesBodyWithoutFirstTokenAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":1,"output_tokens":1}}}`,
			"",
		}, "\n"))),
	}
	released := 0

	result, err := (&OpenAIGatewayService{}).handleStreamingResponseWithReasoningOnAccepted(
		context.Background(), resp, c, &Account{ID: 1}, time.Now(), "gpt-5", "gpt-5", "", func() { released++ },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, released)
}

func TestStreamingPassthroughReleasesBodyWithoutFirstTokenAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`data: {"type":"response.output_text.delta","delta":"hello"}`,
			"",
			`data: {"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":1,"output_tokens":1}}}`,
			"",
		}, "\n"))),
	}
	released := 0

	result, err := (&OpenAIGatewayService{}).handleStreamingResponsePassthroughOnAccepted(
		context.Background(), resp, c, &Account{ID: 1}, time.Now(), "gpt-5", "gpt-5", func() { released++ },
	)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, released)
}
