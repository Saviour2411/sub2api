package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// anthropicMimicEndpoint 用于区分 Messages 与 Count Tokens 的请求体能力。
// Count Tokens 接口只接受输入侧字段，不能携带生成参数或 metadata。
type anthropicMimicEndpoint uint8

const (
	anthropicMimicEndpointMessages anthropicMimicEndpoint = iota
	anthropicMimicEndpointCountTokens
)

const (
	anthropicBetaMCPClient       = "mcp-client-2025-04-04"
	anthropicBetaAdvancedToolUse = "advanced-tool-use-2025-11-20"
)

var anthropicComputerToolBetas = map[string]string{
	"computer_20241022": "computer-use-2024-10-22",
	"computer_20250124": "computer-use-2025-01-24",
	"computer_20251124": "computer-use-2025-11-24",
}

// 这些字段属于 OpenAI Chat/Responses/Codex 请求，而不是 Anthropic Messages。
// 这里有意采用明确列表，而非 Anthropic 顶层字段白名单，以免未来 Anthropic 新增
// 标准字段时被网关静默删除。
var anthropicMimicOpenAIOnlyFields = []string{
	"audio",
	"background",
	"frequency_penalty",
	"function_call",
	"functions",
	"include",
	"input",
	"instructions",
	"logit_bias",
	"logprobs",
	"max_completion_tokens",
	"max_output_tokens",
	"modalities",
	"n",
	"parallel_tool_calls",
	"prediction",
	"presence_penalty",
	"previous_response_id",
	"prompt_cache_key",
	"prompt_cache_retention",
	"reasoning",
	"reasoning_effort",
	"response_format",
	"safety_identifier",
	"seed",
	"stop",
	"store",
	"stream_options",
	"text",
	"top_logprobs",
	"truncation",
	"user",
	"verbosity",
	"web_search_options",
}

// Count Tokens 当前接受 model/messages/system/tools/tool_choice/thinking、顶层
// cache_control 与 output_config。下列生成或调度字段只属于 Messages 请求。
var anthropicCountTokensUnsupportedFields = []string{
	"container",
	"inference_geo",
	"max_tokens",
	"metadata",
	"service_tier",
	"speed",
	"stop_sequences",
	"stream",
	"temperature",
	"top_k",
	"top_p",
}

var anthropicMimicUnsupportedOpenAIToolTypes = map[string]struct{}{
	"apply_patch":          {},
	"code_interpreter":     {},
	"computer":             {},
	"computer_use_preview": {},
	"file_search":          {},
	"image_generation":     {},
	"local_shell":          {},
	"mcp":                  {},
	"namespace":            {},
	"shell":                {},
	"web_search":           {},
	"web_search_preview":   {},
}

// sanitizeClaudeCodeMimicryBody 在工具名改写及 CCH/签名之前，按最终
// anthropic-beta 清理模拟 Claude Code 请求体。
//
// model 只用于已经确认的定向能力规则；其他模型不会按名称猜测或做白名单式清洗。
func sanitizeClaudeCodeMimicryBody(
	body []byte,
	model string,
	finalBeta string,
	endpoint anthropicMimicEndpoint,
) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}

	out := migrateAnthropicOutputFormat(body)
	out = deleteAnthropicMimicPaths(out, anthropicMimicOpenAIOnlyFields)
	if endpoint == anthropicMimicEndpointCountTokens {
		out = deleteAnthropicMimicPaths(out, anthropicCountTokensUnsupportedFields)
	}
	out = sanitizeKnownAnthropicModelParameters(out, model)

	out = sanitizeAnthropicMimicCapabilities(out, finalBeta, endpoint)
	out = sanitizeAnthropicMimicTools(out, finalBeta, endpoint)
	// OpenAI function 工具扁平化会把 function.cache_control 提升到工具顶层，
	// 因此需要在扁平化后再次按最终 beta 校验 scope 与 1h TTL。
	out = sanitizeAnthropicMimicCacheControlCapabilities(out, finalBeta)
	return enforceCacheControlLimit(out)
}

