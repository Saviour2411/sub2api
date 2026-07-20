package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccountgroup "github.com/Wei-Shaw/sub2api/ent/accountgroup"
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

func TestUpstreamRepositoryBindingsReorderAndLifecycle(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-bindings-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = db.Exec(`CREATE TABLE scheduler_outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		account_id INTEGER,
		group_id INTEGER,
		payload BLOB,
		dedup_key TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	repo := NewUpstreamRepository(client)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	countBulkOutbox := func() int {
		var count int
		require.NoError(t, db.QueryRow(
			"SELECT COUNT(*) FROM scheduler_outbox WHERE event_type = ?",
			service.SchedulerOutboxEventAccountBulkChanged,
		).Scan(&count))
		return count
	}

	createSite := func(name, baseURL string) *service.UpstreamSite {
		site := &service.UpstreamSite{
			Name: name, BaseURL: baseURL, Platform: service.UpstreamPlatformSub2API,
			AuthMode: service.UpstreamAuthPassword, Account: "admin", CredentialEncrypted: "encrypted",
			Enabled: true, Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
		}
		require.NoError(t, repo.Create(ctx, site))
		return site
	}
	localGroup, err := client.Group.Create().SetName("本地调度组").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	createAccount := func(name string, priority int, bindToLocalGroup bool) *dbent.Account {
		account, createErr := client.Account.Create().
			SetName(name).
			SetPlatform(service.PlatformOpenAI).
			SetType(service.AccountTypeAPIKey).
			SetCredentials(map[string]any{"api_key": "sk-test"}).
			SetPriority(priority).
			Save(ctx)
		require.NoError(t, createErr)
		if bindToLocalGroup {
			_, createErr = client.AccountGroup.Create().
				SetAccountID(account.ID).
				SetGroupID(localGroup.ID).
				SetPriority(50).
				Save(ctx)
			require.NoError(t, createErr)
		}
		return account
	}
	high := createAccount("高倍率", 70, true)
	lowA := createAccount("低倍率 A", 71, true)
	lowB := createAccount("低倍率 B", 72, true)
	unbound := createAccount("未绑定", 3, true)

	highSite := createSite("高倍率站点", "https://high.example.com")
	lowSite := createSite("低倍率站点", "https://low.example.com")
	syncGroup := func(site *service.UpstreamSite, remoteID, name string, multiplier *float64, syncedAt time.Time) {
		require.NoError(t, repo.CommitSync(ctx, site.ID, &service.UpstreamSyncResult{Groups: []service.UpstreamGroupSnapshot{{
			RemoteID: remoteID, Name: name, Platform: "OpenAI", Multiplier: multiplier,
		}}}, "", syncedAt, nil))
	}
	syncGroup(highSite, "high", "高倍率组", float64Ptr(2), now)
	syncGroup(lowSite, "low", "低倍率组", float64Ptr(0.5), now)
	highGroups, err := repo.ListGroups(ctx, highSite.ID)
	require.NoError(t, err)
	lowGroups, err := repo.ListGroups(ctx, lowSite.ID)
	require.NoError(t, err)
	highGroupID := highGroups[0].ID
	lowGroupID := lowGroups[0].ID
	_, err = repo.SetGroupDisplayed(ctx, highSite.ID, "high", true)
	require.NoError(t, err)
	_, err = repo.SetGroupDisplayed(ctx, lowSite.ID, "low", true)
	require.NoError(t, err)

	_, err = repo.ReplaceGroupBindings(ctx, highSite.ID, highGroupID, []service.UpstreamGroupAccountBindingInput{{
		LocalGroupID: localGroup.ID, AccountID: high.ID,
	}})
	require.NoError(t, err)
	updatedHigh, err := client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, 10, updatedHigh.Priority)

	lowView, err := repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, []service.UpstreamGroupAccountBindingInput{
		{LocalGroupID: localGroup.ID, AccountID: lowA.ID},
		{LocalGroupID: localGroup.ID, AccountID: lowB.ID},
	})
	require.NoError(t, err)
	require.Len(t, lowView.Bindings, 2)
	for _, accountID := range []int64{lowA.ID, lowB.ID} {
		account, getErr := client.Account.Get(ctx, accountID)
		require.NoError(t, getErr)
		require.Equal(t, 10, account.Priority)
	}
	updatedHigh, err = client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, 15, updatedHigh.Priority)
	unchanged, err := client.Account.Get(ctx, unbound.ID)
	require.NoError(t, err)
	require.Equal(t, 3, unchanged.Priority, "未绑定账号不能被自动重排")
	lowSiteView, err := repo.GetByID(ctx, lowSite.ID)
	require.NoError(t, err)
	require.Equal(t, 2, lowSiteView.BindingCount)

	_, err = repo.SetGroupDisplayed(ctx, lowSite.ID, "low", false)
	require.ErrorIs(t, err, service.ErrUpstreamGroupHasBindings)
	_, err = repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, []service.UpstreamGroupAccountBindingInput{
		{LocalGroupID: localGroup.ID, AccountID: lowA.ID},
		{LocalGroupID: localGroup.ID, AccountID: lowB.ID},
		{LocalGroupID: localGroup.ID, AccountID: high.ID},
	})
	require.ErrorIs(t, err, service.ErrUpstreamBindingConflict)

	invalidMultiplier := math.NaN()
	err = repo.CommitSync(ctx, highSite.ID, &service.UpstreamSyncResult{Groups: []service.UpstreamGroupSnapshot{{
		RemoteID: "high", Name: "高倍率组", Multiplier: &invalidMultiplier,
	}}}, "", now.Add(time.Minute), nil)
	require.Error(t, err)

	require.NoError(t, repo.CommitSync(ctx, lowSite.ID, &service.UpstreamSyncResult{}, "", now.Add(2*time.Minute), nil))
	require.NoError(t, client.Account.UpdateOneID(lowA.ID).SetPriority(77).Exec(ctx))
	syncGroup(highSite, "high", "高倍率组", float64Ptr(4), now.Add(3*time.Minute))
	reorderedLow, err := client.Account.Get(ctx, lowA.ID)
	require.NoError(t, err)
	require.Equal(t, 10, reorderedLow.Priority, "不可用上游分组必须使用末次倍率参与排序")
	updatedHigh, err = client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, 15, updatedHigh.Priority)

	require.NoError(t, client.Account.UpdateOneID(lowA.ID).SetPriority(77).Exec(ctx))
	require.NoError(t, repo.CommitSync(ctx, lowSite.ID, &service.UpstreamSyncResult{}, "", now.Add(4*time.Minute), nil))
	reorderedLow, err = client.Account.Get(ctx, lowA.ID)
	require.NoError(t, err)
	require.Equal(t, 10, reorderedLow.Priority, "无倍率变化的成功同步也必须纠正绑定账号优先级")
	_, err = repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, []service.UpstreamGroupAccountBindingInput{
		{LocalGroupID: localGroup.ID, AccountID: lowA.ID},
		{LocalGroupID: localGroup.ID, AccountID: lowB.ID},
		{LocalGroupID: localGroup.ID, AccountID: unbound.ID},
	})
	require.ErrorIs(t, err, service.ErrUpstreamGroupMultiplierUnavailable, "不可用上游分组不能新增绑定")

	require.NoError(t, client.Account.UpdateOneID(lowA.ID).SetPriority(77).Exec(ctx))
	syncGroup(lowSite, "low", "低倍率组", nil, now.Add(5*time.Minute))
	for _, accountID := range []int64{lowA.ID, lowB.ID} {
		account, getErr := client.Account.Get(ctx, accountID)
		require.NoError(t, getErr)
		require.Equal(t, 10, account.Priority, "当前倍率为空时必须使用历史中的最后有效倍率")
	}
	_, err = repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, []service.UpstreamGroupAccountBindingInput{
		{LocalGroupID: localGroup.ID, AccountID: lowA.ID},
		{LocalGroupID: localGroup.ID, AccountID: lowB.ID},
		{LocalGroupID: localGroup.ID, AccountID: unbound.ID},
	})
	require.ErrorIs(t, err, service.ErrUpstreamGroupMultiplierUnavailable, "当前倍率为空的上游分组不能新增绑定")

	require.NoError(t, client.Account.UpdateOneID(lowA.ID).SetPriority(77).Exec(ctx))
	_, err = repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, []service.UpstreamGroupAccountBindingInput{
		{LocalGroupID: localGroup.ID, AccountID: lowA.ID},
		{LocalGroupID: localGroup.ID, AccountID: lowB.ID},
	})
	require.NoError(t, err)
	manualLow, err := client.Account.Get(ctx, lowA.ID)
	require.NoError(t, err)
	require.Equal(t, 10, manualLow.Priority, "无变化的绑定保存必须纠正管理员手工优先级")

	unchangedUpdatedAt := now.Add(-24 * time.Hour)
	_, err = db.Exec("UPDATE accounts SET updated_at = ? WHERE id = ?", unchangedUpdatedAt, high.ID)
	require.NoError(t, err)
	bulkOutboxBefore := countBulkOutbox()
	syncGroup(highSite, "high", "高倍率组", float64Ptr(4), now.Add(6*time.Minute))
	require.Equal(t, bulkOutboxBefore, countBulkOutbox(), "优先级无变化时不能重复发送账号批量刷新事件")
	updatedHigh, err = client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, unchangedUpdatedAt.Unix(), updatedHigh.UpdatedAt.Unix(), "优先级无变化时不能更新账号 updated_at")

	require.NoError(t, client.Account.UpdateOneID(lowA.ID).SetPriority(77).Exec(ctx))
	_, err = repo.ReplaceGroupBindings(ctx, lowSite.ID, lowGroupID, nil)
	require.NoError(t, err)
	manualLow, err = client.Account.Get(ctx, lowA.ID)
	require.NoError(t, err)
	require.Equal(t, 77, manualLow.Priority, "解绑账号应保留当前手工优先级")
	lowBAfterUnbind, err := client.Account.Get(ctx, lowB.ID)
	require.NoError(t, err)
	require.Equal(t, 10, lowBAfterUnbind.Priority, "解绑账号应保留最后一次自动优先级")
	updatedHigh, err = client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, 10, updatedHigh.Priority, "剩余绑定账号应从 10 重新排名")

	require.NoError(t, repo.Delete(ctx, highSite.ID))
	updatedHigh, err = client.Account.Get(ctx, high.ID)
	require.NoError(t, err)
	require.Equal(t, 10, updatedHigh.Priority, "站点删除解绑后应保留账号当前优先级")
	var outboxCount int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM scheduler_outbox").Scan(&outboxCount))
	require.Greater(t, outboxCount, 0)
	require.Greater(t, countBulkOutbox(), 0, "优先级变化必须发送账号批量刷新事件")
}

func TestRecalculateUpstreamBindingPrioritiesKeepsAccountWithoutValidMultiplier(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:upstream-bindings-missing-rate-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = db.Exec(`CREATE TABLE scheduler_outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		account_id INTEGER,
		group_id INTEGER,
		payload BLOB,
		dedup_key TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	site := &service.UpstreamSite{
		Name: "无倍率站点", BaseURL: "https://missing-rate.example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "encrypted", Enabled: true,
		Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, NewUpstreamRepository(client).Create(ctx, site))
	knownGroup, err := client.UpstreamGroup.Create().
		SetSiteID(site.ID).
		SetRemoteID("known").
		SetName("已知倍率").
		SetMultiplier(0.5).
		SetDisplayed(true).
		SetLastSyncedAt(now).
		Save(ctx)
	require.NoError(t, err)
	unknownGroup, err := client.UpstreamGroup.Create().
		SetSiteID(site.ID).
		SetRemoteID("unknown").
		SetName("未知倍率").
		SetDisplayed(true).
		SetLastSyncedAt(now).
		Save(ctx)
	require.NoError(t, err)
	localGroup, err := client.Group.Create().SetName("本地分组").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	createAccount := func(name string, priority int) *dbent.Account {
		account, createErr := client.Account.Create().
			SetName(name).
			SetPlatform(service.PlatformOpenAI).
			SetType(service.AccountTypeAPIKey).
			SetCredentials(map[string]any{"api_key": "sk-test"}).
			SetPriority(priority).
			Save(ctx)
		require.NoError(t, createErr)
		_, createErr = client.AccountGroup.Create().
			SetAccountID(account.ID).
			SetGroupID(localGroup.ID).
			SetPriority(1).
			Save(ctx)
		require.NoError(t, createErr)
		return account
	}
	knownAccount := createAccount("已知倍率账号", 70)
	unknownAccount := createAccount("未知倍率账号", 77)
	_, err = client.UpstreamGroupAccountBinding.Create().
		SetUpstreamGroupID(knownGroup.ID).
		SetLocalGroupID(localGroup.ID).
		SetAccountID(knownAccount.ID).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UpstreamGroupAccountBinding.Create().
		SetUpstreamGroupID(unknownGroup.ID).
		SetLocalGroupID(localGroup.ID).
		SetAccountID(unknownAccount.ID).
		Save(ctx)
	require.NoError(t, err)

	updatedIDs, err := recalculateUpstreamBindingPriorities(ctx, client, []int64{localGroup.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{knownAccount.ID}, updatedIDs)
	knownAccount, err = client.Account.Get(ctx, knownAccount.ID)
	require.NoError(t, err)
	require.Equal(t, 10, knownAccount.Priority)
	unknownAccount, err = client.Account.Get(ctx, unknownAccount.ID)
	require.NoError(t, err)
	require.Equal(t, 77, unknownAccount.Priority, "当前和历史均无有效倍率时必须保留账号原优先级")
}

func TestNormalizeUpstreamBindingMultiplier(t *testing.T) {
	left := 1.0000001
	right := 1.0000002
	require.True(t, equalNormalizedOptionalFloat(&left, &right))
	_, ok := normalizeUpstreamBindingMultiplier(math.Inf(1))
	require.False(t, ok)
	_, ok = normalizeUpstreamBindingMultiplier(-0.1)
	require.False(t, ok)
	zero, ok := normalizeUpstreamBindingMultiplier(0)
	require.True(t, ok)
	require.Zero(t, zero)
}

func TestLockUpstreamBindingAccountsByIDPostgres(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil).AddRow(nil))
	mock.ExpectClose()

	require.NoError(t, lockUpstreamBindingAccountsByID(context.Background(), client, []int64{9, 3, 9}))
	require.NoError(t, client.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepositoryCleansBindingsOnMembershipChangeAndDelete(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:account-upstream-bindings-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = db.Exec(`CREATE TABLE scheduler_outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		account_id INTEGER,
		group_id INTEGER,
		payload BLOB,
		dedup_key TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE UNIQUE INDEX scheduler_outbox_dedup_unique
		ON scheduler_outbox (dedup_key) WHERE dedup_key IS NOT NULL`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE scheduled_test_plans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_id INTEGER NOT NULL
	)`)
	require.NoError(t, err)
	ctx := context.Background()
	upstreamRepo := NewUpstreamRepository(client)
	accountRepo := newAccountRepositoryWithSQL(client, db, nil)
	now := time.Now().UTC().Truncate(time.Second)
	site := &service.UpstreamSite{
		Name: "账号生命周期站点", BaseURL: "https://lifecycle.example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "encrypted", Enabled: true,
		Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, upstreamRepo.Create(ctx, site))
	require.NoError(t, upstreamRepo.CommitSync(ctx, site.ID, &service.UpstreamSyncResult{Groups: []service.UpstreamGroupSnapshot{
		{RemoteID: "first", Name: "第一组", Multiplier: float64Ptr(1)},
		{RemoteID: "second", Name: "第二组", Multiplier: float64Ptr(2)},
	}}, "", now, nil))
	_, err = upstreamRepo.SetGroupDisplayed(ctx, site.ID, "first", true)
	require.NoError(t, err)
	_, err = upstreamRepo.SetGroupDisplayed(ctx, site.ID, "second", true)
	require.NoError(t, err)
	groups, err := upstreamRepo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	groupIDByRemoteID := map[string]int64{groups[0].RemoteID: groups[0].ID, groups[1].RemoteID: groups[1].ID}

	localGroup, err := client.Group.Create().SetName("生命周期本地组").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	createBoundAccount := func(name string) *dbent.Account {
		account, createErr := client.Account.Create().
			SetName(name).
			SetPlatform(service.PlatformOpenAI).
			SetType(service.AccountTypeAPIKey).
			SetCredentials(map[string]any{"api_key": "sk-test"}).
			SetPriority(50).
			Save(ctx)
		require.NoError(t, createErr)
		_, createErr = client.AccountGroup.Create().
			SetAccountID(account.ID).
			SetGroupID(localGroup.ID).
			SetPriority(42).
			Save(ctx)
		require.NoError(t, createErr)
		return account
	}
	first := createBoundAccount("第一账号")
	second := createBoundAccount("第二账号")
	_, err = upstreamRepo.ReplaceGroupBindings(ctx, site.ID, groupIDByRemoteID["first"], []service.UpstreamGroupAccountBindingInput{{
		LocalGroupID: localGroup.ID, AccountID: first.ID,
	}})
	require.NoError(t, err)
	_, err = upstreamRepo.ReplaceGroupBindings(ctx, site.ID, groupIDByRemoteID["second"], []service.UpstreamGroupAccountBindingInput{{
		LocalGroupID: localGroup.ID, AccountID: second.ID,
	}})
	require.NoError(t, err)

	require.NoError(t, accountRepo.BindGroups(ctx, first.ID, []int64{localGroup.ID}))
	membership, err := client.AccountGroup.Query().Where(
		dbaccountgroup.AccountIDEQ(first.ID),
		dbaccountgroup.GroupIDEQ(localGroup.ID),
	).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 42, membership.Priority, "未变化的账号分组关系不能被删除重建")
	bindingCount, err := client.UpstreamGroupAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, bindingCount, "未变化的账号分组关系不能误删倍率绑定")

	require.NoError(t, accountRepo.BindGroups(ctx, first.ID, nil))
	firstAfterUnbind, err := client.Account.Get(ctx, first.ID)
	require.NoError(t, err)
	require.Equal(t, 10, firstAfterUnbind.Priority, "移出本地分组后保留最后自动优先级")
	secondAfterReorder, err := client.Account.Get(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, 10, secondAfterReorder.Priority, "移除绑定后剩余账号必须重排")
	bindingCount, err = client.UpstreamGroupAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, bindingCount)
	_, err = upstreamRepo.ReplaceGroupBindings(ctx, site.ID, groupIDByRemoteID["first"], []service.UpstreamGroupAccountBindingInput{{
		LocalGroupID: localGroup.ID, AccountID: first.ID,
	}})
	require.ErrorIs(t, err, service.ErrUpstreamAccountNotInGroup)

	require.NoError(t, accountRepo.Delete(ctx, second.ID))
	bindingCount, err = client.UpstreamGroupAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, bindingCount, "账号删除必须清理倍率绑定")
}

func TestGroupRepositoryCleansBindingsAndReusesTransaction(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:group-upstream-bindings-%d?mode=memory&cache=shared&_pragma=foreign_keys(1)&_time_format=sqlite", time.Now().UnixNano()))
	require.NoError(t, err)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(entsql.OpenDB(dialect.SQLite, db))))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = db.Exec(`CREATE TABLE scheduler_outbox (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_type TEXT NOT NULL,
		account_id INTEGER,
		group_id INTEGER,
		payload BLOB,
		dedup_key TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE UNIQUE INDEX scheduler_outbox_dedup_unique
		ON scheduler_outbox (dedup_key) WHERE dedup_key IS NOT NULL`)
	require.NoError(t, err)
	ctx := context.Background()
	upstreamRepo := NewUpstreamRepository(client)
	groupRepo := newGroupRepositoryWithSQL(client, db)
	now := time.Now().UTC().Truncate(time.Second)
	site := &service.UpstreamSite{
		Name: "分组生命周期站点", BaseURL: "https://group-lifecycle.example.com", Platform: service.UpstreamPlatformSub2API,
		AuthMode: service.UpstreamAuthPassword, CredentialEncrypted: "encrypted", Enabled: true,
		Status: service.UpstreamStatusPending, TrackingStartedAt: now, CreatedBy: 1,
	}
	require.NoError(t, upstreamRepo.Create(ctx, site))
	require.NoError(t, upstreamRepo.CommitSync(ctx, site.ID, &service.UpstreamSyncResult{Groups: []service.UpstreamGroupSnapshot{{
		RemoteID: "bound", Name: "绑定组", Multiplier: float64Ptr(1),
	}}}, "", now, nil))
	_, err = upstreamRepo.SetGroupDisplayed(ctx, site.ID, "bound", true)
	require.NoError(t, err)
	upstreamGroups, err := upstreamRepo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, upstreamGroups, 1)

	localGroup, err := client.Group.Create().SetName("待清理本地组").SetPlatform(service.PlatformOpenAI).Save(ctx)
	require.NoError(t, err)
	account, err := client.Account.Create().
		SetName("待清理账号").
		SetPlatform(service.PlatformOpenAI).
		SetType(service.AccountTypeAPIKey).
		SetCredentials(map[string]any{"api_key": "sk-test"}).
		SetPriority(50).
		Save(ctx)
	require.NoError(t, err)
	bind := func() {
		_, bindErr := client.AccountGroup.Create().SetAccountID(account.ID).SetGroupID(localGroup.ID).SetPriority(42).Save(ctx)
		require.NoError(t, bindErr)
		_, bindErr = upstreamRepo.ReplaceGroupBindings(ctx, site.ID, upstreamGroups[0].ID, []service.UpstreamGroupAccountBindingInput{{
			LocalGroupID: localGroup.ID, AccountID: account.ID,
		}})
		require.NoError(t, bindErr)
	}
	bind()

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txRepo := newGroupRepositoryWithSQL(tx.Client(), tx.Client())
	affected, err := txRepo.DeleteAccountGroupsByGroupID(ctx, localGroup.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	require.NoError(t, tx.Commit())
	bindingCount, err := client.UpstreamGroupAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, bindingCount, "清空分组账号时必须同步清理倍率绑定")

	bind()
	require.NoError(t, groupRepo.Delete(ctx, localGroup.ID))
	bindingCount, err = client.UpstreamGroupAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, bindingCount, "软删除本地分组前必须显式清理倍率绑定")
	priority, err := client.Account.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, 10, priority.Priority, "解除绑定应保留账号最后一次自动优先级")
}

func float64Ptr(value float64) *float64 { return &value }
