package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	defaultFirstTokenTimeoutSeconds = 60
	maxFirstTokenPreludeBytes       = 8 << 20
)

type firstTokenProtocol uint8

const (
	firstTokenProtocolSSE firstTokenProtocol = iota
	firstTokenProtocolBedrock
	firstTokenProtocolOpenAICompact
)

type firstTokenAttemptState int32

const (
	firstTokenAttemptWaiting firstTokenAttemptState = iota
	firstTokenAttemptReceived
	firstTokenAttemptDecidedWithoutToken
	firstTokenAttemptTimedOut
	firstTokenAttemptClientCanceled
	firstTokenAttemptPreludeOverflow
	firstTokenAttemptStopped
)

var (
	errFirstTokenAttemptTimedOut = errors.New("upstream first token timeout")
	errFirstTokenPreludeTooLarge = errors.New("upstream first token prelude exceeds buffer limit")
)

type firstTokenAttemptContextKey struct{}

// StartFirstTokenAttemptFromContext 在 HTTP 客户端真正发起上游请求前启动首 Token 计时。
func StartFirstTokenAttemptFromContext(ctx context.Context) {
	if ctx == nil {
		return
	}
	attempt, _ := ctx.Value(firstTokenAttemptContextKey{}).(*firstTokenAttempt)
	if attempt != nil {
		attempt.start()
	}
}

func isFirstTokenTimeoutFailover(err error) bool {
	var failoverErr *UpstreamFailoverError
	return errors.As(err, &failoverErr) && failoverErr.FirstTokenTimeout
}

// firstTokenAttempt 覆盖一次账号上游尝试，从发起请求开始计时，直到收到首个有效生成增量。
// 它不会把 deadline 直接挂到请求 context 上，避免首 Token 到达后仍误杀长流。
type firstTokenAttempt struct {
	state          atomic.Int32
	timeout        time.Duration
	timeoutSeconds int
	startedAt      time.Time
	firstTokenMs   atomic.Int64
	model          string
	account        *Account
	rateLimit      *RateLimitService
	failurePolicy  AccountFailureStreakPolicy
	failureLimit   int
	checkPolicy    bool
	timeoutEvent   AccountFailureStreakEvent
	requestCtx     context.Context
	ginCtx         *gin.Context
	cancel         context.CancelFunc
	timer          *time.Timer
	clientStop     func() bool

	mu             sync.Mutex
	startOnce      sync.Once
	closed         bool
	bufferedWriter *firstTokenBufferedResponseWriter
	originalWriter gin.ResponseWriter
	sideEffectOnce sync.Once
	closeOnce      sync.Once
}

func resolveFirstTokenTimeoutSettings(ctx context.Context, rateLimit *RateLimitService) GatewaySettings {
	settings := DefaultGatewaySettings()
	if rateLimit != nil && rateLimit.settingService != nil {
		settings = rateLimit.settingService.GetGatewayRuntime(ctx)
	}
	if settings.FirstTokenTimeoutSeconds < 0 {
		settings.FirstTokenTimeoutSeconds = 0
	}
	if settings.FirstTokenTimeoutSeconds > MaxGatewayFirstTokenTimeoutSeconds {
		settings.FirstTokenTimeoutSeconds = MaxGatewayFirstTokenTimeoutSeconds
	}
	return settings
}

func newFirstTokenAttempt(
	requestCtx context.Context,
	c *gin.Context,
	rateLimit *RateLimitService,
	account *Account,
	model string,
	stream bool,
) *firstTokenAttempt {
	if !stream || account == nil || IsGeneratedImageModel(model) || firstTokenTimeoutExcluded(c) {
		resumeOpenAICompactSSEKeepalive(c)
		return nil
	}
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	settings := resolveFirstTokenTimeoutSettings(requestCtx, rateLimit)
	timeout := time.Duration(settings.FirstTokenTimeoutSeconds) * time.Second
	if timeout <= 0 {
		resumeOpenAICompactSSEKeepalive(c)
		return nil
	}
	attempt := newFirstTokenAttemptWithTimeout(requestCtx, c, rateLimit, account, model, timeout)
	if attempt != nil {
		attempt.failurePolicy = BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, settings)
		attempt.failureLimit = settings.FirstTokenTimeoutConsecutiveThreshold
		attempt.checkPolicy = true
	}
	return attempt
}