// sanitizeKnownAnthropicModelParameters 只应用已经确认的模型定向规则，不把
// 其他 Claude 或第三方兼容模型纳入推测式白名单。
func sanitizeKnownAnthropicModelParameters(body []byte, model string) []byte {
	if !isAnthropicOpus47Or48(model) {
		return body
	}
	out := body
	if temperature := gjson.GetBytes(out, "temperature"); temperature.Exists() &&
		(temperature.Type != gjson.Number || temperature.Float() != 1) {
		out = deleteAnthropicMimicPaths(out, []string{"temperature"})
	}
	if topP := gjson.GetBytes(out, "top_p"); topP.Exists() &&
		(topP.Type != gjson.Number || topP.Float() < 0.99) {
		out = deleteAnthropicMimicPaths(out, []string{"top_p"})
	}
	out = deleteAnthropicMimicPaths(out, []string{"top_k"})
	return out
}

func isAnthropicOpus47Or48(model string) bool {
	lower := strings.ToLower(strings.TrimSpace(model))
	if !strings.Contains(lower, "opus") {
		return false
	}
	matches := claudeVersionRe.FindStringSubmatch(lower)
	if len(matches) < 3 {
		return false
	}
	major, majorErr := strconv.Atoi(matches[1])
	minor, minorErr := strconv.Atoi(matches[2])
	return majorErr == nil && minorErr == nil && major == 4 && (minor == 7 || minor == 8)
}

func migrateAnthropicOutputFormat(body []byte) []byte {
	legacy := gjson.GetBytes(body, "output_format")
	if !legacy.Exists() {
		return body
	}

	out := body
	outputConfig := gjson.GetBytes(out, "output_config")
	if (!outputConfig.Exists() || outputConfig.IsObject()) && !outputConfig.Get("format").Exists() {
		if next, err := sjson.SetRawBytes(out, "output_config.format", []byte(legacy.Raw)); err == nil {
			out = next
		}
	}
	if next, err := sjson.DeleteBytes(out, "output_format"); err == nil {
		out = next
	}
	return out
}

func deleteAnthropicMimicPaths(body []byte, paths []string) []byte {
	out := body
	for _, path := range paths {
		if !gjson.GetBytes(out, path).Exists() {
			continue
		}
		if next, err := sjson.DeleteBytes(out, path); err == nil {
			out = next
		}
	}
	return out
}

func sanitizeAnthropicMimicCapabilities(body []byte, finalBeta string, _ anthropicMimicEndpoint) []byte {
	out := body

	if !anthropicBetaTokensContains(finalBeta, claude.BetaContextManagement) {
		out = deleteAnthropicMimicPaths(out, []string{"context_management"})
	}

	if !anthropicBetaTokensContains(finalBeta, claude.BetaEffort) {
		out = deleteAnthropicMimicPaths(out, []string{"output_config.effort"})
		out = deleteEmptyAnthropicObject(out, "output_config")
	}
	if !anthropicBetaTokensContains(finalBeta, claude.BetaFastMode) {
		out = deleteAnthropicMimicPaths(out, []string{"speed"})
	}

	if !anthropicBetaTokensContains(finalBeta, anthropicBetaMCPClient) {
		out = deleteAnthropicMimicPaths(out, []string{"mcp_servers"})
	}

	return sanitizeAnthropicMimicCacheControlCapabilities(out, finalBeta)
}

func deleteEmptyAnthropicObject(body []byte, path string) []byte {
	value := gjson.GetBytes(body, path)
	if !value.IsObject() || len(value.Map()) != 0 {
		return body
	}
	if next, err := sjson.DeleteBytes(body, path); err == nil {
		return next
	}
	return body
}

