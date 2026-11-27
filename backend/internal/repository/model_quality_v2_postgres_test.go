package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io/fs"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"
)

func qualityV2Postgres(t *testing.T, migrate bool) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MODEL_QUALITY_TEST_DSN")
	if dsn == "" {
		t.Skip("未配置独立PostgreSQL测试库")
	}
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	name := "quality_v2_" + uuid.NewString()[:8]
	_, err = base.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/" + name
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(20)
	t.Cleanup(func() {
		_ = db.Close()
		_, e := base.Exec("DROP DATABASE " + name)
		require.NoError(t, e)
		_ = base.Close()
	})
	_, err = db.Exec("CREATE TABLE settings(id BIGSERIAL PRIMARY KEY,key VARCHAR(100) UNIQUE NOT NULL,value TEXT NOT NULL,updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()); CREATE TABLE schema_migrations(filename TEXT PRIMARY KEY,checksum TEXT NOT NULL,applied_at TIMESTAMPTZ DEFAULT NOW())")
	require.NoError(t, err)
	raw, err := migrations.FS.ReadFile("246_model_quality.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(raw))
	require.NoError(t, err)
	if migrate {
		raw, err = migrations.FS.ReadFile("253_model_quality_v2.sql")
		require.NoError(t, err)
		_, err = db.Exec(string(raw))
		require.NoError(t, err)
	}
	return db
}
func qualityV2ConfigForTest(t *testing.T, r *modelQualityRepository) *service.ModelQualityV2Config {
	t.Helper()
	c, err := r.QualityV2Config(context.Background())
	require.NoError(t, err)
	key := int64(3)
	call := service.ModelQualityCall{GroupID: 7, KeyID: &key, OwnerID: 1, Protocol: "responses", Model: "test-model", MaxOutputTokens: 8192, TimeoutSeconds: 600}
	c.SchemaVersion = 2
	c.Enabled = true
	c.Questions = []service.ModelQualityQuestion{{ID: "number", Name: "数字题", Prompt: "输出36", OutputType: "text", Rule: service.ModelQualityRule{Kind: "number", Expected: "36"}}, {ID: "art", Name: "作品", Prompt: "生成SVG", OutputType: "svg", Rule: service.ModelQualityRule{Kind: "model", Judge: &call, JudgePrompt: "判断"}}}
	c.Groups = []service.ModelQualityV2GroupConfig{{GroupID: 7, IntervalMinutes: 60, Targets: []service.ModelQualityTarget{{ID: "first", ModelQualityCall: call, QuestionIDs: []string{"number", "art"}}, {ID: "second", ModelQualityCall: call, QuestionIDs: []string{"number"}}}}}
	require.NoError(t, r.SaveQualityV2Config(context.Background(), c))
	return c
}
func TestModelQualityV2PostgresConcurrencyRecoveryAndRejudge(t *testing.T) {
	db := qualityV2Postgres(t, true)
	r := &modelQualityRepository{db: db}
	ctx := context.Background()
	c := qualityV2ConfigForTest(t, r)
	require.NoError(t, r.StartQualityV2Round(ctx, c, c.Groups[0], "检测组", true))
	require.ErrorIs(t, r.StartQualityV2Round(ctx, c, c.Groups[0], "检测组", true), service.ErrModelQualityConflict)
	var wg sync.WaitGroup
	var accepted atomic.Int32
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task, err := r.ClaimQualityV2Task(ctx, c.Version, fmt.Sprint(i))
			require.NoError(t, err)
			if task != nil {
				accepted.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.EqualValues(t, 2, accepted.Load())
	_, err := db.Exec("UPDATE model_quality_v2_tasks SET lease_until=NOW()-INTERVAL '1 second' WHERE status='generating'")
	require.NoError(t, err)
	require.NoError(t, r.MaintainQualityV2(ctx))
	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM model_quality_v2_tasks WHERE status='generating'").Scan(&count))
	require.Equal(t, 2, count)
	_, err = db.Exec("UPDATE model_quality_v2_tasks SET deadline_at=NOW()-INTERVAL '1 second' WHERE status='generating'")
	require.NoError(t, err)
	require.NoError(t, r.MaintainQualityV2(ctx))
	require.NoError(t, db.QueryRow("SELECT count(*) FROM model_quality_v2_tasks WHERE status='uncertain'").Scan(&count))
	require.Equal(t, 2, count)
	c.Enabled = false
	require.NoError(t, r.SaveQualityV2Config(ctx, c))
	task, err := r.ClaimQualityV2Task(ctx, c.Version, "disabled")
	require.NoError(t, err)
	require.Nil(t, task)
	require.NoError(t, r.MaintainQualityV2(ctx))
	c.Enabled = true
	require.NoError(t, r.SaveQualityV2Config(ctx, c))
	require.NoError(t, r.StartQualityV2Round(ctx, c, c.Groups[0], "检测组", true))
	var art *service.ModelQualityV2Task
	for i := 0; i < 3; i++ {
		task, err = r.ClaimQualityV2Task(ctx, c.Version, "worker")
		require.NoError(t, err)
		require.NotNil(t, task)
		task.Status = "completed"
		task.Result.Status = "completed"
		task.Result.Verdict = "normal"
		task.Result.Generation = service.ModelQualityPhase{Status: "completed", Answer: "<svg>原作品</svg>", Attempts: 1}
		if task.Specification.Question.ID == "art" {
			art = task
			task.Result.HTML = "<svg>原作品</svg>"
			task.Result.HasHTML = true
			task.Result.Judgment = &service.ModelQualityPhase{Status: "failed", Reason: "网络中断", Attempts: 1}
			task.Status = "failed"
			task.Result.Status = "failed"
			task.Result.Verdict = "unknown"
		}
		ok, e := r.SaveQualityV2Task(ctx, task, "generating")
		require.NoError(t, e)
		require.True(t, ok)
	}
	require.NoError(t, r.MaintainQualityV2(ctx))
	require.NotNil(t, art)
	require.ErrorIs(t, r.RejudgeQualityV2Task(ctx, art.ID, 2), service.ErrModelQualityInvalid)
	require.NoError(t, r.RejudgeQualityV2Task(ctx, art.ID, 1))
	require.ErrorIs(t, r.RejudgeQualityV2Task(ctx, art.ID, 1), service.ErrModelQualityConflict)
	retry, err := r.ClaimQualityV2Task(ctx, c.Version, "other-instance")
	require.NoError(t, err)
	require.NotNil(t, retry)
	require.Equal(t, "judging", retry.Status)
	require.Equal(t, "<svg>原作品</svg>", retry.Result.Generation.Answer)
	require.Len(t, retry.Result.PreviousJudgments, 1)
	stale := *retry
	stale.Owner = "stale"
	ok, err := r.SaveQualityV2Task(ctx, &stale, "judging")
	require.NoError(t, err)
	require.False(t, ok)
	runs, total, err := r.QualityV2Runs(ctx, service.ModelQualityFilter{GroupID: 7, Page: 1, Method: "art"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, runs, 2)
	for _, run := range runs {
		for _, test := range run.Tests {
			require.Empty(t, test.HTML)
			require.Empty(t, test.Generation.Answer)
			for _, p := range test.PreviousJudgments {
				require.Empty(t, p.Answer)
			}
		}
	}
	detail, err := r.QualityV2Run(ctx, "v2-"+art.RunID)
	require.NoError(t, err)
	require.NotEmpty(t, detail.Tests)
	_, err = db.Exec("UPDATE model_quality_v2_runs SET started_at=NOW()-INTERVAL '25 hours' WHERE id=$1", art.RunID)
	require.NoError(t, err)
	_, err = r.QualityV2Run(ctx, "v2-"+art.RunID)
	require.ErrorIs(t, err, service.ErrModelQualityExpired)
}
func TestModelQualityV2PostgresGuard(t *testing.T) {
	for _, mode := range []string{"success", "legacy-enabled", "busy", "collision", "lock", "drift"} {
		t.Run(mode, func(t *testing.T) {
			db := qualityV2Postgres(t, false)
			ctx := context.Background()
			conn, err := db.Conn(ctx)
			require.NoError(t, err)
			defer conn.Close()
			entry := blueGreenQualityV2Migration
			raw, err := migrations.FS.ReadFile(entry.Filename)
			require.NoError(t, err)
			require.NoError(t, validateBlueGreenSQL(entry, string(raw)))
			var blocker *sql.Tx
			switch mode {
			case "legacy-enabled":
				_, err = db.Exec("UPDATE model_quality_config SET enabled=TRUE")
			case "busy":
				_, err = db.Exec("INSERT INTO model_quality_rounds(id,group_id,group_name,parameters,endpoint) VALUES('legacy',7,'旧组','{}','https://example.com'); INSERT INTO model_quality_runs(round_id,model,reasoning_effort) SELECT id,'old','low' FROM model_quality_rounds; INSERT INTO model_quality_tasks(run_id,benchmark,status,idempotency_key) SELECT id,'candy','running','legacy-key' FROM model_quality_runs")
			case "collision":
				_, err = db.Exec("CREATE TABLE model_quality_v2_config (dummy INT)")
			case "lock":
				blocker, err = db.BeginTx(ctx, nil)
				require.NoError(t, err)
				_, err = blocker.Exec("LOCK TABLE model_quality_config IN ACCESS SHARE MODE")
				defer blocker.Rollback()
			}
			require.NoError(t, err)
			started := time.Now()
			err = applyBlueGreenQualityV2(ctx, conn, entry, string(raw))
			if mode != "success" && mode != "drift" {
				require.Error(t, err)
				require.Less(t, time.Since(started), 3*time.Second)
				return
			}
			require.NoError(t, err)
			require.NoError(t, verifyBlueGreenQualityV2Applied(ctx, conn))
			_, err = db.Exec("UPDATE model_quality_config SET enabled=TRUE")
			require.Error(t, err)
			if mode == "drift" {
				_, err = db.Exec("ALTER TABLE model_quality_v2_tasks ADD COLUMN unexpected TEXT")
				require.NoError(t, err)
				require.Error(t, verifyBlueGreenQualityV2Applied(ctx, conn))
			}
		})
	}
	entry := blueGreenQualityV2Migration
	encoded, _ := json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}})
	_, err := parseBlueGreenMigrationPolicy(string(encoded))
	require.NoError(t, err)
	entry.Checksum = "bad"
	encoded, _ = json.Marshal(blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{entry}})
	_, err = parseBlueGreenMigrationPolicy(string(encoded))
	require.Error(t, err)
}

func TestModelQualityV2PostgresFullUpgradeAndReplay(t *testing.T) {
	db := qualityV2Postgres(t, false)
	ctx := context.Background()
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	legacy := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name == blueGreenQualityV2Migration.Filename {
			continue
		}
		raw, e := migrations.FS.ReadFile(name)
		require.NoError(t, e)
		legacy[name] = &fstest.MapFile{Data: raw}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, legacy))
	policy := &blueGreenMigrationPolicy{Version: 1, Migrations: []blueGreenMigration{blueGreenQualityV2Migration}}
	require.NoError(t, applyMigrationsFSWithPolicy(ctx, db, migrations.FS, policy))
	require.NoError(t, applyMigrationsFSWithPolicy(ctx, db, migrations.FS, policy))
	require.NoError(t, applyMigrationsFS(ctx, db, legacy))
	var enabled bool
	require.NoError(t, db.QueryRow("SELECT enabled FROM model_quality_v2_config WHERE id=1").Scan(&enabled))
	require.False(t, enabled)
}
