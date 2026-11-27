package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var qualityIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)
var qualityEffort = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
var qualityNumber = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]{1,3})?$`)

func DefaultModelQualityQuestions() []ModelQualityQuestion {
	return []ModelQualityQuestion{
		{ID: "example-candy", Name: "糖果题示例", Prompt: "一袋糖果，依次分给三个人。每次都给出当时袋中糖果数量的一半，再多给两颗；三次分完后袋中剩一颗。最初有多少颗糖果？只输出一个数字，不要解释。", OutputType: "text", Rule: ModelQualityRule{Kind: "number", Expected: "36"}},
		{ID: "example-svg", Name: "鹈鹕骑自行车 SVG 动画示例", Prompt: "生成一个完整、独立的 SVG 动画，展示一只鹈鹕骑自行车。鹈鹕应有明显的长嘴和喉囊，双脚蹬踏，车轮转动。只输出 SVG 源码，不要 Markdown 代码围栏或说明，不使用外部资源。", OutputType: "svg", Rule: ModelQualityRule{Kind: "model", JudgePrompt: "根据原题和作品源码判断：是否包含可辨认的鹈鹕、自行车、车轮旋转与蹬踏动画，源码是否完整且不依赖外部资源。只从源码判断，不声称已经运行或看到渲染。全部满足时只输出通过，否则只输出不通过。", Judge: &ModelQualityCall{MaxOutputTokens: 8192, TimeoutSeconds: 600}}},
	}
}

func (s *ModelQualityService) qualityV2Repo() (ModelQualityV2Repository, error) {
	repo, ok := s.repo.(ModelQualityV2Repository)
	if !ok {
		return nil, fmt.Errorf("新版检测存储不可用")
	}
	return repo, nil
}

func (s *ModelQualityService) ConfigV2(ctx context.Context) (*ModelQualityV2Config, error) {
	repo, err := s.qualityV2Repo()
	if err != nil {
		return nil, err
	}
	c, err := repo.QualityV2Config(ctx)
	if err != nil {
		return nil, err
	}
	if c.InitializationRequired {
		c.Questions = DefaultModelQualityQuestions()
		old, e := s.repo.Config(ctx)
		if e != nil {
			return nil, e
		}
		c.Groups = []ModelQualityV2GroupConfig{}
		for _, previous := range old.Groups {
			g := ModelQualityV2GroupConfig{GroupID: previous.GroupID, IntervalMinutes: previous.IntervalMinutes, Targets: []ModelQualityTarget{}}
			for i, model := range previous.Models {
				g.Targets = append(g.Targets, ModelQualityTarget{ID: fmt.Sprintf("legacy-%d-%d", g.GroupID, i), ModelQualityCall: ModelQualityCall{GroupID: g.GroupID, KeyID: previous.KeyID, OwnerID: previous.OwnerID, Model: model, ReasoningEffort: previous.ReasoningEffort, MaxOutputTokens: 8192, TimeoutSeconds: 600}, QuestionIDs: []string{"example-candy", "example-svg"}})
			}
			c.Groups = append(c.Groups, g)
		}
		c.Enabled = false
	}
	c.SchemaVersion = ModelQualitySchemaVersion
	raw, err := s.settings.settingRepo.GetValue(ctx, SettingKeyAPIBaseURL)
	if err != nil {
		c.PauseReason = "未配置本站 API 端点地址"
		return c, nil
	}
	c.Endpoint, err = NormalizeModelQualityEndpoint(raw)
	if err != nil {
		c.PauseReason = err.Error()
	} else {
		c.PauseReason = s.endpointDNSReason(ctx, c.Endpoint)
	}
	if c.InitializationRequired && c.PauseReason == "" {
		c.PauseReason = "升级配置尚未完成，请确认题目、接口和判题模型后保存"
	}
	return c, nil
}

func qualityInvalid(message string) error {
	return infraerrors.BadRequest("MODEL_QUALITY_INVALID", message)
}

func validateQualityCall(c *ModelQualityCall, required bool) error {
	c.Model = strings.TrimSpace(c.Model)
	if c.TimeoutSeconds == 0 {
		c.TimeoutSeconds = 600
	}
	if c.MaxOutputTokens == 0 {
		c.MaxOutputTokens = 8192
	}
	if c.ChatTokenField == "" {
		c.ChatTokenField = "max_completion_tokens"
	}
	if !slices.Contains([]string{"max_completion_tokens", "max_tokens"}, c.ChatTokenField) {
		return qualityInvalid("Chat 输出上限字段无效")
	}
	if c.TimeoutSeconds < 30 || c.TimeoutSeconds > 3600 || c.MaxOutputTokens < 1 || c.MaxOutputTokens > 262144 {
		return qualityInvalid("请求超时或输出 token 上限超出范围")
	}
	if (required && (c.Model == "" || c.KeyID == nil || c.GroupID <= 0 || c.Protocol == "")) || len(c.Model) > 200 || strings.ContainsAny(c.Model, "\r\n\x00") {
		return qualityInvalid("请补齐分组、密钥、接口和模型")
	}
	if c.Protocol != "" && !slices.Contains([]string{"chat_completions", "responses", "messages", "gemini"}, c.Protocol) {
		return qualityInvalid("检测接口类型无效")
	}
	if c.Protocol == "gemini" && strings.ContainsAny(strings.TrimPrefix(c.Model, "models/"), "/:?#") {
		return qualityInvalid("Gemini 模型名称不能包含路径或操作名")
	}
	if c.ReasoningEffort != "" && !qualityEffort.MatchString(c.ReasoningEffort) {
		return qualityInvalid("推理强度格式无效")
	}
	if c.ThinkingMode != "" && !slices.Contains([]string{"enabled", "disabled", "adaptive"}, c.ThinkingMode) {
		return qualityInvalid("thinking 模式无效")
	}
	if c.ThinkingBudget != nil && (*c.ThinkingBudget < -1 || *c.ThinkingBudget > 262144) {
		return qualityInvalid("thinking 预算超出范围")
	}
	if c.Protocol == "responses" && (c.ThinkingMode != "" || c.ThinkingBudget != nil) {
		return qualityInvalid("Responses 请使用推理强度参数")
	}
	if c.Protocol == "messages" {
		if c.ThinkingMode == "enabled" && (c.ThinkingBudget == nil || *c.ThinkingBudget < 1024 || *c.ThinkingBudget >= c.MaxOutputTokens) {
			return qualityInvalid("Messages 固定 thinking 预算需至少 1024 且小于输出上限")
		}
		if c.ThinkingMode != "enabled" && c.ThinkingBudget != nil {
			return qualityInvalid("Messages 仅固定 thinking 模式接受预算")
		}
	}
	if c.Protocol == "gemini" && (c.ThinkingMode != "" || c.ThinkingBudget != nil && c.ReasoningEffort != "") {
		return qualityInvalid("Gemini 请选择 thinking 等级或预算其中一种")
	}
	reserved := []string{"model", "messages", "input", "contents", "system", "instructions", "conversation", "prompt", "include", "n", "candidateCount", "systemInstruction", "system_instruction", "stream", "tools", "tool_choice", "toolConfig", "background", "previous_response_id", "store", "api_key", "base_url", "url", "headers", "reasoning", "reasoning_effort", "thinking", "max_tokens", "max_completion_tokens", "max_output_tokens"}
	for key, value := range c.ExtraBody {
		if slices.Contains(reserved, key) || !json.Valid(value) {
			return qualityInvalid("扩展参数不能覆盖受管理字段：" + key)
		}
		if key == "generationConfig" || key == "generation_config" || key == "output_config" {
			var obj map[string]json.RawMessage
			if json.Unmarshal(value, &obj) != nil || obj == nil {
				return qualityInvalid("扩展配置必须是 JSON 对象")
			}
			for _, managed := range []string{"thinkingConfig", "thinking_config", "maxOutputTokens", "max_output_tokens", "candidateCount", "candidate_count", "effort"} {
				if _, ok := obj[managed]; ok {
					return qualityInvalid("扩展配置不能覆盖推理或输出上限")
				}
			}
			if key == "generation_config" {
				return qualityInvalid("扩展参数请使用 generationConfig")
			}
		}
	}
	return nil
}

func validateQualityV2(c *ModelQualityV2Config) error {
	if c.SchemaVersion != ModelQualitySchemaVersion || c.Version < 1 || len(c.Questions) > 50 || len(c.Groups) > 50 {
		return qualityInvalid("配置版本或数量无效")
	}
	questions := map[string]bool{}
	for i := range c.Questions {
		q := &c.Questions[i]
		q.Name, q.Prompt = strings.TrimSpace(q.Name), strings.TrimSpace(q.Prompt)
		if !qualityIdentifier.MatchString(q.ID) || questions[q.ID] || q.Name == "" || len(q.Name) > 200 || len(q.Prompt) > 65536 || len(q.SystemPrompt) > 16384 || len(q.Rule.Expected) > 65536 || len(q.Rule.JudgePrompt) > 65536 {
			return qualityInvalid("题目编号、名称或长度无效")
		}
		questions[q.ID] = true
		if !slices.Contains([]string{"text", "svg", "html"}, q.OutputType) || !slices.Contains([]string{"number", "string", "model"}, q.Rule.Kind) {
			return qualityInvalid("题目输出类型或判定方式无效")
		}
		if q.Rule.Kind == "number" && q.Rule.Expected != "" {
			if _, ok := parseQualityNumber(q.Rule.Expected); !ok {
				return qualityInvalid("预期答案必须是完整十进制数字")
			}
		}
		if q.Rule.Kind == "model" && q.Rule.Judge != nil {
			if err := validateQualityCall(q.Rule.Judge, false); err != nil {
				return err
			}
		}
	}
	groups, targets := map[int64]bool{}, map[string]bool{}
	used := map[string]bool{}
	for i := range c.Groups {
		g := &c.Groups[i]
		if g.IntervalMinutes == 0 {
			g.IntervalMinutes = 60
		}
		if g.GroupID <= 0 || groups[g.GroupID] || g.IntervalMinutes < 5 || g.IntervalMinutes > 1440 || len(g.Targets) > 20 || c.Enabled && len(g.Targets) == 0 {
			return qualityInvalid("分组、检测间隔或模型数量无效")
		}
		groups[g.GroupID] = true
		for j := range g.Targets {
			t := &g.Targets[j]
			t.GroupID = g.GroupID
			if !qualityIdentifier.MatchString(t.ID) || targets[t.ID] || len(t.QuestionIDs) > 10 || c.Enabled && len(t.QuestionIDs) == 0 {
				return qualityInvalid("模型配置编号或题目数量无效")
			}
			targets[t.ID] = true
			if err := validateQualityCall(&t.ModelQualityCall, c.Enabled); err != nil {
				return err
			}
			selected := map[string]bool{}
			for _, id := range t.QuestionIDs {
				if !questions[id] || selected[id] {
					return qualityInvalid("检测题目不存在或重复")
				}
				selected[id], used[id] = true, true
			}
		}
	}
	for i := range c.Questions {
		q := &c.Questions[i]
		if !c.Enabled || !used[q.ID] {
			continue
		}
		if q.Prompt == "" || q.Rule.Kind != "model" && strings.TrimSpace(q.Rule.Expected) == "" {
			return qualityInvalid("启用前请补齐测试提示词和预期答案")
		}
		if q.Rule.Kind == "model" {
			if q.Rule.Judge == nil || strings.TrimSpace(q.Rule.JudgePrompt) == "" {
				return qualityInvalid("启用前请补齐判题提示词和模型")
			}
			if err := validateQualityCall(q.Rule.Judge, true); err != nil {
				return err
			}
		}
	}
	if c.Enabled && len(c.Groups) == 0 {
		return qualityInvalid("至少配置一个检测分组")
	}
	return nil
}

func parseQualityNumber(raw string) (decimal.Decimal, bool) {
	value := strings.TrimSpace(raw)
	if len(value) > 256 || !qualityNumber.MatchString(value) {
		return decimal.Zero, false
	}
	number, err := decimal.NewFromString(value)
	return number, err == nil
}

func (s *ModelQualityService) validQualityCall(ctx context.Context, call ModelQualityCall) (*APIKey, error) {
	group, err := s.groups.GetByID(ctx, call.GroupID)
	if err != nil || group == nil || !domain.IsGroupPlatform(group.Platform) || group.Status != StatusActive {
		return nil, qualityInvalid("检测分组已停用或不存在")
	}
	return s.validKey(ctx, ModelQualityGroupConfig{GroupID: call.GroupID, KeyID: call.KeyID, OwnerID: call.OwnerID})
}

func qualityCalls(c *ModelQualityV2Config) []*ModelQualityCall {
	calls := []*ModelQualityCall{}
	for i := range c.Groups {
		for j := range c.Groups[i].Targets {
			calls = append(calls, &c.Groups[i].Targets[j].ModelQualityCall)
		}
	}
	for i := range c.Questions {
		if c.Questions[i].Rule.Kind == "model" && c.Questions[i].Rule.Judge != nil {
			calls = append(calls, c.Questions[i].Rule.Judge)
		}
	}
	return calls
}

func (s *ModelQualityService) SaveConfigV2(ctx context.Context, adminID int64, c *ModelQualityV2Config) error {
	if err := validateQualityV2(c); err != nil {
		return err
	}
	repo, err := s.qualityV2Repo()
	if err != nil {
		return err
	}
	saved, err := s.ConfigV2(ctx)
	if err != nil {
		return err
	}
	if saved.Version != c.Version {
		return ErrModelQualityConflict
	}
	for _, call := range qualityCalls(c) {
		call.OwnerID = adminID
		if !c.Enabled {
			unchanged := false
			for _, prior := range qualityCalls(saved) {
				if call.GroupID == prior.GroupID && call.KeyID != nil && prior.KeyID != nil && *call.KeyID == *prior.KeyID {
					call.OwnerID = prior.OwnerID
					unchanged = true
					break
				}
			}
			if unchanged {
				continue
			}
		}
		if call.KeyID != nil {
			if _, err := s.validQualityCall(ctx, *call); err != nil {
				return err
			}
		}
	}
	if c.Enabled && saved.Endpoint == "" {
		return qualityInvalid("本站 API 端点尚不可用")
	}
	c.InitializationRequired = false
	c.Endpoint, c.PauseReason = "", ""
	if err = repo.SaveQualityV2Config(ctx, c); err != nil {
		return err
	}
	s.settings.notifyCustomFeatureSettingsUpdated()
	return nil
}

func (s *ModelQualityService) KeyOptionsV2(ctx context.Context, adminID, groupID int64) ([]ModelQualityKeyOption, error) {
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil || group == nil || !domain.IsGroupPlatform(group.Platform) {
		return nil, ErrModelQualityInvalid
	}
	lister, ok := s.keys.(apiKeyAllByUserIDLister)
	if !ok {
		return nil, fmt.Errorf("密钥列表暂不可用")
	}
	keys, err := lister.ListAllByUserID(ctx, adminID, APIKeyListFilters{GroupID: &groupID, Status: StatusActive})
	if err != nil {
		return nil, err
	}
	out := []ModelQualityKeyOption{}
	for _, k := range keys {
		if k.UserID == adminID && k.GroupID != nil && *k.GroupID == groupID && k.IsActive() && !k.IsExpired() && !k.IsQuotaExhausted() && k.Purpose != APIKeyPurposeInfiniteCanvas {
			masked := "****"
			if len(k.Key) > 8 {
				masked = k.Key[:3] + "****" + k.Key[len(k.Key)-4:]
			}
			out = append(out, ModelQualityKeyOption{k.ID, k.Name, masked})
		}
	}
	return out, nil
}

func (s *ModelQualityService) qualityV2Readiness(ctx context.Context, c *ModelQualityV2Config, g ModelQualityV2GroupConfig) (string, string) {
	group, err := s.groups.GetByID(ctx, g.GroupID)
	if err != nil || group == nil {
		return "分组已移除", "分组不存在"
	}
	if c.PauseReason != "" {
		return group.Name, c.PauseReason
	}
	if len(g.Targets) == 0 {
		return group.Name, "待配置模型"
	}
	for _, target := range g.Targets {
		if _, err := s.validQualityCall(ctx, target.ModelQualityCall); err != nil {
			return group.Name, err.Error()
		}
		for _, q := range c.Questions {
			if slices.Contains(target.QuestionIDs, q.ID) && q.Rule.Kind == "model" {
				if q.Rule.Judge == nil {
					return group.Name, "待配置判题模型"
				}
				if _, err := s.validQualityCall(ctx, *q.Rule.Judge); err != nil {
					return group.Name, "判题模型：" + err.Error()
				}
			}
		}
	}
	return group.Name, ""
}

func (s *ModelQualityService) GroupsV2(ctx context.Context) ([]ModelQualityGroupState, error) {
	c, err := s.ConfigV2(ctx)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, ErrModelQualityDisabled
	}
	repo, _ := s.qualityV2Repo()
	states, err := repo.QualityV2States(ctx)
	if err != nil {
		return nil, err
	}
	out := []ModelQualityGroupState{}
	for _, g := range c.Groups {
		state := states[g.GroupID]
		state.GroupID = g.GroupID
		state.Models, state.Methods = []string{}, []string{}
		for _, target := range g.Targets {
			state.Models = append(state.Models, target.Model)
			state.Methods = append(state.Methods, target.QuestionIDs...)
		}
		state.Models, state.Methods = uniqueModelQualityStrings(state.Models), uniqueModelQualityStrings(state.Methods)
		for _, q := range c.Questions {
			if slices.Contains(state.Methods, q.ID) {
				state.Questions = append(state.Questions, ModelQualityQuestionSummary{ID: q.ID, Name: q.Name, OutputType: q.OutputType})
			}
		}
		state.Name, state.PauseReason = s.qualityV2Readiness(ctx, c, g)
		if state.Status == "" {
			state.Status = "pending"
		}
		if state.PauseReason != "" {
			state.Status = "paused"
			state.NextRunAt = nil
		}
		if state.Running {
			state.Status = "running"
		}
		out = append(out, state)
	}
	return out, nil
}

func (s *ModelQualityService) RunNowV2(ctx context.Context, groupID int64) error {
	c, err := s.ConfigV2(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return ErrModelQualityDisabled
	}
	for _, g := range c.Groups {
		if g.GroupID == groupID {
			name, reason := s.qualityV2Readiness(ctx, c, g)
			if reason != "" {
				return qualityInvalid(reason)
			}
			repo, _ := s.qualityV2Repo()
			return repo.StartQualityV2Round(ctx, c, g, name, true)
		}
	}
	return ErrModelQualityInvalid
}

func (s *ModelQualityService) visibleV2(ctx context.Context, groupID int64) error {
	c, err := s.ConfigV2(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return ErrModelQualityDisabled
	}
	for _, g := range c.Groups {
		if g.GroupID == groupID {
			return nil
		}
	}
	return ErrModelQualityExpired
}

func (s *ModelQualityService) RunsV2(ctx context.Context, f ModelQualityFilter) ([]ModelQualityV2Run, int64, error) {
	if err := s.visibleV2(ctx, f.GroupID); err != nil {
		return nil, 0, err
	}
	if f.Page < 1 || f.Page > 10000 || len(f.Method) > 100 || len(f.Model) > 200 || f.Outcome != "" && !slices.Contains([]string{"normal", "degraded", "failed", "running", "unknown"}, f.Outcome) {
		return nil, 0, ErrModelQualityInvalid
	}
	repo, _ := s.qualityV2Repo()
	return repo.QualityV2Runs(ctx, f)
}

func (s *ModelQualityService) RunV2(ctx context.Context, id string) (*ModelQualityV2Run, error) {
	if strings.HasPrefix(id, "v2-") {
		if _, err := uuid.Parse(strings.TrimPrefix(id, "v2-")); err != nil {
			return nil, ErrModelQualityExpired
		}
	} else {
		if n, err := strconv.ParseInt(strings.TrimPrefix(id, "v1-"), 10, 64); err != nil || n <= 0 {
			return nil, ErrModelQualityExpired
		}
	}
	repo, err := s.qualityV2Repo()
	if err != nil {
		return nil, err
	}
	run, err := repo.QualityV2Run(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = s.visibleV2(ctx, run.GroupID); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *ModelQualityService) RejudgeV2(ctx context.Context, id string, adminID int64) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrModelQualityInvalid
	}
	repo, err := s.qualityV2Repo()
	if err != nil {
		return err
	}
	return repo.RejudgeQualityV2Task(ctx, id, adminID)
}