func newFirstTokenAttemptWithTimeout(
	requestCtx context.Context,
	c *gin.Context,
	rateLimit *RateLimitService,
	account *Account,
	model string,
	timeout time.Duration,
) *firstTokenAttempt {
	if timeout <= 0 || account == nil {
		return nil
	}
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	// 账号尝试开始后由本守卫负责等待首 Token，暂停 handler 层预响应心跳，
	// 避免心跳先提交 200 导致后续无法切换账号。
	StopPreResponseKeepaliveBeforeResponseFromContext(requestCtx)
	upstreamCtx, cancel := context.WithCancel(context.WithoutCancel(requestCtx))
	attempt := &firstTokenAttempt{
		timeout:        timeout,
		timeoutSeconds: int((timeout + time.Second - 1) / time.Second),
		model:          strings.TrimSpace(model),
		account:        account,
		rateLimit:      rateLimit,
		requestCtx:     requestCtx,
		ginCtx:         c,
		cancel:         cancel,
	}
	failureSettings := DefaultGatewaySettings()
	failureSettings.FirstTokenTimeoutSeconds = attempt.timeoutSeconds
	attempt.failurePolicy = BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, failureSettings)
	attempt.failureLimit = failureSettings.FirstTokenTimeoutConsecutiveThreshold
	attempt.firstTokenMs.Store(-1)
	attempt.state.Store(int32(firstTokenAttemptWaiting))
	attempt.clientStop = context.AfterFunc(requestCtx, func() {
		if attempt.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptClientCanceled) {
			attempt.stopTimer()
			attempt.dropWriterBuffer()
		}
	})
	attempt.requestCtx = firstTokenDispatchContext{
		Context: context.WithValue(upstreamCtx, firstTokenAttemptContextKey{}, attempt),
		attempt: attempt,
	}
	attempt.wrapWriter(c)
	return attempt
}

func (a *firstTokenAttempt) start() {
	if a == nil {
		return
	}
	a.startOnce.Do(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.closed || a.currentState() != firstTokenAttemptWaiting {
			return
		}
		a.startedAt = time.Now()
		a.timer = time.AfterFunc(a.timeout, func() {
			if a.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptTimedOut) {
				a.setTimeoutEvent(time.Now().UTC())
				if a.cancel != nil {
					a.cancel()
				}
			}
		})
	})
}

func (a *firstTokenAttempt) stopTimer() {
	if a == nil {
		return
	}
	a.mu.Lock()
	timer := a.timer
	a.mu.Unlock()
	if timer != nil {
		timer.Stop()
	}
}

// firstTokenDispatchContext 让自定义 HTTPUpstream 在观察请求取消信号时也能
// 启动计时。生产 HTTPUpstream 会在 client.Do 前显式启动，因此这里通常是幂等兜底。
type firstTokenDispatchContext struct {
	context.Context
	attempt *firstTokenAttempt
}

func (c firstTokenDispatchContext) Deadline() (time.Time, bool) {
	if c.attempt != nil {
		c.attempt.start()
	}
	return c.Context.Deadline()
}

func (c firstTokenDispatchContext) Done() <-chan struct{} {
	if c.attempt != nil {
		c.attempt.start()
	}
	return c.Context.Done()
}

func (c firstTokenDispatchContext) Err() error {
	if c.attempt != nil {
		c.attempt.start()
	}
	return c.Context.Err()
}

func firstTokenTimeoutExcluded(c *gin.Context) bool {
	if c == nil {
		return false
	}
	if GetOpenAIClientTransport(c) == OpenAIClientTransportWS {
		return true
	}
	if excluded, ok := c.Get("first_token_timeout_excluded"); ok {
		if value, ok := excluded.(bool); ok && value {
			return true
		}
	}
	if c.Request == nil || c.Request.URL == nil {
		return false
	}
	path := strings.ToLower(c.Request.URL.Path)
	for _, part := range []string{"/images", "/videos", "/audio", "/embeddings", "/batches", "counttokens", "count_tokens"} {
		if strings.Contains(path, part) {
			return true
		}
	}
	return false
}

func (a *firstTokenAttempt) bindRequest(req *http.Request) *http.Request {
	if a == nil || req == nil {
		return req
	}
	return req.Clone(firstTokenMergedContext{
		lifecycle: a.requestCtx,
		values:    req.Context(),
	})
}