func collectAnthropicCacheControlPaths(body []byte) []string {
	paths := make([]string, 0, maxCacheControlBlocks+2)
	if gjson.GetBytes(body, "cache_control").Exists() {
		paths = append(paths, "cache_control")
	}

	if system := gjson.GetBytes(body, "system"); system.IsArray() {
		for i, block := range system.Array() {
			if block.Get("cache_control").Exists() {
				paths = append(paths, fmt.Sprintf("system.%d.cache_control", i))
			}
		}
	}

	if messages := gjson.GetBytes(body, "messages"); messages.IsArray() {
		for mi, message := range messages.Array() {
			content := message.Get("content")
			if !content.IsArray() {
				continue
			}
			for ci, block := range content.Array() {
				if block.Get("cache_control").Exists() {
					paths = append(paths, fmt.Sprintf("messages.%d.content.%d.cache_control", mi, ci))
				}
			}
		}
	}

	if tools := gjson.GetBytes(body, "tools"); tools.IsArray() {
		for i, tool := range tools.Array() {
			if tool.Get("cache_control").Exists() {
				paths = append(paths, fmt.Sprintf("tools.%d.cache_control", i))
			}
		}
	}
	return paths
}

func sanitizeAnthropicMimicCacheControlCapabilities(body []byte, finalBeta string) []byte {
	allowScope := anthropicBetaTokensContains(finalBeta, claude.BetaPromptCachingScope)
	allowTTL1h := anthropicBetaTokensContains(finalBeta, claude.BetaExtendedCacheTTL)

	out := body
	for _, path := range collectAnthropicCacheControlPaths(body) {
		value := gjson.GetBytes(out, path)
		if !value.Exists() {
			continue
		}
		if !isValidAnthropicMimicCacheControl(value) {
			out = deleteAnthropicMimicPaths(out, []string{path})
			continue
		}
		if !allowScope && value.Get("scope").Exists() {
			out = deleteAnthropicMimicPaths(out, []string{path + ".scope"})
		}
		if !allowTTL1h && value.Get("ttl").String() == "1h" {
			out = deleteAnthropicMimicPaths(out, []string{path + ".ttl"})
		}
	}
	return out
}

func isValidAnthropicMimicCacheControl(value gjson.Result) bool {
	if !value.IsObject() {
		return false
	}
	cacheType := value.Get("type")
	if cacheType.Type != gjson.String || cacheType.String() != "ephemeral" {
		return false
	}
	ttl := value.Get("ttl")
	if !ttl.Exists() {
		return true
	}
	if ttl.Type != gjson.String {
		return false
	}
	return ttl.String() == "5m" || ttl.String() == "1h"
}

