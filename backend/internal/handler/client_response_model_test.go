//go:build unit

package handler

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func modelTestMiddleware(mode string) gin.HandlerFunc {
	settings := service.NewSettingService(&safeHandlerSettingsRepo{values: map[string]string{
		service.SettingKeyGatewayResponseModelRewriteEnabled: fmt.Sprint(mode != ""),
		service.SettingKeyGatewayResponseModelRewriteMode:    mode,
	}}, nil)
	return ResponseModelMiddleware(settings, nil)
}

func TestClientResponseModelHTTP(t *testing.T) {
	const request = "claude-sonnet-5-5"
	const upstream = "claude-sonnet-5.5"
	for _, tc := range []struct{ path, input, field string }{
		{"/v1/messages", `{"model":"claude-sonnet-5.5","content":[{"text":"claude-sonnet-5.5","input":{"model":"tool"}}]}`, "model"},
		{"/chat/completions", `{"model":"claude-sonnet-5.5","choices":[]}`, "model"},
		{"/backend-api/codex/responses/compact", `{"model":"claude-sonnet-5.5","output":[]}`, "model"},
		{"/v1beta/models/test:generateContent", `{"modelVersion":"claude-sonnet-5.5","candidates":[]}`, "modelVersion"},
		{"/v1/systemone", `{"model":"claude-sonnet-5.5"}`, "model"},
		{"/v1/embeddings", `{"model":"claude-sonnet-5.5","data":[]}`, "model"},
		{"/images/generations", `{"model":"claude-sonnet-5.5","data":[]}`, "model"},
	} {
		for _, mode := range []string{"", service.ResponseModelModeRequest, service.ResponseModelModeEmpty} {
			t.Run(tc.path+"/"+mode, func(t *testing.T) {
				router := gin.New()
				router.Use(modelTestMiddleware(mode))
				router.NoRoute(func(c *gin.Context) {
					setOpsRequestContext(c, request, false)
					c.Header("Content-Length", fmt.Sprint(len(tc.input)))
					c.Data(200, "application/json", []byte(tc.input))
				})
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest("POST", tc.path, nil))
				want := upstream
				switch mode {
				case service.ResponseModelModeRequest:
					want = request
				case service.ResponseModelModeEmpty:
					want = ""
				}
				require.Equal(t, strings.Replace(tc.input, upstream, want, 1), rec.Body.String())
				require.Equal(t, gjson.String, gjson.GetBytes(rec.Body.Bytes(), tc.field).Type)
			})
		}
	}
}

func TestClientResponseModelFragmentedStream(t *testing.T) {
	const start = "id: 123\r\nevent: message_start\r\ndata:  {\"type\":\"message_start\",\r\ndata:  \"message\":{\"model\":\"claude-sonnet-5.5\",\"usage\":{\"input_tokens\":45}}}\r\n\r\n"
	const next = ": ping\n\ndata: {\"type\":\"content_block_start\",\"content_block\":{\"input\":{\"model\":\"keep\"}}}\n\ndata: [DONE]\n\n"
	for _, size := range []int{1, 5, 37, len(start + next)} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			router := gin.New()
			router.Use(modelTestMiddleware(service.ResponseModelModeEmpty))
			router.POST("/antigravity/v1/messages", func(c *gin.Context) {
				setOpsRequestContext(c, "claude-sonnet-5-5", true)
				c.Header("Content-Type", "text/event-stream")
				for data := start + next; len(data) > 0; {
					n := min(size, len(data))
					written, err := c.Writer.WriteString(data[:n])
					require.NoError(t, err)
					require.Equal(t, n, written)
					data = data[n:]
				}
				c.Writer.Flush()
			})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest("POST", "/antigravity/v1/messages", nil))
			require.Equal(t, strings.Replace(start+next, "claude-sonnet-5.5", "", 1), rec.Body.String())
		})
	}
}