// firstTokenMergedContext 保留 builder 写入请求 context 的 transport/profile 值，
// 同时只让首 Token 守卫控制请求的取消生命周期。
type firstTokenMergedContext struct {
	lifecycle context.Context
	values    context.Context
}

func (c firstTokenMergedContext) Deadline() (time.Time, bool) {
	return c.lifecycle.Deadline()
}

func (c firstTokenMergedContext) Done() <-chan struct{} {
	return c.lifecycle.Done()
}

func (c firstTokenMergedContext) Err() error {
	return c.lifecycle.Err()
}

func (c firstTokenMergedContext) Value(key any) any {
	if c.values != nil {
		if value := c.values.Value(key); value != nil {
			return value
		}
	}
	return c.lifecycle.Value(key)
}

func (a *firstTokenAttempt) upstreamContext(fallback context.Context) context.Context {
	if a == nil || a.requestCtx == nil {
		return fallback
	}
	return a.requestCtx
}

func (a *firstTokenAttempt) wrapResponse(resp *http.Response, c *gin.Context, protocol firstTokenProtocol) {
	if a == nil || resp == nil || resp.Body == nil {
		return
	}
	// 生产路径会在 HTTPUpstream.Do 紧邻网络请求前启动；这里作为自定义
	// HTTPUpstream 和单元测试的兜底，确保读取响应体前计时器已经启动。
	a.start()
	a.wrapWriter(c)
	resp.Body = &firstTokenReadCloser{
		upstream: resp.Body,
		attempt:  a,
		protocol: protocol,
	}
}

func (a *firstTokenAttempt) wrapWriter(c *gin.Context) {
	if a == nil || c == nil || c.Writer == nil {
		return
	}

	state := a.currentState()
	a.mu.Lock()
	if a.bufferedWriter != nil {
		a.mu.Unlock()
		return
	}
	a.originalWriter = c.Writer
	a.bufferedWriter = newFirstTokenBufferedResponseWriter(c.Writer, a)
	// 已主动停止的兼容重试不再受首 Token 计时约束，但仍使用同一包装，
	// 以保证 finish 可以统一恢复 gin writer，且不会无限缓存后续长流。
	switch state {
	case firstTokenAttemptClientCanceled:
		a.bufferedWriter.DropBufferedWrites()
	case firstTokenAttemptStopped, firstTokenAttemptReceived, firstTokenAttemptDecidedWithoutToken:
		a.bufferedWriter.EnablePassthrough()
	}
	c.Writer = a.bufferedWriter
	a.mu.Unlock()
}

func (a *firstTokenAttempt) cleanup() {
	if a == nil {
		return
	}
	a.closeOnce.Do(func() {
		a.mu.Lock()
		a.closed = true
		timer := a.timer
		a.mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		if a.clientStop != nil {
			a.clientStop()
		}
		if a.cancel != nil {
			a.cancel()
		}
	})
}

type firstTokenCleanupReadCloser struct {
	upstream io.ReadCloser
	cleanup  func()
	once     sync.Once
}

func (r *firstTokenCleanupReadCloser) Read(p []byte) (int, error) {
	n, err := r.upstream.Read(p)
	if err != nil {
		r.runCleanup()
	}
	return n, err
}

func (r *firstTokenCleanupReadCloser) Close() error {
	err := r.upstream.Close()
	r.runCleanup()
	return err
}

func (r *firstTokenCleanupReadCloser) runCleanup() {
	if r == nil {
		return
	}
	r.once.Do(func() {
		if r.cleanup != nil {
			r.cleanup()
		}
	})
}

func (a *firstTokenAttempt) stopBeforeStreaming(responses ...*http.Response) {
	if a == nil {
		return
	}
	stopped := a.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptStopped)
	if stopped {
		a.stopTimer()
	}
	if a.clientStop != nil {
		a.clientStop()
	}
	if stopped || a.currentState() == firstTokenAttemptStopped {
		a.enableWriterPassthroughAndRestore()
	}

	var resp *http.Response
	if len(responses) > 0 {
		resp = responses[0]
	}
	if resp == nil || resp.Body == nil {
		a.cleanup()
		return
	}
	resp.Body = &firstTokenCleanupReadCloser{
		upstream: resp.Body,
		cleanup:  a.cleanup,
	}
}

