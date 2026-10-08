package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var kimiDynamicToolName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]{0,255}$`)

// PrepareKimiDynamicTools 在排队和协议转换前校验原始角色，避免非法声明被转换吞掉。
// 开关沿用请求级配置快照；合并结果幂等，重试和切号不会重复追加工具。
func (s *OpenAIGatewayService) PrepareKimiDynamicTools(ctx context.Context, c *gin.Context, body []byte) ([]byte, error) {
	state := s.kimiParameterCompat(ctx, c)
	if state == nil || !state.settings.KimiDynamicToolsEnabled {
		return body, nil
	}
	return normalizeKimiDynamicTools(body)
}

func normalizeKimiDynamicTools(body []byte) ([]byte, error) {
	if !kimiUniqueJSONObject(body) {
		return nil, fmt.Errorf("请求必须是无重复字段的 JSON 对象")
	}
	root := gjson.ParseBytes(body)
	messages := root.Get("messages")
	if !messages.Exists() {
		return body, nil
	}
	if !messages.IsArray() {
		return nil, fmt.Errorf("messages 必须是数组")
	}

	var dynamic []json.RawMessage
	kept := make([]json.RawMessage, 0, len(messages.Array()))
	names := make(map[string]struct{})
	hasDynamic := false
	for index, message := range messages.Array() {
		path := fmt.Sprintf("messages[%d]", index)
		if !kimiUniqueJSONObject([]byte(message.Raw)) {
			return nil, fmt.Errorf("%s 必须是无重复字段的对象", path)
		}
		role := message.Get("role")
		if role.Type != gjson.String {
			return nil, fmt.Errorf("%s.role 必须是字符串", path)
		}
		if role.String() == "tool" {
			id := message.Get("tool_call_id")
			if id.Type != gjson.String || strings.TrimSpace(id.String()) == "" {
				return nil, fmt.Errorf("%s.tool_call_id 必须是非空字符串", path)
			}
		}
		tools := message.Get("tools")
		if !tools.Exists() {
			kept = append(kept, json.RawMessage(message.Raw))
			continue
		}
		hasDynamic = true
		if role.String() != "system" {
			return nil, fmt.Errorf("%s.tools 只允许声明在 system 消息中", path)
		}
		// KVV 的合法声明使用空字符串；空正文不能按字段存在性误判为非法。
		content := message.Get("content")
		if content.Exists() && content.Type != gjson.Null && (content.Type != gjson.String || content.String() != "") {
			return nil, fmt.Errorf("%s 不能同时携带动态 tools 和非空 content", path)
		}
		parsed, err := validateKimiDynamicToolList(tools, path+".tools", names)
		if err != nil {
			return nil, err
		}
		dynamic = append(dynamic, parsed...)
		// 只删除纯声明消息；扩展元数据保留在原位置，不影响其前后历史顺序。
		var remaining map[string]json.RawMessage
		if err := json.Unmarshal([]byte(message.Raw), &remaining); err != nil {
			return nil, err
		}
		delete(remaining, "tools")
		if len(remaining) > 2 || (len(remaining) == 2 && remaining["content"] == nil) {
			if remaining["content"] == nil || content.Type == gjson.Null {
				remaining["content"] = json.RawMessage(`""`)
			}
			encoded, err := json.Marshal(remaining)
			if err != nil {
				return nil, err
			}
			kept = append(kept, encoded)
		}
	}
	if !hasDynamic {
		return body, nil
	}
	var global []json.RawMessage
	if tools := root.Get("tools"); tools.Exists() {
		var err error
		global, err = validateKimiDynamicToolList(tools, "tools", names)
		if err != nil {
			return nil, err
		}
	}
	if len(kept) == 0 {
		return nil, fmt.Errorf("动态工具声明之外至少需要一条对话消息")
	}
	updated, err := sjson.SetBytes(body, "messages", kept)
	if err != nil {
		return nil, err
	}
	if len(global)+len(dynamic) == 0 {
		return updated, nil
	}
	return sjson.SetBytes(updated, "tools", append(global, dynamic...))
}

func validateKimiDynamicToolList(tools gjson.Result, path string, names map[string]struct{}) ([]json.RawMessage, error) {
	if !tools.IsArray() {
		return nil, fmt.Errorf("%s 必须是数组", path)
	}
	result := make([]json.RawMessage, 0, len(tools.Array()))
	for index, tool := range tools.Array() {
		itemPath := fmt.Sprintf("%s[%d]", path, index)
		if !kimiUniqueJSONObject([]byte(tool.Raw)) || tool.Get("type").String() != "function" {
			return nil, fmt.Errorf("%s 必须是 type=function 的无重复字段对象", itemPath)
		}
		function := tool.Get("function")
		if !kimiUniqueJSONObject([]byte(function.Raw)) {
			return nil, fmt.Errorf("%s.function 必须是无重复字段的对象", itemPath)
		}
		name := function.Get("name")
		if name.Type != gjson.String || !kimiDynamicToolName.MatchString(name.String()) {
			return nil, fmt.Errorf("%s.function.name 必须以字母或下划线开头，且仅含字母、数字、下划线或连字符，长度为1至256", itemPath)
		}
		if _, exists := names[name.String()]; exists {
			return nil, fmt.Errorf("%s.function.name 与其他工具重名", itemPath)
		}
		names[name.String()] = struct{}{}
		if parameters := function.Get("parameters"); parameters.Exists() && !parameters.IsObject() {
			return nil, fmt.Errorf("%s.function.parameters 必须是对象", itemPath)
		}
		if description := function.Get("description"); description.Exists() && description.Type != gjson.String {
			return nil, fmt.Errorf("%s.function.description 必须是字符串", itemPath)
		}
		if strict := function.Get("strict"); strict.Exists() && strict.Type != gjson.True && strict.Type != gjson.False {
			return nil, fmt.Errorf("%s.function.strict 必须是布尔值", itemPath)
		}
		result = append(result, json.RawMessage(tool.Raw))
	}
	return result, nil
}
