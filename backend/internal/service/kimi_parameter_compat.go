package service

import (
	"bytes"
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type kimiCompatRule int

const (
	kimiCompatSampling kimiCompatRule = iota
	kimiCompatReasoning
	kimiCompatToolChoice
	kimiCompatBudget
	kimiCompatThinkingType
	kimiCompatRuleCount
)

var kimiSamplingFields = [...]string{"temperature", "top_p", "top_k", "presence_penalty", "frequency_penalty"}

var (
	kimiErrorPrefix         = regexp.MustCompile(`(?i)^(?:[a-z_*][a-z0-9_*]*\.)*InvalidParameter:\s*`)
	kimiUnsupportedValue    = regexp.MustCompile("(?i)^Parameter ['\"`]?([a-z_]+)['\"`]?=([^\\s]+) is not supported for [a-z0-9._/-]+ model[.!]?$")
	kimiFixedValue          = regexp.MustCompile(`(?i)^field ([a-z_]+) invalid, only (-?[0-9]+(?:\.[0-9]+)?) is allowed for this model[.!]?$`)
	kimiInvalidType         = regexp.MustCompile("(?i)^['\"`]?([a-z_]+)['\"`]? must be (float|integer|number|a number|an integer)[.!]?$")
	kimiInvalidRange        = regexp.MustCompile(`(?i)^([a-z_]+) should be in ([\[(])(-?[0-9]+(?:\.[0-9]+)?),\s*(-?[0-9]+(?:\.[0-9]+)?)([\])])[.!]?$`)
	kimiUnsupportedField    = regexp.MustCompile("(?i)^Unsupported parameter: ['\"`]([a-z_]+)['\"`][.!]?$")
	kimiUnsupportedLevel    = regexp.MustCompile("(?i)^level ['\"`]([^'\"`]+)['\"`] not supported, valid levels: ([a-z_, ]+)[.!]?$")
	kimiInvalidEffort       = regexp.MustCompile("(?i)^Invalid value for ['\"`]?reasoning_effort['\"`]?: ['\"`]([^'\"`]+)['\"`][.!]?(?: (?:Supported values are|Valid values): ([a-z_'\"`, ]+)[.!]?)?$")
	kimiInvalidToolChoice   = regexp.MustCompile("^Invalid value for [`\"']?tool_choice[`\"']?: ([^!\\r\\n]+)! Only named tools, \"none\", \"auto\" or \"required\" are supported\\.$")
	kimiMissingFunction     = regexp.MustCompile("^Expected field [`\"']function[`\"'] in [`\"']tool_choice[`\"'](?:\\. Correct usage: `?(\\{[^\\r\\n]+\\})`?)?\\.?$")
	kimiBudgetConflict      = regexp.MustCompile(`(?i)^max_completion_tokens\s*(?:\[\s*([0-9]+)\s*\])? must be (?:greater|larger) than thinking_budget\s*(?:\[\s*([0-9]+)\s*\])?[.!]?$`)
	kimiInvalidThinkingType = regexp.MustCompile("^['\"`]?type['\"`]? must be in \\[\\s*\"enabled\"\\s*,\\s*\"disabled\"\\s*,\\s*\"auto\"\\s*\\][.!]?$")
)

func (rule kimiCompatRule) String() string {
	switch rule {
	case kimiCompatSampling:
		return "sampling_parameters"
	case kimiCompatReasoning:
		return "reasoning_effort"
	case kimiCompatToolChoice:
		return "tool_choice"
	case kimiCompatBudget:
		return "max_completion_tokens"
	case kimiCompatThinkingType:
		return "thinking_type"
	default:
		return ""
	}
}

func (rule kimiCompatRule) enabled(settings GatewaySettings) bool {
	switch rule {
	case kimiCompatSampling:
		return settings.KimiSamplingParameterRetryEnabled
	case kimiCompatReasoning:
		return settings.KimiReasoningEffortRetryEnabled
	case kimiCompatToolChoice:
		return settings.KimiToolChoiceRetryEnabled
	case kimiCompatBudget:
		return settings.KimiMaxCompletionTokensRetryEnabled
	case kimiCompatThinkingType:
		return settings.KimiThinkingTypeRetryEnabled
	default:
		return false
	}
}

type kimiParameterError struct {
	Message string `json:"message"`
	Param   string `json:"param"`
	Code    string `json:"code"`
	Type    string `json:"type"`
}

func parseKimiParameterError(body []byte) (kimiParameterError, bool) {
	if !kimiUniqueJSONObject(body) {
		return kimiParameterError{}, false
	}
	var envelope struct {
		Error json.RawMessage `json:"error"`
		kimiParameterError
	}
	if json.Unmarshal(body, &envelope) != nil {
		return kimiParameterError{}, false
	}
	rejection := envelope.kimiParameterError
	if len(envelope.Error) > 0 {
		if !kimiUniqueJSONObject(envelope.Error) {
			return kimiParameterError{}, false
		}
		rejection = kimiParameterError{}
		if json.Unmarshal(envelope.Error, &rejection) != nil {
			return kimiParameterError{}, false
		}
	} else if rejection.Code == "" {
		return kimiParameterError{}, false
	}
	for _, label := range []string{rejection.Code, rejection.Type} {
		switch strings.ToLower(label) {
		case "", "invalid_parameter_error", "invalid_request_error", "invalidparameter", "unsupported_parameter", "unsupported_value", "invalid_value":
		default:
			return kimiParameterError{}, false
		}
	}
	rejection.Message = strings.TrimSpace(rejection.Message)
	rejection.Message = strings.TrimSpace(strings.TrimPrefix(rejection.Message, "<400>"))
	rejection.Message = kimiErrorPrefix.ReplaceAllString(rejection.Message, "")
	return rejection, rejection.Message != ""
}

func kimiUniqueJSONObject(body []byte) bool {
	if !gjson.ValidBytes(body) {
		return false
	}
	object := gjson.ParseBytes(body)
	if !object.IsObject() {
		return false
	}
	seen := make(map[string]bool)
	unique := true
	object.ForEach(func(key, _ gjson.Result) bool {
		name := strings.ToLower(key.String())
		if seen[name] {
			unique = false
			return false
		}
		seen[name] = true
		return true
	})
	return unique
}

func kimiParameterName(name string) string {
	switch strings.ToLower(name) {
	case "temperature":
		return "temperature"
	case "topp", "top_p":
		return "top_p"
	case "topk", "top_k":
		return "top_k"
	case "presencepenalty", "presence_penalty":
		return "presence_penalty"
	case "frequencypenalty", "frequency_penalty":
		return "frequency_penalty"
	case "reasoningeffort", "reasoning_effort":
		return "reasoning_effort"
	case "toolchoice", "tool_choice":
		return "tool_choice"
	case "maxcompletiontokens", "max_completion_tokens":
		return "max_completion_tokens"
	default:
		return ""
	}
}

func kimiRejectedValueMatches(value gjson.Result, rejected string) bool {
	if !value.Exists() {
		return false
	}
	rejected = strings.Trim(rejected, "'\"`")
	if value.Type == gjson.Number {
		parsed, err := strconv.ParseFloat(rejected, 64)
		return err == nil && value.Float() == parsed
	}
	return value.Type == gjson.String && value.String() == rejected
}

func kimiAllowedLow(values string) bool {
	if values == "" {
		return true
	}
	for _, value := range strings.FieldsFunc(values, func(character rune) bool {
		return character == ',' || character == ' ' || character == '\'' || character == '"' || character == '`'
	}) {
		if value == "low" {
			return true
		}
	}
	return false
}

func matchKimiParameterRejection(requestBody, errorBody []byte) (kimiCompatRule, bool) {
	if !kimiUniqueJSONObject(requestBody) {
		return 0, false
	}
	rejection, ok := parseKimiParameterError(errorBody)
	if !ok {
		return 0, false
	}
	fieldMatches := func(field string) bool {
		return field != "" && gjson.GetBytes(requestBody, field).Exists() &&
			(rejection.Param == "" || kimiParameterName(rejection.Param) == field)
	}
	message := rejection.Message
	if kimiInvalidThinkingType.MatchString(message) {
		switch rejection.Param {
		case "", "type", "thinking.type":
		default:
			return 0, false
		}
		thinking := gjson.GetBytes(requestBody, "thinking")
		if !kimiUniqueJSONObject([]byte(thinking.Raw)) {
			return 0, false
		}
		thinkingType := thinking.Get("type")
		if !thinkingType.Exists() {
			return 0, false
		}
		if thinkingType.Type == gjson.String {
			switch thinkingType.String() {
			case "enabled", "disabled", "auto":
				return 0, false
			}
		}
		return kimiCompatThinkingType, true
	}
	for _, pattern := range []*regexp.Regexp{kimiUnsupportedValue, kimiFixedValue, kimiInvalidType, kimiInvalidRange, kimiUnsupportedField} {
		matched := pattern.FindStringSubmatch(message)
		if matched == nil {
			continue
		}
		field := kimiParameterName(matched[1])
		if !fieldMatches(field) || (pattern == kimiUnsupportedValue && !kimiRejectedValueMatches(gjson.GetBytes(requestBody, field), matched[2])) {
			return 0, false
		}
		value := gjson.GetBytes(requestBody, field)
		if pattern == kimiFixedValue && kimiRejectedValueMatches(value, matched[2]) {
			return 0, false
		}
		if pattern == kimiInvalidType && value.Type == gjson.Number {
			integerExpected := strings.Contains(strings.ToLower(matched[2]), "integer")
			if !integerExpected || math.Trunc(value.Float()) == value.Float() {
				return 0, false
			}
		}
		if pattern == kimiInvalidRange {
			minimum, _ := strconv.ParseFloat(matched[3], 64)
			maximum, _ := strconv.ParseFloat(matched[4], 64)
			if value.Type != gjson.Number || minimum >= maximum {
				return 0, false
			}
			aboveMinimum := value.Float() > minimum || (matched[2] == "[" && value.Float() == minimum)
			belowMaximum := value.Float() < maximum || (matched[5] == "]" && value.Float() == maximum)
			if aboveMinimum && belowMaximum {
				return 0, false
			}
		}
		for _, samplingField := range kimiSamplingFields {
			if field == samplingField {
				return kimiCompatSampling, true
			}
		}
		if field == "reasoning_effort" && pattern == kimiUnsupportedValue && gjson.GetBytes(requestBody, field).String() != "low" {
			return kimiCompatReasoning, true
		}
		return 0, false
	}
	for _, pattern := range []*regexp.Regexp{kimiUnsupportedLevel, kimiInvalidEffort} {
		matched := pattern.FindStringSubmatch(message)
		if matched != nil && fieldMatches("reasoning_effort") {
			effort := gjson.GetBytes(requestBody, "reasoning_effort")
			return kimiCompatReasoning, effort.String() != "low" && kimiRejectedValueMatches(effort, matched[1]) && kimiAllowedLow(matched[2])
		}
	}
	if matched := kimiInvalidToolChoice.FindStringSubmatch(message); matched != nil && fieldMatches("tool_choice") {
		choice := gjson.GetBytes(requestBody, "tool_choice")
		switch choice.String() {
		case "auto", "none", "required":
			return 0, false
		}
		return kimiCompatToolChoice, kimiRejectedValueMatches(choice, matched[1])
	}
	if matched := kimiMissingFunction.FindStringSubmatch(message); matched != nil && fieldMatches("tool_choice") {
		choice := gjson.GetBytes(requestBody, "tool_choice")
		if matched[1] != "" && !gjson.Valid(matched[1]) {
			return 0, false
		}
		return kimiCompatToolChoice, choice.IsObject() && !choice.Get("function").IsObject()
	}
	if matched := kimiBudgetConflict.FindStringSubmatch(message); matched != nil && fieldMatches("max_completion_tokens") {
		if gjson.GetBytes(requestBody, "max_completion_tokens").Type != gjson.Number {
			return 0, false
		}
		if matched[1] != "" && !kimiRejectedValueMatches(gjson.GetBytes(requestBody, "max_completion_tokens"), matched[1]) {
			return 0, false
		}
		if budget := gjson.GetBytes(requestBody, "thinking_budget"); budget.Exists() {
			if budget.Type != gjson.Number || gjson.GetBytes(requestBody, "max_completion_tokens").Float() > budget.Float() ||
				(matched[2] != "" && !kimiRejectedValueMatches(budget, matched[2])) {
				return 0, false
			}
		}
		if matched[1] != "" && matched[2] != "" {
			maximum, _ := strconv.ParseFloat(matched[1], 64)
			budget, _ := strconv.ParseFloat(matched[2], 64)
			if maximum > budget {
				return 0, false
			}
		}
		return kimiCompatBudget, true
	}
	return 0, false
}

func applyKimiParameterRepair(body []byte, rule kimiCompatRule) ([]byte, []string, error) {
	fields := []string{rule.String()}
	switch rule {
	case kimiCompatSampling:
		fields = kimiSamplingFields[:]
	case kimiCompatThinkingType:
		fields = []string{"thinking"}
	}
	modified := body
	var changed []string
	for _, field := range fields {
		if !gjson.GetBytes(modified, field).Exists() && rule != kimiCompatReasoning {
			continue
		}
		var next []byte
		var err error
		if rule == kimiCompatReasoning {
			next, err = sjson.SetBytes(modified, field, "low")
		} else {
			next, err = sjson.DeleteBytes(modified, field)
		}
		if err != nil {
			return body, nil, err
		}
		if !bytes.Equal(next, modified) {
			changed = append(changed, field)
			modified = next
		}
	}
	return modified, changed, nil
}