func (a *firstTokenAttempt) enableWriterPassthroughAndRestore() {
	if a == nil {
		return
	}
	a.mu.Lock()
	w := a.bufferedWriter
	a.mu.Unlock()
	if w != nil {
		w.EnablePassthrough()
	}
	a.restoreWriter()
}

func (a *firstTokenAttempt) finish(streamErr error) error {
	if a == nil {
		return streamErr
	}
	a.cleanup()

	state := a.currentState()
	if state == firstTokenAttemptTimedOut {
		a.discardAndRestoreWriter()
		a.recordTimeoutSideEffects()
		return a.timeoutFailoverError()
	}
	if state == firstTokenAttemptClientCanceled {
		a.dropWriterBuffer()
		a.restoreWriter()
		return streamErr
	}
	if state == firstTokenAttemptPreludeOverflow {
		a.discardAndRestoreWriter()
		return a.preludeOverflowFailoverError()
	}
	if state == firstTokenAttemptDecidedWithoutToken {
		var failoverErr *UpstreamFailoverError
		if errors.As(streamErr, &failoverErr) {
			a.discardAndRestoreWriter()
			return streamErr
		}
		a.releaseAndRestoreWriter()
		return streamErr
	}
	if state == firstTokenAttemptReceived {
		a.releaseAndRestoreWriter()
		return streamErr
	}
	a.discardAndRestoreWriter()
	return streamErr
}

func (a *firstTokenAttempt) finishRequestError(requestErr error) error {
	if a == nil {
		return requestErr
	}
	return a.finish(requestErr)
}

func (a *firstTokenAttempt) markReceived() {
	if a == nil || !a.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptReceived) {
		return
	}
	firstTokenMs := time.Since(a.startedAt).Milliseconds()
	if firstTokenMs < 0 {
		firstTokenMs = 0
	}
	a.firstTokenMs.Store(firstTokenMs)
	a.stopTimer()
	if a.clientStop != nil {
		a.clientStop()
	}
	a.releaseWriter()
}

func (a *firstTokenAttempt) observedFirstTokenMs() *int {
	if a == nil {
		return nil
	}
	value := a.firstTokenMs.Load()
	if value < 0 {
		return nil
	}
	result := int(value)
	return &result
}

func (a *firstTokenAttempt) markDecidedWithoutToken() {
	if a == nil || !a.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptDecidedWithoutToken) {
		return
	}
	a.stopTimer()
	if a.clientStop != nil {
		a.clientStop()
	}
}

func (a *firstTokenAttempt) markPreludeOverflow() {
	if a == nil || !a.compareAndSwapState(firstTokenAttemptWaiting, firstTokenAttemptPreludeOverflow) {
		return
	}
	a.stopTimer()
	if a.clientStop != nil {
		a.clientStop()
	}
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *firstTokenAttempt) currentState() firstTokenAttemptState {
	if a == nil {
		return firstTokenAttemptStopped
	}
	return firstTokenAttemptState(a.state.Load())
}

func (a *firstTokenAttempt) compareAndSwapState(old, next firstTokenAttemptState) bool {
	return a.state.CompareAndSwap(int32(old), int32(next))
}

func (a *firstTokenAttempt) timeoutFailoverError() *UpstreamFailoverError {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    "first_token_timeout",
			"message": fmt.Sprintf("upstream did not return a first token within %d seconds", a.timeoutSeconds),
		},
	})
	return &UpstreamFailoverError{
		StatusCode:             http.StatusGatewayTimeout,
		ResponseBody:           body,
		RetryableOnSameAccount: false,
		FirstTokenTimeout:      true,
	}
}

func (a *firstTokenAttempt) preludeOverflowFailoverError() *UpstreamFailoverError {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    "upstream_protocol_error",
			"message": "upstream sent too much protocol prelude before the first token",
		},
	})
	return &UpstreamFailoverError{
		StatusCode:             http.StatusBadGateway,
		ResponseBody:           body,
		RetryableOnSameAccount: false,
	}
}

