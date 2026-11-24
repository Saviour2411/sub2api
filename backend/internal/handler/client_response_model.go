package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const responseModelModeKey = "client_response_model_mode"

// ResponseModelMiddleware 在协议转换和上游审计之后统一处理实际交付的型号。
func ResponseModelMiddleware(settings *service.SettingService, cfg *config.Config) gin.HandlerFunc {
	jsonLimit, sseLimit := service.ResponseModelBufferLimits(cfg)
	// 在接收流量前读取一次，后续请求只取进程内快照。
	if settings != nil {
		settings.GetGatewayRuntime(context.Background())
	}
	return func(c *gin.Context) {
		mode := settings.ClientResponseModelMode()
		if mode == "" {
			c.Next()
			return
		}
		fields := responseModelFields(c)
		if len(fields) == 0 {
			c.Next()
			return
		}
		c.Set(responseModelModeKey, mode)
		writer := newResponseModelWriter(c, mode, fields)
		writer.jsonLimit, writer.sseLimit = jsonLimit, sseLimit
		original := c.Writer
		c.Writer = writer
		defer func() { c.Writer = original }()
		c.Next()
		writer.finish()
	}
}

func responseModelFields(c *gin.Context) []string {
	endpoint := GetInboundEndpoint(c)
	// GET 模型目录、显式缓存、用量等资源的型号是资源标识。
	if c.Request.Method == http.MethodGet {
		switch endpoint {
		case EndpointImageTasks:
			return []string{"result.model"}
		case EndpointVideos, EndpointVideosGenerations, EndpointVideosEdits, EndpointVideosExtensions, EndpointSeedanceTasks:
			if !strings.HasSuffix(c.FullPath(), "/content") {
				return []string{"model"}
			}
		}
		return nil
	}
	if c.Request.Method != http.MethodPost {
		return nil
	}
	switch endpoint {
	case EndpointMessages:
		return []string{"model", "message.model"}
	case EndpointResponses, EndpointResponsesCompact:
		return []string{"model", "response.model"}
	case EndpointGeminiModels:
		return []string{"modelVersion", "response.modelVersion", "response.response.modelVersion"}
	case EndpointGeminiCachedContents, "/v1/images/batches", "/images/batches":
		return nil
	default:
		return []string{"model"}
	}
}

type responseModelWriter struct {
	gin.ResponseWriter
	mu                  sync.Mutex // 提前保活和正式回包可能来自不同的 goroutine。
	publicModel         string
	c                   *gin.Context
	mode                string
	fields              []string
	model               service.ClientResponseModel
	ready               bool
	partial             []byte
	jsonPartial         []byte
	jsonContainsSSE     bool
	sseLimit, jsonLimit int64
	err                 error
}

func newResponseModelWriter(c *gin.Context, mode string, fields []string) *responseModelWriter {
	return &responseModelWriter{ResponseWriter: c.Writer, c: c, mode: mode, fields: fields, publicModel: clientRequestedModel(c, "")}
}

func (w *responseModelWriter) responseModel() service.ClientResponseModel {
	if !w.ready {
		// handler 已完成解析。优先使用合成路由保存的原型号，再取渠道映射前的请求型号。
		requested := w.publicModel
		if requested == "" {
			requested = w.c.GetString(opsModelKey)
		}
		w.model = service.NewClientResponseModel(w.mode, requested)
		w.ready = requested != "" || w.mode == service.ResponseModelModeEmpty
	}
	return w.model
}

func (w *responseModelWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// 已接收的部分帧也算本次尝试写出的字节，沿用网关的换号和保活判定。
func (w *responseModelWriter) Size() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	size := w.ResponseWriter.Size()
	if pending := len(w.partial) + len(w.jsonPartial); pending > 0 {
		return max(0, size) + pending
	}
	return size
}

func (w *responseModelWriter) Written() bool { return w.Size() >= 0 }

