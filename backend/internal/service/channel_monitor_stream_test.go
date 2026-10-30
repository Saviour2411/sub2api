//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMonitorStream_LargeResponsesEvents(t *testing.T) {
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		answer := answerFromOpenAIRequest(body)
		w.Header().Set("Content-Type", "text/event-stream")
		// 线上上游在答案之前发送多个约 95 KiB 的元数据事件。
		for _, event := range []string{"response.created", "response.in_progress"} {
			_, _ = fmt.Fprintf(w, "event: %s\ndata: {\"type\":%q,\"response\":{\"instructions\":%q}}\n\n", event, event, strings.Repeat("x", 96*1024))
		}
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", answer)
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}))
	t.Cleanup(srv.Close)
	result := runCheckForModel(context.Background(), MonitorProviderOpenAI, srv.URL, "test-key", "test-model", &CheckOptions{APIMode: MonitorAPIModeResponses, StreamEnabled: true})
	if result.Status != MonitorStatusOperational {
		t.Fatalf("完整流应通过检测，得到 %s: %s", result.Status, result.Message)
	}
}

func TestMonitorStream_EventFramingAndTermination(t *testing.T) {
	text := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"42\"}\n\n"
	completed := "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"
	for _, tc := range []struct {
		name, provider, apiMode, stream, want, errorContains string
	}{
		{"responses", "openai", "responses", text + completed, "42", ""},
		{"response_done", "openai", "responses", text + "data: {\"type\":\"response.done\",\"response\":{\"status\":\"completed\"}}\n\n", "42", ""},
		{"crlf", "openai", "responses", strings.ReplaceAll(text+completed, "\n", "\r\n"), "42", ""},
		{"empty_data_heartbeat", "openai", "responses", "data: \n\n" + text + completed, "42", ""},
		{"multiline_and_event_name", "openai", "responses", ": heartbeat\n\nevent: response.output_text.delta\ndata: {\ndata: \"delta\":\"42\"}\n\nevent: response.completed\ndata: {\"response\":{\"status\":\"completed\"}}\n\n", "42", ""},
		{"no_final_blank_line", "openai", "responses", text + strings.TrimSpace(completed), "42", ""},
		{"completed_only", "openai", "responses", "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"42\"}]}]}}\n\n", "42", ""},
		{"partial_eof", "openai", "responses", text, "", "before terminal"},
		{"done_without_responses_terminal", "openai", "responses", text + "data: [DONE]\n\n", "", "before protocol terminal"},
		{"incomplete", "openai", "responses", text + "data: {\"type\":\"response.incomplete\",\"response\":{\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n", "", "max_output_tokens"},
		{"failed_status_in_completed", "openai", "responses", text + "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"failed\"}}\n\n", "", "failed"},
		{"error_in_completed", "openai", "responses", text + "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"error\":{\"message\":\"upstream broken\"}}}\n\n", "", "upstream broken"},
		{"bare_error", "openai", "responses", "event: error\ndata: {\"message\":\"upstream broken\"}\n\n", "", "upstream broken"},
		{"invalid_json", "openai", "responses", "data: {broken}\n\n", "", "invalid JSON"},
		{"non_object", "openai", "responses", "data: []\n\n", "", "invalid JSON"},
		{"chat_done", "openai", "chat_completions", "data: {\"choices\":[{\"delta\":{\"content\":\"42\"}}]}\n\ndata: [DONE]\n\n", "42", ""},
		{"chat_finish", "openai", "chat_completions", "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"stop\"}]}\n\n", "42", ""},
		{"chat_error_after_finish", "openai", "chat_completions", "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"stop\"}]}\n\ndata: {\"error\":{\"message\":\"late failure\"}}\n\n", "", "late failure"},
		{"chat_cutoff", "openai", "chat_completions", "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"length\"}]}\n\n", "", "length"},
		{"grok", "grok", "chat_completions", "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"stop\"}]}\n\n", "42", ""},
		{"anthropic", "anthropic", "chat_completions", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"42\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", "42", ""},
		{"anthropic_cutoff", "anthropic", "chat_completions", "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"}}\n\n", "", "max_tokens"},
		{"gemini", "gemini", "chat_completions", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"hidden\"},{\"text\":\"42\"}]},\"finishReason\":\"STOP\"}]}\n\n", "42", ""},
		{"gemini_cutoff", "gemini", "chat_completions", "data: {\"candidates\":[{\"finishReason\":\"MAX_TOKENS\"}]}\n\n", "", "MAX_TOKENS"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := readMonitorEventStream(strings.NewReader(tc.stream), tc.provider, tc.apiMode)
			if tc.errorContains != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorContains) || got != "" {
					t.Fatalf("需要错误 %q 且不返回部分答案，得到 text=%q err=%v", tc.errorContains, got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("需要 %q，得到 %q, %v", tc.want, got, err)
			}
		})
	}
}

