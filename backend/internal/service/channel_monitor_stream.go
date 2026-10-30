package service

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tidwall/gjson"
)

// readMonitorEventStream 只保留一个有界事件及回答文本；大型元数据处理后立即丢弃。
// HTTP client / 请求 context 继续负责既有超时与取消，成功必须读到协议终态。
func readMonitorEventStream(body io.Reader, provider, apiMode string) (string, error) {
	limited := &io.LimitedReader{R: body, N: monitorStreamMaxBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 4096), monitorStreamEventMaxBytes+2)
	state := monitorStreamState{provider: provider, apiMode: apiMode}
	var data strings.Builder
	eventName, eventBytes := "", 0
	consume := func() (bool, error) {
		payload := strings.TrimSpace(data.String())
		if payload == "" {
			return false, nil
		}
		return state.consume(eventName, payload)
	}
	for scanner.Scan() {
		if limited.N == 0 {
			return "", fmt.Errorf("monitor stream exceeds %d bytes", monitorStreamMaxBytes)
		}
		line := scanner.Text()
		if line == "" {
			done, err := consume()
			if err != nil {
				return "", err
			}
			if done {
				return state.text.String(), nil
			}
			data.Reset()
			eventName, eventBytes = "", 0
			continue
		}
		eventBytes += len(line) + 1
		if eventBytes > monitorStreamEventMaxBytes {
			return "", fmt.Errorf("monitor SSE event exceeds %d bytes", monitorStreamEventMaxBytes)
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			eventName = value
		case "data":
			data.WriteString(value)
			data.WriteByte('\n')
		}
	}
	if limited.N == 0 {
		return "", fmt.Errorf("monitor stream exceeds %d bytes", monitorStreamMaxBytes)
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return "", fmt.Errorf("monitor SSE event exceeds %d bytes", monitorStreamEventMaxBytes)
		}
		return "", fmt.Errorf("read monitor stream: %w", err)
	}
	// 兼容最后一个终态事件没有空行分隔符，但不能接受缺失终态的 EOF。
	if done, err := consume(); err != nil {
		return "", err
	} else if done {
		return state.text.String(), nil
	}
	if state.chatFinished {
		return state.text.String(), nil
	}
	return "", fmt.Errorf("monitor stream ended before terminal event: %w", io.ErrUnexpectedEOF)
}

type monitorStreamState struct {
	provider     string
	apiMode      string
	text         strings.Builder
	chatFinished bool
}

func (s *monitorStreamState) appendText(text string) error {
	if len(text) > monitorResponseMaxBytes-s.text.Len() {
		return fmt.Errorf("monitor stream text exceeds %d bytes", monitorResponseMaxBytes)
	}
	s.text.WriteString(text)
	return nil
}

func (s *monitorStreamState) consume(eventName, data string) (bool, error) {
	if strings.TrimSpace(data) == "[DONE]" {
		if s.provider != MonitorProviderAnthropic && s.provider != MonitorProviderGemini && defaultAPIMode(s.apiMode) != MonitorAPIModeResponses {
			return true, nil
		}
		return false, fmt.Errorf("monitor stream received [DONE] before protocol terminal event")
	}
	if !gjson.Valid(data) || !gjson.Parse(data).IsObject() {
		return false, fmt.Errorf("monitor stream contains invalid JSON event")
	}
	event := gjson.Parse(data)
	kind := event.Get("type").String()
	if kind == "" {
		kind = eventName
	}
	if problem := event.Get("error"); kind == "error" || (problem.Exists() && problem.Type != gjson.Null) {
		return false, monitorStreamEventError(kind, event)
	}
	switch s.provider {
	case MonitorProviderOpenAI:
		if defaultAPIMode(s.apiMode) == MonitorAPIModeResponses {
			return s.consumeResponses(kind, event)
		}
	case MonitorProviderAnthropic:
		if kind == "content_block_delta" && event.Get("delta.type").String() == "text_delta" {
			return false, s.appendText(event.Get("delta.text").String())
		}
		if kind == "message_delta" && event.Get("delta.stop_reason").String() == "max_tokens" {
			return false, fmt.Errorf("monitor stream incomplete: max_tokens")
		}
		return kind == "message_stop", nil
	case MonitorProviderGemini:
		if err := s.appendText(strings.Join(extractGeminiMonitorTextParts(event), "")); err != nil {
			return false, err
		}
		if reason := event.Get("candidates.0.finishReason").String(); reason != "" {
			if reason != "STOP" {
				return false, fmt.Errorf("monitor stream incomplete: %s", reason)
			}
			return true, nil
		}
		return false, nil
	}
	// Chat Completions 的 finish_reason 后仍可能有 usage 或错误，继续读取到
	// [DONE]；兼容不发 [DONE]、以正常 EOF 结束的上游。
	for _, choice := range event.Get("choices").Array() {
		if err := s.appendText(choice.Get("delta.content").String()); err != nil {
			return false, err
		}
		if reason := choice.Get("finish_reason").String(); reason != "" {
			if reason != "stop" {
				return false, fmt.Errorf("monitor stream incomplete: %s", reason)
			}
			s.chatFinished = true
		}
	}
	return false, nil
}

func (s *monitorStreamState) consumeResponses(kind string, event gjson.Result) (bool, error) {
	switch kind {
	case "response.failed", "response.incomplete":
		return false, monitorStreamEventError(kind, event)
	case "response.output_text.delta":
		return false, s.appendText(event.Get("delta").String())
	case "response.completed", "response.done":
		response := event.Get("response")
		status := response.Get("status").String()
		problem := response.Get("error")
		if (status != "" && status != "completed") || (problem.Exists() && problem.Type != gjson.Null) {
			return false, monitorStreamEventError(kind, event)
		}
		// 部分兼容上游只在 completed.output 中提供答案，避免与 delta 重复拼接。
		if s.text.Len() == 0 {
			if err := s.appendText(extractOpenAIResponsesText([]byte(response.Raw))); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

func monitorStreamEventError(kind string, event gjson.Result) error {
	detail := ""
	for _, path := range []string{"response.error.message", "error.message", "message", "response.incomplete_details.reason", "response.status"} {
		if detail = event.Get(path).String(); detail != "" {
			break
		}
	}
	return fmt.Errorf("monitor stream %s: %s", kind, truncateMessage(sanitizeErrorMessage(detail)))
}
