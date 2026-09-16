package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type anthropicFrameKind uint8

const (
	anthropicFramePrelude anthropicFrameKind = iota
	anthropicFrameContent
	anthropicFrameTerminal
	anthropicFrameError
)

// inspectAnthropicFrame 仅在完整空行边界调用。非白名单事件保守进入不可重放状态。
func inspectAnthropicFrame(frame []byte) (anthropicFrameKind, string, string, error) {
	var name string
	var data []string
	unknownField := false
	for _, line := range strings.Split(string(frame), "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch {
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case line == "" || strings.HasPrefix(line, ":"):
		default:
			unknownField = true
		}
	}
	payload := strings.TrimSpace(strings.Join(data, "\n"))
	if unknownField {
		return anthropicFrameContent, "unknown_sse_field", "", nil
	}
	if payload == "" {
		if name == "" {
			return anthropicFramePrelude, "comment", "", nil
		}
		return 0, name, "", &AnthropicStreamFailure{Kind: "invalid_event"}
	}
	if payload == "[DONE]" {
		if name != "" && name != "message_stop" {
			return 0, name, "", &AnthropicStreamFailure{Kind: "event_type_mismatch"}
		}
		return anthropicFrameTerminal, "[DONE]", payload, nil
	}
	if !gjson.Valid(payload) || !gjson.Parse(payload).IsObject() {
		return 0, name, "", &AnthropicStreamFailure{Kind: "invalid_json"}
	}
	root := gjson.Parse(payload)
	typ := root.Get("type").String()
	if name != "" && typ != "" && name != typ {
		return 0, name, "", &AnthropicStreamFailure{Kind: "event_type_mismatch"}
	}
	if typ == "" {
		typ = name
	}
	switch typ {
	case "message_stop":
		return anthropicFrameTerminal, typ, payload, nil
	case "error":
		return anthropicFrameError, typ, payload, nil
	case "ping":
		return anthropicFramePrelude, typ, payload, nil
	case "message_start":
		content := root.Get("message.content")
		if content.IsArray() && len(content.Array()) == 0 {
			return anthropicFramePrelude, typ, payload, nil
		}
	case "content_block_start":
		block := root.Get("content_block")
		t := block.Get("type").String()
		if (t == "text" || t == "thinking") && block.Get("text").String() == "" && block.Get("thinking").String() == "" && block.Get("signature").String() == "" {
			return anthropicFramePrelude, typ, payload, nil
		}
	}
	return anthropicFrameContent, boundedAnthropicEventType(typ), payload, nil
}

func restoreAnthropicHeader(header, snapshot http.Header) {
	for key := range header {
		delete(header, key)
	}
	for key, values := range snapshot {
		header[key] = append([]string(nil), values...)
	}
}