func TestMonitorStream_ResourceLimits(t *testing.T) {
	for _, tc := range []struct {
		name, stream, errorContains string
	}{
		{"single_line", "data: " + strings.Repeat("x", monitorStreamEventMaxBytes+1), "event exceeds"},
		{"multiline_event", strings.Repeat("data: "+strings.Repeat("x", 1024)+"\n", 1024), "event exceeds"},
		{"total_with_heartbeats", strings.Repeat(":"+strings.Repeat("x", 4093)+"\n\n", monitorStreamMaxBytes/4096+1), "stream exceeds"},
		{"answer", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"" + strings.Repeat("x", monitorResponseMaxBytes+1) + "\"}\n\n", "text exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readMonitorEventStream(strings.NewReader(tc.stream), MonitorProviderOpenAI, MonitorAPIModeResponses)
			if err == nil || !strings.Contains(err.Error(), tc.errorContains) {
				t.Fatalf("需要明确的超限错误 %q，得到 %v", tc.errorContains, err)
			}
		})
	}
}

func TestMonitorStream_StopsAtTerminalAndClosesBody(t *testing.T) {
	swapMonitorHTTPClient(t)
	closed := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(closed)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", answerFromOpenAIRequest(body))
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.completed\"}\n\n")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
			return
		}
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := runCheckForModel(ctx, MonitorProviderOpenAI, srv.URL, "test-key", "test-model", &CheckOptions{APIMode: MonitorAPIModeResponses, StreamEnabled: true})
	if result.Status != MonitorStatusOperational || ctx.Err() != nil {
		t.Fatalf("终态后不应等待 HTTP EOF：%s, %s, %v", result.Status, result.Message, ctx.Err())
	}
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("检测返回后上游请求未关闭")
	}
}

func TestMonitorStream_Cancellation(t *testing.T) {
	swapMonitorHTTPClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, ": heartbeat\n\n")
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
		cancel()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	result := runCheckForModel(ctx, MonitorProviderOpenAI, srv.URL, "test-key", "test-model", &CheckOptions{APIMode: MonitorAPIModeResponses, StreamEnabled: true})
	if result.Status != MonitorStatusError || !strings.Contains(result.Message, "canceled") {
		t.Fatalf("取消必须结束流读取并报错：%s %s", result.Status, result.Message)
	}
}

func TestMonitorStream_JSONAndHTTPErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		stream bool
		body   string
		want   string
	}{
		{"large_json", 200, false, strings.Repeat(" ", monitorResponseMaxBytes+1), "response exceeds"},
		{"http_failure", 503, true, `{"error":{"message":"unavailable"}}`, "upstream HTTP 503"},
		{"large_http_failure", 503, true, strings.Repeat("x", monitorResponseMaxBytes+1), "upstream HTTP 503"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			swapMonitorHTTPClient(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			result := runCheckForModel(context.Background(), MonitorProviderOpenAI, srv.URL, "test-key", "test-model", &CheckOptions{APIMode: MonitorAPIModeResponses, StreamEnabled: tc.stream})
			if result.Status != MonitorStatusError || !strings.Contains(result.Message, tc.want) {
				t.Fatalf("需要保留错误 %q，得到 %s %s", tc.want, result.Status, result.Message)
			}
		})
	}
}

func TestMonitorStream_DoesNotAcceptTextBeforeFailure(t *testing.T) {
	swapMonitorHTTPClient(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", answerFromOpenAIRequest(body))
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"message\":\"failed after text\"}}}\n\n")
	}))
	t.Cleanup(srv.Close)
	result := runCheckForModel(context.Background(), MonitorProviderOpenAI, srv.URL, "test-key", "test-model", &CheckOptions{APIMode: MonitorAPIModeResponses, StreamEnabled: true})
	if result.Status != MonitorStatusError {
		t.Fatalf("错误终态不应被答案掩盖，得到 %s: %s", result.Status, result.Message)
	}
}