func (w *responseModelWriter) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func (w *responseModelWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return 0, w.err
	}
	if len(p) == 0 {
		return w.ResponseWriter.Write(p)
	}
	header := w.Header()
	contentType := header.Get("Content-Type")
	encoding := header.Get("Content-Encoding")
	if w.Status() >= 300 || (encoding != "" && !strings.EqualFold(encoding, "identity")) {
		return w.ResponseWriter.Write(p)
	}
	// 部分兼容上游以 text/plain 返回已由网关识别的 JSON 或 SSE。
	if contentType == "" || strings.HasPrefix(contentType, "text/plain") {
		contentType = "application/json"
		if w.c.GetBool(opsStreamKey) {
			contentType = "text/event-stream"
		}
	}
	switch {
	case strings.HasPrefix(contentType, "text/event-stream"):
		if !w.ResponseWriter.Written() {
			header.Del("Content-Length")
		}
		w.writeSSE(p)
	case strings.HasPrefix(contentType, "application/json"), strings.Contains(contentType, "+json"):
		w.writeJSON(p)
	default:
		return w.ResponseWriter.Write(p)
	}
	if w.err != nil {
		return 0, w.err
	}
	return len(p), nil
}

// 完整 JSON 在本次写出；分段 Write 的 JSON 到齐再替换。纯空白保活随到随发。
func (w *responseModelWriter) writeJSON(p []byte) {
	if w.jsonContainsSSE {
		w.writeSSE(p)
		return
	}
	owned := len(w.jsonPartial) > 0
	if owned {
		p = append(w.jsonPartial, p...)
		w.jsonPartial = nil
	}
	trimmed := bytes.TrimSpace(p)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		// 上游 SSE 聚合无法生成终态 JSON 时，兼容接口可能沿用原 Content-Type。
		if bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte(":")) {
			w.jsonContainsSSE = true
			w.Header().Del("Content-Length")
			w.writeSSE(p)
			return
		}
		w.writeWire(p)
		return
	}
	last := trimmed[len(trimmed)-1]
	if (last != '}' && last != ']') || !gjson.ValidBytes(trimmed) {
		if w.jsonLimit > 0 && int64(len(p)) > w.jsonLimit {
			w.err = service.ErrUpstreamResponseBodyTooLarge
			return
		}
		if owned {
			w.jsonPartial = p
		} else {
			w.jsonPartial = bytes.Clone(p)
		}
		return
	}
	body, changed := w.responseModel().Rewrite(p, w.fields...)
	if changed {
		w.Header().Del("Content-Length")
	}
	w.writeWire(body)
}

func (w *responseModelWriter) writeWire(p []byte) {
	if len(p) == 0 || w.err != nil {
		return
	}
	n, err := w.ResponseWriter.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	w.err = err
}

// 一个 Write 可以包含多帧，也可能在型号字符串中间结束。完整帧立即发送，
// 只保留当前跨 Write 的一帧。识别 LF 和 CRLF，并保留原始行尾。
func (w *responseModelWriter) writeSSE(p []byte) {
	offset := 0
	owned := len(w.partial) != 0
	if owned {
		offset = max(0, len(w.partial)-3)
		p = append(w.partial, p...)
		w.partial = nil
	}
	for len(p) > 0 && w.err == nil {
		end := sseModelFrameEnd(p, offset)
		if end == 0 {
			if w.sseLimit > 0 && int64(len(p)) > w.sseLimit {
				w.err = errors.New("网关流式响应超出已有事件大小上限")
				return
			}
			if owned {
				w.partial = p
			} else {
				w.partial = append(w.partial[:0], p...)
			}
			return
		}
		w.writeWire(w.rewriteSSEFrame(p[:end]))
		p, offset = p[end:], 0
	}
}

