//go:build unit

package service

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
)

func TestCanDailyCheckinRoleAllowsUsersAndAdmins(t *testing.T) {
	if !canDailyCheckinRole(RoleUser) {
		t.Fatalf("普通用户应该允许每日签到")
	}
	if !canDailyCheckinRole(RoleAdmin) {
		t.Fatalf("管理员应该允许每日签到")
	}
	if canDailyCheckinRole("guest") {
		t.Fatalf("非用户和管理员角色不应该允许每日签到")
	}
}

func TestPublicPrizeViewsHideProbabilityFields(t *testing.T) {
	prizes := []DailyCheckinPrizeView{
		{
			DailyCheckinPrizeConfig: DailyCheckinPrizeConfig{
				ID:             "balance_1",
				Name:           "余额奖励",
				Type:           DailyCheckinPrizeTypeBalance,
				ProbabilityBps: 7500,
				Enabled:        true,
				SortOrder:      1,
				BalanceMode:    "fixed",
				Amount:         1.5,
			},
			EffectiveProbabilityBps: 5000,
		},
		{
			DailyCheckinPrizeConfig: DailyCheckinPrizeConfig{
				ID:             "hidden",
				Name:           "不可中奖",
				Type:           DailyCheckinPrizeTypeNone,
				ProbabilityBps: 2500,
				Enabled:        true,
				SortOrder:      2,
			},
			EffectiveProbabilityBps: 0,
		},
	}

	publicPrizes := publicPrizeViews(prizes)
	if len(publicPrizes) != 1 {
		t.Fatalf("公开奖品应只返回有效奖品，got %d", len(publicPrizes))
	}
	if publicPrizes[0].ID != "balance_1" || publicPrizes[0].Amount != 1.5 {
		t.Fatalf("公开奖品基础字段不正确: %+v", publicPrizes[0])
	}

	payload, err := json.Marshal(publicPrizes)
	if err != nil {
		t.Fatalf("序列化公开奖品失败: %v", err)
	}
	body := string(payload)
	for _, forbidden := range []string{"probability_bps", "effective_probability_bps", "enabled"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("公开奖品不应包含 %s 字段: %s", forbidden, body)
		}
	}
}

func TestQueryDailyCheckinRecordsReturnsConcurrencyDelta(t *testing.T) {
	tests := []struct {
		name      string
		before    sql.NullInt64
		after     sql.NullInt64
		wantValue int
		wantOK    bool
	}{
		{
			name:      "returns delta when before and after exist",
			before:    sql.NullInt64{Int64: 5, Valid: true},
			after:     sql.NullInt64{Int64: 6, Valid: true},
			wantValue: 1,
			wantOK:    true,
		},
		{
			name:      "keeps legacy after value when before is missing",
			after:     sql.NullInt64{Int64: 6, Valid: true},
			wantValue: 6,
			wantOK:    true,
		},
		{
			name:   "returns empty when no concurrency values exist",
			before: sql.NullInt64{Int64: 5, Valid: true},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := dailyCheckinConcurrencyDelta(tt.before, tt.after)
			if gotValue != tt.wantValue || gotOK != tt.wantOK {
				t.Fatalf("dailyCheckinConcurrencyDelta() = (%d, %v), want (%d, %v)", gotValue, gotOK, tt.wantValue, tt.wantOK)
			}
		})
	}
}