func sanitizeAnthropicMimicTools(body []byte, finalBeta string, _ anthropicMimicEndpoint) []byte {
	out, removedNames, removedTypes, droppedNestedTool := normalizeAnthropicMimicOpenAIFunctionTools(body)
	out = normalizeAnthropicMimicOpenAIToolChoice(out)
	out = sanitizeAnthropicMimicAdvancedToolFields(out, finalBeta)
	removeMCPHistory := !anthropicBetaTokensContains(finalBeta, anthropicBetaMCPClient)
	removedServerNames := make(map[string]struct{})
	tools := gjson.GetBytes(out, "tools")
	if !tools.IsArray() {
		out = deleteAnthropicMimicPaths(out, []string{"tool_choice"})
		return sanitizeAnthropicMimicToolHistory(out, removedNames, removedServerNames, removeMCPHistory)
	}

	kept := make([]json.RawMessage, 0, len(tools.Array()))
	keptNames := make(map[string]struct{})
	keptServerNames := make(map[string]struct{})
	keptTypes := make(map[string]struct{})
	removedTool := false
	for _, tool := range tools.Array() {
		if shouldRemoveAnthropicMimicTool(tool, finalBeta) || !isValidAnthropicMimicOrdinaryTool(tool) {
			removedTool = true
			if name := anthropicMimicToolReferenceName(tool); name != "" {
				removedNames[name] = struct{}{}
				if isAnthropicMimicServerTool(tool) {
					removedServerNames[name] = struct{}{}
				}
			}
			if toolType := strings.TrimSpace(tool.Get("type").String()); toolType != "" {
				removedTypes[toolType] = struct{}{}
				if toolType == "mcp" || toolType == "mcp_toolset" || strings.HasPrefix(toolType, "mcp_") {
					removeMCPHistory = true
				}
			}
			continue
		}
		kept = append(kept, json.RawMessage(tool.Raw))
		if name := anthropicMimicToolReferenceName(tool); name != "" {
			keptNames[name] = struct{}{}
			if isAnthropicMimicServerTool(tool) {
				keptServerNames[name] = struct{}{}
			}
		}
		if toolType := strings.TrimSpace(tool.Get("type").String()); toolType != "" {
			keptTypes[toolType] = struct{}{}
		}
	}

	if !removedTool && !droppedNestedTool && len(removedNames) == 0 && len(removedTypes) == 0 && !removeMCPHistory &&
		(len(kept) != 0 || !gjson.GetBytes(out, "tool_choice").Exists()) {
		return out
	}
	// 同名的普通工具仍然存在时，历史 tool_use/tool_result 不能仅凭名称删除。
	for name := range keptNames {
		delete(removedNames, name)
	}
	// custom 工具与被删 server tool 同名时，只能保留普通 tool_use；只有仍存在的
	// 同名 server tool 才能让历史 server_tool_use 继续有效。
	for name := range keptServerNames {
		delete(removedServerNames, name)
	}
	for toolType := range keptTypes {
		delete(removedTypes, toolType)
	}
	if removedTool {
		if encoded, err := json.Marshal(kept); err == nil {
			if next, setErr := sjson.SetRawBytes(out, "tools", encoded); setErr == nil {
				out = next
			}
		}
	}

	out = sanitizeAnthropicMimicToolChoice(out, removedNames, removedTypes, len(kept) == 0)
	return sanitizeAnthropicMimicToolHistory(out, removedNames, removedServerNames, removeMCPHistory)
}

func normalizeAnthropicMimicOpenAIToolChoice(body []byte) []byte {
	choice := gjson.GetBytes(body, "tool_choice")
	if choice.Type == gjson.String {
		choiceType := ""
		switch strings.ToLower(strings.TrimSpace(choice.String())) {
		case "auto":
			choiceType = "auto"
		case "required":
			choiceType = "any"
		case "none":
			choiceType = "none"
		default:
			return deleteAnthropicMimicPaths(body, []string{"tool_choice"})
		}
		encoded, err := json.Marshal(map[string]string{"type": choiceType})
		if err != nil {
			return body
		}
		if next, err := sjson.SetRawBytes(body, "tool_choice", encoded); err == nil {
			return next
		}
		return body
	}
	if !choice.IsObject() || choice.Get("type").String() != "function" || !choice.Get("function").Exists() {
		return body
	}
	name := strings.TrimSpace(choice.Get("function.name").String())
	if name == "" {
		return deleteAnthropicMimicPaths(body, []string{"tool_choice"})
	}
	normalized := map[string]any{"type": "tool", "name": name}
	if disableParallel := choice.Get("disable_parallel_tool_use"); disableParallel.Type == gjson.True || disableParallel.Type == gjson.False {
		normalized["disable_parallel_tool_use"] = disableParallel.Bool()
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return body
	}
	if next, err := sjson.SetRawBytes(body, "tool_choice", encoded); err == nil {
		return next
	}
	return body
}

func isValidAnthropicMimicOrdinaryTool(tool gjson.Result) bool {
	typeResult := tool.Get("type")
	if typeResult.Exists() && typeResult.Type != gjson.String {
		return false
	}
	toolType := strings.TrimSpace(typeResult.String())
	if toolType != "" && toolType != "custom" && toolType != "function" {
		return true
	}
	name := tool.Get("name")
	return name.Type == gjson.String && strings.TrimSpace(name.String()) != "" && tool.Get("input_schema").IsObject()
}

