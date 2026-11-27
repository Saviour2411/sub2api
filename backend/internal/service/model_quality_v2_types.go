package service

import (
	"context"
	"encoding/json"
	"time"
)

const ModelQualitySchemaVersion = 2
const ModelQualityMaxResponseBytes = 5 << 20

type ModelQualityQuestionSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OutputType string `json:"output_type"`
}

type ModelQualityCall struct {
	GroupID         int64                      `json:"group_id"`
	KeyID           *int64                     `json:"key_id"`
	OwnerID         int64                      `json:"owner_id,omitempty"`
	Protocol        string                     `json:"protocol"`
	Model           string                     `json:"model"`
	ReasoningEffort string                     `json:"reasoning_effort,omitempty"`
	ThinkingMode    string                     `json:"thinking_mode,omitempty"`
	ThinkingBudget  *int                       `json:"thinking_budget,omitempty"`
	MaxOutputTokens int                        `json:"max_output_tokens"`
	ChatTokenField  string                     `json:"chat_token_field,omitempty"`
	TimeoutSeconds  int                        `json:"timeout_seconds"`
	ExtraBody       map[string]json.RawMessage `json:"extra_body,omitempty"`
}

type ModelQualityQuestion struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Prompt       string           `json:"prompt"`
	SystemPrompt string           `json:"system_prompt,omitempty"`
	OutputType   string           `json:"output_type"`
	Rule         ModelQualityRule `json:"rule"`
}

type ModelQualityRule struct {
	Kind        string            `json:"kind"`
	Expected    string            `json:"expected,omitempty"`
	JudgePrompt string            `json:"judge_prompt,omitempty"`
	Judge       *ModelQualityCall `json:"judge,omitempty"`
}

type ModelQualityTarget struct {
	ID string `json:"id"`
	ModelQualityCall
	QuestionIDs []string `json:"question_ids"`
}

type ModelQualityV2GroupConfig struct {
	GroupID         int64                `json:"group_id"`
	IntervalMinutes int                  `json:"interval_minutes"`
	Targets         []ModelQualityTarget `json:"targets"`
}

type ModelQualityV2Config struct {
	SchemaVersion          int                         `json:"schema_version"`
	Enabled                bool                        `json:"enabled"`
	Version                int                         `json:"version"`
	Questions              []ModelQualityQuestion      `json:"questions"`
	Groups                 []ModelQualityV2GroupConfig `json:"groups"`
	Endpoint               string                      `json:"endpoint,omitempty"`
	PauseReason            string                      `json:"pause_reason,omitempty"`
	InitializationRequired bool                        `json:"initialization_required"`
}

type ModelQualityPhase struct {
	Status          string     `json:"status"`
	Answer          string     `json:"answer,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	FinishReason    string     `json:"finish_reason,omitempty"`
	DurationMS      int64      `json:"duration_ms"`
	InputTokens     *int64     `json:"input_tokens,omitempty"`
	OutputTokens    *int64     `json:"output_tokens,omitempty"`
	ReasoningTokens *int64     `json:"reasoning_tokens,omitempty"`
	Attempts        int        `json:"attempts"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
}

type ModelQualityV2Test struct {
	ModelQualityTest
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	OutputType        string              `json:"output_type"`
	RuleKind          string              `json:"rule_kind"`
	JudgeModel        string              `json:"judge_model,omitempty"`
	JudgeProtocol     string              `json:"judge_protocol,omitempty"`
	Generation        ModelQualityPhase   `json:"generation"`
	Judgment          *ModelQualityPhase  `json:"judgment,omitempty"`
	PreviousJudgments []ModelQualityPhase `json:"previous_judgments,omitempty"`
}

type ModelQualityV2Run struct {
	ID              string               `json:"id"`
	Source          string               `json:"source"`
	GroupID         int64                `json:"group_id"`
	GroupName       string               `json:"group_name"`
	Model           string               `json:"model"`
	Protocol        string               `json:"protocol"`
	ReasoningEffort string               `json:"reasoning_effort"`
	Outcome         string               `json:"outcome"`
	StartedAt       time.Time            `json:"started_at"`
	FinishedAt      *time.Time           `json:"finished_at"`
	ExpiresAt       time.Time            `json:"expires_at"`
	Tests           []ModelQualityV2Test `json:"tests"`
}

type ModelQualitySpecification struct {
	Target   ModelQualityTarget   `json:"target"`
	Question ModelQualityQuestion `json:"question"`
	Endpoint string               `json:"endpoint"`
}

type ModelQualityV2Task struct {
	ID, RunID, Status, Owner string
	Specification            ModelQualitySpecification
	Result                   ModelQualityV2Test
	StartedAt, NextAttemptAt time.Time
	DeadlineAt               *time.Time
}

type ModelQualityV2Repository interface {
	QualityV2Config(context.Context) (*ModelQualityV2Config, error)
	SaveQualityV2Config(context.Context, *ModelQualityV2Config) error
	QualityV2States(context.Context) (map[int64]ModelQualityGroupState, error)
	StartQualityV2Round(context.Context, *ModelQualityV2Config, ModelQualityV2GroupConfig, string, bool) error
	ClaimQualityV2Task(context.Context, int, string) (*ModelQualityV2Task, error)
	RefreshQualityV2Lease(context.Context, *ModelQualityV2Task) (bool, error)
	SaveQualityV2Task(context.Context, *ModelQualityV2Task, string) (bool, error)
	MaintainQualityV2(context.Context) error
	QualityV2Runs(context.Context, ModelQualityFilter) ([]ModelQualityV2Run, int64, error)
	QualityV2Run(context.Context, string) (*ModelQualityV2Run, error)
	RejudgeQualityV2Task(context.Context, string, int64) error
}