func (a *firstTokenAttempt) recordTimeoutSideEffects() {
	a.sideEffectOnce.Do(func() {
		if a.rateLimit != nil {
			_ = a.rateLimit.handleFirstTokenTimeoutOutcome(
				a.requestCtx,
				a.account,
				a.model,
				a.timeoutSeconds,
				a.failurePolicy,
				a.failureLimit,
				a.firstTokenTimeoutEvent(),
				a.checkPolicy,
			)
		}
		if a.ginCtx != nil && a.account != nil {
			message := fmt.Sprintf("first token timeout after %d seconds", a.timeoutSeconds)
			appendOpsUpstreamError(a.ginCtx, OpsUpstreamErrorEvent{
				Platform:           a.account.Platform,
				AccountID:          a.account.ID,
				AccountName:        a.account.Name,
				UpstreamStatusCode: http.StatusGatewayTimeout,
				Kind:               "first_token_timeout",
				Message:            message,
				Detail:             fmt.Sprintf("source=first_token_timeout model=%s timeout_seconds=%d", a.model, a.timeoutSeconds),
			})
			setOpsUpstreamError(a.ginCtx, http.StatusGatewayTimeout, message, "")
		}
	})
}

func (a *firstTokenAttempt) setTimeoutEvent(occurredAt time.Time) {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.timeoutEvent.OccurredAt.IsZero() || strings.TrimSpace(a.timeoutEvent.ID) == "" {
		a.timeoutEvent = NewAccountFailureStreakEvent(occurredAt)
	}
	a.mu.Unlock()
}

func (a *firstTokenAttempt) firstTokenTimeoutEvent() AccountFailureStreakEvent {
	if a == nil {
		return NewAccountFailureStreakEvent(time.Now().UTC())
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.timeoutEvent.OccurredAt.IsZero() || strings.TrimSpace(a.timeoutEvent.ID) == "" {
		a.timeoutEvent = NewAccountFailureStreakEvent(time.Now().UTC())
	}
	return a.timeoutEvent
}

func (a *firstTokenAttempt) releaseWriter() {
	a.mu.Lock()
	w := a.bufferedWriter
	a.mu.Unlock()
	if w != nil {
		w.Release()
	}
}

func (a *firstTokenAttempt) releaseAndRestoreWriter() {
	a.releaseWriter()
	a.restoreWriter()
}

func (a *firstTokenAttempt) discardAndRestoreWriter() {
	a.discardWriter()
	a.restoreWriter()
}

func (a *firstTokenAttempt) discardWriter() {
	if a == nil {
		return
	}
	a.mu.Lock()
	w := a.bufferedWriter
	a.mu.Unlock()
	if w != nil {
		w.Discard()
	}
}

func (a *firstTokenAttempt) dropWriterBuffer() {
	if a == nil {
		return
	}
	a.mu.Lock()
	w := a.bufferedWriter
	a.mu.Unlock()
	if w != nil {
		w.DropBufferedWrites()
	}
}

func (a *firstTokenAttempt) restoreWriter() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ginCtx != nil && a.originalWriter != nil && a.ginCtx.Writer == a.bufferedWriter {
		a.ginCtx.Writer = a.originalWriter
	}
}

type firstTokenReadCloser struct {
	upstream io.ReadCloser
	attempt  *firstTokenAttempt
	protocol firstTokenProtocol

	mu       sync.Mutex
	buffer   []byte
	readable bool
}

func (r *firstTokenReadCloser) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for {
		if len(r.buffer) > 0 && r.readable {
			n := copy(p, r.buffer)
			r.buffer = r.buffer[n:]
			return n, nil
		}
		switch r.attempt.currentState() {
		case firstTokenAttemptTimedOut:
			return 0, errFirstTokenAttemptTimedOut
		case firstTokenAttemptPreludeOverflow:
			return 0, errFirstTokenPreludeTooLarge
		case firstTokenAttemptClientCanceled:
			if len(r.buffer) > 0 {
				r.readable = true
				continue
			}
			return r.upstream.Read(p)
		case firstTokenAttemptStopped, firstTokenAttemptReceived, firstTokenAttemptDecidedWithoutToken:
			return r.upstream.Read(p)
		}

		chunk := make([]byte, 32*1024)
		n, err := r.upstream.Read(chunk)
		switch r.attempt.currentState() {
		case firstTokenAttemptTimedOut:
			return 0, errFirstTokenAttemptTimedOut
		case firstTokenAttemptPreludeOverflow:
			return 0, errFirstTokenPreludeTooLarge
		}
		if n > 0 {
			r.buffer = append(r.buffer, chunk[:n]...)
			if len(r.buffer) > maxFirstTokenPreludeBytes {
				r.buffer = nil
				r.attempt.markPreludeOverflow()
				return 0, errFirstTokenPreludeTooLarge
			}
			received, decided := inspectFirstTokenBuffer(r.buffer, r.protocol)
			if received {
				r.attempt.markReceived()
				r.readable = true
				continue
			}
			if decided {
				r.attempt.markDecidedWithoutToken()
				r.readable = true
				continue
			}
		}
		if err != nil {
			state := r.attempt.currentState()
			switch state {
			case firstTokenAttemptTimedOut:
				return 0, errFirstTokenAttemptTimedOut
			}
			if errors.Is(err, io.EOF) {
				r.attempt.markDecidedWithoutToken()
				r.readable = true
				if len(r.buffer) > 0 {
					continue
				}
			}
			return 0, err
		}
	}
}

