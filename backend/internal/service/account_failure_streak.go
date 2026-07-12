package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AccountFailureStreakSource 标识独立计算连续失败次数的故障来源。
type AccountFailureStreakSource string

const (
	AccountFailureStreakSourceFirstTokenTimeout AccountFailureStreakSource = "first_token_timeout"
	AccountFailureStreakSourceUpstreamError     AccountFailureStreakSource = "upstream_error"
)

// AccountFailureStreakOutcome 表示一次按发生时间排序的账号结果。
type AccountFailureStreakOutcome string

const (
	AccountFailureStreakOutcomeIncrement AccountFailureStreakOutcome = "increment"
	AccountFailureStreakOutcomeReset     AccountFailureStreakOutcome = "reset"
)

// AccountFailureStreakState 是原子更新后的连续失败状态。
type AccountFailureStreakState struct {
	Count          int64
	Applied        bool
	PolicyRevision int64
}

// AccountFailureStreakPolicy 标识一次不可回退的失败策略代次。
type AccountFailureStreakPolicy struct {
	Revision    int64
	Fingerprint string
}

// AccountFailureStreakEvent 是一次不可变的账号结果事件。
type AccountFailureStreakEvent struct {
	OccurredAt time.Time
	ID         string
}

// AccountFailureOutcomeSnapshot 固化账号结果形成时的事件与失败策略。
// 延迟结算只能使用该快照，不能把旧结果套用到后来保存的新策略。
type AccountFailureOutcomeSnapshot struct {
	Event    AccountFailureStreakEvent
	Settings GatewaySettings
}

// NewAccountFailureStreakEvent 为结果形成时刻生成唯一事件标识。
func NewAccountFailureStreakEvent(occurredAt time.Time) AccountFailureStreakEvent {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	return AccountFailureStreakEvent{
		OccurredAt: occurredAt.UTC(),
		ID:         uuid.NewString(),
	}
}

// AccountFailureStreakCache 原子维护每个账号、每类故障的连续失败次数。
type AccountFailureStreakCache interface {
	ApplyOutcome(
		ctx context.Context,
		accountID int64,
		source AccountFailureStreakSource,
		policy AccountFailureStreakPolicy,
		outcome AccountFailureStreakOutcome,
		event AccountFailureStreakEvent,
	) (AccountFailureStreakState, error)
}

// BuildAccountFailureStreakPolicy 构造带单调代次的缓存策略。
func BuildAccountFailureStreakPolicy(source AccountFailureStreakSource, settings GatewaySettings) AccountFailureStreakPolicy {
	revision := settings.FailurePolicyRevision
	if revision <= 0 {
		revision = DefaultGatewayFailurePolicyRevision
	}
	return AccountFailureStreakPolicy{
		Revision:    revision,
		Fingerprint: BuildAccountFailureStreakPolicyFingerprint(source, settings),
	}
}

// BuildGatewayFailurePolicyFingerprint 仅包含会改变连续失败语义的四项配置。
func BuildGatewayFailurePolicyFingerprint(settings GatewaySettings) string {
	return fmt.Sprintf(
		"first_token={%s};upstream={%s}",
		BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceFirstTokenTimeout, settings),
		BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceUpstreamError, settings),
	)
}

// BuildAccountFailureStreakPolicyFingerprint 为失败策略生成稳定指纹。
// 策略变更后，缓存会从下一条新事件开始重新计算连续次数。
func BuildAccountFailureStreakPolicyFingerprint(source AccountFailureStreakSource, settings GatewaySettings) string {
	switch source {
	case AccountFailureStreakSourceFirstTokenTimeout:
		return fmt.Sprintf(
			"timeout_seconds=%d;threshold=%d",
			settings.FirstTokenTimeoutSeconds,
			settings.FirstTokenTimeoutConsecutiveThreshold,
		)
	case AccountFailureStreakSourceUpstreamError:
		codes := normalizeRetryStatusCodes(settings.UpstreamErrorStatusCodes)
		parts := make([]string, len(codes))
		for i, code := range codes {
			parts[i] = strconv.Itoa(code)
		}
		return fmt.Sprintf(
			"status_codes=%s;threshold=%d",
			strings.Join(parts, ","),
			settings.UpstreamErrorConsecutiveThreshold,
		)
	default:
		return string(source)
	}
}
