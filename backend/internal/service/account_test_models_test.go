//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func Test账号测试默认模型与受限接入(t *testing.T) {
	for _, tc := range []struct {
		name, platform, kind, oauth, want string
	}{
		{"OpenAI", PlatformOpenAI, AccountTypeAPIKey, "", "gpt-6-astra"},
		{"OpenAI OAuth", PlatformOpenAI, AccountTypeOAuth, "", "gpt-6-astra"},
		{"Anthropic", PlatformAnthropic, AccountTypeAPIKey, "", "claude-opus-5"},
		{"Gemini", PlatformGemini, AccountTypeAPIKey, "", "gemini-3.8-flash"},
		{"Gemini Code Assist", PlatformGemini, AccountTypeOAuth, "code_assist", "gemini-3.8-flash"},
		{"Grok", PlatformGrok, AccountTypeOAuth, "", "grok-4.6"},
		{"Antigravity", PlatformAntigravity, AccountTypeOAuth, "", "claude-opus-5"},
		{"Antigravity API Key", PlatformAntigravity, AccountTypeAPIKey, "", "claude-opus-5"},
		{"Bedrock", PlatformAnthropic, AccountTypeBedrock, "", claude.DefaultTestModel},
		{"Google One", PlatformGemini, AccountTypeOAuth, "google_one", geminicli.DefaultTestModel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &Account{Platform: tc.platform, Type: tc.kind, Credentials: map[string]any{"oauth_type": tc.oauth}}
			model, err := DefaultAccountTestModel(account)
			require.NoError(t, err)
			require.Equal(t, tc.want, model)
			_, explicit, err := prepareAccountTestModel(account, "explicit-model")
			require.NoError(t, err)
			require.Equal(t, "explicit-model", explicit)
		})
	}
	// 这些常量还被非测试功能使用，不能随测试默认值一同改变。
	require.Equal(t, "gpt-5.4", openai.DefaultTestModel)
	require.Equal(t, "grok-4.5", grokDefaultResponsesModel)
	require.Equal(t, "claude-sonnet-4-5-20250929", claude.DefaultTestModel)
}

func Test国产测试模型确定性选择与单次映射(t *testing.T) {
	for _, platform := range []string{PlatformKimi, PlatformZhipu, PlatformDeepseek, PlatformMiniMax} {
		t.Run(platform, func(t *testing.T) {
			account := &Account{Platform: platform, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"model_mapping": map[string]any{
					"z-text": "native-z", "a-text": "native-a", "native-a": "wrong-second-mapping",
					"0-image": "image-model", "0-text-to-video": "text-model", "0-embedding": "embedding-model",
					"0-empty": "", "0-wildcard-target": "bad-*", "*": "wildcard-native",
				},
			}}
			prepared, model, err := prepareAccountTestModel(account, "")
			require.NoError(t, err)
			require.Same(t, account, prepared)
			require.Equal(t, "a-text", model)
			require.Equal(t, "native-a", prepared.GetMappedModel(model))
			require.Equal(t, "a-text", CNAccountTestModels(account)[0])

			account.Credentials["model_mapping"] = map[string]any{"a*": "b-text", "b*": "c-text", "c*": "c-text"}
			prepared, model, err = prepareAccountTestModel(account, "")
			require.NoError(t, err)
			require.NotSame(t, account, prepared)
			require.Equal(t, "b-text", model)
			require.Equal(t, "b-text", prepared.GetMappedModel(model))
			require.Equal(t, "c-text", account.GetMappedModel(model))
			require.Equal(t, []string{"b-text"}, CNAccountTestModels(account))
		})
	}
}

func Test国产无具体文本模型不发送请求(t *testing.T) {
	for _, mapping := range []map[string]any{nil, {"*": "*"}, {"image-model": "gpt-image-2"}, {"embedding": "embedding"}} {
		account := &Account{ID: 1, Platform: PlatformKimi, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"api_key": "test", "model_mapping": mapping,
		}}
		upstream := &queuedHTTPUpstream{}
		svc := &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}, httpUpstream: upstream}
		result, err := svc.RunTestBackground(context.Background(), 1, "")
		require.NoError(t, err)
		require.Equal(t, "failed", result.Status)
		require.Contains(t, result.ErrorMessage, "具体文本模型")
		require.Empty(t, upstream.requests)
		_, explicit, err := prepareAccountTestModel(account, "specified-model")
		require.NoError(t, err)
		require.Equal(t, "specified-model", explicit)
	}
}

