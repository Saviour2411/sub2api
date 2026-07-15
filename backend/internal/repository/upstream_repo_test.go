package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestUpstreamRepositoryCommitSyncIdempotentAndCascadeDelete(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Now().In(loc)
	site := &service.UpstreamSite{
		Name: "测试站点", BaseURL: "https://example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, Account: "admin", CredentialEncrypted: "encrypted",
		Enabled: true, Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, repo.Create(ctx, site))

	balance := 10.0
	legacyBalance := 42.0
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	legacyDate := date.AddDate(0, 0, -1)
	longDescription := strings.Repeat("x", 1201)
	_, err = client.UpstreamDailyStat.Create().
		SetSiteID(site.ID).
		SetUsageDate(legacyDate).
		SetBalanceUsd(legacyBalance).
		SetCostUsd(999).
		SetCostBasisVersion(1).
		Save(ctx)
	require.NoError(t, err)
	result := &service.UpstreamSyncResult{
		BalanceUSD: &balance,
		Groups: []service.UpstreamGroupSnapshot{{
			RemoteID: "g1", Name: "默认组", Platform: "OpenAI", Description: longDescription,
			Multiplier: float64Ptr(1.5), TodayTokens: 100, TodayCostUSD: 0.5,
		}},
		Daily: []service.UpstreamDailySnapshot{
			{Date: date, BalanceUSD: &balance, Tokens: 100, CostUSD: 0.5},
			{Date: legacyDate, Tokens: 50, CostUSD: 0.25},
		},
	}
	next := now.Add(5 * time.Minute)
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "updated-encrypted", now, &next))

	result.Daily[0].Tokens = 150
	result.Daily[0].CostUSD = 0.75
	result.Groups[0].TodayTokens = 150
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now.Add(time.Minute), &next))
	historyPointCount, err := client.UpstreamGroupMultiplierHistory.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, historyPointCount, "倍率没有变化时不应保存重复快照")

	result.Groups[0].Multiplier = float64Ptr(2)
	result.Groups[0].Description = "倍率已调整"
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now.Add(2*time.Minute), &next))

	updated, err := repo.GetByID(ctx, site.ID)
	require.NoError(t, err)
	require.Equal(t, service.UpstreamStatusHealthy, updated.Status)
	require.Equal(t, int64(150), updated.TodayTokens)
	require.Equal(t, int64(200), updated.TotalTokens)
	require.Equal(t, "updated-encrypted", updated.CredentialEncrypted)
	history, err := repo.ListHistory(ctx, site.ID, legacyDate, date)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, int64(50), history[0].Tokens)
	require.Equal(t, service.UpstreamCostBasisActual, history[0].CostBasisVersion)
	require.NotNil(t, history[0].BalanceUSD)
	require.InDelta(t, legacyBalance, *history[0].BalanceUSD, 1e-9, "实际扣费回填不能清空已有余额样本")
	require.Equal(t, int64(150), history[1].Tokens)
	require.Equal(t, service.UpstreamCostBasisActual, history[1].CostBasisVersion)
	require.InDelta(t, 1.0, updated.TotalCostUSD, 1e-9, "实际消耗只能汇总版本 2 成本")
	groups, err := repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(150), groups[0].TodayTokens)
	require.Equal(t, "倍率已调整", groups[0].Description)
	require.Equal(t, 2.0, *groups[0].Multiplier)

	historyPointCount, err = client.UpstreamGroupMultiplierHistory.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, historyPointCount)
	multiplierHistory, err := repo.ListMultiplierHistory(ctx, site.ID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, multiplierHistory, 1)
	require.Equal(t, "g1", multiplierHistory[0].RemoteID)
	require.Equal(t, "倍率已调整", multiplierHistory[0].Description)
	require.Equal(t, 2.0, *multiplierHistory[0].CurrentMultiplier)
	require.Len(t, multiplierHistory[0].Points, 3)
	require.Equal(t, 1.5, *multiplierHistory[0].Points[0].Multiplier)
	require.Equal(t, 2.0, *multiplierHistory[0].Points[1].Multiplier)
	require.Equal(t, now.Add(time.Hour), multiplierHistory[0].Points[2].RecordedAt)

	withoutGroups := &service.UpstreamSyncResult{BalanceUSD: &balance, Daily: result.Daily}
	require.NoError(t, repo.CommitSync(ctx, site.ID, withoutGroups, "", now.Add(150*time.Second), &next))
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now.Add(160*time.Second), &next))
	historyPointCount, err = client.UpstreamGroupMultiplierHistory.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, historyPointCount, "分组消失后以相同倍率恢复时不应保存重复快照")

	failed := &service.UpstreamSyncResult{
		Groups: []service.UpstreamGroupSnapshot{{RemoteID: "", Name: "无效分组"}},
		Daily:  []service.UpstreamDailySnapshot{{Date: date, Tokens: 999, CostUSD: 999}},
	}
	require.Error(t, repo.CommitSync(ctx, site.ID, failed, "", now.Add(3*time.Minute), &next))
	history, err = repo.ListHistory(ctx, site.ID, date, date)
	require.NoError(t, err)
	require.Equal(t, int64(150), history[0].Tokens, "失败事务不能覆盖旧历史")
	groups, err = repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Equal(t, "g1", groups[0].RemoteID, "失败事务不能覆盖当前分组")
	historyPointCount, err = client.UpstreamGroupMultiplierHistory.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, historyPointCount, "失败事务不能留下倍率记录")

	require.NoError(t, repo.Delete(ctx, site.ID))
	groupCount, err := client.UpstreamGroup.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, groupCount)
	historyCount, err := client.UpstreamDailyStat.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, historyCount)
	historyPointCount, err = client.UpstreamGroupMultiplierHistory.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, historyPointCount)
}

func TestUpstreamRepositoryGroupDisplayLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-display-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	site := &service.UpstreamSite{
		Name: "展示测试站点", BaseURL: "https://example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, Account: "admin", CredentialEncrypted: "encrypted",
		Enabled: true, Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, repo.Create(ctx, site))
	balance := 10.0
	result := &service.UpstreamSyncResult{
		BalanceUSD: &balance,
		Groups: []service.UpstreamGroupSnapshot{{
			RemoteID: "vip", Name: "VIP", Platform: "OpenAI", Description: "高优先级",
			Multiplier: float64Ptr(1.5), TodayTokens: 123, TodayCostUSD: 4.5,
		}},
	}
	next := now.Add(5 * time.Minute)
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now, &next))

	groups, err := repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.False(t, groups[0].Displayed, "新同步分组默认不展示")
	require.True(t, groups[0].Available)

	displayed, err := repo.SetGroupDisplayed(ctx, site.ID, "vip", true)
	require.NoError(t, err)
	require.Equal(t, 1, displayed.DisplayedGroupCount)
	require.True(t, displayed.Group.Displayed)

	result.Groups[0].TodayTokens = 456
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now.Add(time.Minute), &next))
	groups, err = repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.True(t, groups[0].Displayed, "同步更新不能覆盖展示选择")
	require.Equal(t, int64(456), groups[0].TodayTokens)

	require.NoError(t, repo.CommitSync(ctx, site.ID, &service.UpstreamSyncResult{BalanceUSD: &balance}, "", now.Add(2*time.Minute), &next))
	groups, err = repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.True(t, groups[0].Displayed)
	require.False(t, groups[0].Available)
	require.Equal(t, int64(456), groups[0].TodayTokens, "暂不可用占位保留末次指标")

	items, _, err := repo.List(ctx, service.UpstreamListParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, 1, items[0].DisplayedGroupCount)

	hidden, err := repo.SetGroupDisplayed(ctx, site.ID, "vip", false)
	require.NoError(t, err)
	require.Zero(t, hidden.DisplayedGroupCount)
	groups, err = repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Empty(t, groups, "隐藏暂不可用分组后应清理占位记录")

	_, err = repo.SetGroupDisplayed(ctx, site.ID, "vip", true)
	require.ErrorIs(t, err, service.ErrUpstreamGroupNotFound)
}