func (r *firstTokenReadCloser) Close() error {
	if r == nil || r.upstream == nil {
		return nil
	}
	return r.upstream.Close()
}

func inspectFirstTokenBuffer(buffer []byte, protocol firstTokenProtocol) (received bool, decided bool) {
	switch protocol {
	case firstTokenProtocolBedrock:
		return inspectBedrockFirstToken(buffer)
	case firstTokenProtocolOpenAICompact:
		return inspectOpenAICompactFirstToken(buffer)
	default:
		return inspectSSEFirstToken(buffer)
	}
}

func inspectOpenAICompactFirstToken(buffer []byte) (received bool, decided bool) {
	if bodyHasSSEFraming(buffer) {
		return inspectSSEFirstToken(buffer)
	}
	trimmed := bytes.TrimSpace(buffer)
	if len(trimmed) == 0 || !gjson.ValidBytes(trimmed) {
		return false, false
	}
	root := gjson.ParseBytes(trimmed)
	for _, item := range root.Get("output").Array() {
		if openAICompactOutputItemMeaningful(item) {
			return true, true
		}
	}
	status := strings.ToLower(strings.TrimSpace(root.Get("status").String()))
	if root.Get("error").Exists() || status == "completed" || status == "failed" || status == "cancelled" || status == "incomplete" {
		return false, true
	}
	return false, false
}

func inspectSSEFirstToken(buffer []byte) (received bool, decided bool) {
	normalized := bytes.ReplaceAll(buffer, []byte("\r\n"), []byte("\n"))
	events := bytes.Split(normalized, []byte("\n\n"))
	for i := 0; i < len(events)-1; i++ {
		var eventType string
		var dataLines []string
		for _, rawLine := range bytes.Split(events[i], []byte("\n")) {
			line := strings.TrimSpace(string(rawLine))
			if parsedEventType, ok := extractOpenAISSEEventLine(line); ok {
				eventType = parsedEventType
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if len(dataLines) == 0 {
			continue
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			decided = true
			continue
		}
		data = openAICompatPayloadWithEventType(data, eventType)
		if isMeaningfulFirstTokenJSON([]byte(data)) {
			return true, true
		}
		if isTerminalOrErrorStreamJSON([]byte(data)) {
			decided = true
		}
	}
	return false, decided
}

func inspectBedrockFirstToken(buffer []byte) (received bool, decided bool) {
	for offset := 0; offset+12 <= len(buffer); {
		totalLength := int(binary.BigEndian.Uint32(buffer[offset : offset+4]))
		headersLength := int(binary.BigEndian.Uint32(buffer[offset+4 : offset+8]))
		if totalLength < 16 || headersLength < 0 || 12+headersLength+4 > totalLength {
			return false, false
		}
		if offset+totalLength > len(buffer) {
			return false, decided
		}
		payloadStart := offset + 12 + headersLength
		payloadEnd := offset + totalLength - 4
		if payloadStart <= payloadEnd {
			encoded := gjson.GetBytes(buffer[payloadStart:payloadEnd], "bytes").String()
			if encoded != "" {
				if data, err := base64.StdEncoding.DecodeString(encoded); err == nil {
					if isMeaningfulFirstTokenJSON(data) {
						return true, true
					}
					if isTerminalOrErrorStreamJSON(data) {
						decided = true
					}
				}
			}
		}
		offset += totalLength
	}
	return false, decided
}

func isMeaningfulFirstTokenJSON(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}
	eventType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(data, "type").String()))
	switch eventType {
	case "response.output_text.delta", "response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.refusal.delta":
		return gjson.GetBytes(data, "delta").String() != ""
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		return firstTokenToolPayloadMeaningful(gjson.GetBytes(data, "delta"))
	case "response.output_item.added", "response.output_item.done":
		item := gjson.GetBytes(data, "item")
		itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
		if itemType == "function_call" || itemType == "custom_tool_call" {
			return firstTokenFunctionCallMeaningful(item)
		}
		return openAICompactOutputItemMeaningful(item)
	case "content_block_start":
		contentBlock := gjson.GetBytes(data, "content_block")
		return strings.EqualFold(strings.TrimSpace(contentBlock.Get("type").String()), "tool_use") &&
			firstTokenFunctionCallMeaningful(contentBlock)
	case "content_block_delta":
		deltaType := strings.ToLower(gjson.GetBytes(data, "delta.type").String())
		switch deltaType {
		case "text_delta", "thinking_delta":
			return gjson.GetBytes(data, "delta.text").String() != "" || gjson.GetBytes(data, "delta.thinking").String() != ""
		case "input_json_delta":
			return firstTokenToolPayloadMeaningful(gjson.GetBytes(data, "delta.partial_json"))
		}
	}

	choices := gjson.GetBytes(data, "choices")
	if choices.IsArray() {
		for _, choice := range choices.Array() {
			delta := choice.Get("delta")
			if delta.Get("content").String() != "" ||
				delta.Get("reasoning_content").String() != "" ||
				delta.Get("reasoning").String() != "" ||
				delta.Get("refusal").String() != "" ||
				firstTokenToolCallsMeaningful(delta.Get("tool_calls")) ||
				firstTokenFunctionCallMeaningful(delta.Get("function_call")) {
				return true
			}
		}
	}

	return geminiJSONHasGeneratedPart(gjson.ParseBytes(data)) || geminiJSONHasGeneratedPart(gjson.GetBytes(data, "response"))
}

