package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiFirstTokenAliasProbeUpstream struct {
	calls                    int
	firstTokenAttemptStarted bool
}

func (s *geminiFirstTokenAliasProbeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.calls++
	_, s.firstTokenAttemptStarted = req.Context().(firstTokenMergedContext)
	return &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"invalid request"}}`)),
		Request:    req,
	}, nil
}

func (s *geminiFirstTokenAliasProbeUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestGeminiFirstTokenTimeoutExclusionUsesMappedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	paths := []struct {
		name    string
		path    string
		forward func(*GeminiMessagesCompatService, *gin.Context, *Account, string) error
	}{
		{
			name: "Chat Completions 兼容入口",
			path: "/v1/chat/completions",
			forward: func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account, model string) error {
				body := []byte(`{"model":"` + model + `","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
				_, err := svc.ForwardAsChatCompletions(context.Background(), c, account, body)
				return err
			},
		},
		{
			name: "Messages 兼容入口",
			path: "/v1/messages",
			forward: func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account, model string) error {
				body := []byte(`{"model":"` + model + `","stream":true,"max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
				_, err := svc.Forward(context.Background(), c, account, body)
				return err
			},
		},
		{
			name: "Gemini Native 入口",
			path: "/v1beta/models/model:streamGenerateContent",
			forward: func(svc *GeminiMessagesCompatService, c *gin.Context, account *Account, model string) error {
				body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`)
				_, err := svc.ForwardNative(context.Background(), c, account, model, "streamGenerateContent", true, body)
				return err
			},
		},
	}

	cases := []struct {
		name         string
		requested    string
		mapped       string
		wantExcluded bool
	}{
		{
			name:         "文本别名映射到图片模型时排除",
			requested:    "custom-text-alias",
			mapped:       "gemini-2.5-flash-image-preview",
			wantExcluded: true,
		},
		{
			name:         "图片模型名映射到文本模型时不排除",
			requested:    "gemini-2.5-flash-image-preview",
			mapped:       "gemini-2.5-flash",
			wantExcluded: false,
		},
	}

	for _, path := range paths {
		path := path
		for _, tc := range cases {
			tc := tc
			t.Run(path.name+"/"+tc.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, path.path, bytes.NewReader(nil))
				probe := &geminiFirstTokenAliasProbeUpstream{}

				account := &Account{
					ID:       1,
					Platform: PlatformGemini,
					Type:     AccountTypeAPIKey,
					Credentials: map[string]any{
						"api_key":       "test-key",
						"model_mapping": map[string]any{tc.requested: tc.mapped},
					},
				}
				svc := &GeminiMessagesCompatService{httpUpstream: probe, cfg: &config.Config{}}

				err := path.forward(svc, c, account, tc.requested)
				require.Error(t, err)
				require.Equal(t, 1, probe.calls)
				require.Equal(t, !tc.wantExcluded, probe.firstTokenAttemptStarted)
				require.False(t, c.GetBool("first_token_timeout_excluded"))
			})
		}
	}
}

func TestGeminiFirstTokenTimeoutExclusionDoesNotLeakAcrossAccountSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	const requestedModel = "shared-alias"
	imageAccount := &Account{
		ID:   1,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{requestedModel: "gemini-2.5-flash-image-preview"},
		},
	}
	textAccount := &Account{
		ID:   2,
		Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{requestedModel: "gemini-2.5-flash"},
		},
	}

	imageAttempt := newFirstTokenAttempt(c.Request.Context(), c, nil, imageAccount, imageAccount.GetMappedModel(requestedModel), true)
	require.Nil(t, imageAttempt)
	require.False(t, c.GetBool("first_token_timeout_excluded"))

	textAttempt := newFirstTokenAttempt(c.Request.Context(), c, nil, textAccount, textAccount.GetMappedModel(requestedModel), true)
	require.NotNil(t, textAttempt)
	textAttempt.stopBeforeStreaming()
}
