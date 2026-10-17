package manxue

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClientSubmitAndPoll(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method == http.MethodPost {
			require.Equal(t, "mq_test-key", r.Header.Get("Idempotency-Key"))
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "model-explicit", body["model"])
			require.Equal(t, "low", body["reasoning_effort"])
			require.NotContains(t, body, "protocol")
			require.NotContains(t, body, "service_tier")
			w.WriteHeader(202)
			_, _ = w.Write([]byte(`{"id":"job123","status":"running"}`))
			return
		}
		require.Equal(t, "/api/v1/tests/job123", r.URL.Path)
		_, _ = w.Write([]byte(`{"id":"job123","status":"succeeded","candy":{"status":"passed","answer":"21","input_tokens":34}}`))
	}))
	defer server.Close()
	c := NewClient()
	c.BaseURL = server.URL
	result, err := c.Submit(context.Background(), Request{Benchmark: "candy", BaseURL: "https://test.example/v1", APIKey: "secret", Model: "model-explicit", ReasoningEffort: "low"}, "mq_test-key")
	require.NoError(t, err)
	result, err = c.Poll(context.Background(), result.ID)
	require.NoError(t, err)
	require.Equal(t, "21", result.Candy.Answer)
	require.EqualValues(t, 34, *result.Candy.InputTokens)
	require.Equal(t, 2, calls)
}
func TestClientLimitsAndUncertainSubmission(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		ambiguous bool
	}{{"限流", 429, `{"error":"secret"}`, false}, {"服务繁忙", 503, "", false}, {"提交异常", 500, "", true}, {"响应损坏", 202, `{"status":"running"}`, true}, {"拒绝重定向", 307, "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "120")
				w.Header().Set("Location", "https://example.com")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			c := NewClient()
			c.BaseURL = server.URL
			_, err := c.Submit(context.Background(), Request{}, "test")
			var remote *Error
			require.ErrorAs(t, err, &remote)
			require.Equal(t, tc.ambiguous, remote.Ambiguous)
			require.NotContains(t, err.Error(), "secret")
			if tc.status == 429 || tc.status == 503 {
				require.Equal(t, 120*time.Second, remote.RetryAfter)
			}
		})
	}
	now := time.Now().UTC().Truncate(time.Second)
	require.Equal(t, 48*time.Hour, ParseRetryAfter("172800", now))
	require.Equal(t, time.Duration(1<<63-1), ParseRetryAfter("9223372036854775807", now))
	require.Equal(t, time.Minute, ParseRetryAfter(now.Add(time.Minute).Format(http.TimeFormat), now))
}

func TestClientRejectionReasonsDoNotExposeRemoteBody(t *testing.T) {
	for _, tc := range []struct {
		name, body, reason string
	}{
		{"旧参数", `{"error":"Invalid API request parameters"}`, "检测接口参数不兼容"},
		{"模型限制", `{"error":"model is required and must be gpt-6-astra for pelican, creative and html"}`, "鹈鹕检测目前仅支持 gpt-6-astra"},
		{"未知错误", `{"error":"sk-secret https://private.example/v1"}`, "请检查模型和端点配置"},
		{"已知错误附带凭据", `{"error":"Invalid API request parameters sk-secret"}`, "请检查模型和端点配置"},
		{"无效JSON", `{"error":`, "请检查模型和端点配置"},
		{"超长响应", `{"error":"Invalid API request parameters","extra":"` + strings.Repeat("x", maxRejectionBytes) + `"}`, "请检查模型和端点配置"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client := NewClient()
			client.BaseURL = server.URL
			_, err := client.Submit(context.Background(), Request{}, "test")
			var remote *Error
			require.ErrorAs(t, err, &remote)
			require.False(t, remote.Ambiguous)
			require.Contains(t, remote.RejectionReason(), "HTTP 400")
			require.Contains(t, remote.RejectionReason(), tc.reason)
			require.NotContains(t, remote.RejectionReason(), "sk-secret")
			require.NotContains(t, remote.RejectionReason(), "private.example")
			require.NotContains(t, remote.Error(), "sk-secret")
		})
	}
}

func TestClientDoesNotReplayUncertainPost(t *testing.T) {
	var submissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":"warmup","status":"running"}`))
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		if submissions.Add(1) == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("测试服务不支持连接接管")
				return
			}
			connection, _, err := hijacker.Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"duplicate","status":"running"}`))
	}))
	defer server.Close()
	c := NewClient()
	c.BaseURL = server.URL
	// 先复用连接，再模拟服务已读完提交但响应丢失。
	_, err := c.Poll(context.Background(), "warmup")
	require.NoError(t, err)
	_, err = c.Submit(context.Background(), Request{Benchmark: "candy", Model: "model-a"}, "stable-key")
	var remote *Error
	require.ErrorAs(t, err, &remote)
	require.True(t, remote.Ambiguous)
	require.EqualValues(t, 1, submissions.Load())
}