func TestOpenAI默认测试实际请求模型(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test"}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.completed\"}\n\n")}}
	svc := &AccountTestService{accountRepo: &mockAccountRepoForGemini{accountsByID: map[int64]*Account{1: account}}, httpUpstream: upstream}
	result, err := svc.RunTestBackground(context.Background(), 1, "")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "gpt-6-astra", readQueuedRequestJSON(t, upstream.requests[0])["model"])
}

func TestAntigravity默认Opus5映射不改写自定义映射(t *testing.T) {
	account := &Account{Platform: PlatformAntigravity}
	require.Equal(t, "claude-opus-5", mapAntigravityModel(account, defaultAntigravityTestModel))
	account.Credentials = map[string]any{"model_mapping": map[string]any{"claude-opus-5": "custom-opus"}}
	require.Equal(t, "custom-opus", mapAntigravityModel(account, defaultAntigravityTestModel))
}

func TestFetchOpenAIAccountModelsOAuthPopulatesPickerFields(t *testing.T) {
	_, calls := newCodexModelsOAuthCacheServer(t, `{"models":[{"slug":"new-oauth-model"},{"slug":"gpt-6-astra"}]}`)
	gateway := &OpenAIGatewayService{}
	svc := &AccountTestService{openaiGatewayService: gateway}
	account := newCodexModelsTestAccount()
	ctx := context.Background()

	before, err := gateway.FetchOpenAIModelsList(ctx, account)
	require.NoError(t, err)
	models, err := svc.FetchOpenAIAccountModels(ctx, account)
	require.NoError(t, err)
	require.Greater(t, len(models), 2)
	for i, id := range []string{"new-oauth-model", "gpt-6-astra"} {
		require.Equal(t, id, models[i].ID)
		require.Equal(t, id, models[i].DisplayName)
		require.Equal(t, "model", models[i].Type)
	}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	require.Contains(t, ids, "gpt-image-2.5-flare")
	require.Contains(t, ids, "gpt-image-2.5-sunburst")

	after, err := gateway.FetchOpenAIModelsList(ctx, account)
	require.NoError(t, err)
	require.Equal(t, before.Body, after.Body, "picker fields must not change the shared catalog")
	require.NotContains(t, string(after.Body), "display_name")
	require.EqualValues(t, 1, calls.Load(), "picker must reuse the shared discovery cache")
}

func TestFetchOpenAIAccountModelsAPIKeyPopulatesPickerFields(t *testing.T) {
	gateway := newCodexModelsAPIKeyTestService(&codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		return ordinaryModelsUpstreamResponse(`{"data":[
			{"id":"new-api-model","owned_by":"provider","created":123},
			{"id":"blank-label","display_name":"  ","type":""},
			{"id":"named-model","display_name":"Provider Model","type":"model"}
		]}`), nil
	}})
	svc := &AccountTestService{openaiGatewayService: gateway}
	models, err := svc.FetchOpenAIAccountModels(context.Background(), newCodexModelsAPIKeyTestAccount("https://models.example/v1"))
	require.NoError(t, err)
	require.Len(t, models, 3)
	for i, name := range []string{"new-api-model", "blank-label", "Provider Model"} {
		require.Equal(t, name, models[i].DisplayName)
		require.Equal(t, "model", models[i].Type)
	}
	require.Equal(t, "provider", models[0].OwnedBy)
	require.EqualValues(t, 123, models[0].Created)
	require.Equal(t, "named-model", models[2].ID)
}

func TestFetchOpenAIAccountModelsPreservesEmptyCatalog(t *testing.T) {
	gateway := newCodexModelsAPIKeyTestService(&codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		return ordinaryModelsUpstreamResponse(`{"data":[]}`), nil
	}})
	svc := &AccountTestService{openaiGatewayService: gateway}
	models, err := svc.FetchOpenAIAccountModels(context.Background(), newCodexModelsAPIKeyTestAccount("https://models.example/v1"))
	require.NoError(t, err)
	require.Empty(t, models, "an empty upstream catalog must not become a static model list")
}

func TestFetchOpenAIAccountModelsOAuthRespectsImageAllowlist(t *testing.T) {
	newCodexModelsOAuthCacheServer(t, `{"models":[{"slug":"gpt-6-astra"}]}`)
	svc := &AccountTestService{openaiGatewayService: &OpenAIGatewayService{}}
	account := newCodexModelsTestAccount()
	account.Credentials["model_mapping"] = map[string]any{"gpt-image-2.5-flare": "gpt-image-2.5-flare"}
	models, err := svc.FetchOpenAIAccountModels(context.Background(), account)
	require.NoError(t, err)
	ids := []string{}
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	require.Contains(t, ids, "gpt-image-2.5-flare")
	require.NotContains(t, ids, "gpt-image-2.5-sunburst")
}
