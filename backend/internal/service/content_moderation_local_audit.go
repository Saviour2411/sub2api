package service

import (
	"bytes"
	"context"
	"encoding/json"
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
)

const (
	contentModerationLocalAuditCleanupInterval = 5 * time.Minute
	contentModerationLocalAuditCleanupTimeout  = 2 * time.Minute
)

var contentModerationLocalAuditIDPattern = regexp.MustCompile(`^[a-f0-9-]{36}$`)

type ContentModerationLocalAuditInput struct {
	RequestID     string
	UserID        int64
	UserEmail     string
	APIKeyID      int64
	APIKeyName    string
	GroupID       *int64
	GroupName     string
	Endpoint      string
	Provider      string
	Model         string
	UpstreamModel string
	Protocol      string
	SessionID     string
	Stream        bool
	Body          []byte
	Usage         any
}

type ContentModerationLocalAuditMetadata struct {
	ID              string    `json:"id"`
	RequestID       string    `json:"request_id"`
	SessionID       string    `json:"session_id"`
	UserID          *int64    `json:"user_id,omitempty"`
	UserEmail       string    `json:"user_email"`
	APIKeyID        *int64    `json:"api_key_id,omitempty"`
	APIKeyName      string    `json:"api_key_name"`
	GroupID         *int64    `json:"group_id,omitempty"`
	GroupName       string    `json:"group_name"`
	Endpoint        string    `json:"endpoint"`
	Provider        string    `json:"provider"`
	Model           string    `json:"model"`
	UpstreamModel   string    `json:"upstream_model,omitempty"`
	Protocol        string    `json:"protocol"`
	Stream          bool      `json:"stream"`
	SystemPrompt    string    `json:"system_prompt_excerpt"`
	MessageCount    int       `json:"message_count"`
	ToolCount       int       `json:"tool_count"`
	ToolCallCount   int       `json:"tool_call_count"`
	ToolResultCount int       `json:"tool_result_count"`
	FileSizeBytes   int64     `json:"file_size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type ContentModerationLocalAuditRecord struct {
	ContentModerationLocalAuditMetadata
	SystemPrompt any   `json:"system_prompt,omitempty"`
	Messages     any   `json:"messages,omitempty"`
	Tools        any   `json:"tools,omitempty"`
	ToolCalls    []any `json:"tool_calls,omitempty"`
	ToolResults  []any `json:"tool_results,omitempty"`
	Usage        any   `json:"usage,omitempty"`
	RawRequest   any   `json:"raw_request,omitempty"`
}

type ContentModerationLocalAuditStats struct {
	Enabled                 bool       `json:"enabled"`
	StoragePath             string     `json:"storage_path"`
	MaxStorageGB            float64    `json:"max_storage_gb"`
	RetainedBytes           int64      `json:"retained_bytes"`
	RetainedRecords         int64      `json:"retained_records"`
	QueueSize               int        `json:"queue_size"`
	QueueLength             int        `json:"queue_length"`
	Active                  int64      `json:"active"`
	Enqueued                int64      `json:"enqueued"`
	Dropped                 int64      `json:"dropped"`
	Written                 int64      `json:"written"`
	Errors                  int64      `json:"errors"`
	LastCleanupAt           *time.Time `json:"last_cleanup_at,omitempty"`
	LastCleanupDeleted      int64      `json:"last_cleanup_deleted"`
	LastCleanupDeletedBytes int64      `json:"last_cleanup_deleted_bytes"`
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
	record, err := buildContentModerationLocalAuditRecord(task.input, task.createdAt)
	if err != nil {
		return err
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
	record.ContentModerationLocalAuditMetadata.FileSizeBytes = record.FileSizeBytes
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

func buildContentModerationLocalAuditRecord(input ContentModerationLocalAuditInput, createdAt time.Time) (*ContentModerationLocalAuditRecord, error) {
	var decoded any
	if err := json.Unmarshal(input.Body, &decoded); err != nil {
		return nil, fmt.Errorf("parse local audit request: %w", err)
	}
	root, _ := decoded.(map[string]any)
	safeRequest := redactSensitiveJSON(decoded)
	systemPrompt, messages, tools, toolCalls, toolResults := extractLocalAuditConversation(input.Protocol, root)
	if input.SessionID == "" {
		input.SessionID = inferLocalAuditSessionID(root)
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
		ID:              uuid.NewString(),
		RequestID:       strings.TrimSpace(input.RequestID),
		SessionID:       strings.TrimSpace(input.SessionID),
		UserID:          userID,
		UserEmail:       strings.TrimSpace(input.UserEmail),
		APIKeyID:        apiKeyID,
		APIKeyName:      strings.TrimSpace(input.APIKeyName),
		GroupID:         cloneInt64Ptr(input.GroupID),
		GroupName:       strings.TrimSpace(input.GroupName),
		Endpoint:        strings.TrimSpace(input.Endpoint),
		Provider:        strings.TrimSpace(input.Provider),
		Model:           strings.TrimSpace(input.Model),
		UpstreamModel:   strings.TrimSpace(input.UpstreamModel),
		Protocol:        strings.TrimSpace(input.Protocol),
		Stream:          input.Stream,
		SystemPrompt:    trimRunes(localAuditTextExcerpt(systemPrompt), maxModerationExcerptRunes),
		MessageCount:    localAuditCount(messages),
		ToolCount:       localAuditCount(tools),
		ToolCallCount:   len(toolCalls),
		ToolResultCount: len(toolResults),
		CreatedAt:       createdAt,
	}
	return &ContentModerationLocalAuditRecord{
		ContentModerationLocalAuditMetadata: meta,
		SystemPrompt:                        systemPrompt,
		Messages:                            messages,
		Tools:                               tools,
		ToolCalls:                           redactSensitiveJSONArray(toolCalls),
		ToolResults:                         redactSensitiveJSONArray(toolResults),
		Usage:                               input.Usage,
		RawRequest:                          safeRequest,
	}, nil
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
	s.refreshLocalAuditStats()
	var lastCleanupAt *time.Time
	if unix := s.localAuditLastCleanupUnix.Load(); unix > 0 {
		t := time.Unix(unix, 0)
		lastCleanupAt = &t
	}
	queueLength := 0
	queueSize := 0
	if s != nil && s.localAuditQueue != nil {
		queueLength = len(s.localAuditQueue)
		queueSize = cap(s.localAuditQueue)
	}
	stats := ContentModerationLocalAuditStats{
		StoragePath:             s.localAuditRoot(),
		QueueSize:               queueSize,
		QueueLength:             queueLength,
		Active:                  s.localAuditActive.Load(),
		Enqueued:                s.localAuditEnqueued.Load(),
		Dropped:                 s.localAuditDropped.Load(),
		Written:                 s.localAuditWritten.Load(),
		Errors:                  s.localAuditErrors.Load(),
		RetainedBytes:           s.localAuditRetainedBytes.Load(),
		RetainedRecords:         s.localAuditRetainedRecords.Load(),
		LastCleanupAt:           lastCleanupAt,
		LastCleanupDeleted:      s.localAuditLastCleanupDeleted.Load(),
		LastCleanupDeletedBytes: s.localAuditLastCleanupDeletedBytes.Load(),
	}
	if cfg != nil {
		stats.Enabled = cfg.LocalAuditEnabled
		stats.MaxStorageGB = cfg.LocalAuditMaxStorageGB
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
			meta.UserEmail,
			meta.APIKeyName,
			meta.GroupName,
			meta.Endpoint,
			meta.Provider,
			meta.Model,
			meta.UpstreamModel,
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
