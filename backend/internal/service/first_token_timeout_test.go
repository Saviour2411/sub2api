package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsMeaningfulFirstTokenJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "OpenAI 前导事件", data: `{"type":"response.created","response":{"id":"resp_1"}}`, want: false},
		{name: "OpenAI usage-only", data: `{"type":"response.completed","response":{"usage":{"input_tokens":1}}}`, want: false},
		{name: "OpenAI 正文增量", data: `{"type":"response.output_text.delta","delta":"你"}`, want: true},
		{name: "OpenAI refusal 增量", data: `{"type":"response.refusal.delta","delta":"无法处理"}`, want: true},
		{name: "OpenAI 空 refusal 增量", data: `{"type":"response.refusal.delta","delta":""}`, want: false},
		{name: "OpenAI 工具参数增量", data: `{"type":"response.function_call_arguments.delta","delta":"{"}`, want: true},
		{name: "OpenAI 空工具参数增量", data: `{"type":"response.function_call_arguments.delta","delta":""}`, want: false},
		{name: "OpenAI 空对象工具参数增量", data: `{"type":"response.function_call_arguments.delta","delta":"{}"}`, want: false},
		{name: "OpenAI 函数调用项", data: `{"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_1","name":"lookup"}}`, want: true},
		{name: "OpenAI 空函数调用项", data: `{"type":"response.output_item.added","item":{"type":"function_call"}}`, want: false},
		{name: "OpenAI 普通输出项前导", data: `{"type":"response.output_item.added","item":{"type":"message","id":"msg_1"}}`, want: false},
		{name: "Chat Completions usage-only", data: `{"choices":[],"usage":{"prompt_tokens":1}}`, want: false},
		{name: "Chat Completions 思考增量", data: `{"choices":[{"delta":{"reasoning_content":"思考"}}]}`, want: true},
		{name: "Chat Completions refusal 增量", data: `{"choices":[{"delta":{"refusal":"拒绝"}}]}`, want: true},
		{name: "Chat Completions 工具调用", data: `{"choices":[{"delta":{"tool_calls":[{"id":"call_1","function":{"name":"lookup"}}]}}]}`, want: true},
		{name: "Chat Completions 空工具数组", data: `{"choices":[{"delta":{"tool_calls":[]}}]}`, want: false},
		{name: "Chat Completions 空工具对象", data: `{"choices":[{"delta":{"tool_calls":[{}]}}]}`, want: false},
		{name: "Chat Completions 空函数对象", data: `{"choices":[{"delta":{"function_call":{}}}]}`, want: false},
		{name: "Chat Completions 空函数参数", data: `{"choices":[{"delta":{"function_call":{"arguments":""}}}]}`, want: false},
		{name: "Anthropic message_start", data: `{"type":"message_start","message":{"usage":{"input_tokens":1}}}`, want: false},
		{name: "Anthropic thinking_delta", data: `{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"分析"}}`, want: true},
		{name: "Anthropic tool start", data: `{"type":"content_block_start","content_block":{"type":"tool_use","id":"toolu_1","name":"lookup","input":{}}}`, want: true},
		{name: "Anthropic 空 tool start", data: `{"type":"content_block_start","content_block":{"type":"tool_use","input":{}}}`, want: false},
		{name: "Anthropic tool delta", data: `{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{"}}`, want: true},
		{name: "Anthropic 空 tool delta", data: `{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{}"}}`, want: false},
		{name: "Gemini usage-only", data: `{"usageMetadata":{"promptTokenCount":1}}`, want: false},
		{name: "Gemini 文本", data: `{"candidates":[{"content":{"parts":[{"text":"结果"}]}}]}`, want: true},
		{name: "Gemini 函数调用", data: `{"response":{"candidates":[{"content":{"parts":[{"functionCall":{"name":"tool"}}]}}]}}`, want: true},
		{name: "Gemini 空函数调用", data: `{"candidates":[{"content":{"parts":[{"functionCall":{}}]}}]}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isMeaningfulFirstTokenJSON([]byte(tt.data)))
		})
	}
}

func TestInspectSSEFirstTokenIgnoresPreludeAndUsage(t *testing.T) {
	t.Parallel()
	buffer := []byte("event: response.created\ndata: {\"type\":\"response.created\"}\n\n" +
		": ping\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1}}}\n\n")
	received, decided := inspectSSEFirstToken(buffer)
	require.False(t, received)
	require.True(t, decided)

	buffer = append(buffer, []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")...)
	received, decided = inspectSSEFirstToken(buffer)
	require.True(t, received)
	require.True(t, decided)
}

func TestInspectSSEFirstTokenUsesEventNameWhenPayloadOmitsType(t *testing.T) {
	t.Parallel()

	received, decided := inspectSSEFirstToken([]byte(
		"event: response.output_text.delta\n" +
			"data: {\"delta\":\"ok\"}\n\n",
	))
	require.True(t, received)
	require.True(t, decided)

	received, decided = inspectSSEFirstToken([]byte(
		"event: response.completed\n" +
			"data: {\"response\":{\"status\":\"completed\"}}\n\n",
	))
	require.False(t, received)
	require.True(t, decided)
}

func TestInspectBedrockFirstToken(t *testing.T) {
	t.Parallel()
	data := []byte(`{"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}`)
	payload := []byte(`{"bytes":"` + base64.StdEncoding.EncodeToString(data) + `"}`)
	totalLength := 12 + len(payload) + 4
	frame := make([]byte, totalLength)
	binary.BigEndian.PutUint32(frame[0:4], uint32(totalLength))
	binary.BigEndian.PutUint32(frame[4:8], 0)
	copy(frame[12:], payload)

	received, decided := inspectBedrockFirstToken(frame)
	require.True(t, received)
	require.True(t, decided)
}

func TestFirstTokenAttemptBuffersDownstreamUntilMeaningfulDelta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", time.Second)
	reader, writer := io.Pipe()
	resp := &http.Response{Body: reader}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)

	_, err := c.Writer.WriteString("event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
	require.NoError(t, err)
	require.Empty(t, recorder.Body.String())

	go func() {
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.created\"}\n\n"+
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
	}()
	buf := make([]byte, 4096)
	n, err := resp.Body.Read(buf)
	require.NoError(t, err)
	require.Contains(t, string(buf[:n]), "response.output_text.delta")
	require.Contains(t, recorder.Body.String(), "message_start")
	require.NoError(t, attempt.finish(nil))
	require.Same(t, attempt.originalWriter, c.Writer)
	_ = writer.Close()
}