func openAICompactOutputItemMeaningful(item gjson.Result) bool {
	itemType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	if itemType != "compaction" && itemType != "compaction_summary" {
		return false
	}
	if strings.TrimSpace(item.Get("encrypted_content").String()) != "" {
		return true
	}
	for _, summary := range item.Get("summary").Array() {
		if strings.TrimSpace(summary.Get("text").String()) != "" {
			return true
		}
	}
	return false
}

func geminiJSONHasGeneratedPart(root gjson.Result) bool {
	if !root.Exists() {
		return false
	}
	candidates := root.Get("candidates")
	if !candidates.IsArray() {
		return false
	}
	for _, candidate := range candidates.Array() {
		parts := candidate.Get("content.parts")
		if !parts.IsArray() {
			continue
		}
		for _, part := range parts.Array() {
			if part.Get("text").String() != "" ||
				firstTokenFunctionCallMeaningful(part.Get("functionCall")) ||
				firstTokenFunctionCallMeaningful(part.Get("function_call")) {
				return true
			}
		}
	}
	return false
}

func firstTokenToolCallsMeaningful(toolCalls gjson.Result) bool {
	if !toolCalls.IsArray() {
		return false
	}
	for _, toolCall := range toolCalls.Array() {
		if strings.TrimSpace(toolCall.Get("id").String()) != "" ||
			firstTokenFunctionCallMeaningful(toolCall.Get("function")) {
			return true
		}
	}
	return false
}

func firstTokenFunctionCallMeaningful(call gjson.Result) bool {
	if !call.IsObject() {
		return false
	}
	for _, key := range []string{"id", "call_id", "name"} {
		if strings.TrimSpace(call.Get(key).String()) != "" {
			return true
		}
	}
	for _, key := range []string{"arguments", "args", "input"} {
		if firstTokenToolPayloadMeaningful(call.Get(key)) {
			return true
		}
	}
	return false
}

func firstTokenToolPayloadMeaningful(payload gjson.Result) bool {
	if !payload.Exists() {
		return false
	}
	if payload.IsObject() {
		return len(payload.Map()) > 0
	}
	if payload.IsArray() {
		return len(payload.Array()) > 0
	}
	value := strings.TrimSpace(payload.String())
	return value != "" && value != "{}" && value != "[]" && !strings.EqualFold(value, "null")
}