func isAnthropicMimicServerTool(tool gjson.Result) bool {
	toolType := strings.TrimSpace(tool.Get("type").String())
	return toolType != "" && toolType != "custom" && toolType != "function"
}

// normalizeAnthropicMimicOpenAIFunctionTools 将 OpenAI 的嵌套 function 工具
// 扁平化为 Anthropic 工具。只复制两边语义明确对应的字段，并通过 encoding/json
// 的稳定键排序保证同一工具跨轮次生成完全一致的缓存前缀。
func normalizeAnthropicMimicOpenAIFunctionTools(body []byte) ([]byte, map[string]struct{}, map[string]struct{}, bool) {
	removedNames := make(map[string]struct{})
	removedTypes := make(map[string]struct{})
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return body, removedNames, removedTypes, false
	}

	changed := false
	dropped := false
	kept := make([]json.RawMessage, 0, len(tools.Array()))
	for _, tool := range tools.Array() {
		if tool.Get("type").String() != "function" || !tool.Get("function").Exists() {
			kept = append(kept, json.RawMessage(tool.Raw))
			continue
		}
		changed = true
		flattened, name, ok := flattenAnthropicMimicOpenAIFunctionTool(tool)
		if !ok {
			dropped = true
			if name == "" {
				name = strings.TrimSpace(tool.Get("name").String())
			}
			if name != "" {
				removedNames[name] = struct{}{}
			}
			removedTypes["function"] = struct{}{}
			continue
		}
		kept = append(kept, flattened)
	}
	if !changed {
		return body, removedNames, removedTypes, false
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return body, map[string]struct{}{}, map[string]struct{}{}, false
	}
	next, err := sjson.SetRawBytes(body, "tools", encoded)
	if err != nil {
		return body, map[string]struct{}{}, map[string]struct{}{}, false
	}
	return next, removedNames, removedTypes, dropped
}

func flattenAnthropicMimicOpenAIFunctionTool(tool gjson.Result) (json.RawMessage, string, bool) {
	function := tool.Get("function")
	nameResult := function.Get("name")
	name := strings.TrimSpace(nameResult.String())
	parameters := function.Get("parameters")
	if nameResult.Type != gjson.String || name == "" || !parameters.IsObject() {
		return nil, name, false
	}

	// map 的 JSON 编码按键排序，避免请求字段原始顺序差异导致缓存前缀漂移。
	inputSchema, ok := normalizeAnthropicMimicFunctionInputSchema(parameters)
	if !ok {
		return nil, name, false
	}
	flattened := map[string]json.RawMessage{"input_schema": inputSchema}
	nameJSON, err := json.Marshal(name)
	if err != nil {
		return nil, name, false
	}
	flattened["name"] = nameJSON

	copyRaw := func(target string, candidates ...gjson.Result) {
		for _, candidate := range candidates {
			if candidate.Exists() {
				flattened[target] = json.RawMessage(candidate.Raw)
				return
			}
		}
	}
	if description := function.Get("description"); description.Type == gjson.String {
		copyRaw("description", description)
	}
	if strict := function.Get("strict"); strict.Type == gjson.True || strict.Type == gjson.False {
		copyRaw("strict", strict)
	}
	copyRaw("cache_control", tool.Get("cache_control"), function.Get("cache_control"))
	copyRaw("allowed_callers", tool.Get("allowed_callers"), function.Get("allowed_callers"))
	copyRaw("defer_loading", tool.Get("defer_loading"), function.Get("defer_loading"))
	copyRaw("eager_input_streaming", tool.Get("eager_input_streaming"), function.Get("eager_input_streaming"))
	copyRaw("input_examples", tool.Get("input_examples"), function.Get("input_examples"))

	encoded, err := json.Marshal(flattened)
	if err != nil {
		return nil, name, false
	}
	return json.RawMessage(encoded), name, true
}