func TestFirstTokenAttemptTimeoutDiscardsPreludeAndReturns504(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "claude-test", 30*time.Millisecond)
	resp := &http.Response{Body: &contextReadCloser{ctx: attempt.requestCtx}}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)
	_, err := c.Writer.WriteString("event: message_start\ndata: {}\n\n")
	require.NoError(t, err)

	buf := make([]byte, 32)
	_, readErr := resp.Body.Read(buf)
	require.ErrorIs(t, readErr, errFirstTokenAttemptTimedOut)
	finishErr := attempt.finish(readErr)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, finishErr, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.True(t, failoverErr.FirstTokenTimeout)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.Empty(t, recorder.Body.String())
	require.Same(t, attempt.originalWriter, c.Writer)
}

func TestFirstTokenAttemptWrapsResponseAfterTimerAlreadyWon(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", time.Hour)
	require.True(t, attempt.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptTimedOut))
	attempt.cancel()

	resp := &http.Response{Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"late\"}\n\n"))}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)
	_, err := c.Writer.WriteString("data: {\"type\":\"response.created\"}\n\n")
	require.NoError(t, err)
	require.Empty(t, recorder.Body.String())

	_, readErr := resp.Body.Read(make([]byte, 256))
	require.ErrorIs(t, readErr, errFirstTokenAttemptTimedOut)
	finishErr := attempt.finish(readErr)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, finishErr, &failoverErr)
	require.Equal(t, http.StatusGatewayTimeout, failoverErr.StatusCode)
	require.Empty(t, recorder.Body.String())
	require.Same(t, attempt.originalWriter, c.Writer)
}

func TestFirstTokenAttemptTimeoutRestoresHeadersBeforeJSONError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Header("X-Request-Scope", "kept")

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", time.Hour)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)
	c.Header("Content-Type", "text/event-stream")
	c.Header("X-Upstream-Only", "discarded")
	require.True(t, attempt.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptTimedOut))
	attempt.cancel()

	finishErr := attempt.finish(errFirstTokenAttemptTimedOut)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, finishErr, &failoverErr)
	require.Equal(t, "kept", c.Writer.Header().Get("X-Request-Scope"))
	require.Empty(t, c.Writer.Header().Get("X-Upstream-Only"))
	require.Empty(t, c.Writer.Header().Get("Content-Type"))

	c.JSON(http.StatusGatewayTimeout, gin.H{"error": gin.H{"type": "first_token_timeout"}})
	require.Equal(t, http.StatusGatewayTimeout, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.Contains(t, recorder.Body.String(), "first_token_timeout")
}

