package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

const (
	contentModerationLocalAuditCleanupInterval = 5 * time.Minute
	contentModerationLocalAuditCleanupTimeout  = 2 * time.Minute
)

var contentModerationLocalAuditIDPattern = regexp.MustCompile(`^[a-f0-9-]{36}$`)
var errContentModerationLocalAuditSkipped = errors.New("content moderation local audit skipped")

type ContentModerationLocalAuditInput struct {
	RequestID          string
	UserID             int64
	UserEmail          string
	APIKeyID           int64
	APIKeyName         string
	GroupID            *int64
	GroupName          string
	Endpoint           string
	Provider           string
	Model              string
	UpstreamModel      string
	Protocol           string
	SessionID          string
	ClientSessionID    string
	SessionSource      string
	UserAgent          string
	Originator         string
	ResponseID         string
	PreviousResponseID string
	Stream             bool
	Body               []byte
	RawResponse        []byte
	Usage              any
}

type ContentModerationLocalAuditMetadata struct {
	ID                  string    `json:"id"`
	RequestID           string    `json:"request_id"`
	SessionID           string    `json:"session_id"`
	ClientSessionID     string    `json:"client_session_id,omitempty"`
	SessionSource       string    `json:"session_source,omitempty"`
	UserID              *int64    `json:"user_id,omitempty"`
	UserEmail           string    `json:"user_email"`
	APIKeyID            *int64    `json:"api_key_id,omitempty"`
	APIKeyName          string    `json:"api_key_name"`
	GroupID             *int64    `json:"group_id,omitempty"`
	GroupName           string    `json:"group_name"`
	Endpoint            string    `json:"endpoint"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	UpstreamModel       string    `json:"upstream_model,omitempty"`
	Protocol            string    `json:"protocol"`
	ResponseID          string    `json:"response_id,omitempty"`
	PreviousResponseID  string    `json:"previous_response_id,omitempty"`
	Scene               string    `json:"scene,omitempty"`
	SceneSignals        []string  `json:"scene_signals,omitempty"`
	InputToolCallCount  int       `json:"input_tool_call_count"`
	OutputToolCallCount int       `json:"output_tool_call_count"`
	FileReadCount       int       `json:"file_read_count"`
	FileWriteCount      int       `json:"file_write_count"`
	Stream              bool      `json:"stream"`
	SystemPrompt        string    `json:"system_prompt_excerpt"`
	MessageCount        int       `json:"message_count"`
	ToolCount           int       `json:"tool_count"`
	ToolCallCount       int       `json:"tool_call_count"`
	ToolResultCount     int       `json:"tool_result_count"`
	FileSizeBytes       int64     `json:"file_size_bytes"`
	CreatedAt           time.Time `json:"created_at"`
}

type ContentModerationLocalAuditRecord struct {
	ContentModerationLocalAuditMetadata
	UserAgent         string `json:"user_agent,omitempty"`
	Originator        string `json:"originator,omitempty"`
	SystemPrompt      any    `json:"system_prompt,omitempty"`
	Messages          any    `json:"messages,omitempty"`
	Tools             any    `json:"tools,omitempty"`
	ToolCalls         []any  `json:"tool_calls,omitempty"`
	ToolResults       []any  `json:"tool_results,omitempty"`
	AssistantOutput   any    `json:"assistant_output,omitempty"`
	OutputToolCalls   []any  `json:"output_tool_calls,omitempty"`
	OutputToolResults []any  `json:"output_tool_results,omitempty"`
	Usage             any    `json:"usage,omitempty"`
	RawRequest        any    `json:"raw_request,omitempty"`
	RawResponse       any    `json:"raw_response,omitempty"`
}

type localAuditSceneAssessment struct {
	Scene               string
	Signals             []string
	InputToolCallCount  int
	OutputToolCallCount int
	FileReadCount       int
	FileWriteCount      int
}

type ContentModerationLocalAuditStats struct {
	Enabled                   bool       `json:"enabled"`
	StoragePath               string     `json:"storage_path"`
	MaxStorageGB              float64    `json:"max_storage_gb"`
	CaptureMaxConcurrency     int        `json:"capture_max_concurrency"`
	ResponseCaptureLimitBytes int        `json:"response_capture_limit_bytes"`
	RetainedBytes             int64      `json:"retained_bytes"`
	RetainedRecords           int64      `json:"retained_records"`
	QueueSize                 int        `json:"queue_size"`
	QueueLength               int        `json:"queue_length"`
	CaptureActive             int64      `json:"capture_active"`
	OverloadActive            bool       `json:"overload_active"`
	OverloadSkipped           int64      `json:"overload_skipped"`
	Active                    int64      `json:"active"`
	Enqueued                  int64      `json:"enqueued"`
	Dropped                   int64      `json:"dropped"`
	Written                   int64      `json:"written"`
	Errors                    int64      `json:"errors"`
	LastCleanupAt             *time.Time `json:"last_cleanup_at,omitempty"`
	LastCleanupDeleted        int64      `json:"last_cleanup_deleted"`
	LastCleanupDeletedBytes   int64      `json:"last_cleanup_deleted_bytes"`
}

type ContentModerationLocalAuditListFilter struct {
	Pagination pagination.PaginationParams
	GroupID    *int64
	Endpoint   string
	Model      string
	Search     string
	From       *time.Time
	To         *time.Time
}

type contentModerationLocalAuditTask struct {
	input     ContentModerationLocalAuditInput
	config    *ContentModerationConfig
	createdAt time.Time
}

type contentModerationLocalAuditFile struct {
	metaPath   string
	recordPath string
	meta       ContentModerationLocalAuditMetadata
	createdAt  time.Time
	sizeBytes  int64
}

func (s *ContentModerationService) RecordSuccessfulConversation(ctx context.Context, input ContentModerationLocalAuditInput) {
	if s == nil || s.settingRepo == nil || len(input.Body) == 0 {
		return
	}
	if !s.isRiskControlEnabled(ctx) {
		return
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		slog.Warn("content_moderation.local_audit_config_failed", "error", err)
		return
	}
	if !cfg.LocalAuditEnabled {
		return
	}
	if !cfg.includesGroup(input.GroupID) || !cfg.includesModel(input.Model) {
		return
	}
	task := contentModerationLocalAuditTask{
		input:     cloneContentModerationLocalAuditInput(input),
		config:    cloneContentModerationConfig(cfg),
		createdAt: time.Now(),
	}
	select {
	case s.localAuditQueue <- task:
		s.localAuditEnqueued.Add(1)
	default:
		s.localAuditDropped.Add(1)
		slog.Warn("content_moderation.local_audit_queue_full", "request_id", input.RequestID, "endpoint", input.Endpoint)
	}
}

func (s *ContentModerationService) TryBeginLocalAuditCapture(ctx context.Context) (func(), bool) {
	if s == nil || s.settingRepo == nil {
		return func() {}, false
	}
	if !s.isRiskControlEnabled(ctx) {
		return func() {}, false
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		slog.Warn("content_moderation.local_audit_capture_config_failed", "error", err)
		return func() {}, false
	}
	if !cfg.LocalAuditEnabled {
		return func() {}, false
	}
	maxCapture := cfg.LocalAuditMaxCaptureConcurrency
	if maxCapture > 0 {
		for {
			active := s.localAuditCaptureActive.Load()
			if active >= int64(maxCapture) {
				s.localAuditOverloadSkipped.Add(1)
				slog.Warn("content_moderation.local_audit_capture_overload",
					"active", active,
					"capture_max_concurrency", maxCapture,
				)
				return func() {}, false
			}
			if s.localAuditCaptureActive.CompareAndSwap(active, active+1) {
				return s.releaseLocalAuditCapture, true
			}
		}
	}
	s.localAuditCaptureActive.Add(1)
	return s.releaseLocalAuditCapture, true
}

func (s *ContentModerationService) releaseLocalAuditCapture() {
	if s == nil {
		return
	}
	for {
		active := s.localAuditCaptureActive.Load()
		if active <= 0 {
			return
		}
		if s.localAuditCaptureActive.CompareAndSwap(active, active-1) {
			return
		}
	}
}

func (s *ContentModerationService) ListLocalAuditRecords(ctx context.Context, filter ContentModerationLocalAuditListFilter) ([]ContentModerationLocalAuditMetadata, *pagination.PaginationResult, error) {
	if err := s.ensureLocalAuditReadable(ctx); err != nil {
		return nil, nil, err
	}
	params := normalizeLocalAuditPagination(filter.Pagination)
	files, err := s.scanLocalAuditFiles()
	if err != nil {
		return nil, nil, err
	}
	items := make([]ContentModerationLocalAuditMetadata, 0, len(files))
	for _, file := range files {
		if !localAuditMetadataMatches(file.meta, filter) {
			continue
		}
		items = append(items, file.meta)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	total := int64(len(items))
	offset := params.Offset()
	limit := params.Limit()
	if offset >= len(items) {
		items = []ContentModerationLocalAuditMetadata{}
	} else {
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		items = items[offset:end]
	}
	return items, localAuditPaginationResult(total, params), nil
}

func (s *ContentModerationService) GetLocalAuditRecord(ctx context.Context, id string) (*ContentModerationLocalAuditRecord, error) {
	raw, _, err := s.ReadLocalAuditRecordBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	var out ContentModerationLocalAuditRecord
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse local audit record: %w", err)
	}
	return &out, nil
}

func (s *ContentModerationService) ReadLocalAuditRecordBytes(ctx context.Context, id string) ([]byte, string, error) {
	if err := s.ensureLocalAuditReadable(ctx); err != nil {
		return nil, "", err
	}
	file, err := s.findLocalAuditFileByID(id)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(file.recordPath)
	if err != nil {
		return nil, "", fmt.Errorf("read local audit record: %w", err)
	}
	return raw, filepath.Base(file.recordPath), nil
}

func (s *ContentModerationService) DeleteLocalAuditRecord(ctx context.Context, id string) error {
	if err := s.ensureLocalAuditReadable(ctx); err != nil {
		return err
	}
	file, err := s.findLocalAuditFileByID(id)
	if err != nil {
		return err
	}
	if err := os.Remove(file.recordPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local audit record: %w", err)
	}
	if err := os.Remove(file.metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete local audit metadata: %w", err)
	}
	s.refreshLocalAuditStats()
	return nil
}

func (s *ContentModerationService) localAuditWorker() {
	for task := range s.localAuditQueue {
		s.localAuditActive.Add(1)
		func() {
			defer s.localAuditActive.Add(-1)
			if err := s.writeLocalAuditRecord(task); err != nil {
				if errors.Is(err, errContentModerationLocalAuditSkipped) {
					return
				}
				s.localAuditErrors.Add(1)
				slog.Warn("content_moderation.local_audit_write_failed", "request_id", task.input.RequestID, "error", err)
				return
			}
			s.localAuditWritten.Add(1)
			s.runLocalAuditCleanupOnce()
		}()
	}
}

func (s *ContentModerationService) localAuditCleanupWorker() {
	timer := time.NewTimer(contentModerationLocalAuditCleanupInterval)
	defer timer.Stop()
	for {
		<-timer.C
		s.runLocalAuditCleanupOnce()
		timer.Reset(contentModerationLocalAuditCleanupInterval)
	}
}

func (s *ContentModerationService) writeLocalAuditRecord(task contentModerationLocalAuditTask) error {
	cfg := task.config
	if cfg == nil || !cfg.LocalAuditEnabled {
		return nil
	}
	record, err := buildContentModerationLocalAuditRecord(cfg, task.input, task.createdAt)
	if err != nil {
		return err
	}
	if record == nil {
		return errContentModerationLocalAuditSkipped
	}
	dir := filepath.Join(s.localAuditRoot(), record.CreatedAt.Format("2006"), record.CreatedAt.Format("01"), record.CreatedAt.Format("02"))
	recordPath := filepath.Join(dir, record.ID+".json")
	metaPath := filepath.Join(dir, record.ID+".meta.json")
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal local audit record: %w", err)
	}
	record.FileSizeBytes = int64(len(raw))
	meta := record.ContentModerationLocalAuditMetadata
	meta.FileSizeBytes = record.FileSizeBytes
	metaRaw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal local audit metadata: %w", err)
	}
	raw, err = json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal local audit record with size: %w", err)
	}
	if err := writeLocalAuditFileAtomic(recordPath, raw); err != nil {
		return err
	}
	if err := writeLocalAuditFileAtomic(metaPath, metaRaw); err != nil {
		return err
	}
	s.refreshLocalAuditStats()
	return nil
}

func buildContentModerationLocalAuditRecord(cfg *ContentModerationConfig, input ContentModerationLocalAuditInput, createdAt time.Time) (*ContentModerationLocalAuditRecord, error) {
	var decoded any
	if err := json.Unmarshal(input.Body, &decoded); err != nil {
		return nil, fmt.Errorf("parse local audit request: %w", err)
	}
	root, _ := decoded.(map[string]any)
	if shouldSkipContentModerationLocalAuditRecord(cfg, input, root) {
		return nil, nil
	}
	safeRequest := redactSensitiveJSON(decoded)
	systemPrompt, messages, tools, toolCalls, toolResults := extractLocalAuditConversation(input.Protocol, root)
	if input.SessionID == "" {
		input.SessionID = inferLocalAuditSessionID(root)
	}
	if input.ClientSessionID == "" {
		input.ClientSessionID = inferLocalAuditSessionID(root)
	}
	if input.PreviousResponseID == "" {
		input.PreviousResponseID = strings.TrimSpace(fmt.Sprint(firstExistingLocalAuditValue(root, "previous_response_id")))
		if input.PreviousResponseID == "<nil>" {
			input.PreviousResponseID = ""
		}
	}
	safeResponse, assistantOutput, outputToolCalls, outputToolResults := extractLocalAuditResponseArtifacts(input.Protocol, input.RawResponse)
	scene := assessLocalAuditScene(cfg, input, tools, toolCalls, toolResults, outputToolCalls, outputToolResults)
	if cfg != nil && cfg.LocalAuditScenePolicy == ContentModerationLocalAuditSceneProgrammingOnly && scene.Scene != "programming" {
		return nil, nil
	}
	var userID *int64
	if input.UserID > 0 {
		userID = &input.UserID
	}
	var apiKeyID *int64
	if input.APIKeyID > 0 {
		apiKeyID = &input.APIKeyID
	}
	meta := ContentModerationLocalAuditMetadata{
		ID:                  uuid.NewString(),
		RequestID:           strings.TrimSpace(input.RequestID),
		SessionID:           strings.TrimSpace(input.SessionID),
		ClientSessionID:     strings.TrimSpace(input.ClientSessionID),
		SessionSource:       strings.TrimSpace(input.SessionSource),
		UserID:              userID,
		UserEmail:           strings.TrimSpace(input.UserEmail),
		APIKeyID:            apiKeyID,
		APIKeyName:          strings.TrimSpace(input.APIKeyName),
		GroupID:             cloneInt64Ptr(input.GroupID),
		GroupName:           strings.TrimSpace(input.GroupName),
		Endpoint:            strings.TrimSpace(input.Endpoint),
		Provider:            strings.TrimSpace(input.Provider),
		Model:               strings.TrimSpace(input.Model),
		UpstreamModel:       strings.TrimSpace(input.UpstreamModel),
		Protocol:            strings.TrimSpace(input.Protocol),
		ResponseID:          firstNonEmptyLocalAuditString(input.ResponseID, strings.TrimSpace(gjson.GetBytes(input.RawResponse, "id").String())),
		PreviousResponseID:  strings.TrimSpace(input.PreviousResponseID),
		Scene:               scene.Scene,
		SceneSignals:        append([]string(nil), scene.Signals...),
		InputToolCallCount:  len(toolCalls),
		OutputToolCallCount: len(outputToolCalls),
		FileReadCount:       scene.FileReadCount,
		FileWriteCount:      scene.FileWriteCount,
		Stream:              input.Stream,
		SystemPrompt:        trimRunes(localAuditTextExcerpt(systemPrompt), maxModerationExcerptRunes),
		MessageCount:        localAuditCount(messages),
		ToolCount:           localAuditCount(tools),
		ToolCallCount:       len(toolCalls),
		ToolResultCount:     len(toolResults),
		CreatedAt:           createdAt,
	}
	return &ContentModerationLocalAuditRecord{
		ContentModerationLocalAuditMetadata: meta,
		UserAgent:                           strings.TrimSpace(input.UserAgent),
		Originator:                          strings.TrimSpace(input.Originator),
		SystemPrompt:                        systemPrompt,
		Messages:                            messages,
		Tools:                               tools,
		ToolCalls:                           redactSensitiveJSONArray(toolCalls),
		ToolResults:                         redactSensitiveJSONArray(toolResults),
		AssistantOutput:                     redactSensitiveJSON(assistantOutput),
		OutputToolCalls:                     redactSensitiveJSONArray(outputToolCalls),
		OutputToolResults:                   redactSensitiveJSONArray(outputToolResults),
		Usage:                               input.Usage,
		RawRequest:                          safeRequest,
		RawResponse:                         redactSensitiveJSON(safeResponse),
	}, nil
}

func shouldSkipContentModerationLocalAuditRecord(cfg *ContentModerationConfig, input ContentModerationLocalAuditInput, root map[string]any) bool {
	if cfg == nil {
		return false
	}
	if !cfg.LocalAuditExcludeImage {
		return false
	}
	if input.Protocol == ContentModerationProtocolOpenAIImages {
		return true
	}
	return IsImageGenerationIntentMap(input.Endpoint, input.Model, root)
}

func redactSensitiveJSONArray(items []any) []any {
	redacted, ok := redactSensitiveJSON(items).([]any)
	if !ok {
		return nil
	}
	return redacted
}

func extractLocalAuditConversation(protocol string, root map[string]any) (any, any, any, []any, []any) {
	if root == nil {
		return nil, nil, nil, nil, nil
	}
	var systemPrompt any
	var messages any
	var tools any
	switch protocol {
	case ContentModerationProtocolAnthropicMessages:
		systemPrompt = root["system"]
		messages = root["messages"]
		tools = root["tools"]
	case ContentModerationProtocolOpenAIChat:
		systemPrompt = extractOpenAIChatSystemMessages(root["messages"])
		messages = root["messages"]
		tools = root["tools"]
	case ContentModerationProtocolOpenAIResponses:
		systemPrompt = root["instructions"]
		messages = root["input"]
		tools = root["tools"]
	case ContentModerationProtocolGemini:
		if v, ok := root["system_instruction"]; ok {
			systemPrompt = v
		} else {
			systemPrompt = root["systemInstruction"]
		}
		messages = root["contents"]
		tools = root["tools"]
	default:
		systemPrompt = firstExistingLocalAuditValue(root, "system", "instructions", "system_instruction", "systemInstruction")
		messages = firstExistingLocalAuditValue(root, "messages", "input", "contents")
		tools = root["tools"]
	}
	var toolCalls []any
	var toolResults []any
	collectLocalAuditToolArtifacts(messages, &toolCalls, &toolResults)
	return redactSensitiveJSON(systemPrompt), redactSensitiveJSON(messages), redactSensitiveJSON(tools), toolCalls, toolResults
}

func extractOpenAIChatSystemMessages(messages any) any {
	arr, ok := messages.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0)
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(m["role"])))
		if role == "system" || role == "developer" {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectLocalAuditToolArtifacts(value any, calls *[]any, results *[]any) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			collectLocalAuditToolArtifacts(item, calls, results)
		}
	case map[string]any:
		typ := strings.ToLower(strings.TrimSpace(fmt.Sprint(v["type"])))
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(v["role"])))
		if rawCalls, ok := v["tool_calls"].([]any); ok {
			*calls = append(*calls, rawCalls...)
		}
		if fc, ok := v["functionCall"]; ok {
			*calls = append(*calls, fc)
		}
		if fr, ok := v["functionResponse"]; ok {
			*results = append(*results, fr)
		}
		switch typ {
		case "tool_use", "function_call", "tool_call":
			*calls = append(*calls, v)
		case "tool_result", "function_call_output":
			*results = append(*results, v)
		}
		if role == "tool" {
			*results = append(*results, v)
		}
		for _, child := range v {
			collectLocalAuditToolArtifacts(child, calls, results)
		}
	}
}

func extractLocalAuditResponseArtifacts(protocol string, raw []byte) (any, any, []any, []any) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil, nil, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var decoded any
		if err := json.Unmarshal(trimmed, &decoded); err == nil {
			assistantOutput, outputToolCalls, outputToolResults := extractLocalAuditResponseJSONArtifacts(protocol, decoded)
			return decoded, assistantOutput, outputToolCalls, outputToolResults
		}
	}
	if bytes.Contains(trimmed, []byte("data:")) {
		return extractLocalAuditSSEResponseArtifacts(protocol, trimmed)
	}
	return string(trimmed), nil, nil, nil
}

func extractLocalAuditResponseJSONArtifacts(protocol string, decoded any) (any, []any, []any) {
	root, _ := decoded.(map[string]any)
	switch protocol {
	case ContentModerationProtocolOpenAIChat:
		return extractOpenAIChatResponseArtifacts(root)
	case ContentModerationProtocolOpenAIResponses:
		return extractOpenAIResponsesResponseArtifacts(root)
	case ContentModerationProtocolAnthropicMessages:
		return extractAnthropicResponseArtifacts(root)
	case ContentModerationProtocolGemini:
		return extractGeminiResponseArtifacts(root)
	default:
		var toolCalls []any
		var toolResults []any
		collectLocalAuditToolArtifacts(decoded, &toolCalls, &toolResults)
		return decoded, toolCalls, toolResults
	}
}

func extractLocalAuditSSEResponseArtifacts(protocol string, raw []byte) (any, any, []any, []any) {
	lines := strings.Split(string(raw), "\n")
	events := make([]any, 0)
	textParts := make([]string, 0)
	var latestTerminal any
	var outputToolCalls []any
	var outputToolResults []any
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" || !gjson.Valid(payload) {
			continue
		}
		var decoded any
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			continue
		}
		events = append(events, decoded)
		if text := extractLocalAuditSSETextDelta(protocol, payload); text != "" {
			textParts = append(textParts, text)
		}
		if openAIStreamEventIsTerminal(payload) || anthropicStreamEventIsTerminal("", payload) {
			latestTerminal = decoded
			assistantOutput, toolCalls, toolResults := extractLocalAuditResponseJSONArtifacts(protocol, decoded)
			if assistantOutput != nil && len(textParts) == 0 {
				textParts = append(textParts, localAuditTextExcerpt(assistantOutput))
			}
			outputToolCalls = append(outputToolCalls, toolCalls...)
			outputToolResults = append(outputToolResults, toolResults...)
		}
	}
	var assistantOutput any
	if len(textParts) > 0 {
		assistantOutput = strings.Join(textParts, "")
	}
	if latestTerminal == nil {
		latestTerminal = events
	}
	return latestTerminal, assistantOutput, outputToolCalls, outputToolResults
}

func extractLocalAuditSSETextDelta(protocol string, payload string) string {
	switch protocol {
	case ContentModerationProtocolOpenAIResponses:
		return gjson.Get(payload, "delta").String()
	case ContentModerationProtocolOpenAIChat:
		if value := gjson.Get(payload, "choices.0.delta.content"); value.Exists() {
			return value.String()
		}
	case ContentModerationProtocolAnthropicMessages:
		if value := gjson.Get(payload, "delta.text"); value.Exists() {
			return value.String()
		}
	case ContentModerationProtocolGemini:
		if value := gjson.Get(payload, "candidates.0.content.parts.0.text"); value.Exists() {
			return value.String()
		}
	}
	return ""
}

func extractOpenAIChatResponseArtifacts(root map[string]any) (any, []any, []any) {
	if root == nil {
		return nil, nil, nil
	}
	choices, _ := root["choices"].([]any)
	var assistant []any
	var toolCalls []any
	var toolResults []any
	for _, choice := range choices {
		choiceMap, _ := choice.(map[string]any)
		message, _ := choiceMap["message"].(map[string]any)
		if message == nil {
			continue
		}
		if content := message["content"]; content != nil {
			assistant = append(assistant, content)
		}
		collectLocalAuditToolArtifacts(message, &toolCalls, &toolResults)
	}
	return singleOrSlice(assistant), toolCalls, toolResults
}

func extractOpenAIResponsesResponseArtifacts(root map[string]any) (any, []any, []any) {
	if root == nil {
		return nil, nil, nil
	}
	output, _ := root["output"].([]any)
	texts := make([]string, 0)
	var toolCalls []any
	var toolResults []any
	for _, item := range output {
		itemMap, _ := item.(map[string]any)
		if itemMap == nil {
			continue
		}
		switch strings.TrimSpace(fmt.Sprint(itemMap["type"])) {
		case "message":
			if content, ok := itemMap["content"].([]any); ok {
				for _, part := range content {
					if text := extractResponseContentText(part); text != "" {
						texts = append(texts, text)
					}
				}
			}
		}
		collectLocalAuditToolArtifacts(itemMap, &toolCalls, &toolResults)
	}
	if outputText := strings.TrimSpace(gjson.GetBytes(mustJSON(root), "output_text").String()); outputText != "" {
		texts = append(texts, outputText)
	}
	return strings.Join(texts, "\n"), toolCalls, toolResults
}

func extractAnthropicResponseArtifacts(root map[string]any) (any, []any, []any) {
	if root == nil {
		return nil, nil, nil
	}
	content, _ := root["content"].([]any)
	texts := make([]string, 0)
	var toolCalls []any
	var toolResults []any
	for _, item := range content {
		itemMap, _ := item.(map[string]any)
		if itemMap == nil {
			continue
		}
		if strings.TrimSpace(fmt.Sprint(itemMap["type"])) == "text" {
			if text := strings.TrimSpace(fmt.Sprint(itemMap["text"])); text != "" && text != "<nil>" {
				texts = append(texts, text)
			}
		}
		collectLocalAuditToolArtifacts(itemMap, &toolCalls, &toolResults)
	}
	return strings.Join(texts, "\n"), toolCalls, toolResults
}

func extractGeminiResponseArtifacts(root map[string]any) (any, []any, []any) {
	if root == nil {
		return nil, nil, nil
	}
	candidates, _ := root["candidates"].([]any)
	texts := make([]string, 0)
	var toolCalls []any
	var toolResults []any
	for _, candidate := range candidates {
		candidateMap, _ := candidate.(map[string]any)
		content, _ := candidateMap["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, part := range parts {
			partMap, _ := part.(map[string]any)
			if text := strings.TrimSpace(fmt.Sprint(partMap["text"])); text != "" && text != "<nil>" {
				texts = append(texts, text)
			}
			collectLocalAuditToolArtifacts(partMap, &toolCalls, &toolResults)
		}
	}
	return strings.Join(texts, "\n"), toolCalls, toolResults
}

func extractResponseContentText(value any) string {
	part, _ := value.(map[string]any)
	if part == nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	if text := strings.TrimSpace(fmt.Sprint(part["text"])); text != "" && text != "<nil>" {
		return text
	}
	return strings.TrimSpace(fmt.Sprint(part["content"]))
}

func assessLocalAuditScene(cfg *ContentModerationConfig, input ContentModerationLocalAuditInput, tools any, inputToolCalls []any, inputToolResults []any, outputToolCalls []any, outputToolResults []any) localAuditSceneAssessment {
	names := collectLocalAuditToolNames(tools)
	names = append(names, collectLocalAuditToolNames(inputToolCalls)...)
	names = append(names, collectLocalAuditToolNames(inputToolResults)...)
	names = append(names, collectLocalAuditToolNames(outputToolCalls)...)
	names = append(names, collectLocalAuditToolNames(outputToolResults)...)
	scene := localAuditSceneAssessment{
		Scene:               "general",
		InputToolCallCount:  len(inputToolCalls),
		OutputToolCallCount: len(outputToolCalls),
	}
	if matchesAnyLocalAuditPattern(strings.ToLower(strings.TrimSpace(input.UserAgent)), cfg.LocalAuditClientPatterns) {
		scene.Signals = append(scene.Signals, "user_agent")
	}
	if matchesAnyLocalAuditPattern(strings.ToLower(strings.TrimSpace(input.Originator)), cfg.LocalAuditClientPatterns) {
		scene.Signals = append(scene.Signals, "originator")
	}
	for _, name := range names {
		if matchesAnyLocalAuditPattern(name, cfg.LocalAuditToolPatterns) {
			scene.Signals = append(scene.Signals, "tool:"+name)
		}
		if isLocalAuditFileReadTool(name) {
			scene.FileReadCount++
		}
		if isLocalAuditFileWriteTool(name) {
			scene.FileWriteCount++
		}
	}
	scene.Signals = dedupeLocalAuditStrings(scene.Signals)
	if len(scene.Signals) > 0 || scene.FileReadCount > 0 || scene.FileWriteCount > 0 || len(inputToolCalls) > 0 || len(outputToolCalls) > 0 {
		scene.Scene = "programming"
	}
	return scene
}

func collectLocalAuditToolNames(value any) []string {
	out := make([]string, 0)
	collectLocalAuditToolNamesInto(value, &out)
	return dedupeLocalAuditStrings(out)
}

func collectLocalAuditToolNamesInto(value any, out *[]string) {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			collectLocalAuditToolNamesInto(item, out)
		}
	case map[string]any:
		if fn, ok := v["function"].(map[string]any); ok {
			if name := strings.ToLower(strings.TrimSpace(fmt.Sprint(fn["name"]))); name != "" && name != "<nil>" {
				*out = append(*out, name)
			}
		}
		if name := strings.ToLower(strings.TrimSpace(fmt.Sprint(v["tool_name"]))); name != "" && name != "<nil>" {
			*out = append(*out, name)
		}
		if name := strings.ToLower(strings.TrimSpace(fmt.Sprint(v["name"]))); name != "" && name != "<nil>" {
			if _, hasType := v["type"]; hasType {
				*out = append(*out, name)
			} else if _, hasInput := v["input"]; hasInput {
				*out = append(*out, name)
			} else if _, hasArgs := v["arguments"]; hasArgs {
				*out = append(*out, name)
			}
		}
		for _, child := range v {
			collectLocalAuditToolNamesInto(child, out)
		}
	}
}

func matchesAnyLocalAuditPattern(value string, patterns []string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if pattern == "" {
			continue
		}
		if strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func isLocalAuditFileReadTool(name string) bool {
	for _, token := range []string{"read", "open", "cat", "grep", "search", "glob", "ls"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func isLocalAuditFileWriteTool(name string) bool {
	for _, token := range []string{"write", "edit", "patch", "apply", "create", "delete", "rename"} {
		if strings.Contains(name, token) {
			return true
		}
	}
	return false
}

func dedupeLocalAuditStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func singleOrSlice(values []any) any {
	switch len(values) {
	case 0:
		return nil
	case 1:
		return values[0]
	default:
		return values
	}
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}

func firstNonEmptyLocalAuditString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *ContentModerationService) runLocalAuditCleanupOnce() {
	if s == nil || s.settingRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), contentModerationLocalAuditCleanupTimeout)
	defer cancel()
	cfg, err := s.loadConfig(ctx)
	if err != nil || !cfg.LocalAuditEnabled {
		s.refreshLocalAuditStats()
		return
	}
	files, err := s.scanLocalAuditFiles()
	if err != nil {
		slog.Warn("content_moderation.local_audit_scan_failed", "error", err)
		return
	}
	var total int64
	for _, file := range files {
		total += file.sizeBytes
	}
	maxBytes := localAuditMaxBytes(cfg)
	if maxBytes <= 0 || total <= maxBytes {
		s.localAuditRetainedBytes.Store(total)
		s.localAuditRetainedRecords.Store(int64(len(files)))
		return
	}
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].createdAt.Before(files[j].createdAt)
	})
	var deleted int64
	var deletedBytes int64
	for _, file := range files {
		if total <= maxBytes {
			break
		}
		if err := os.Remove(file.recordPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("content_moderation.local_audit_delete_record_failed", "path", file.recordPath, "error", err)
			continue
		}
		if err := os.Remove(file.metaPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("content_moderation.local_audit_delete_meta_failed", "path", file.metaPath, "error", err)
		}
		total -= file.sizeBytes
		deleted++
		deletedBytes += file.sizeBytes
	}
	s.localAuditLastCleanupUnix.Store(time.Now().Unix())
	s.localAuditLastCleanupDeleted.Store(deleted)
	s.localAuditLastCleanupDeletedBytes.Store(deletedBytes)
	s.refreshLocalAuditStats()
}

func (s *ContentModerationService) localAuditStats(cfg *ContentModerationConfig) ContentModerationLocalAuditStats {
	if s == nil {
		return ContentModerationLocalAuditStats{}
	}
	s.refreshLocalAuditStats()
	var lastCleanupAt *time.Time
	if unix := s.localAuditLastCleanupUnix.Load(); unix > 0 {
		t := time.Unix(unix, 0)
		lastCleanupAt = &t
	}
	queueLength := 0
	queueSize := 0
	if s.localAuditQueue != nil {
		queueLength = len(s.localAuditQueue)
		queueSize = cap(s.localAuditQueue)
	}
	stats := ContentModerationLocalAuditStats{
		StoragePath:               s.localAuditRoot(),
		ResponseCaptureLimitBytes: ContentModerationLocalAuditResponseCaptureLimitBytes,
		QueueSize:                 queueSize,
		QueueLength:               queueLength,
		CaptureActive:             s.localAuditCaptureActive.Load(),
		OverloadSkipped:           s.localAuditOverloadSkipped.Load(),
		Active:                    s.localAuditActive.Load(),
		Enqueued:                  s.localAuditEnqueued.Load(),
		Dropped:                   s.localAuditDropped.Load(),
		Written:                   s.localAuditWritten.Load(),
		Errors:                    s.localAuditErrors.Load(),
		RetainedBytes:             s.localAuditRetainedBytes.Load(),
		RetainedRecords:           s.localAuditRetainedRecords.Load(),
		LastCleanupAt:             lastCleanupAt,
		LastCleanupDeleted:        s.localAuditLastCleanupDeleted.Load(),
		LastCleanupDeletedBytes:   s.localAuditLastCleanupDeletedBytes.Load(),
	}
	if cfg != nil {
		stats.Enabled = cfg.LocalAuditEnabled
		stats.MaxStorageGB = cfg.LocalAuditMaxStorageGB
		stats.CaptureMaxConcurrency = cfg.LocalAuditMaxCaptureConcurrency
		stats.OverloadActive = cfg.LocalAuditMaxCaptureConcurrency > 0 && stats.CaptureActive >= int64(cfg.LocalAuditMaxCaptureConcurrency)
	}
	return stats
}

func (s *ContentModerationService) refreshLocalAuditStats() {
	files, err := s.scanLocalAuditFiles()
	if err != nil {
		return
	}
	var total int64
	for _, file := range files {
		total += file.sizeBytes
	}
	s.localAuditRetainedBytes.Store(total)
	s.localAuditRetainedRecords.Store(int64(len(files)))
}

func (s *ContentModerationService) ensureLocalAuditReadable(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return infraerrors.InternalServer("LOCAL_AUDIT_UNAVAILABLE", "本地人工审计服务不可用")
	}
	if !s.isRiskControlEnabled(ctx) {
		return infraerrors.NotFound("RISK_CONTROL_DISABLED", "风控中心未启用")
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return err
	}
	if !cfg.LocalAuditEnabled {
		return infraerrors.NotFound("LOCAL_AUDIT_DISABLED", "本地人工审计未启用")
	}
	return nil
}

func (s *ContentModerationService) findLocalAuditFileByID(id string) (*contentModerationLocalAuditFile, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if !contentModerationLocalAuditIDPattern.MatchString(id) {
		return nil, infraerrors.BadRequest("INVALID_LOCAL_AUDIT_ID", "本地审计记录 ID 无效")
	}
	files, err := s.scanLocalAuditFiles()
	if err != nil {
		return nil, err
	}
	for i := range files {
		if strings.EqualFold(files[i].meta.ID, id) {
			return &files[i], nil
		}
	}
	return nil, infraerrors.NotFound("LOCAL_AUDIT_NOT_FOUND", "本地审计记录不存在")
}

func (s *ContentModerationService) scanLocalAuditFiles() ([]contentModerationLocalAuditFile, error) {
	root := s.localAuditRoot()
	entries := make([]contentModerationLocalAuditFile, 0)
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, fmt.Errorf("stat local audit root: %w", err)
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".meta.json") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var meta ContentModerationLocalAuditMetadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil
		}
		recordPath := strings.TrimSuffix(path, ".meta.json") + ".json"
		size := meta.FileSizeBytes
		if st, err := os.Stat(recordPath); err == nil {
			size = st.Size()
		}
		if meta.CreatedAt.IsZero() {
			if st, err := os.Stat(path); err == nil {
				meta.CreatedAt = st.ModTime()
			}
		}
		entries = append(entries, contentModerationLocalAuditFile{
			metaPath:   path,
			recordPath: recordPath,
			meta:       meta,
			createdAt:  meta.CreatedAt,
			sizeBytes:  size + int64(len(raw)),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan local audit files: %w", err)
	}
	return entries, nil
}

func (s *ContentModerationService) localAuditRoot() string {
	base := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if base == "" {
		base = "./data"
	}
	return filepath.Clean(filepath.Join(base, "risk-control", "conversation-audits"))
}

func writeLocalAuditFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local audit dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write local audit temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit local audit file: %w", err)
	}
	return nil
}

func cloneContentModerationLocalAuditInput(input ContentModerationLocalAuditInput) ContentModerationLocalAuditInput {
	input.Body = append([]byte(nil), input.Body...)
	input.RawResponse = append([]byte(nil), input.RawResponse...)
	input.GroupID = cloneInt64Ptr(input.GroupID)
	return input
}

func localAuditMaxBytes(cfg *ContentModerationConfig) int64 {
	if cfg == nil {
		return int64(defaultContentModerationLocalAuditStorageGB * 1024 * 1024 * 1024)
	}
	return int64(cfg.LocalAuditMaxStorageGB * 1024 * 1024 * 1024)
}

func normalizeLocalAuditPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	return params
}

func localAuditPaginationResult(total int64, params pagination.PaginationParams) *pagination.PaginationResult {
	pages := 0
	if params.PageSize > 0 && total > 0 {
		pages = int((total + int64(params.PageSize) - 1) / int64(params.PageSize))
	}
	return &pagination.PaginationResult{
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    pages,
	}
}

func localAuditMetadataMatches(meta ContentModerationLocalAuditMetadata, filter ContentModerationLocalAuditListFilter) bool {
	if filter.GroupID != nil && (meta.GroupID == nil || *meta.GroupID != *filter.GroupID) {
		return false
	}
	if endpoint := strings.TrimSpace(filter.Endpoint); endpoint != "" && meta.Endpoint != endpoint {
		return false
	}
	if model := strings.TrimSpace(filter.Model); model != "" && !strings.Contains(strings.ToLower(meta.Model), strings.ToLower(model)) {
		return false
	}
	if filter.From != nil && meta.CreatedAt.Before(*filter.From) {
		return false
	}
	if filter.To != nil && meta.CreatedAt.After(*filter.To) {
		return false
	}
	if search := strings.ToLower(strings.TrimSpace(filter.Search)); search != "" {
		haystack := strings.ToLower(strings.Join([]string{
			meta.ID,
			meta.RequestID,
			meta.SessionID,
			meta.ClientSessionID,
			meta.SessionSource,
			meta.UserEmail,
			meta.APIKeyName,
			meta.GroupName,
			meta.Endpoint,
			meta.Provider,
			meta.Model,
			meta.UpstreamModel,
			meta.ResponseID,
			meta.PreviousResponseID,
			meta.Scene,
			strings.Join(meta.SceneSignals, " "),
			meta.SystemPrompt,
		}, " "))
		if !strings.Contains(haystack, search) {
			return false
		}
	}
	return true
}

func firstExistingLocalAuditValue(root map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := root[key]; ok {
			return value
		}
	}
	return nil
}

func inferLocalAuditSessionID(root map[string]any) string {
	for _, key := range []string{"session_id", "conversation_id", "thread_id", "previous_response_id", "prompt_cache_key"} {
		if value, ok := root[key]; ok {
			if out := strings.TrimSpace(fmt.Sprint(value)); out != "" && out != "<nil>" {
				return out
			}
		}
	}
	if metadata, ok := root["metadata"].(map[string]any); ok {
		for _, key := range []string{"session_id", "conversation_id", "user_id"} {
			if value, ok := metadata[key]; ok {
				if out := strings.TrimSpace(fmt.Sprint(value)); out != "" && out != "<nil>" {
					return out
				}
			}
		}
	}
	return ""
}

func localAuditCount(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case map[string]any:
		return len(v)
	case nil:
		return 0
	default:
		return 1
	}
}

func localAuditTextExcerpt(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return strings.TrimSpace(buf.String())
}
