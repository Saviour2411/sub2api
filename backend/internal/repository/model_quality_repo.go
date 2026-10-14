package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type modelQualityRepository struct{ db *sql.DB }

func NewModelQualityRepository(db *sql.DB) service.ModelQualityRepository {
	return &modelQualityRepository{db}
}

func (r *modelQualityRepository) Config(ctx context.Context) (*service.ModelQualityConfig, error) {
	c := &service.ModelQualityConfig{}
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT enabled, version, groups FROM model_quality_config WHERE id=1`).Scan(&c.Enabled, &c.Version, &raw)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &c.Groups); err != nil {
		return nil, err
	}
	return c, nil
}

func (r *modelQualityRepository) SaveConfig(ctx context.Context, c *service.ModelQualityConfig) error {
	raw, err := json.Marshal(c.Groups)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE model_quality_config SET enabled=$1, groups=$2, version=version+1 WHERE id=1 AND version=$3`, c.Enabled, raw, c.Version)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return service.ErrModelQualityConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES ($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, service.SettingKeyModelQualityEnabled, fmt.Sprint(c.Enabled))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		c.Version++
	}
	return err
}

func (r *modelQualityRepository) States(ctx context.Context) (map[int64]service.ModelQualityGroupState, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(q.group_id,s.group_id),q.started_at,q.id IS NOT NULL AND q.finished_at IS NULL,s.next_run_at,
 CASE WHEN q.id IS NULL THEN 'pending' ELSE (SELECT CASE WHEN bool_or(outcome='running') THEN 'running' WHEN bool_or(outcome='degraded') THEN 'degraded' WHEN bool_or(outcome='failed') THEN 'failed' WHEN bool_or(outcome='unknown') THEN 'unknown' ELSE 'normal' END FROM model_quality_runs WHERE round_id=q.id) END
 FROM (SELECT DISTINCT ON (group_id) * FROM model_quality_rounds WHERE started_at>NOW()-INTERVAL '24 hours' ORDER BY group_id,started_at DESC,id DESC) q
 FULL OUTER JOIN model_quality_schedules s ON s.group_id=q.group_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]service.ModelQualityGroupState{}
	for rows.Next() {
		var s service.ModelQualityGroupState
		if err = rows.Scan(&s.GroupID, &s.LastStartedAt, &s.Running, &s.NextRunAt, &s.Status); err != nil {
			return nil, err
		}
		out[s.GroupID] = s
	}
	return out, rows.Err()
}