func TestFirstTokenAttemptRejectsOversizedUpstreamPrelude(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", time.Hour)
	resp := &http.Response{Body: io.NopCloser(bytes.NewReader(bytes.Repeat([]byte(":"), maxFirstTokenPreludeBytes+1)))}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)

	_, readErr := resp.Body.Read(make([]byte, 1))
	require.ErrorIs(t, readErr, errFirstTokenPreludeTooLarge)
	finishErr := attempt.finish(readErr)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, finishErr, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.False(t, failoverErr.FirstTokenTimeout)
	require.Empty(t, recorder.Body.String())
}

func TestFirstTokenAttemptRejectsOversizedDownstreamPrelude(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	attempt := newFirstTokenAttemptWithTimeout(c.Request.Context(), c, nil, &Account{ID: 1}, "gpt-test", time.Hour)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(""))}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)

	_, writeErr := c.Writer.Write(bytes.Repeat([]byte("x"), maxFirstTokenPreludeBytes+1))
	require.ErrorIs(t, writeErr, errFirstTokenPreludeTooLarge)
	finishErr := attempt.finish(writeErr)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, finishErr, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, recorder.Body.String())
}

func TestFirstTokenAttemptBindRequestPreservesBuilderContextValues(t *testing.T) {
	type contextKey struct{}
	const wantValue = "openai-transport-profile"

	attempt := newFirstTokenAttemptWithTimeout(context.Background(), nil, nil, &Account{ID: 1}, "gpt-test", time.Second)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextKey{}, wantValue))

	bound := attempt.bindRequest(req)
	require.Equal(t, wantValue, bound.Context().Value(contextKey{}))
	attempt.cleanup()
	require.ErrorIs(t, bound.Context().Err(), context.Canceled)
}

func TestFirstTokenAttemptClientCancelDoesNotMarkTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestCtx, cancel := context.WithCancel(context.Background())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(requestCtx)

	attempt := newFirstTokenAttemptWithTimeout(requestCtx, c, nil, &Account{ID: 1}, "gpt-test", time.Second)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n"))}
	attempt.wrapResponse(resp, c, firstTokenProtocolSSE)
	cancel()
	require.Eventually(t, func() bool {
		return attempt.currentState() == firstTokenAttemptClientCanceled
	}, time.Second, time.Millisecond)

	payload, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	require.Contains(t, string(payload), "response.output_text.delta")
	finishErr := attempt.finish(nil)
	require.NoError(t, finishErr)
	require.False(t, isFirstTokenTimeoutFailover(finishErr))
}

func TestFirstTokenAttemptClientCancelPreservesBufferedPrelude(t *testing.T) {
	requestCtx, cancel := context.WithCancel(context.Background())
	attempt := newFirstTokenAttemptWithTimeout(requestCtx, nil, nil, &Account{ID: 1}, "gpt-test", time.Second)
	upstream := &cancelAfterFirstReadCloser{
		cancel: cancel,
		chunks: [][]byte{
			[]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\""),
			[]byte("ok\"}\n\n"),
		},
	}
	resp := &http.Response{Body: upstream}
	attempt.wrapResponse(resp, nil, firstTokenProtocolSSE)

	payload, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n", string(payload))
	require.NoError(t, attempt.finish(nil))
}

type cancelAfterFirstReadCloser struct {
	cancel context.CancelFunc
	chunks [][]byte
	reads  int
}

func (r *cancelAfterFirstReadCloser) Read(p []byte) (int, error) {
	if r.reads >= len(r.chunks) {
		return 0, io.EOF
	}
	n := copy(p, r.chunks[r.reads])
	r.reads++
	if r.reads == 1 {
		r.cancel()
	}
	return n, nil
}

func (r *cancelAfterFirstReadCloser) Close() error { return nil }

type contextReadCloser struct {
	ctx context.Context
}

func (r *contextReadCloser) Read([]byte) (int, error) {
	<-r.ctx.Done()
	return 0, r.ctx.Err()
}

func (r *contextReadCloser) Close() error { return nil }

type firstTokenAccountRepoStub struct {
	AccountRepository
	schedulableID    int64
	schedulableValue bool
	extraID          int64
	extraUpdates     map[string]any
}

func (r *firstTokenAccountRepoStub) SetSchedulable(_ context.Context, id int64, schedulable bool) error {
	r.schedulableID = id
	r.schedulableValue = schedulable
	return nil
}

func (r *firstTokenAccountRepoStub) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	r.extraID = id
	r.extraUpdates = updates
	return nil
}

type firstTokenProbeSchedulerStub struct {
	accountID int64
}

func (s *firstTokenProbeSchedulerStub) EnsureAutoManagedProbe(_ context.Context, accountID int64) error {
	s.accountID = accountID
	return nil
}

