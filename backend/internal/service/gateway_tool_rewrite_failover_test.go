package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGatewayForwardEntrypointsClearStaleToolNameRewrite(t *testing.T) {
	svc := &GatewayService{}
	tests := []struct {
		name string
		call func(*gin.Context) error
	}{
		{
			name: "Messages",
			call: func(c *gin.Context) error {
				_, err := svc.Forward(context.Background(), c, nil, nil)
				return err
			},
		},
		{
			name: "CountTokens",
			call: func(c *gin.Context) error {
				return svc.ForwardCountTokens(context.Background(), c, nil, nil)
			},
		},
		{
			name: "ChatCompletions",
			call: func(c *gin.Context) error {
				_, err := svc.ForwardAsChatCompletions(context.Background(), c, nil, []byte("{"), nil)
				return err
			},
		},
		{
			name: "Responses",
			call: func(c *gin.Context) error {
				_, err := svc.ForwardAsResponses(context.Background(), c, nil, []byte("{"), nil)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set(toolNameRewriteKey, &ToolNameRewrite{
				Forward: map[string]string{"old_tool": "stale_alias"},
			})

			if err := tt.call(c); err == nil {
				t.Fatal("期望无效请求返回错误")
			}
			if got := toolNameRewriteFromContext(c); got != nil {
				t.Fatalf("转发入口未清除上一次账号尝试的工具名映射: %#v", got)
			}
		})
	}
}