// handleGuardedAnthropicStream 单层缓存前导帧，直接读取上游，保持每一行的真实空闲计时。
func (s *GatewayService) handleGuardedAnthropicStream(ctx context.Context, resp *http.Response, c *gin.Context, account *Account, start time.Time, model string, retry *AnthropicStreamRetryState, first *firstTokenAttempt) (result *streamingResult, retErr error) {
	responseStarted := time.Now()
	retry.startResponse()
	headerSnapshot := c.Writer.Header().Clone()
	writeAnthropicPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	if id := resp.Header.Get("x-request-id"); id != "" {
		c.Header("x-request-id", id)
	}
	if s.rateLimitService != nil {
		s.rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)
	}
	observer := upstreamResponseModelObserverFromContext(c)
	if observer == nil {
		observer = beginUpstreamResponseModelObservation(c)
	}
	usage := &ClaudeUsage{}
	result = &streamingResult{usage: usage}
	d := &AnthropicStreamDiagnostic{}
	var prelude, frame bytes.Buffer
	var lastRead atomic.Int64
	var readBytes atomic.Int64
	lastRead.Store(time.Now().UnixNano())
	defer func() {
		if !retry.Committed() {
			restoreAnthropicHeader(c.Writer.Header(), headerSnapshot)
		}
		d.PendingFrameBytes = frame.Len()
		d.UpstreamBytes = readBytes.Load()
		d.LastReadAgeMs = nonnegativeStreamMillis(time.Since(time.Unix(0, lastRead.Load())).Milliseconds())
		d.Usage = *usage
		if c.Writer.Written() {
			d.WireStatus = c.Writer.Status()
		}
		if retErr != nil {
			var failure *AnthropicStreamFailure
			if errors.As(retErr, &failure) {
				d.FailureKind = failure.Kind
				d.LogicalStatus = failure.ClientStatus()
			} else {
				d.FailureKind = "stream_read_error"
				d.LogicalStatus = http.StatusBadGateway
				if errors.Is(retErr, errFirstTokenAttemptTimedOut) {
					d.FailureKind = "first_token_timeout"
					d.LogicalStatus = http.StatusGatewayTimeout
				}
			}
			retry.Diagnostic(d)
			appendOpsUpstreamError(c, OpsUpstreamErrorEvent{Platform: account.Platform, AccountID: account.ID, AccountName: account.Name, ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account), Passthrough: true, UpstreamStatusCode: resp.StatusCode, UpstreamRequestID: resp.Header.Get("x-request-id"), Kind: "stream_failure", Stage: "stream", Reason: d.FailureKind, Message: "Claude 透传流未正常完成", StreamDiagnostic: d})
		}
	}()
	type readEvent struct {
		line string
		err  error
	}
	events := make(chan readEvent, 16)
	done := make(chan struct{})
	scanner := bufio.NewScanner(resp.Body)
	maxLine := defaultMaxLineSize
	if s.cfg != nil && s.cfg.Gateway.MaxLineSize > 0 {
		maxLine = s.cfg.Gateway.MaxLineSize
	}
	scanBuf := getSSEScannerBuf64K()
	scanner.Buffer(scanBuf[:0], maxLine)
	go func() {
		defer close(events)
		defer putSSEScannerBuf64K(scanBuf)
		send := func(e readEvent) bool {
			select {
			case events <- e:
				return true
			case <-done:
				return false
			}
		}
		for scanner.Scan() {
			line := scanner.Text()
			lastRead.Store(time.Now().UnixNano())
			readBytes.Add(int64(len(line) + 1))
			if !send(readEvent{line: line}) {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			send(readEvent{err: err})
		}
	}()
	// 自定义传输和测试响应体也必须在取消时可退出，不能只依赖 net/http 的实现。
	stopBudget := context.AfterFunc(retry.WaitContext(), func() {
		if !retry.Committed() {
			_ = resp.Body.Close()
		}
	})
	stopFirst := func() bool { return false }
	if first != nil {
		stopFirst = context.AfterFunc(first.upstreamContext(ctx), func() {
			if !retry.Committed() || first.currentState() == firstTokenAttemptTimedOut {
				_ = resp.Body.Close()
			}
		})
	}
	defer func() { stopBudget(); stopFirst(); close(done); _ = resp.Body.Close() }()
	interval := time.Duration(0)
	keepalive := time.Duration(0)
	if s.cfg != nil {
		interval = time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout) * time.Second
		keepalive = time.Duration(s.cfg.Gateway.StreamKeepaliveInterval) * time.Second
	}
	var idleTimer, pingTimer *time.Timer
	var idleCh, pingCh <-chan time.Time
	if interval > 0 {
		idleTimer = time.NewTimer(interval)
		idleCh = idleTimer.C
		defer idleTimer.Stop()
	}
	if keepalive > 0 {
		pingTimer = time.NewTimer(keepalive)
		pingCh = pingTimer.C
		defer pingTimer.Stop()
	}
	contentTimeout := time.Duration(retry.settings.AnthropicStreamSafeRetryFirstContentTimeoutSeconds) * time.Second
	var contentTimer *time.Timer
	var contentCh <-chan time.Time
	var contentDeadline time.Time
	if contentTimeout > 0 {
		contentDeadline = responseStarted.Add(contentTimeout)
		contentTimer = time.NewTimer(time.Until(contentDeadline))
		contentCh = contentTimer.C
		defer contentTimer.Stop()
	}
	cancelCh := retry.WaitContext().Done()
	writeFrame := func(b []byte) {
		if result.clientDisconnect || len(b) == 0 {
			return
		}
		restored := reverseToolNamesIfPresent(c, b)
		if _, err := c.Writer.Write(restored); err != nil {
			result.clientDisconnect = true
			return
		}
		c.Writer.Flush()
	}
	failure := func(kind string, cause error, replayable bool) error {
		if err := retry.Check(); err != nil && (!retry.Committed() || retry.ClientContext().Err() != nil) {
			return err
		}
		if isOpenAIRequestSentPluginError(cause) {
			replayable = false
		}
		return &AnthropicStreamFailure{Kind: kind, Cause: cause, Replayable: replayable && !retry.Committed() && !result.clientDisconnect}
	}
	for {
		if !contentDeadline.IsZero() && !time.Now().Before(contentDeadline) {
			return result, failure("first_content_timeout", nil, true)
		}
		select {
		case <-contentCh:
			return result, failure("first_content_timeout", nil, true)
		case <-cancelCh:
			if !retry.Committed() {
				return result, retry.Check()
			}
			result.clientDisconnect = true
			cancelCh = nil
		case <-idleCh:
			if remaining := interval - time.Since(time.Unix(0, lastRead.Load())); remaining > 0 {
				idleTimer.Reset(remaining)
				continue
			}
			if !result.clientDisconnect && s.rateLimitService != nil {
				s.rateLimitService.HandleStreamTimeout(ctx, account, model)
			}
			return result, failure("idle_timeout", nil, true)
		case <-pingCh:
			// 前导阶段不交付尝试相关心跳；内容交付后继续既有下游保活，且不续上游计时。
			if retry.Committed() && frame.Len() == 0 {
				writeFrame([]byte("event: ping\ndata: {\"type\":\"ping\"}\n\n"))
			}
			pingTimer.Reset(keepalive)
		case ev, ok := <-events:
			if !ok || ev.err != nil {
				if first != nil && first.currentState() == firstTokenAttemptTimedOut {
					return result, errFirstTokenAttemptTimedOut
				}
				if err := retry.Check(); err != nil && !retry.Committed() {
					return result, err
				}
				if frame.Len() != 0 {
					return result, failure("truncated_event", io.ErrUnexpectedEOF, false)
				}
				if ev.err != nil && !errors.Is(ev.err, io.EOF) && !errors.Is(ev.err, io.ErrUnexpectedEOF) {
					return result, failure("stream_read_error", ev.err, false)
				}
				kind := "missing_terminal"
				if readBytes.Load() == 0 {
					kind = "empty_stream"
				}
				return result, failure(kind, ev.err, true)
			}
			if !retry.Committed() && prelude.Len()+frame.Len()+len(ev.line)+1 > maxAnthropicStreamPreludeBytes {
				return result, failure("prelude_overflow", nil, false)
			}
			if frame.Len()+len(ev.line)+1 > maxLine {
				return result, failure("event_too_large", nil, false)
			}
			if strings.TrimSpace(ev.line) == "event: message_stop" {
				d.TerminalCandidateSeen = true
			}
			if data, ok := extractAnthropicSSEDataLine(ev.line); ok && anthropicStreamEventIsTerminal("", data) {
				d.TerminalCandidateSeen = true
			}
			_, _ = frame.WriteString(ev.line)
			_ = frame.WriteByte('\n')
			if ev.line != "" {
				continue
			}
			kind, typ, payload, err := inspectAnthropicFrame(frame.Bytes())
			d.LastEventType = boundedAnthropicEventType(typ)
			if err != nil {
				return result, err
			}
			if payload != "" {
				observer.ObserveAnthropic([]byte(payload))
				parseSSEUsagePassthrough(payload, usage)
				if result.firstTokenMs == nil && payload != "[DONE]" {
					ms := int(time.Since(start).Milliseconds())
					result.firstTokenMs = &ms
				}
			}
			if typ == "ping" || typ == "comment" {
				d.Heartbeats++
			}
			if kind == anthropicFrameError {
				return result, failure("upstream_error_event", nil, false)
			}
			if contentTimeout > 0 && !retry.Committed() && kind == anthropicFrameContent {
				switch typ {
				case "content_block_delta":
					delta := gjson.Get(payload, "delta")
					switch delta.Get("type").String() {
					case "text_delta":
						if text := delta.Get("text"); text.Type == gjson.String && strings.TrimSpace(text.String()) == "" {
							kind = anthropicFramePrelude
						}
					case "thinking_delta":
						if thinking := delta.Get("thinking"); thinking.Type == gjson.String && strings.TrimSpace(thinking.String()) == "" {
							kind = anthropicFramePrelude
						}
					}
				case "content_block_start":
					block := gjson.Get(payload, "content_block")
					blockType := block.Get("type").String()
					if (blockType == "text" || blockType == "thinking") && strings.TrimSpace(block.Get("text").String()) == "" && strings.TrimSpace(block.Get("thinking").String()) == "" && block.Get("signature").String() == "" {
						kind = anthropicFramePrelude
					}
				case "content_block_stop":
					kind = anthropicFramePrelude
				}
			}
			if !retry.Committed() && kind == anthropicFramePrelude {
				_, _ = prelude.Write(frame.Bytes())
				d.PreludeBytes = prelude.Len()
				frame.Reset()
				continue
			}
			// 安全重放资格和首 Token 语义各自独立。指定分组的纯空白不能解除首 Token 守卫。
			strict := contentTimeout > 0 || (first != nil && first.strictOutput)
			meaningful := isMeaningfulFirstTokenJSON([]byte(payload), strict)
			if typ == "content_block_start" {
				block := gjson.Get(payload, "content_block")
				meaningful = meaningful || firstTokenTextMeaningful(block.Get("text").String(), strict) || firstTokenTextMeaningful(block.Get("thinking").String(), strict)
			}
			if typ == "message_start" {
				for _, block := range gjson.Get(payload, "message.content").Array() {
					meaningful = meaningful || firstTokenTextMeaningful(block.Get("text").String(), strict) || firstTokenTextMeaningful(block.Get("thinking").String(), strict)
					if block.Get("type").String() == "tool_use" {
						meaningful = meaningful || firstTokenFunctionCallMeaningful(block)
					}
				}
			}
			if first != nil && first.currentState() == firstTokenAttemptTimedOut {
				return result, errFirstTokenAttemptTimedOut
			}
			if contentTimer != nil && (meaningful || kind == anthropicFrameTerminal) {
				contentTimer.Stop()
				contentCh = nil
				contentDeadline = time.Time{}
			}
			if !retry.Committed() {
				if err := retry.commit(); err != nil {
					return result, err
				}
				writeFrame(prelude.Bytes())
				prelude.Reset()
			}
			if meaningful {
				if d.FirstSemanticMs == nil {
					ms := time.Since(start).Milliseconds()
					d.FirstSemanticMs = &ms
				}
				first.markReceived()
			}
			if kind == anthropicFrameTerminal {
				first.markDecidedWithoutToken()
			}
			writeFrame(frame.Bytes())
			frame.Reset()
			if kind == anthropicFrameTerminal {
				d.TerminalComplete = true
				return result, nil
			}
		}
	}
}

// 未知事件统一归类，防止上游把任意正文放入 type 字段后进入诊断日志。
func boundedAnthropicEventType(typ string) string {
	switch typ {
	case "message_start", "message_delta", "message_stop", "content_block_start", "content_block_delta", "content_block_stop", "ping", "error", "[DONE]", "comment", "unknown_sse_field":
		return typ
	default:
		return "unknown_event"
	}
}