type firstTokenAtomicRepoStub struct {
	AccountRepository
	err       error
	called    bool
	accountID int64
	marker    map[string]any
}

func (r *firstTokenAtomicRepoStub) PersistFirstTokenTimeoutState(_ context.Context, accountID int64, marker map[string]any, _ time.Time) error {
	r.called = true
	r.accountID = accountID
	r.marker = marker
	return r.err
}

func TestHandleFirstTokenTimeoutPersistsMarkerAndStartsProbe(t *testing.T) {
	repo := &firstTokenAccountRepoStub{}
	probe := &firstTokenProbeSchedulerStub{}
	svc := &RateLimitService{accountRepo: repo, autoManagedProbe: probe}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Schedulable: true, Extra: map[string]any{}}

	svc.HandleFirstTokenTimeout(context.Background(), account, "gpt-test", 17)

	require.False(t, account.Schedulable)
	require.Equal(t, int64(42), repo.schedulableID)
	require.False(t, repo.schedulableValue)
	require.Equal(t, int64(42), repo.extraID)
	require.Equal(t, int64(42), probe.accountID)
	marker, ok := repo.extraUpdates[accountFailureStrategyUnscheduledKey].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "first_token_timeout", marker[accountFailureStrategyUnscheduledSourceKey])
	require.Equal(t, "gpt-test", marker[accountFailureStrategyUnscheduledModelKey])
	require.Equal(t, 17, marker[accountFailureStrategyUnscheduledTimeoutSecondsKey])
	require.Equal(t, http.StatusGatewayTimeout, marker[accountFailureStrategyUnscheduledStatusCodeKey])
	require.True(t, strings.Contains(marker[accountFailureStrategyUnscheduledReasonKey].(string), "source=first_token_timeout"))
}

func TestHandleFirstTokenTimeoutAtomicFailureKeepsAccountSchedulable(t *testing.T) {
	persistErr := errors.New("transaction failed")
	repo := &firstTokenAtomicRepoStub{err: persistErr}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Schedulable: true, Extra: map[string]any{}}

	err := svc.HandleFirstTokenTimeout(context.Background(), account, "gpt-test", 17)

	require.ErrorIs(t, err, persistErr)
	require.True(t, repo.called)
	require.Equal(t, int64(42), repo.accountID)
	require.True(t, account.Schedulable)
	require.False(t, account.HasFailureStrategyUnscheduledMarker())
}

func TestHandleFirstTokenTimeoutAtomicSuccessMutatesMemoryAfterCommit(t *testing.T) {
	repo := &firstTokenAtomicRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Schedulable: true, Extra: map[string]any{}}

	err := svc.HandleFirstTokenTimeout(context.Background(), account, "gpt-test", 17)

	require.NoError(t, err)
	require.True(t, repo.called)
	require.False(t, account.Schedulable)
	require.True(t, account.HasFailureStrategyUnscheduledMarker())
}

func TestFirstTokenTimeoutErrorIsNotSameAccountRetryable(t *testing.T) {
	attempt := &firstTokenAttempt{timeoutSeconds: 60}
	err := attempt.timeoutFailoverError()
	require.False(t, err.RetryableOnSameAccount)
	require.True(t, err.FirstTokenTimeout)
}

func TestNewFirstTokenAttemptExcludesNonTextAndClientWebSocketRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := &Account{ID: 1}

	tests := []struct {
		name      string
		path      string
		stream    bool
		excluded  bool
		transport OpenAIClientTransport
	}{
		{name: "非流式", path: "/v1/responses", stream: false},
		{name: "图片", path: "/v1/images/generations", stream: true},
		{name: "视频", path: "/v1/videos", stream: true},
		{name: "音频", path: "/v1/audio/speech", stream: true},
		{name: "Embedding", path: "/v1/embeddings", stream: true},
		{name: "批量任务", path: "/v1/batches", stream: true},
		{name: "Gemini Count Tokens", path: "/v1beta/models/test:countTokens", stream: true},
		{name: "兼容 Count Tokens", path: "/v1/models/test:count_tokens", stream: true},
		{name: "显式排除", path: "/v1/responses", stream: true, excluded: true},
		{name: "客户端 WebSocket", path: "/v1/responses", stream: true, transport: OpenAIClientTransportWS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			if tt.excluded {
				c.Set("first_token_timeout_excluded", true)
			}
			if tt.transport != OpenAIClientTransportUnknown {
				SetOpenAIClientTransport(c, tt.transport)
			}
			require.Nil(t, newFirstTokenAttempt(c.Request.Context(), c, nil, account, "model", tt.stream))
		})
	}
}
