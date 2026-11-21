package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMappedResponseModelPreservesOtherData(t *testing.T) {
	svc := &OpenAIGatewayService{}
	body := `{"model":"alias","text":"mapped alias","tool":{"model":"mapped","arguments":"{\"model\":\"alias\"}"}}`
	want := `{"model":"public","text":"mapped alias","tool":{"model":"mapped","arguments":"{\"model\":\"alias\"}"}}`
	require.Equal(t, want, string(svc.replaceModelInResponseBody([]byte(body), "mapped", "public")))
	require.Equal(t, "data: "+want, svc.replaceModelInSSELine("data: "+body, "mapped", "public"))
	for _, body := range []string{
		`{"model":"alias",`, `{"model":"alias"} trailing`, `{"model":null}`, `{"model":42}`, `{"model":{}}`, `{"model":[]}`, `{"model":true}`,
		`{"text":"alias","tool":{"model":"alias"}}`,
	} {
		require.Equal(t, body, string(svc.replaceModelInResponseBody([]byte(body), "mapped", "public")))
		require.Equal(t, "data: "+body, svc.replaceModelInSSELine("data: "+body, "mapped", "public"))
	}
	for _, models := range [][2]string{{"", "public"}, {"mapped", ""}} {
		body := `{"model":"alias","response":{"model":"alias"}}`
		require.Equal(t, body, string(svc.replaceModelInResponseBody([]byte(body), models[0], models[1])))
		require.Equal(t, "data: "+body, svc.replaceModelInSSELine("data: "+body, models[0], models[1]))
	}
	require.Equal(t, `data: {"response":{"model":42}}`, svc.replaceModelInSSELine(`data: {"response":{"model":42}}`, "mapped", "public"))
	require.Equal(t, `{"model":"public"}`, string(svc.replaceModelInResponseBody([]byte(`{"model":""}`), "mapped", "public")))
}

func TestUnmappedResponseModelRewrite(t *testing.T) {
	svc := &OpenAIGatewayService{}
	for _, model := range []string{"deepseek-flash", "deepseek-v4-pro"} {
		t.Run(model, func(t *testing.T) {
			body := `{"model":"runtime-version","text":"runtime-version","tool":{"model":"runtime-version"}}`
			want := strings.Replace(body, `"model":"runtime-version"`, `"model":"`+model+`"`, 1)
			require.Equal(t, want, string(svc.replaceModelInResponseBody([]byte(body), model, model)))
			require.Equal(t, "data: "+want, svc.replaceModelInSSELine("data: "+body, model, model))
			require.Equal(t, `data: {"response":{"model":"`+model+`"}}`, svc.replaceModelInSSELine(`data: {"response":{"model":"runtime-version"}}`, model, model))
			unchanged := []byte(want)
			require.Same(t, &unchanged[0], &svc.replaceModelInResponseBody(unchanged, model, model)[0], "型号一致时复用原响应")
			require.Equal(t, "data:\t"+want, svc.replaceModelInSSELine("data:\t"+want, model, model))
		})
	}
}

// 覆盖普通与透传路径，未映射请求同样回写，同时保留上游原始型号。
func TestMappedResponseModelForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, returned := range []string{"zhipu/glm-5.3", "glm-5.3-alias"} {
			for _, mapped := range []string{"ZHIPU/GLM-5.3", "public"} {
				for _, kind := range []string{"json", "chat", "responses", "aggregated"} {
					name := kind + "/" + returned + "/" + mapped
					if passthrough {
						name += "/passthrough"
					}
					t.Run(name, func(t *testing.T) {
						model := "public"
						payload := `{"model":"` + returned + `","choices":[{"delta":{"content":"keep alias","tool_calls":[{"function":{"arguments":"{\"model\":\"alias\"}"}}]}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`
						want := strings.Replace(payload, `"model":"`+returned+`"`, `"model":"`+model+`"`, 1)
						contentType := "application/json"
						body := payload
						if kind == "responses" || kind == "aggregated" {
							payload = `{"type":"response.completed","response":{"id":"resp_1","created_at":1700000000,"model":"` + returned + `","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`
							want = strings.Replace(payload, `"model":"`+returned+`"`, `"model":"`+model+`"`, 1)
						}
						if kind != "json" {
							contentType = "text/event-stream"
							body = "data: " + payload + "\n\ndata: [DONE]\n\n"
						}
						rec := httptest.NewRecorder()
						c, _ := gin.CreateTestContext(rec)
						c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
						resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body))}
						svc := &OpenAIGatewayService{}
						account := &Account{ID: 1}
						var err error
						if kind == "json" || kind == "aggregated" {
							if passthrough {
								_, err = svc.handleNonStreamingResponsePassthrough(context.Background(), resp, c, account, "public", mapped)
							} else {
								_, err = svc.handleNonStreamingResponse(context.Background(), resp, c, account, "public", mapped)
							}
						} else {
							if passthrough {
								_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "public", mapped)
							} else {
								_, err = svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "public", mapped)
							}
						}
						require.NoError(t, err)
						if kind == "aggregated" {
							want = `"model":"public"`
						}
						require.Contains(t, rec.Body.String(), want)
						require.Equal(t, returned, observedUpstreamResponseModel(c))
					})
				}
			}
		}
	}
}
