package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestModelQualitySlotsRequireConfigRowLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewModelQualityRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT enabled,version.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"enabled", "version"}).AddRow(true, 3))
	mock.ExpectQuery("SELECT COUNT.*status IN").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectRollback()
	accepted, err := repo.PrepareTask(context.Background(), &service.ModelQualityTask{ID: 1}, 3)
	require.NoError(t, err)
	require.False(t, accepted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelQualityPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("MODEL_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("未配置独立 PostgreSQL 检测测试库")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = base.Close() }()
	schema := "quality_" + uuid.NewString()[:8]
	_, err = base.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	defer func() { _, _ = base.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) }()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(12)
	_, err = db.Exec(`CREATE TABLE settings(key TEXT PRIMARY KEY,value TEXT NOT NULL)`)
	require.NoError(t, err)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "246_model_quality.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	first, second := NewModelQualityRepository(db), NewModelQualityRepository(db)
	ctx := context.Background()
	c, err := first.Config(ctx)
	require.NoError(t, err)
	require.False(t, c.Enabled)
	c.Enabled = true
	c.Groups = []service.ModelQualityGroupConfig{{GroupID: 7, Models: []string{"model-a", "model-b"}, Methods: []string{"candy", "pelican"}, IntervalMinutes: 60, ReasoningEffort: "low"}}
	require.NoError(t, first.SaveConfig(ctx, c))
	var flag string
	require.NoError(t, db.QueryRow(`SELECT value FROM settings WHERE key='model_quality_enabled'`).Scan(&flag))
	require.Equal(t, "true", flag)
	var started atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if first.StartRound(ctx, c, c.Groups[0], "测试分组", "https://example.net/v1", true) == nil {
				started.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, started.Load())
	tasks, err := second.Tasks(ctx)
	require.NoError(t, err)
	require.Len(t, tasks, 4)
	var accepted atomic.Int32
	for _, task := range tasks {
		wg.Add(1)
		go func(task service.ModelQualityTask) {
			defer wg.Done()
			now := time.Now()
			deadline := now.Add(11 * time.Minute)
			task.SubmittedAt = &now
			task.DeadlineAt = &deadline
			task.Attempts = 1
			ok, e := second.PrepareTask(ctx, &task, c.Version)
			if e == nil && ok {
				accepted.Add(1)
			}
		}(task)
	}
	wg.Wait()
	require.EqualValues(t, 2, accepted.Load())
	persisted, err := first.Tasks(ctx)
	require.NoError(t, err)
	active := 0
	for _, task := range persisted {
		if task.Status == "submitting" {
			active++
		}
	}
	require.Equal(t, 2, active)
	// 模拟两种测试各自返回，一项失败不会覆盖另一项的作品或判定。
	for i := range persisted {
		task := &persisted[i]
		now := time.Now()
		task.FinishedAt = &now
		if task.Benchmark == "pelican" {
			task.Status = "completed"
			task.Verdict = "normal"
			task.HTML = "<canvas></canvas>"
			task.RemoteID = "private-id"
		} else {
			task.Status = "failed"
			task.Verdict = "unknown"
			task.Reason = "检测超时"
		}
		require.NoError(t, first.SaveTask(ctx, task))
	}
	require.NoError(t, first.FinishRounds(ctx))
	states, err := first.States(ctx)
	require.NoError(t, err)
	require.False(t, states[7].Running)
	require.True(t, states[7].NextRunAt.After(time.Now().Add(59*time.Minute)))
	runs, total, err := first.Runs(ctx, service.ModelQualityFilter{GroupID: 7, Page: 1})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Greater(t, runs[0].ID, runs[1].ID)
	require.Equal(t, "failed", runs[0].Outcome)
	for _, test := range runs[0].Tests {
		require.Empty(t, test.HTML)
	}
	detail, err := first.Run(ctx, runs[0].ID)
	require.NoError(t, err)
	found := false
	for _, test := range detail.Tests {
		if test.Benchmark == "pelican" {
			require.Equal(t, "<canvas></canvas>", test.HTML)
			require.Equal(t, "normal", test.Verdict)
			found = true
		}
	}
	require.True(t, found)
	require.NoError(t, first.StartRound(ctx, c, c.Groups[0], "测试分组", "https://example.net/v1", false))
	after, _, err := first.Runs(ctx, service.ModelQualityFilter{GroupID: 7, Page: 1})
	require.NoError(t, err)
	require.Len(t, after, 2)
	c.Enabled = false
	require.NoError(t, first.SaveConfig(ctx, c))
	require.Error(t, first.StartRound(ctx, c, c.Groups[0], "测试分组", "https://example.net/v1", true))
	_, err = db.Exec(`UPDATE model_quality_runs SET started_at=NOW()-INTERVAL '25 hours'; UPDATE model_quality_rounds SET started_at=NOW()-INTERVAL '25 hours'`)
	require.NoError(t, err)
	_, err = first.Run(ctx, runs[0].ID)
	require.ErrorIs(t, err, service.ErrModelQualityExpired)
	require.NoError(t, first.Cleanup(ctx))
	for _, table := range []string{"model_quality_runs", "model_quality_tasks", "model_quality_rounds"} {
		var count int
		require.NoError(t, db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count))
		require.Zero(t, count)
	}
	states, err = first.States(ctx)
	require.NoError(t, err)
	require.Nil(t, states[7].LastStartedAt)
	require.False(t, states[7].Running)
	require.Equal(t, "pending", states[7].Status)
	require.NotNil(t, states[7].NextRunAt)
}
