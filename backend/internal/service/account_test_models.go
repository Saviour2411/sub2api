package service

import (
	"errors"
	"maps"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
)

// 测试默认值独立于业务请求、计费回退和额度探测所使用的模型。
const (
	defaultOpenAIAccountTestModel    = "gpt-6-astra"
	defaultAnthropicAccountTestModel = "claude-opus-5"
	defaultGeminiAccountTestModel    = "gemini-3.8-flash"
	defaultGrokAccountTestModel      = "grok-4.6"
)

// DefaultAccountTestModel 返回账号未显式指定测试模型时的默认选择，供管理界面同步预选。
func DefaultAccountTestModel(account *Account) (string, error) {
	model, _, err := resolveDefaultAccountTestModel(account)
	return model, err
}

// CNAccountTestModels 为测试界面提供具体模型，默认模型排在首位，不把通配符当作模型发送。
func CNAccountTestModels(account *Account) []string {
	candidates := make(map[string]struct{})
	for request, target := range account.GetModelMapping() {
		request, target = strings.TrimSpace(request), strings.TrimSpace(target)
		if request != "" && !strings.Contains(request, "*") && target != "" && !strings.Contains(target, "*") {
			candidates[request] = struct{}{}
		}
	}
	preferred, _ := DefaultAccountTestModel(account)
	if preferred != "" {
		candidates[preferred] = struct{}{}
	}
	models := make([]string, 0, len(candidates))
	for model := range candidates {
		if model != preferred {
			models = append(models, model)
		}
	}
	sort.Strings(models)
	if preferred != "" {
		models = append([]string{preferred}, models...)
	}
	return models
}

func resolveDefaultAccountTestModel(account *Account) (string, bool, error) {
	if account == nil {
		return "", false, ErrAccountNilInput
	}
	if account.IsBedrock() {
		return claude.DefaultTestModel, false, nil
	}
	if account.IsGeminiGoogleOne() {
		return geminicli.DefaultTestModel, false, nil
	}
	if account.IsCNProvider() {
		return selectCNAccountTestModel(account)
	}
	switch account.Platform {
	case PlatformOpenAI:
		return defaultOpenAIAccountTestModel, false, nil
	case PlatformGemini:
		return defaultGeminiAccountTestModel, false, nil
	case PlatformGrok:
		return defaultGrokAccountTestModel, false, nil
	case PlatformAntigravity:
		return defaultAntigravityTestModel, false, nil
	default:
		return defaultAnthropicAccountTestModel, false, nil
	}
}

func prepareAccountTestModel(account *Account, model string) (*Account, string, error) {
	if model = strings.TrimSpace(model); model != "" {
		return account, model, nil
	}
	model, direct, err := resolveDefaultAccountTestModel(account)
	if err != nil {
		return account, "", err
	}
	if direct {
		account = accountWithExactTestModel(account, model)
	}
	return account, model, nil
}

func selectCNAccountTestModel(account *Account) (string, bool, error) {
	requests := make(map[string]struct{})
	targets := make(map[string]struct{})
	for request, target := range account.GetModelMapping() {
		request, target = strings.TrimSpace(request), strings.TrimSpace(target)
		if request == "" || !isTextAccountTestModel(target) {
			continue
		}
		if !strings.Contains(request, "*") {
			if isTextAccountTestModel(request) {
				requests[request] = struct{}{}
			}
		} else if strings.HasSuffix(request, "*") && strings.Count(request, "*") == 1 {
			targets[target] = struct{}{}
		}
	}
	for i, candidates := range []map[string]struct{}{requests, targets} {
		if len(candidates) == 0 {
			continue
		}
		models := make([]string, 0, len(candidates))
		for model := range candidates {
			models = append(models, model)
		}
		sort.Strings(models)
		return models[0], i == 1, nil
	}
	return "", false, errors.New("账号未配置可用于测试的具体文本模型，请在模型白名单或测试计划中指定模型")
}

func isTextAccountTestModel(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" || strings.Contains(model, "*") || IsGeneratedImageModel(model) || isGrokVideoGenerationModel(model) {
		return false
	}
	for _, family := range []string{"image", "video", "audio", "voice", "speech", "tts", "stt", "whisper", "realtime", "embedding", "rerank"} {
		if strings.Contains(model, family) {
			return false
		}
	}
	return true
}

// 已经选中上游目标时，只在请求副本中固定映射，避免通配符或链式映射再次改写。
func accountWithExactTestModel(account *Account, model string) *Account {
	copy := *account
	copy.Credentials = maps.Clone(account.Credentials)
	if copy.Credentials == nil {
		copy.Credentials = make(map[string]any)
	}
	mapping := make(map[string]any)
	for request, target := range account.GetModelMapping() {
		mapping[request] = target
	}
	mapping[model] = model
	copy.Credentials["model_mapping"] = mapping
	copy.modelMappingCacheReady = false
	return &copy
}