func sseModelFrameEnd(p []byte, start int) int {
	// 单独的空行也可能是上一行已 Flush 之后送来的事件边界。
	if p[0] == '\n' {
		return 1
	}
	if bytes.HasPrefix(p, []byte("\r\n")) {
		return 2
	}
	for start < len(p) {
		i := bytes.IndexByte(p[start:], '\n')
		if i < 0 {
			break
		}
		start += i + 1
		if start < len(p) && p[start] == '\n' {
			return start + 1
		}
		if bytes.HasPrefix(p[start:], []byte("\r\n")) {
			return start + 2
		}
	}
	return 0
}

func (w *responseModelWriter) rewriteSSEFrame(frame []byte) []byte {
	// 绝大多数文本增量和心跳不含型号，直接交给原 writer。
	if !bytes.Contains(frame, []byte(`"model`)) && !bytes.Contains(frame, []byte(`\u`)) {
		return frame
	}
	// 常见的单行 data 直接在整帧替换型号，避免再复制一次长工具参数。
	start := 0
	if !bytes.HasPrefix(frame, []byte("data:")) {
		pos := bytes.Index(frame, []byte("\ndata:"))
		if pos < 0 {
			body, _ := w.responseModel().Rewrite(frame, w.fields...)
			return body
		}
		start = pos + 1
	}
	start += len("data:")
	stop := len(frame)
	if i := bytes.IndexByte(frame[start:], '\n'); i >= 0 {
		stop = start + i
	}
	if !bytes.Contains(frame[stop:], []byte("\ndata:")) {
		if body := bytes.TrimSpace(frame[start:stop]); len(body) > 0 && body[0] == '[' {
			if rewritten, changed := w.responseModel().Rewrite(body, w.fields...); changed {
				return bytes.Replace(frame, body, rewritten, 1)
			}
			return frame
		}
		out, _ := w.responseModel().RewriteObjectInFrame(frame, start, stop, w.fields...)
		return out
	}
	_, data := parseOpsSSEFrameEnvelope(frame)
	rewritten, changed := w.responseModel().Rewrite(data, w.fields...)
	if !changed {
		return frame
	}
	// SSE 多行 data 之间有隐式换行。保留 event、id、retry、注释和每个 data 前缀。
	out := make([]byte, 0, len(frame)+len(rewritten)-len(data))
	for line := range bytes.SplitAfterSeq(frame, []byte{'\n'}) {
		value, isData := bytes.CutPrefix(line, []byte("data:"))
		if !isData {
			out = append(out, line...)
			continue
		}
		body := bytes.TrimSpace(value)
		offset := bytes.Index(line[len("data:"):], body) + len("data:")
		first, rest, _ := bytes.Cut(rewritten, []byte{'\n'})
		out = append(out, line[:offset]...)
		out = append(out, first...)
		out = append(out, line[offset+len(body):]...)
		rewritten = rest
	}
	return out
}

// 有些转发器会在 data 行后立即 Flush，再单独写空行；完整 JSON 当次即交付。
func (w *responseModelWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.partial) > 0 {
		_, data := parseOpsSSEFrameEnvelope(w.partial)
		if (len(data) == 0 && bytes.HasSuffix(w.partial, []byte{'\n'})) || bytes.Equal(data, []byte("[DONE]")) || gjson.ValidBytes(data) {
			w.writeWire(w.rewriteSSEFrame(w.partial))
			w.partial = nil
		}
	}
	w.removeJSONContentLength()
	w.ResponseWriter.Flush()
}

func (w *responseModelWriter) WriteHeaderNow() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.removeJSONContentLength()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *responseModelWriter) removeJSONContentLength() {
	contentType := w.Header().Get("Content-Type")
	if !w.ResponseWriter.Written() && w.Status() < 300 && (strings.HasPrefix(contentType, "text/event-stream") || strings.Contains(contentType, "json")) {
		w.Header().Del("Content-Length")
	}
}

func (w *responseModelWriter) finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.jsonPartial) > 0 {
		w.writeWire(w.jsonPartial)
		w.jsonPartial = nil
	}
	if len(w.partial) > 0 {
		w.writeWire(w.rewriteSSEFrame(w.partial))
		w.partial = nil
	}
}
