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
)

var _ service.ModelQualityV2Repository = (*modelQualityRepository)(nil)

func (r *modelQualityRepository) QualityV2Config(ctx context.Context) (*service.ModelQualityV2Config, error) {
	var raw []byte
	var enabled bool
	var version int
	if err := r.db.QueryRowContext(ctx, `SELECT enabled,version,definition FROM model_quality_v2_config WHERE id=1`).Scan(&enabled, &version, &raw); err != nil {
		return nil, err
	}
	c := &service.ModelQualityV2Config{}
	if err := json.Unmarshal(raw, c); err != nil {
		return nil, err
	}
	c.InitializationRequired = c.SchemaVersion != service.ModelQualitySchemaVersion
	c.Enabled, c.Version = enabled, version
	if c.Questions == nil {
		c.Questions = []service.ModelQualityQuestion{}
	}
	if c.Groups == nil {
		c.Groups = []service.ModelQualityV2GroupConfig{}
	}
	return c, nil
}

func (r *modelQualityRepository) SaveQualityV2Config(ctx context.Context, c *service.ModelQualityV2Config) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var legacy bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM model_quality_config WHERE id=1 FOR UPDATE`).Scan(&legacy); err != nil {
		return err
	}
	if c.Enabled && legacy {
		return fmt.Errorf("旧版检测仍启用，请先完成升级停用步骤")
	}
	result, err := tx.ExecContext(ctx, `UPDATE model_quality_v2_config SET enabled=$1,version=version+1,definition=$2 WHERE id=1 AND version=$3`, c.Enabled, raw, c.Version)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return service.ErrModelQualityConflict
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, service.SettingKeyModelQualityEnabled, fmt.Sprint(c.Enabled)); err != nil {
		return err
	}
	if err = tx.Commit(); err == nil {
		c.Version++
	}
	return err
}

func (r *modelQualityRepository) QualityV2States(ctx context.Context) (map[int64]service.ModelQualityGroupState, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(q.group_id,s.group_id),q.started_at,q.id IS NOT NULL AND q.finished_at IS NULL,s.next_run_at,
 CASE WHEN q.id IS NULL THEN 'pending' ELSE (SELECT CASE WHEN bool_or(outcome='running') THEN 'running' WHEN bool_or(outcome='degraded') THEN 'degraded' WHEN bool_or(outcome='failed') THEN 'failed' WHEN bool_or(outcome='unknown') THEN 'unknown' ELSE 'normal' END FROM model_quality_v2_runs WHERE round_id=q.id) END
 FROM (SELECT DISTINCT ON(group_id) * FROM model_quality_v2_rounds WHERE started_at>NOW()-INTERVAL '24 hours' ORDER BY group_id,started_at DESC,id DESC) q
 FULL OUTER JOIN model_quality_v2_schedules s ON s.group_id=q.group_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[int64]service.ModelQualityGroupState{}
	for rows.Next() {
		var item service.ModelQualityGroupState
		if err = rows.Scan(&item.GroupID, &item.LastStartedAt, &item.Running, &item.NextRunAt, &item.Status); err != nil {
			return nil, err
		}
		out[item.GroupID] = item
	}
	return out, rows.Err()
}

func (r *modelQualityRepository) StartQualityV2Round(ctx context.Context, c *service.ModelQualityV2Config, g service.ModelQualityV2GroupConfig, name string, manual bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var enabled bool
	var version int
	if err = tx.QueryRowContext(ctx, `SELECT enabled,version FROM model_quality_v2_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &version); err != nil {
		return err
	}
	if !enabled || version != c.Version {
		return service.ErrModelQualityConflict
	}
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM model_quality_v2_rounds WHERE group_id=$1 AND finished_at IS NULL)`, g.GroupID).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return service.ErrModelQualityConflict
	}
	if !manual {
		var due bool
		if err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM model_quality_v2_schedules WHERE group_id=$1 AND next_run_at>NOW())`, g.GroupID).Scan(&due); err != nil {
			return err
		}
		if !due {
			return nil
		}
	}
	round := uuid.NewString()
	if _, err = tx.ExecContext(ctx, `INSERT INTO model_quality_v2_rounds(id,group_id,group_name,interval_minutes) VALUES($1,$2,$3,$4)`, round, g.GroupID, name, g.IntervalMinutes); err != nil {
		return err
	}
	questions := map[string]service.ModelQualityQuestion{}
	for _, q := range c.Questions {
		questions[q.ID] = q
	}
	for _, target := range g.Targets {
		runID := uuid.NewString()
		if _, err = tx.ExecContext(ctx, `INSERT INTO model_quality_v2_runs(id,round_id,target_id,model,protocol,reasoning_effort) VALUES($1,$2,$3,$4,$5,$6)`, runID, round, target.ID, target.Model, target.Protocol, target.ReasoningEffort); err != nil {
			return err
		}
		for _, id := range target.QuestionIDs {
			q, ok := questions[id]
			if !ok {
				return service.ErrModelQualityInvalid
			}
			taskID := uuid.NewString()
			spec, e := json.Marshal(service.ModelQualitySpecification{Target: target, Question: q, Endpoint: c.Endpoint})
			if e != nil {
				return e
			}
			result := service.ModelQualityV2Test{ID: taskID, Name: q.Name, OutputType: q.OutputType, RuleKind: q.Rule.Kind, ModelQualityTest: service.ModelQualityTest{Benchmark: q.ID, Status: "pending", Verdict: "unknown"}, Generation: service.ModelQualityPhase{Status: "pending"}}
			if q.Rule.Judge != nil {
				result.JudgeModel, result.JudgeProtocol = q.Rule.Judge.Model, q.Rule.Judge.Protocol
			}
			raw, e := json.Marshal(result)
			if e != nil {
				return e
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO model_quality_v2_tasks(id,run_id,question_id,specification,result) VALUES($1,$2,$3,$4,$5)`, taskID, runID, id, spec, raw); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func scanQualityV2Task(row interface{ Scan(...any) error }) (*service.ModelQualityV2Task, error) {
	t := &service.ModelQualityV2Task{}
	var spec, result []byte
	err := row.Scan(&t.ID, &t.RunID, &t.Status, &t.Owner, &t.StartedAt, &t.NextAttemptAt, &t.DeadlineAt, &spec, &result)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(spec, &t.Specification); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(result, &t.Result); err != nil {
		return nil, err
	}
	return t, nil
}

const qualityV2TaskColumns = "id,run_id,status,owner,started_at,next_attempt_at,deadline_at,specification,result"

func (r *modelQualityRepository) ClaimQualityV2Task(ctx context.Context, version int, owner string) (*service.ModelQualityV2Task, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var enabled bool
	var current, slots int
	if err = tx.QueryRowContext(ctx, `SELECT enabled,version FROM model_quality_v2_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &current); err != nil {
		return nil, err
	}
	if !enabled || current != version {
		return nil, nil
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_quality_v2_tasks WHERE status IN ('generating','judging')`).Scan(&slots); err != nil {
		return nil, err
	}
	if slots >= 2 {
		return nil, nil
	}
	task, err := scanQualityV2Task(tx.QueryRowContext(ctx, `SELECT `+qualityV2TaskColumns+` FROM model_quality_v2_tasks WHERE status IN ('pending','ready_judge') AND next_attempt_at<=NOW() ORDER BY started_at,id LIMIT 1 FOR UPDATE SKIP LOCKED`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	timeout := task.Specification.Target.TimeoutSeconds
	if task.Status == "pending" {
		task.Status = "generating"
		task.Result.Generation.Status = "running"
		task.Result.Generation.Attempts++
	} else {
		if task.Specification.Question.Rule.Judge == nil {
			return nil, service.ErrModelQualityInvalid
		}
		task.Status = "judging"
		timeout = task.Specification.Question.Rule.Judge.TimeoutSeconds
		if task.Result.Judgment == nil {
			task.Result.Judgment = &service.ModelQualityPhase{}
		}
		task.Result.Judgment.Status = "running"
		task.Result.Judgment.Attempts++
	}
	task.Owner = owner
	task.Result.Status = task.Status
	deadline := time.Now().Add(time.Duration(timeout+60) * time.Second)
	task.DeadlineAt = &deadline
	raw, err := json.Marshal(task.Result)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET status=$2,owner=$3,lease_until=NOW()+INTERVAL '90 seconds',deadline_at=$4,result=$5 WHERE id=$1`, task.ID, task.Status, owner, deadline, raw); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return task, nil
}

func (r *modelQualityRepository) RefreshQualityV2Lease(ctx context.Context, t *service.ModelQualityV2Task) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET lease_until=NOW()+INTERVAL '90 seconds' WHERE id=$1 AND owner=$2 AND status=$3`, t.ID, t.Owner, t.Status)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *modelQualityRepository) SaveQualityV2Task(ctx context.Context, t *service.ModelQualityV2Task, previous string) (bool, error) {
	raw, err := json.Marshal(t.Result)
	if err != nil {
		return false, err
	}
	result, err := r.db.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET status=$4,verdict=$5,result=$6,next_attempt_at=$7,owner='',lease_until=NULL,deadline_at=NULL,
 finished_at=CASE WHEN $4 IN ('pending','ready_judge') THEN NULL ELSE NOW() END WHERE id=$1 AND owner=$2 AND status=$3`, t.ID, t.Owner, previous, t.Status, t.Result.Verdict, raw, t.NextAttemptAt)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *modelQualityRepository) MaintainQualityV2(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var enabled bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled FROM model_quality_v2_config WHERE id=1 FOR UPDATE`).Scan(&enabled); err != nil {
		return err
	}
	// 租约丢失只结束不明确的调用，不重新领取发送；为原请求保留完整超时及存档余量。
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET status='uncertain',verdict='unknown',finished_at=NOW(),owner='',lease_until=NULL,
 result=jsonb_set(result||jsonb_build_object('status','uncertain','verdict','unknown','reason','执行实例失联，调用结果不明确，未自动重发','finished_at',NOW()),
 ARRAY[CASE WHEN status='generating' THEN 'generation' ELSE 'judgment' END],
 COALESCE(result->CASE WHEN status='generating' THEN 'generation' ELSE 'judgment' END,'{}'::jsonb)||jsonb_build_object('status','uncertain','reason','调用结果不明确','finished_at',NOW()))
 WHERE status IN ('generating','judging') AND lease_until<NOW() AND deadline_at<NOW()`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET status=CASE WHEN $1 THEN 'failed' ELSE 'cancelled' END,verdict='unknown',finished_at=NOW(),
 result=jsonb_set(result||jsonb_build_object('status',CASE WHEN $1 THEN 'failed' ELSE 'cancelled' END,'verdict','unknown','reason',CASE WHEN $1 THEN '排队超过一小时' ELSE '检测已关闭，未发送后续请求' END,'finished_at',NOW()),
 ARRAY[CASE WHEN status='pending' THEN 'generation' ELSE 'judgment' END],
 COALESCE(result->CASE WHEN status='pending' THEN 'generation' ELSE 'judgment' END,'{}'::jsonb)||jsonb_build_object('status',CASE WHEN $1 THEN 'failed' ELSE 'cancelled' END,'reason',CASE WHEN $1 THEN '排队超过一小时' ELSE '检测已关闭' END,'finished_at',NOW()))
 WHERE status IN ('pending','ready_judge') AND (NOT $1 OR next_attempt_at<NOW()-INTERVAL '1 hour')`, enabled); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_runs u SET outcome=x.outcome,finished_at=CASE WHEN x.active THEN NULL ELSE NOW() END FROM
 (SELECT run_id,bool_or(finished_at IS NULL) AS active,CASE WHEN bool_or(finished_at IS NULL) THEN 'running' WHEN bool_or(verdict='degraded') THEN 'degraded'
 WHEN bool_or(status IN ('failed','cancelled','uncertain')) THEN 'failed' WHEN bool_or(verdict='unknown') THEN 'unknown' ELSE 'normal' END AS outcome
 FROM model_quality_v2_tasks WHERE run_id IN (SELECT id FROM model_quality_v2_runs WHERE finished_at IS NULL) GROUP BY run_id) x WHERE u.id=x.run_id AND u.finished_at IS NULL`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `WITH finished AS (UPDATE model_quality_v2_rounds q SET finished_at=NOW() WHERE q.finished_at IS NULL
 AND NOT EXISTS(SELECT 1 FROM model_quality_v2_runs u WHERE u.round_id=q.id AND u.finished_at IS NULL) RETURNING group_id,interval_minutes)
 INSERT INTO model_quality_v2_schedules(group_id,next_run_at) SELECT group_id,NOW()+(interval_minutes*INTERVAL '1 minute') FROM finished
 ON CONFLICT(group_id) DO UPDATE SET next_run_at=EXCLUDED.next_run_at`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM model_quality_v2_rounds WHERE started_at<=NOW()-INTERVAL '24 hours' AND finished_at IS NOT NULL`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM model_quality_rounds WHERE started_at<=NOW()-INTERVAL '24 hours' AND finished_at IS NOT NULL`); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *modelQualityRepository) RejudgeQualityV2Task(ctx context.Context, id string, adminID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var enabled bool
	var definition []byte
	if err = tx.QueryRowContext(ctx, `SELECT enabled,definition FROM model_quality_v2_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &definition); err != nil {
		return err
	}
	if !enabled {
		return service.ErrModelQualityDisabled
	}
	task, err := scanQualityV2Task(tx.QueryRowContext(ctx, `SELECT `+qualityV2TaskColumns+` FROM model_quality_v2_tasks WHERE id=$1 AND started_at>NOW()-INTERVAL '24 hours' FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrModelQualityExpired
	}
	if err != nil {
		return err
	}
	if task.Specification.Question.Rule.Kind != "model" || task.Specification.Question.Rule.Judge == nil || task.Result.Generation.Status != "completed" || task.Specification.Target.OwnerID != adminID || task.Specification.Question.Rule.Judge.OwnerID != adminID || len(task.Result.PreviousJudgments) >= 3 {
		return service.ErrModelQualityInvalid
	}
	if task.Status == "generating" || task.Status == "judging" || task.Status == "pending" || task.Status == "ready_judge" {
		return service.ErrModelQualityConflict
	}
	var config service.ModelQualityV2Config
	if err = json.Unmarshal(definition, &config); err != nil {
		return err
	}
	visible := false
	for _, g := range config.Groups {
		if g.GroupID == task.Specification.Target.GroupID {
			visible = true
		}
	}
	if !visible {
		return service.ErrModelQualityExpired
	}
	var round string
	var busy bool
	if err = tx.QueryRowContext(ctx, `SELECT round_id FROM model_quality_v2_runs WHERE id=$1`, task.RunID).Scan(&round); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM model_quality_v2_rounds WHERE group_id=$1 AND finished_at IS NULL AND id<>$2)`, task.Specification.Target.GroupID, round).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return service.ErrModelQualityConflict
	}
	if task.Result.Judgment != nil {
		task.Result.PreviousJudgments = append(task.Result.PreviousJudgments, *task.Result.Judgment)
	}
	task.Result.Judgment = &service.ModelQualityPhase{Status: "pending"}
	task.Result.Status = "ready_judge"
	task.Result.Verdict = "unknown"
	task.Result.Reason = "等待重新判题"
	task.Result.FinishedAt = nil
	raw, err := json.Marshal(task.Result)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_tasks SET result=$2,status='ready_judge',verdict='unknown',finished_at=NULL,next_attempt_at=NOW(),owner='',lease_until=NULL,deadline_at=NULL WHERE id=$1`, id, raw); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_runs SET outcome='running',finished_at=NULL WHERE id=$1`, task.RunID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE model_quality_v2_rounds SET finished_at=NULL WHERE id=$1`, round); err != nil {
		return err
	}
	return tx.Commit()
}