func normalizeAnthropicMimicFunctionInputSchema(parameters gjson.Result) (json.RawMessage, bool) {
	var schema map[string]json.RawMessage
	if err := json.Unmarshal([]byte(parameters.Raw), &schema); err != nil {
		return nil, false
	}
	typeRaw, hasType := schema["type"]
	if !hasType || string(typeRaw) == "null" {
		schema["type"] = json.RawMessage(`"object"`)
	} else {
		var schemaType string
		if err := json.Unmarshal(typeRaw, &schemaType); err != nil || schemaType != "object" {
			return nil, false
		}
	}
	if _, hasProperties := schema["properties"]; !hasProperties {
		schema["properties"] = json.RawMessage(`{}`)
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(encoded), true
}

func sanitizeAnthropicMimicAdvancedToolFields(body []byte, finalBeta string) []byte {
	allowAdvanced := anthropicBetaTokensContains(finalBeta, anthropicBetaAdvancedToolUse)
	allowEagerInput := anthropicBetaTokensContains(finalBeta, claude.BetaFineGrainedToolStreaming)
	if allowAdvanced && allowEagerInput {
		return body
	}

	out := body
	tools := gjson.GetBytes(body, "tools")
	if !tools.IsArray() {
		return body
	}
	for i := range tools.Array() {
		base := fmt.Sprintf("tools.%d.", i)
		if !allowAdvanced {
			out = deleteAnthropicMimicPaths(out, []string{
				base + "allowed_callers",
				base + "defer_loading",
				base + "input_examples",
				base + "custom.defer_loading",
				base + "function.allowed_callers",
				base + "function.defer_loading",
				base + "function.input_examples",
			})
			out = deleteEmptyAnthropicObject(out, base+"custom")
		}
		if !allowEagerInput {
			out = deleteAnthropicMimicPaths(out, []string{
				base + "eager_input_streaming",
				base + "function.eager_input_streaming",
			})
		}
	}
	return out
}

func shouldRemoveAnthropicMimicTool(tool gjson.Result, finalBeta string) bool {
	toolType := strings.TrimSpace(tool.Get("type").String())
	if _, unsupported := anthropicMimicUnsupportedOpenAIToolTypes[toolType]; unsupported {
		return true
	}

	if requiredBeta, isComputer := anthropicComputerToolBetas[toolType]; isComputer {
		return !anthropicBetaTokensContains(finalBeta, requiredBeta)
	}
	if strings.HasPrefix(toolType, "computer_") {
		// 未识别版本没有可安全补齐的 beta token，按不支持的 server tool 处理。
		return true
	}
	if toolType == "tool_search_tool_regex_20251119" || toolType == "tool_search_tool_bm25_20251119" {
		return !anthropicBetaTokensContains(finalBeta, anthropicBetaAdvancedToolUse)
	}
	if toolType == "mcp_toolset" || strings.HasPrefix(toolType, "mcp_") {
		return !anthropicBetaTokensContains(finalBeta, anthropicBetaMCPClient)
	}
	return false
}

func anthropicMimicToolReferenceName(tool gjson.Result) string {
	if name := strings.TrimSpace(tool.Get("name").String()); name != "" {
		return name
	}
	toolType := strings.TrimSpace(tool.Get("type").String())
	switch {
	case strings.HasPrefix(toolType, "computer_"):
		return "computer"
	case strings.HasPrefix(toolType, "tool_search_tool_"):
		return "tool_search"
	case toolType == "mcp_toolset" || strings.HasPrefix(toolType, "mcp_"):
		return "mcp"
	default:
		return toolType
	}
}

func sanitizeAnthropicMimicToolChoice(
	body []byte,
	removedNames map[string]struct{},
	removedTypes map[string]struct{},
	noTools bool,
) []byte {
	choice := gjson.GetBytes(body, "tool_choice")
	if !choice.Exists() {
		return body
	}
	if noTools {
		return deleteAnthropicMimicPaths(body, []string{"tool_choice"})
	}

	remove := false
	if choice.Type == gjson.String {
		_, remove = removedNames[choice.String()]
		if !remove {
			_, remove = removedTypes[choice.String()]
		}
	} else if choice.IsObject() {
		if name := strings.TrimSpace(choice.Get("name").String()); name != "" {
			_, remove = removedNames[name]
		}
		if !remove {
			if name := strings.TrimSpace(choice.Get("function.name").String()); name != "" {
				_, remove = removedNames[name]
			}
		}
		if !remove {
			_, remove = removedTypes[strings.TrimSpace(choice.Get("type").String())]
		}
	}
	if remove {
		return deleteAnthropicMimicPaths(body, []string{"tool_choice"})
	}
	return body
}

func sanitizeAnthropicMimicToolHistory(
	body []byte,
	removedNames map[string]struct{},
	removedServerNames map[string]struct{},
	removeMCP bool,
) []byte {
	if len(removedNames) == 0 && len(removedServerNames) == 0 && !removeMCP {
		return body
	}
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return body
	}

	removedUseIDs := make(map[string]struct{})
	for _, message := range messages.Array() {
		content := message.Get("content")
		if !content.IsArray() {
			continue
		}
		for _, block := range content.Array() {
			blockType := block.Get("type").String()
			if blockType == "mcp_tool_use" && removeMCP {
				if id := strings.TrimSpace(block.Get("id").String()); id != "" {
					removedUseIDs[id] = struct{}{}
				}
				continue
			}
			if blockType != "tool_use" && blockType != "server_tool_use" {
				continue
			}
			name := strings.TrimSpace(block.Get("name").String())
			_, removed := removedNames[name]
			if blockType == "server_tool_use" && !removed {
				_, removed = removedServerNames[name]
			}
			if !removed {
				continue
			}
			if id := strings.TrimSpace(block.Get("id").String()); id != "" {
				removedUseIDs[id] = struct{}{}
			}
		}
	}

	keptMessages := make([]json.RawMessage, 0, len(messages.Array()))
	modified := false
	for _, message := range messages.Array() {
		content := message.Get("content")
		if !content.IsArray() {
			keptMessages = append(keptMessages, json.RawMessage(message.Raw))
			continue
		}

		keptBlocks := make([]json.RawMessage, 0, len(content.Array()))
		for _, block := range content.Array() {
			blockType := block.Get("type").String()
			remove := false
			if blockType == "mcp_tool_use" && removeMCP {
				remove = true
			} else if blockType == "tool_use" || blockType == "server_tool_use" {
				name := strings.TrimSpace(block.Get("name").String())
				_, remove = removedNames[name]
				if blockType == "server_tool_use" && !remove {
					_, remove = removedServerNames[name]
				}
			} else if blockType == "mcp_tool_result" && removeMCP {
				remove = true
			} else if blockType == "tool_result" || strings.HasSuffix(blockType, "_tool_result") {
				_, remove = removedUseIDs[strings.TrimSpace(block.Get("tool_use_id").String())]
			}
			if !remove {
				keptBlocks = append(keptBlocks, json.RawMessage(block.Raw))
			} else {
				modified = true
			}
		}

		// 删除只包含已移除工具调用/结果的空消息，避免向上游发送 content:[]。
		if len(keptBlocks) == 0 && len(content.Array()) != 0 {
			continue
		}
		messageRaw := []byte(message.Raw)
		if encoded, err := json.Marshal(keptBlocks); err == nil {
			if next, setErr := sjson.SetRawBytes(messageRaw, "content", encoded); setErr == nil {
				messageRaw = next
			}
		}
		keptMessages = append(keptMessages, json.RawMessage(messageRaw))
	}
	if !modified {
		return body
	}

	encoded, err := json.Marshal(keptMessages)
	if err != nil {
		return body
	}
	if next, setErr := sjson.SetRawBytes(body, "messages", encoded); setErr == nil {
		return next
	}
	return body
}
