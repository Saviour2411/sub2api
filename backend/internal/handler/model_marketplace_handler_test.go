package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRequestFormatsForGroup_OnlyConversationFormats(t *testing.T) {
	cases := []struct {
		name      string
		group     service.Group
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
			formats := requestFormatsForGroup(tt.group)
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
		})
	}
}
