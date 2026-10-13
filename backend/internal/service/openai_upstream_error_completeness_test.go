//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReadOpenAIUpstreamErrorCompletenessAndPluginError(t *testing.T) {
	rejection := `{"error":{"code":"invalid_type","param":"input","message":"Expected a string, but got an array instead."}}`
	limit := int(openAIUpstreamErrorBodyReadLimitForConfig(nil))
	pluginErr := &PluginTransportError{Code: "UPSTREAM_EOF", Message: "读取中断", RequestSent: true}
	for _, tc := range []struct {
		name     string
		body     io.ReadCloser
		complete bool
		readErr  error
	}{
		{"完整正文", io.NopCloser(strings.NewReader(rejection)), true, nil},
		{"恰好达到上限", io.NopCloser(strings.NewReader(rejection + strings.Repeat(" ", limit-len(rejection)))), true, nil},
		{"超过上限", io.NopCloser(strings.NewReader(rejection + strings.Repeat(" ", limit-len(rejection)+1))), false, nil},
		{"插件已发送后读取失败", &errTailReader{data: []byte(rejection), err: pluginErr}, false, pluginErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: http.StatusBadRequest, Body: tc.body}
			body, message, complete, err := (&OpenAIGatewayService{}).readOpenAIUpstreamErrorWithCompleteness(resp)
			require.ErrorIs(t, err, tc.readErr)
			require.Equal(t, tc.complete, complete)
			require.LessOrEqual(t, len(body), limit)
			require.NotEmpty(t, message)
			rewound, readErr := io.ReadAll(resp.Body)
			require.NoError(t, readErr)
			require.Equal(t, body, rewound)
		})
	}
}

func TestForwardAsChatCompletions_StructuredInputPluginErrorDoesNotRetry(t *testing.T) {
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	pluginErr := &PluginTransportError{Code: "UPSTREAM_EOF", Message: "读取中断", RequestSent: true}
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": {"application/json"}},
		Body: &errTailReader{data: []byte(`{"error":{"code":"invalid_type","param":"input","message":"Expected a string, but got an array instead."}}`), err: pluginErr},
	}}
	svc := &OpenAIGatewayService{cfg: structuredInputRetryTestConfig(), httpUpstream: upstream}
	result, err := svc.ForwardAsChatCompletions(context.Background(), c, structuredInputRetryTestAccount(), body, "", "")
	require.ErrorIs(t, err, pluginErr)
	require.Nil(t, result)
	require.Len(t, upstream.requests, 1, "插件已经发送的请求不能重放")
}
