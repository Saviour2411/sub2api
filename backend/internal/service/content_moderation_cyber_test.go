package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func cyberPolicyTestConfig(t *testing.T, mutate func(*ContentModerationConfig)) string {
	t.Helper()
	cfg := defaultContentModerationConfig()
	cfg.Enabled = true
	cfg.Mode = ContentModerationModeObserve
	cfg.AllGroups = true
	cfg.AutoBanEnabled = false
	if mutate != nil {
		mutate(cfg)
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	return string(raw)
}

// cyberOrderingTestRepo records the sequence of repo calls to verify F7 ordering.
type cyberOrderingTestRepo struct {
	mu         sync.Mutex
	calls      []string
	emailSents []bool // EmailSent value captured at each CreateLog call
}

func (r *cyberOrderingTestRepo) CreateLog(ctx context.Context, log *ContentModerationLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "create")
	if log != nil {
		r.emailSents = append(r.emailSents, log.EmailSent)
		log.ID = 1 // simulate DB-assigned ID so UpdateLogEmailSent guard passes
	}
	return nil
}

func (r *cyberOrderingTestRepo) UpdateLogEmailSent(ctx context.Context, id int64, sent bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, "update_email_sent")
	return nil
}

func (r *cyberOrderingTestRepo) ListLogs(ctx context.Context, filter ContentModerationLogFilter) ([]ContentModerationLog, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *cyberOrderingTestRepo) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time, excludeCyberPolicy bool) (int, error) {
	return 0, nil
}

func (r *cyberOrderingTestRepo) CleanupExpiredLogs(ctx context.Context, hitBefore time.Time, nonHitBefore time.Time) (*ContentModerationCleanupResult, error) {
	return &ContentModerationCleanupResult{}, nil
}

func (r *cyberOrderingTestRepo) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

func (r *cyberOrderingTestRepo) snapshotEmailSents() []bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]bool, len(r.emailSents))
	copy(out, r.emailSents)
	return out
}