func isTerminalOrErrorStreamJSON(data []byte) bool {
	if !gjson.ValidBytes(data) {
		return false
	}
	eventType := strings.ToLower(strings.TrimSpace(gjson.GetBytes(data, "type").String()))
	switch eventType {
	case "error", "response.failed", "response.completed", "response.done", "response.cancelled", "response.canceled", "response.incomplete", "message_stop":
		return true
	}
	if gjson.GetBytes(data, "error").Exists() {
		return true
	}
	if candidates := gjson.GetBytes(data, "candidates"); candidates.IsArray() {
		for _, candidate := range candidates.Array() {
			if strings.TrimSpace(candidate.Get("finishReason").String()) != "" || strings.TrimSpace(candidate.Get("finish_reason").String()) != "" {
				return true
			}
		}
	}
	return false
}

type firstTokenBufferedResponseWriter struct {
	gin.ResponseWriter
	mu             sync.Mutex
	buffer         bytes.Buffer
	headerSnapshot http.Header
	attempt        *firstTokenAttempt
	status         int
	released       bool
	discarded      bool
}

func newFirstTokenBufferedResponseWriter(writer gin.ResponseWriter, attempts ...*firstTokenAttempt) *firstTokenBufferedResponseWriter {
	var headerSnapshot http.Header
	if writer != nil && writer.Header() != nil {
		headerSnapshot = writer.Header().Clone()
	}
	result := &firstTokenBufferedResponseWriter{
		ResponseWriter: writer,
		headerSnapshot: headerSnapshot,
	}
	if len(attempts) > 0 {
		result.attempt = attempts[0]
	}
	return result
}

func (w *firstTokenBufferedResponseWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.discarded {
		return
	}
	if w.released {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status == 0 {
		w.status = code
	}
}

func (w *firstTokenBufferedResponseWriter) WriteHeaderNow() {
	w.WriteHeader(w.Status())
}

func (w *firstTokenBufferedResponseWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	if w.discarded {
		w.mu.Unlock()
		return len(p), nil
	}
	if w.released {
		writer := w.ResponseWriter
		w.mu.Unlock()
		return writer.Write(p)
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.buffer.Len()+len(p) > maxFirstTokenPreludeBytes {
		attempt := w.attempt
		w.mu.Unlock()
		if attempt != nil {
			attempt.markPreludeOverflow()
		}
		return 0, errFirstTokenPreludeTooLarge
	}
	n, err := w.buffer.Write(p)
	w.mu.Unlock()
	return n, err
}

func (w *firstTokenBufferedResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *firstTokenBufferedResponseWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.released && !w.discarded {
		w.ResponseWriter.Flush()
	}
}

func (w *firstTokenBufferedResponseWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status != 0 {
		return w.status
	}
	return w.ResponseWriter.Status()
}

func (w *firstTokenBufferedResponseWriter) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.released {
		return w.ResponseWriter.Size()
	}
	return w.ResponseWriter.Size()
}

func (w *firstTokenBufferedResponseWriter) Written() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.released && w.ResponseWriter.Written()
}

// EnablePassthrough 让已停止计时的兼容重试继续使用统一 writer 包装，
// 但不提前 Flush，避免尚未产生正文时就提交 200。
func (w *firstTokenBufferedResponseWriter) EnablePassthrough() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.discarded {
		return
	}
	w.released = true
}

func (w *firstTokenBufferedResponseWriter) Release() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.released || w.discarded {
		return
	}
	w.released = true
	if w.status != 0 && !w.ResponseWriter.Written() {
		w.ResponseWriter.WriteHeader(w.status)
	}
	if w.buffer.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.buffer.Bytes())
		w.buffer.Reset()
	}
	w.ResponseWriter.Flush()
}

func (w *firstTokenBufferedResponseWriter) Discard() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.released {
		return
	}
	w.discarded = true
	w.buffer.Reset()
	w.restoreHeaderLocked()
}

// DropBufferedWrites 用于客户端已取消的并发回调：停止缓存后续输出，但不改写底层 Header。
// Header 的恢复只能由请求处理线程在 finish 阶段执行，避免与流转发并发写 map。
func (w *firstTokenBufferedResponseWriter) DropBufferedWrites() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.released {
		return
	}
	w.discarded = true
	w.buffer.Reset()
}

func (w *firstTokenBufferedResponseWriter) restoreHeaderLocked() {
	if w.ResponseWriter == nil || w.Header() == nil {
		return
	}
	header := w.Header()
	for key := range header {
		delete(header, key)
	}
	for key, values := range w.headerSnapshot {
		header[key] = append([]string(nil), values...)
	}
}