func (r *modelQualityRepository) StartRound(ctx context.Context, c *service.ModelQualityConfig, g service.ModelQualityGroupConfig, name, endpoint string, manual bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 与配置保存串行，关闭开关后旧扫描结果不能再创建轮次。
	var enabled bool
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT enabled,version FROM model_quality_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &version); err != nil {
		return err
	}
	if !enabled || version != c.Version {
		return service.ErrModelQualityConflict
	}
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM model_quality_rounds WHERE group_id=$1 AND finished_at IS NULL)`, g.GroupID).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return service.ErrModelQualityConflict
	}
	if !manual {
		var due bool
		if err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM model_quality_schedules WHERE group_id=$1 AND next_run_at>NOW())`, g.GroupID).Scan(&due); err != nil {
			return err
		}
		if !due {
			return nil
		}
	}
	raw, err := json.Marshal(g)
	if err != nil {
		return err
	}
	round := uuid.NewString()
	now := time.Now()
	_, err = tx.ExecContext(ctx, `INSERT INTO model_quality_rounds(id,group_id,group_name,parameters,endpoint,started_at) VALUES($1,$2,$3,$4,$5,$6)`, round, g.GroupID, name, raw, endpoint, now)
	if err != nil {
		return err
	}
	for _, model := range g.Models {
		var runID int64
		err = tx.QueryRowContext(ctx, `INSERT INTO model_quality_runs(round_id,model,reasoning_effort,started_at) VALUES($1,$2,$3,$4) RETURNING id`, round, model, g.ReasoningEffort, now).Scan(&runID)
		if err != nil {
			return err
		}
		for _, method := range g.Methods {
			_, err = tx.ExecContext(ctx, `INSERT INTO model_quality_tasks(run_id,benchmark,idempotency_key) VALUES($1,$2,$3)`, runID, method, "mq_"+uuid.NewString())
			if err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (r *modelQualityRepository) Tasks(ctx context.Context) ([]service.ModelQualityTask, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id,t.run_id,t.benchmark,t.status,t.verdict,t.remote_id,t.idempotency_key,t.next_poll_at,t.submitted_at,t.deadline_at,t.attempts,q.parameters,q.endpoint,u.model,u.started_at
 FROM model_quality_tasks t JOIN model_quality_runs u ON u.id=t.run_id JOIN model_quality_rounds q ON q.id=u.round_id
 WHERE t.status IN ('pending','submitting','running','uncertain') ORDER BY CASE WHEN t.status='pending' THEN 1 ELSE 0 END,t.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.ModelQualityTask{}
	for rows.Next() {
		var t service.ModelQualityTask
		var raw []byte
		if err = rows.Scan(&t.ID, &t.RunID, &t.Benchmark, &t.Status, &t.Verdict, &t.RemoteID, &t.IdempotencyKey, &t.NextPollAt, &t.SubmittedAt, &t.DeadlineAt, &t.Attempts, &raw, &t.Endpoint, &t.Model, &t.StartedAt); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &t.Group); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *modelQualityRepository) SaveTask(ctx context.Context, t *service.ModelQualityTask) error {
	_, err := r.db.ExecContext(ctx, `UPDATE model_quality_tasks SET status=$2,verdict=$3,remote_id=$4,next_poll_at=$5,submitted_at=$6,deadline_at=$7,finished_at=$8,attempts=$9,reason=$10,answer=$11,html=$12,duration_ms=$13,input_tokens=$14,output_tokens=$15,reasoning_tokens=$16 WHERE id=$1 AND status IN ('pending','submitting','running','uncertain')`, t.ID, t.Status, t.Verdict, t.RemoteID, t.NextPollAt, t.SubmittedAt, t.DeadlineAt, t.FinishedAt, t.Attempts, t.Reason, t.Answer, t.HTML, t.DurationMS, t.InputTokens, t.OutputTokens, t.ReasoningTokens)
	return err
}

func (r *modelQualityRepository) PrepareTask(ctx context.Context, t *service.ModelQualityTask, version int) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	// 配置行锁把全局两个槽位的检查与占用变成一个原子操作，跨实例也不能超额提交。
	var enabled bool
	var current int
	if err = tx.QueryRowContext(ctx, `SELECT enabled,version FROM model_quality_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &current); err != nil {
		return false, err
	}
	if !enabled || current != version {
		return false, nil
	}
	var slots int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_quality_tasks WHERE status IN ('submitting','running','uncertain')`).Scan(&slots); err != nil {
		return false, err
	}
	if slots >= 2 {
		return false, nil
	}
	res, err := tx.ExecContext(ctx, `UPDATE model_quality_tasks SET status='submitting',submitted_at=$2,deadline_at=$3,attempts=$4 WHERE id=$1 AND status='pending'`, t.ID, t.SubmittedAt, t.DeadlineAt, t.Attempts)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *modelQualityRepository) FinishRounds(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE model_quality_runs u SET outcome=x.outcome,finished_at=CASE WHEN x.active THEN NULL ELSE NOW() END
 FROM (SELECT run_id,bool_or(status IN ('pending','submitting','running','uncertain')) AS active,
 CASE WHEN bool_or(status IN ('pending','submitting','running')) THEN 'running'
 WHEN bool_or(verdict='degraded') THEN 'degraded' WHEN bool_or(status IN ('failed','cancelled','uncertain')) THEN 'failed'
 WHEN bool_or(verdict='unknown') THEN 'unknown' ELSE 'normal' END AS outcome
 FROM model_quality_tasks WHERE run_id IN (SELECT id FROM model_quality_runs WHERE finished_at IS NULL)
 GROUP BY run_id) x WHERE u.id=x.run_id AND u.finished_at IS NULL`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `WITH finished AS (
 UPDATE model_quality_rounds q SET finished_at=NOW() WHERE finished_at IS NULL AND NOT EXISTS(SELECT 1 FROM model_quality_runs u WHERE u.round_id=q.id AND u.finished_at IS NULL)
 RETURNING group_id,parameters)
 INSERT INTO model_quality_schedules(group_id,next_run_at) SELECT group_id,NOW()+((parameters->>'interval_minutes')::integer*INTERVAL '1 minute') FROM finished
 ON CONFLICT(group_id) DO UPDATE SET next_run_at=EXCLUDED.next_run_at`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *modelQualityRepository) Cleanup(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM model_quality_rounds WHERE started_at<=NOW()-INTERVAL '24 hours'`)
	return err
}

