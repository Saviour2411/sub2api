package repository

import (
	"context"
	"database/sql"
	"fmt"
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
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	result := &service.UpstreamSyncResult{
		BalanceUSD: &balance,
		Groups:     []service.UpstreamGroupSnapshot{{RemoteID: "g1", Name: "默认组", Multiplier: float64Ptr(1.5), TodayTokens: 100, TodayCostUSD: 0.5}},
		Daily:      []service.UpstreamDailySnapshot{{Date: date, BalanceUSD: &balance, Tokens: 100, CostUSD: 0.5}},
	}
	next := now.Add(5 * time.Minute)
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "updated-encrypted", now, &next))

	result.Daily[0].Tokens = 150
	result.Daily[0].CostUSD = 0.75
	result.Groups[0].TodayTokens = 150
	require.NoError(t, repo.CommitSync(ctx, site.ID, result, "", now.Add(time.Minute), &next))

	updated, err := repo.GetByID(ctx, site.ID)
	require.NoError(t, err)
	require.Equal(t, service.UpstreamStatusHealthy, updated.Status)
	require.Equal(t, int64(150), updated.TodayTokens)
	require.Equal(t, int64(150), updated.TotalTokens)
	require.InDelta(t, 0.75, updated.TotalCostUSD, 1e-9)
	require.Equal(t, "updated-encrypted", updated.CredentialEncrypted)
	history, err := repo.ListHistory(ctx, site.ID, date, date)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, int64(150), history[0].Tokens)
	groups, err := repo.ListGroups(ctx, site.ID)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(150), groups[0].TodayTokens)

	require.NoError(t, repo.Delete(ctx, site.ID))
	groupCount, err := client.UpstreamGroup.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, groupCount)
	historyCount, err := client.UpstreamDailyStat.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, historyCount)
}

func float64Ptr(value float64) *float64 { return &value }