func TestUpstreamRepositoryListMultiplierHistoryIncludesDisappearedGroups(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-multiplier-history-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	site := &service.UpstreamSite{
		Name: "倍率历史站点", BaseURL: "https://example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, Account: "admin", CredentialEncrypted: "encrypted",
		Enabled: true, Status: service.UpstreamStatusHealthy, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, repo.Create(ctx, site))

	_, err = client.UpstreamGroup.Create().
		SetSiteID(site.ID).
		SetRemoteID("current").
		SetName("Bravo").
		SetPlatform("OpenAI").
		SetDescription("当前描述").
		SetMultiplier(3).
		Save(ctx)
	require.NoError(t, err)
	for _, point := range []struct {
		remoteID    string
		name        string
		description string
		multiplier  float64
		recordedAt  time.Time
	}{
		{remoteID: "z-removed", name: "Alpha", description: "旧描述", multiplier: 1, recordedAt: now.Add(-30 * time.Minute)},
		{remoteID: "z-removed", name: "Alpha", description: "最后描述", multiplier: 2, recordedAt: now.Add(-10 * time.Minute)},
		{remoteID: "a-removed", name: "Alpha", description: "另一个分组", multiplier: 1.5, recordedAt: now.Add(-20 * time.Minute)},
		{remoteID: "current", name: "历史名称", description: "历史描述", multiplier: 2.5, recordedAt: now.Add(-15 * time.Minute)},
	} {
		_, err = client.UpstreamGroupMultiplierHistory.Create().
			SetSiteID(site.ID).
			SetRemoteID(point.remoteID).
			SetName(point.name).
			SetPlatform("OpenAI").
			SetDescription(point.description).
			SetMultiplier(point.multiplier).
			SetRecordedAt(point.recordedAt).
			Save(ctx)
		require.NoError(t, err)
	}

	history, err := repo.ListMultiplierHistory(ctx, site.ID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, []string{"a-removed", "z-removed", "current"}, []string{history[0].RemoteID, history[1].RemoteID, history[2].RemoteID})
	require.Equal(t, "最后描述", history[1].Description)
	require.NotNil(t, history[1].CurrentMultiplier)
	require.InDelta(t, 2.0, *history[1].CurrentMultiplier, 1e-9)
	require.Len(t, history[1].Points, 3)
	require.Equal(t, "Bravo", history[2].Name, "当前分组元数据应优先于历史快照")
	require.NotNil(t, history[2].CurrentMultiplier)
	require.InDelta(t, 3.0, *history[2].CurrentMultiplier, 1e-9)
}