const modelQualityRunColumns = `u.id,q.group_id,q.group_name,u.model,u.reasoning_effort,u.outcome,u.started_at,u.finished_at`

func scanQualityRun(row interface{ Scan(...any) error }) (service.ModelQualityRun, error) {
	var r service.ModelQualityRun
	err := row.Scan(&r.ID, &r.GroupID, &r.GroupName, &r.Model, &r.ReasoningEffort, &r.Outcome, &r.StartedAt, &r.FinishedAt)
	r.ExpiresAt = r.StartedAt.Add(24 * time.Hour)
	r.Tests = []service.ModelQualityTest{}
	return r, err
}

func (r *modelQualityRepository) Runs(ctx context.Context, f service.ModelQualityFilter) ([]service.ModelQualityRun, int64, error) {
	where := ` FROM model_quality_runs u JOIN model_quality_rounds q ON q.id=u.round_id WHERE u.started_at>NOW()-INTERVAL '24 hours' AND q.group_id=$1 AND ($2='' OR u.model=$2) AND ($3='' OR EXISTS(SELECT 1 FROM model_quality_tasks t WHERE t.run_id=u.id AND t.benchmark=$3)) AND ($4='' OR u.outcome=$4)`
	args := []any{f.GroupID, f.Model, f.Method, f.Outcome}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+modelQualityRunColumns+where+` ORDER BY u.started_at DESC,u.id DESC LIMIT 12 OFFSET $5`, append(args, (f.Page-1)*12)...)
	if err != nil {
		return nil, 0, err
	}
	out := []service.ModelQualityRun{}
	for rows.Next() {
		item, e := scanQualityRun(rows)
		if e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		out = append(out, item)
	}
	err = rows.Err()
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, 0, err
	}
	if err = r.loadTests(ctx, out, false); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *modelQualityRepository) Run(ctx context.Context, id int64) (*service.ModelQualityRun, error) {
	item, err := scanQualityRun(r.db.QueryRowContext(ctx, `SELECT `+modelQualityRunColumns+` FROM model_quality_runs u JOIN model_quality_rounds q ON q.id=u.round_id WHERE u.id=$1 AND u.started_at>NOW()-INTERVAL '24 hours'`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrModelQualityExpired
	}
	if err != nil {
		return nil, err
	}
	out := []service.ModelQualityRun{item}
	if err = r.loadTests(ctx, out, true); err != nil {
		return nil, err
	}
	return &out[0], nil
}

func (r *modelQualityRepository) loadTests(ctx context.Context, runs []service.ModelQualityRun, detail bool) error {
	if len(runs) == 0 {
		return nil
	}
	ids := []int64{}
	indexes := map[int64]int{}
	for i, r := range runs {
		ids = append(ids, r.ID)
		indexes[r.ID] = i
	}
	answer, html := "LEFT(answer,180)", "''"
	if detail {
		answer, html = "answer", "html"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT run_id,benchmark,status,verdict,reason,`+answer+`,`+html+`,html<>'',duration_ms,input_tokens,output_tokens,reasoning_tokens,finished_at FROM model_quality_tasks WHERE run_id=ANY($1) ORDER BY id`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var t service.ModelQualityTest
		if err = rows.Scan(&id, &t.Benchmark, &t.Status, &t.Verdict, &t.Reason, &t.Answer, &t.HTML, &t.HasHTML, &t.DurationMS, &t.InputTokens, &t.OutputTokens, &t.ReasoningTokens, &t.FinishedAt); err != nil {
			return err
		}
		i := indexes[id]
		runs[i].Tests = append(runs[i].Tests, t)
	}
	return rows.Err()
}