func TestClientResponseModelFirstFrameDoesNotWaitForNext(t *testing.T) {
	const first = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-5.5\"}}\n\n"
	for _, mode := range []string{service.ResponseModelModeRequest, service.ResponseModelModeEmpty} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{}, 2)
			router := gin.New()
			router.Use(modelTestMiddleware(mode))
			router.POST("/v1/messages", func(c *gin.Context) {
				setOpsRequestContext(c, "claude-sonnet-5-5", true)
				c.Header("Content-Type", "text/event-stream")
				_, _ = io.WriteString(c.Writer, first)
				c.Writer.Flush()
				<-received
				_, _ = io.WriteString(c.Writer, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"首字\"}}\n\n")
				c.Writer.Flush()
				<-received
				_, _ = io.WriteString(c.Writer, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			})
			server := httptest.NewServer(router)
			defer server.Close()
			defer close(received)
			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Post(server.URL+"/v1/messages", "application/json", nil)
			require.NoError(t, err)
			defer resp.Body.Close()
			reader := bufio.NewReader(resp.Body)
			event, err := reader.ReadString('\n')
			require.NoError(t, err)
			require.Equal(t, "event: message_start\n", event)
			line, err := reader.ReadString('\n')
			require.NoError(t, err)
			model := gjson.Get(strings.TrimPrefix(line, "data: "), "message.model")
			require.Equal(t, gjson.String, model.Type)
			want := "claude-sonnet-5-5"
			if mode == service.ResponseModelModeEmpty {
				want = ""
			}
			require.Equal(t, want, model.Str)
			received <- struct{}{}
			separator, err := reader.ReadString('\n')
			require.NoError(t, err)
			require.Equal(t, "\n", separator)
			event, err = reader.ReadString('\n')
			require.NoError(t, err)
			require.Equal(t, "event: content_block_delta\n", event)
			line, err = reader.ReadString('\n')
			require.NoError(t, err)
			require.Equal(t, "首字", gjson.Get(strings.TrimPrefix(line, "data: "), "delta.text").Str)
			received <- struct{}{}
		})
	}
}

func TestClientResponseModelAnthropicUpstreamAudit(t *testing.T) {
	const sent = "claude-sonnet-5-5"
	const observed = "claude-sonnet-5.5"
	prelude := strings.Replace(safeHandlerPrelude, `"content":[]`, `"model":"`+observed+`","content":[]`, 1)
	for _, transport := range []string{"SSE", "guarded_SSE", "JSON"} {
		for _, mode := range []string{service.ResponseModelModeRequest, service.ResponseModelModeEmpty} {
			t.Run(transport+"/"+mode, func(t *testing.T) {
				body := prelude + safeHandlerOutput + safeHandlerStop
				if transport == "JSON" {
					body = `{"type":"message","model":"claude-sonnet-5.5","content":[],"usage":{"input_tokens":7,"output_tokens":3}}`
				}
				up := &safeHandlerUpstream{clientModelMode: mode, requestedModel: sent, plainStream: transport != "guarded_SSE", nonStreaming: transport == "JSON", bodies: []string{body}}
				rec, usage := runSafeHandlerFixture(t, up, true)
				require.Equal(t, 200, rec.Code)
				require.Len(t, up.requests, 1)
				require.Equal(t, sent, gjson.GetBytes(up.requests[0], "model").Str)
				want := sent
				if mode == service.ResponseModelModeEmpty {
					want = ""
				}
				require.Contains(t, rec.Body.String(), `"model":"`+want+`"`)
				require.NotContains(t, rec.Body.String(), observed)
				require.Len(t, usage.logs, 1)
				log := usage.logs[0]
				require.NotNil(t, log.UpstreamResponseModel)
				require.Equal(t, observed, *log.UpstreamResponseModel)
				require.Equal(t, sent, log.RequestedModel)
				require.NotNil(t, log.UpstreamModelMismatch)
				require.True(t, *log.UpstreamModelMismatch)
				require.Equal(t, 3, log.OutputTokens)
			})
		}
	}
}