func TestRecordCyberPolicyEvent_DisabledWhenRiskControlOff(t *testing.T) {
	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "false",
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamBody:    `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus:  400,
	})

	require.Empty(t, repo.snapshotLogs(), "CreateLog must NOT be called when risk_control_enabled is off")
}

func TestRecordCyberPolicyEvent_WritesLogWhenEnabled(t *testing.T) {
	repo := &contentModerationTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: cyberPolicyTestConfig(t, nil),
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil, // emailService=nil: email path safely skipped
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamBody:    `{"error":{"code":"cyber_policy"}}`,
		UpstreamStatus:  400,
	})

	logs := repo.snapshotLogs()
	require.Len(t, logs, 1)
	log := logs[0]

	require.Equal(t, "cyber_policy", log.Action)
	require.True(t, log.Flagged)
	require.Equal(t, "cyber_policy", log.HighestCategory)
	require.Contains(t, log.Error, "flagged")
	require.False(t, log.AutoBanned)
	// emailService is nil, so EmailSent must be false
	require.False(t, log.EmailSent)

	// UserID pointer must be set
	require.NotNil(t, log.UserID)
	require.Equal(t, int64(1), *log.UserID)

	// score for cyber_policy is always 1.0
	require.Equal(t, 1.0, log.HighestScore)

	// mode must be post_upstream
	require.Equal(t, "post_upstream", log.Mode)

	// provider
	require.Equal(t, "openai", log.Provider)

	// model
	require.Equal(t, "gpt-5", log.Model)

	// endpoint
	require.Equal(t, "/v1/responses", log.Endpoint)

	// violation count >= 1 (side-effects ran)
	require.GreaterOrEqual(t, log.ViolationCount, 1)

	// Error field should also contain the upstream body JSON
	require.True(t, strings.Contains(log.Error, "cyber_policy") || strings.Contains(log.Error, "flagged"),
		"Error should mention flagged or cyber_policy")
}

func TestRecordCyberPolicyEvent_RequiresConfiguredAuditScope(t *testing.T) {
	group26 := int64(26)
	tests := []struct {
		name   string
		mutate func(*ContentModerationConfig)
		input  CyberPolicyRecordInput
		want   int
	}{
		{
			name: "content moderation disabled",
			mutate: func(cfg *ContentModerationConfig) {
				cfg.Enabled = false
			},
			input: CyberPolicyRecordInput{GroupID: &group26, Model: "gpt-5"},
		},
		{
			name: "mode off",
			mutate: func(cfg *ContentModerationConfig) {
				cfg.Mode = ContentModerationModeOff
			},
			input: CyberPolicyRecordInput{GroupID: &group26, Model: "gpt-5"},
		},
		{
			name: "group outside scope",
			mutate: func(cfg *ContentModerationConfig) {
				cfg.AllGroups = false
				cfg.GroupIDs = []int64{26}
			},
			input: func() CyberPolicyRecordInput {
				group8 := int64(8)
				return CyberPolicyRecordInput{GroupID: &group8, Model: "gpt-5"}
			}(),
		},
		{
			name: "model outside scope",
			mutate: func(cfg *ContentModerationConfig) {
				cfg.ModelFilter = ContentModerationModelFilter{Type: ContentModerationModelFilterInclude, Models: []string{"gpt-5"}}
			},
			input: CyberPolicyRecordInput{GroupID: &group26, Model: "gpt-4.1"},
		},
		{
			name: "group and model in scope",
			mutate: func(cfg *ContentModerationConfig) {
				cfg.AllGroups = false
				cfg.GroupIDs = []int64{26}
				cfg.ModelFilter = ContentModerationModelFilter{Type: ContentModerationModelFilterInclude, Models: []string{"gpt-5"}}
			},
			input: CyberPolicyRecordInput{GroupID: &group26, Model: "gpt-5"},
			want:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &banCountArgsTestRepo{}
			svc := NewContentModerationService(
				&contentModerationTestSettingRepo{values: map[string]string{
					SettingKeyRiskControlEnabled:      "true",
					SettingKeyContentModerationConfig: cyberPolicyTestConfig(t, tt.mutate),
				}},
				repo, nil, nil, nil, nil, nil,
			)
			tt.input.UserID = 7
			tt.input.Endpoint = "/v1/responses"
			tt.input.UpstreamMessage = "blocked"
			svc.RecordCyberPolicyEvent(context.Background(), tt.input)
			require.Len(t, repo.snapshotLogs(), tt.want)
			if tt.want == 0 {
				require.Empty(t, repo.snapshotCountCalls())
			}
		})
	}
}

func TestRecordCyberPolicyEvent_RuntimeLoadFailureHasNoSideEffects(t *testing.T) {
	settings := &contentModerationRuntimeSettingRepo{getMultipleErr: errors.New("database unavailable")}
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(settings, repo, nil, nil, nil, nil, nil)
	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{UserID: 7, Model: "gpt-5"})
	require.Empty(t, repo.snapshotLogs())
	require.Empty(t, repo.snapshotCountCalls())
}

func TestRecordCyberPolicyEvent_EmailOnHitControlsCyberNotice(t *testing.T) {
	for _, emailOnHit := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[emailOnHit], func(t *testing.T) {
			smtpServer := startNotificationEmailTestSMTPServer(t)
			settings := newNotificationEmailMemorySettingRepo()
			values := smtpServer.settings()
			values[SettingKeyRiskControlEnabled] = "true"
			values[SettingKeyContentModerationConfig] = cyberPolicyTestConfig(t, func(cfg *ContentModerationConfig) {
				cfg.EmailOnHit = emailOnHit
			})
			require.NoError(t, settings.SetMultiple(context.Background(), values))

			repo := &cyberOrderingTestRepo{}
			emailService := NewEmailService(settings, nil)
			svc := NewContentModerationService(settings, repo, nil, nil, nil, nil, emailService)
			groupID := int64(26)
			svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
				UserID:          7,
				UserEmail:       "user@example.com",
				GroupID:         &groupID,
				Model:           "gpt-5",
				Endpoint:        "/v1/responses",
				UpstreamMessage: "blocked",
			})

			wantMessages := int64(0)
			if emailOnHit {
				wantMessages = 1
			}
			require.Equal(t, wantMessages, smtpServer.messageCount())
			calls := repo.snapshot()
			require.Equal(t, "create", calls[0])
			if emailOnHit {
				require.Contains(t, calls, "update_email_sent")
			} else {
				require.NotContains(t, calls, "update_email_sent")
			}
		})
	}
}

// TestRecordCyberPolicyEvent_CreateLogBeforeEmail verifies F7: the moderation
// log is persisted BEFORE email delivery, and EmailSent is patched afterwards —
// SMTP hangs can no longer swallow the audit record.
//
// With emailService=nil the email block is skipped and UpdateLogEmailSent is not
// called (correct: logPersisted && emailSent guard). This test asserts:
//  1. CreateLog runs first (calls[0]=="create").
//  2. The log is stored with EmailSent=false (not pre-set to true).
func TestRecordCyberPolicyEvent_CreateLogBeforeEmail(t *testing.T) {
	repo := &cyberOrderingTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: cyberPolicyTestConfig(t, nil),
		}},
		repo,
		nil,
		nil,
		nil,
		nil,
		nil, // emailService=nil: email path safely skipped; see doc comment above
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		RequestID:       "req-1",
		UserID:          7,
		UserEmail:       "u@example.com",
		Model:           "gpt-5",
		UpstreamMessage: "blocked",
	})

	calls := repo.snapshot()
	require.GreaterOrEqual(t, len(calls), 1, "CreateLog must be called")
	require.Equal(t, "create", calls[0], "CreateLog must run first (F7: log-before-email)")

	// EmailSent must be false when the log is first persisted (new code sets it
	// false before CreateLog; email result is patched via UpdateLogEmailSent).
	emailSents := repo.snapshotEmailSents()
	require.NotEmpty(t, emailSents, "CreateLog must have captured EmailSent value")
	require.False(t, emailSents[0], "log must be stored with EmailSent=false initially (F7)")

	// With emailService=nil, no email is sent, so UpdateLogEmailSent must NOT
	// be called (logPersisted && emailSent guard correctly suppresses the patch).
	require.NotContains(t, calls, "update_email_sent",
		"UpdateLogEmailSent must not be called when no email was sent")
}

// banCountArgsTestRepo 在 contentModerationTestRepo 基础上记录
// CountFlaggedByUserSince 收到的 excludeCyberPolicy 参数，供透传断言。
type banCountArgsTestRepo struct {
	contentModerationTestRepo
	argsMu     sync.Mutex
	countCalls []bool
}

func (r *banCountArgsTestRepo) CountFlaggedByUserSince(ctx context.Context, userID int64, since time.Time, excludeCyberPolicy bool) (int, error) {
	r.argsMu.Lock()
	r.countCalls = append(r.countCalls, excludeCyberPolicy)
	r.argsMu.Unlock()
	return r.contentModerationTestRepo.CountFlaggedByUserSince(ctx, userID, since, excludeCyberPolicy)
}

func (r *banCountArgsTestRepo) snapshotCountCalls() []bool {
	r.argsMu.Lock()
	defer r.argsMu.Unlock()
	out := make([]bool, len(r.countCalls))
	copy(out, r.countCalls)
	return out
}

func TestApplyFlaggedAccountSideEffects_PassesExcludeCyberFlag(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{}},
		repo, nil, nil, nil, nil, nil,
	)
	userID := int64(42)

	cfgExclude := defaultContentModerationConfig()
	cfgExclude.CyberPolicyExcludeFromBanCount = true
	svc.applyFlaggedAccountSideEffects(context.Background(), cfgExclude, &ContentModerationLog{Flagged: true, UserID: &userID})

	cfgDefault := defaultContentModerationConfig() // 默认 false
	svc.applyFlaggedAccountSideEffects(context.Background(), cfgDefault, &ContentModerationLog{Flagged: true, UserID: &userID})

	require.Equal(t, []bool{true, false}, repo.snapshotCountCalls(),
		"applyFlaggedAccountSideEffects 必须把 cfg.CyberPolicyExcludeFromBanCount 透传给 COUNT 查询")
}

func TestRecordCyberPolicyEvent_ExcludeFromBanCount_SkipsBanJudgment(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled: "true",
			SettingKeyContentModerationConfig: cyberPolicyTestConfig(t, func(cfg *ContentModerationConfig) {
				cfg.CyberPolicyExcludeFromBanCount = true
			}),
		}},
		repo, nil, nil, nil, nil, nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamStatus:  400,
	})

	require.Empty(t, repo.snapshotCountCalls(), "开关开时不得执行封号计数查询")
	logs := repo.snapshotLogs()
	require.Len(t, logs, 1, "风控日志必须照记")
	require.True(t, logs[0].Flagged, "日志仍标记 Flagged=true（列表可见可筛）")
	require.Equal(t, "cyber_policy", logs[0].Action)
	require.Equal(t, 0, logs[0].ViolationCount, "不参与计数时 ViolationCount 保持 0")
	require.False(t, logs[0].AutoBanned)
}

func TestRecordCyberPolicyEvent_DefaultCountsTowardBan(t *testing.T) {
	repo := &banCountArgsTestRepo{}
	svc := NewContentModerationService(
		&contentModerationTestSettingRepo{values: map[string]string{
			SettingKeyRiskControlEnabled:      "true",
			SettingKeyContentModerationConfig: cyberPolicyTestConfig(t, nil),
		}},
		repo, nil, nil, nil, nil, nil,
	)

	svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
		UserID:          1,
		UserEmail:       "u@x.com",
		Model:           "gpt-5",
		Endpoint:        "/v1/responses",
		UpstreamMessage: "flagged",
		UpstreamStatus:  400,
	})

	require.Equal(t, []bool{false}, repo.snapshotCountCalls(),
		"默认配置必须执行计数查询且不排除 cyber 行")
	logs := repo.snapshotLogs()
	require.Len(t, logs, 1)
	require.GreaterOrEqual(t, logs[0].ViolationCount, 1, "默认路径行为不变（现状回归）")
}
