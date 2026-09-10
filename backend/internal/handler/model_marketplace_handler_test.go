package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelMarketplaceModelsForGroup_Allowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name      string
		allowlist service.GroupModelAllowlist
		want      []string
	}{
		{"未启用时保留账号模型", service.GroupModelAllowlist{}, []string{"gpt-custom-allowed", "gpt-custom-denied"}},
		{"启用后仅展示允许的模型", service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-custom-allowed"}}, []string{"gpt-custom-allowed"}},
		{"支持通配条目", service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-custom-*"}}, []string{"gpt-custom-allowed", "gpt-custom-denied"}},
		{"无匹配不回填平台默认列表", service.GroupModelAllowlist{Enabled: true, Models: []string{"missing-model"}}, []string{}},
		{"遗留启用空配置不回填默认列表", service.GroupModelAllowlist{Enabled: true}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			group := service.Group{ID: 21, Platform: service.PlatformOpenAI, ModelAllowlist: tc.allowlist}
			repo := &gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
				group.ID: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{
					"model_mapping": map[string]any{"gpt-custom-allowed": "mapped-a", "gpt-custom-denied": "mapped-b"},
				}}},
			}}
			h := &ModelMarketplaceHandler{gatewayService: newGatewayModelsHandlerForTest(repo).gatewayService}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-marketplace", nil)
			got := h.modelsForGroup(c, group)
			require.Equal(t, tc.want, got)
			for _, model := range got {
				require.True(t, group.ModelAllowlist.Allows(model), "公开模型必须通过相同的请求准入规则")
			}
		})
	}
}

func TestRequestFormatsForGroup_OnlyConversationFormats(t *testing.T) {
	cases := []struct {
		name      string
		group     service.Group
		models    []string
		wantPaths []string
	}{
		{
			name: "openai 默认展示 chat 和 responses",
			group: service.Group{
				Platform: service.PlatformOpenAI,
			},
			wantPaths: []string{"/v1/chat/completions", "/v1/responses"},
		},
		{
			name: "openai 启用 messages 调度时追加 messages",
			group: service.Group{
				Platform:              service.PlatformOpenAI,
				AllowMessagesDispatch: true,
			},
			wantPaths: []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"},
		},
		{
			name: "openai 生图专用分组只展示图片接口",
			group: service.Group{
				Platform:             service.PlatformOpenAI,
				AllowImageGeneration: true,
			},
			models:    []string{"gpt-image-2"},
			wantPaths: []string{"/v1/images/generations", "/v1/images/edits"},
		},
		{
			name: "openai 混合模型分组仍展示对话接口",
			group: service.Group{
				Platform:             service.PlatformOpenAI,
				AllowImageGeneration: true,
			},
			models:    []string{"gpt-5.5", "gpt-image-2"},
			wantPaths: []string{"/v1/chat/completions", "/v1/responses"},
		},
		{
			name: "openai 未开启生图权限不按生图专用展示",
			group: service.Group{
				Platform: service.PlatformOpenAI,
			},
			models:    []string{"gpt-image-2"},
			wantPaths: []string{"/v1/chat/completions", "/v1/responses"},
		},
		{
			name: "anthropic 只展示 messages",
			group: service.Group{
				Platform: service.PlatformAnthropic,
			},
			wantPaths: []string{"/v1/messages"},
		},
		{
			name: "gemini 只展示生成内容接口",
			group: service.Group{
				Platform: service.PlatformGemini,
			},
			wantPaths: []string{"/v1beta/models/{model}:generateContent"},
		},
		{
			name: "antigravity 展示两类生成接口",
			group: service.Group{
				Platform: service.PlatformAntigravity,
			},
			wantPaths: []string{"/antigravity/v1/messages", "/antigravity/v1beta/models/{model}:generateContent"},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			formats := requestFormatsForGroup(tt.group, tt.models)
			paths := make([]string, 0, len(formats))
			for _, format := range formats {
				paths = append(paths, format.Path)
			}

			require.Equal(t, tt.wantPaths, paths)
			require.NotContains(t, paths, "/v1/models")
			require.NotContains(t, paths, "/v1/embeddings")
			require.NotContains(t, paths, "/v1/messages/count_tokens")
			require.NotContains(t, paths, "/v1beta/models")
			require.NotContains(t, paths, "/antigravity/models")
			if len(tt.models) > 0 && tt.wantPaths[0] == "/v1/images/generations" {
				require.NotContains(t, paths, "/v1/chat/completions")
				require.NotContains(t, paths, "/v1/responses")
				require.Contains(t, formats[0].Body, "\"prompt\"")
				require.Contains(t, formats[1].Body, "\"images\"")
			}
		})
	}
}