func BenchmarkClientResponseModelHTTP(b *testing.B) {
	for _, tc := range []struct{ name, path, frame string }{
		{"Claude首帧", "/v1/messages", "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"model\":\"claude-sonnet-5.5\",\"content\":[],\"usage\":{\"input_tokens\":45}}}\n\n"},
		{"Chat短分块", "/v1/chat/completions", "data: {\"model\":\"claude-sonnet-5.5\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"},
		{"Gemini分块", "/v1beta/models/a:streamGenerateContent", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hi\"}]}}],\"modelVersion\":\"claude-sonnet-5.5\"}\n\n"},
		{"普通文本增量", "/v1/messages", "data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hi\"}}\n\n"},
		{"Chat工具分块4KiB", "/v1/chat/completions", "data: {\"model\":\"claude-sonnet-5.5\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"" + strings.Repeat("x", 4096) + "\"}}]}}]}\n\n"},
	} {
		for _, mode := range []string{"off", service.ResponseModelModeRequest, service.ResponseModelModeEmpty} {
			b.Run(tc.name+"/"+mode, func(b *testing.B) {
				rec := httptest.NewRecorder()
				rec.Body = new(bytes.Buffer)
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest("POST", tc.path, nil)
				c.Header("Content-Type", "text/event-stream")
				setOpsRequestContext(c, "claude-sonnet-5-5", true)
				w := c.Writer
				if mode != "off" {
					w = newResponseModelWriter(c, mode, responseModelFields(c))
				}
				frame := []byte(tc.frame)
				b.ReportAllocs()
				for b.Loop() {
					rec.Body.Reset()
					_, _ = w.Write(frame)
				}
			})
		}
	}
}

func TestClientResponseModelSplitWritesAndCompositeAlias(t *testing.T) {
	for _, ct := range []string{"application/json", "text/event-stream"} {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/responses", nil)
		c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{
			Matched: true, PublicModel: "public", UpstreamModel: "mapped",
		}))
		setOpsRequestContext(c, "mapped", false)
		c.Header("Content-Type", ct)
		writer := newResponseModelWriter(c, service.ResponseModelModeRequest, responseModelFields(c))
		packet := `{"response":{"model":"build","metadata":{"model":"private"}}}`
		if ct == "text/event-stream" {
			packet = "data: " + packet + "\n\n"
		}
		for i := 0; i < len(packet); i++ {
			_, err := writer.Write([]byte{packet[i]})
			require.NoError(t, err)
			writer.Flush()
		}
		writer.finish()
		require.Equal(t, strings.Replace(packet, "build", "public", 1), rec.Body.String(), ct)
	}
}

func TestClientResponseModelPreservesNonModelOutput(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
	c.Header("Content-Type", "application/json")
	writer := newResponseModelWriter(c, service.ResponseModelModeEmpty, responseModelFields(c))
	for _, data := range []string{`{"content":[{"input":{"model":"keep"}}]}`, `{"tool":{"model":"keep"}}`} {
		_, err := writer.WriteString(data)
		require.NoError(t, err)
		require.Equal(t, data, rec.Body.String())
		rec.Body.Reset()
	}
	c.Header("Content-Type", "audio/mpeg")
	_, err := writer.WriteString(`{"model":"binary"}`)
	require.NoError(t, err)
	require.Equal(t, `{"model":"binary"}`, rec.Body.String())
}

func TestClientResponseModelSSERetainsUpstreamContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	setOpsRequestContext(c, "requested", false)
	c.Header("Content-Type", "application/json")
	writer := newResponseModelWriter(c, service.ResponseModelModeRequest, responseModelFields(c))
	frame := "event: response.created\ndata: {\"response\":{\"model\":\"upstream\"}}\n\n"
	_, err := writer.WriteString(frame)
	require.NoError(t, err)
	require.Equal(t, strings.Replace(frame, "upstream", "requested", 1), rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}