func TestUpstreamRepositoryListFilterSortAndManualOrder(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-list-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, site := range []*service.UpstreamSite{
		{Name: "站点 A", BaseURL: "https://a.example.com", Platform: service.UpstreamPlatformNewAPI, AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "enc", Enabled: true, Status: service.UpstreamStatusHealthy, TrackingStartedAt: now, CreatedBy: 1, SortOrder: 30},
		{Name: "站点 B", BaseURL: "https://b.example.com", Platform: service.UpstreamPlatformSub2API, AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "enc", Enabled: true, Status: service.UpstreamStatusHealthy, TrackingStartedAt: now, CreatedBy: 1, SortOrder: 10},
		{Name: "站点 C", BaseURL: "https://c.example.com", Platform: service.UpstreamPlatformNewAPI, AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "enc", Enabled: true, Status: service.UpstreamStatusHealthy, TrackingStartedAt: now, CreatedBy: 1, SortOrder: 20},
	} {
		require.NoError(t, repo.Create(ctx, site))
	}
	for _, group := range []struct {
		siteID   int64
		name     string
		platform string
	}{
		{1, "A GPT", "OpenAI"}, {2, "B Claude", "Anthropic"}, {3, "C Gemini", "Gemini"},
	} {
		_, err = client.UpstreamGroup.Create().SetSiteID(group.siteID).SetRemoteID(group.name).SetName(group.name).SetPlatform(group.platform).Save(ctx)
		require.NoError(t, err)
	}
	require.NoError(t, client.UpstreamSite.UpdateOneID(1).SetBalanceUsd(10).SetTodayTokens(300).Exec(ctx))
	require.NoError(t, client.UpstreamSite.UpdateOneID(2).SetBalanceUsd(30).SetTodayTokens(100).Exec(ctx))
	require.NoError(t, client.UpstreamSite.UpdateOneID(3).SetBalanceUsd(20).SetTodayTokens(200).Exec(ctx))

	items, total, err := repo.List(ctx, service.UpstreamListParams{Page: 1, PageSize: 20, GroupPlatform: "OpenAI"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "站点 A", items[0].Name)

	items, _, err = repo.List(ctx, service.UpstreamListParams{Page: 1, PageSize: 20, SortBy: "balance_usd", SortOrder: "desc"})
	require.NoError(t, err)
	require.Equal(t, []string{"站点 B", "站点 C", "站点 A"}, []string{items[0].Name, items[1].Name, items[2].Name})
	items, _, err = repo.List(ctx, service.UpstreamListParams{Page: 1, PageSize: 20, SortBy: "today_tokens", SortOrder: "asc"})
	require.NoError(t, err)
	require.Equal(t, []string{"站点 B", "站点 C", "站点 A"}, []string{items[0].Name, items[1].Name, items[2].Name})

	require.NoError(t, repo.UpdateSortOrder(ctx, []service.UpstreamSortOrderUpdate{{ID: 3, SortOrder: 0}, {ID: 1, SortOrder: 10}, {ID: 2, SortOrder: 20}}))
	items, _, err = repo.List(ctx, service.UpstreamListParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, []string{"站点 C", "站点 A", "站点 B"}, []string{items[0].Name, items[1].Name, items[2].Name})
}

func TestUpstreamRepositoryMissingDatesBackfillsLegacyCostBasisWithin366Days(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-missing-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	today := time.Date(2026, 7, 15, 0, 0, 0, 0, loc)
	site := &service.UpstreamSite{
		Name: "回填站点", BaseURL: "https://example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, Account: "admin", CredentialEncrypted: "encrypted",
		Enabled: true, Status: service.UpstreamStatusPending, TrackingStartedAt: today.AddDate(0, 0, -400), CreatedBy: 1,
	}
	require.NoError(t, repo.Create(ctx, site))

	actualDate := today.AddDate(0, 0, -10)
	legacyDate := today.AddDate(0, 0, -5)
	_, err = client.UpstreamDailyStat.Create().
		SetSiteID(site.ID).
		SetUsageDate(actualDate).
		SetCostBasisVersion(service.UpstreamCostBasisActual).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UpstreamDailyStat.Create().
		SetSiteID(site.ID).
		SetUsageDate(legacyDate).
		SetCostBasisVersion(1).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UpstreamDailyStat.Create().
		SetSiteID(site.ID).
		SetUsageDate(today).
		SetCostBasisVersion(service.UpstreamCostBasisActual).
		Save(ctx)
	require.NoError(t, err)

	missing, err := repo.MissingDates(ctx, site.ID, site.TrackingStartedAt, today, loc)
	require.NoError(t, err)
	require.Len(t, missing, 365)
	require.False(t, containsUpstreamDate(missing, actualDate, loc), "版本 2 的历史日期不应重复回填")
	require.True(t, containsUpstreamDate(missing, legacyDate, loc), "版本 1 的历史日期必须回填")
	require.True(t, containsUpstreamDate(missing, today, loc), "今天必须始终刷新")
	require.False(t, missing[0].Before(today.AddDate(0, 0, -365)), "回填范围不能超过 366 天")
}

func containsUpstreamDate(dates []time.Time, target time.Time, loc *time.Location) bool {
	want := target.In(loc).Format("2006-01-02")
	for _, date := range dates {
		if date.In(loc).Format("2006-01-02") == want {
			return true
		}
	}
	return false
}

func float64Ptr(value float64) *float64 { return &value }
