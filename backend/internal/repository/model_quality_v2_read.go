package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const qualityV2RunColumns = "id,source,group_id,group_name,model,protocol,reasoning_effort,outcome,started_at,finished_at"

const qualityV2Union = `SELECT 'v2-'||u.id AS id,'local' AS source,q.group_id,q.group_name,u.model,u.protocol,u.reasoning_effort,u.outcome,u.started_at,u.finished_at
 FROM model_quality_v2_runs u JOIN model_quality_v2_rounds q ON q.id=u.round_id
 WHERE q.group_id=$1 AND u.started_at>NOW()-INTERVAL '24 hours' AND ($2='' OR u.model=$2) AND ($4='' OR u.outcome=$4)
 AND ($3='' OR EXISTS(SELECT 1 FROM model_quality_v2_tasks t WHERE t.run_id=u.id AND t.question_id=$3))
 UNION ALL
 SELECT 'v1-'||u.id::text AS id,'legacy' AS source,q.group_id,q.group_name,u.model,'' AS protocol,u.reasoning_effort,u.outcome,u.started_at,u.finished_at
 FROM model_quality_runs u JOIN model_quality_rounds q ON q.id=u.round_id
 WHERE q.group_id=$1 AND u.started_at>NOW()-INTERVAL '24 hours' AND ($2='' OR u.model=$2) AND ($4='' OR u.outcome=$4)
 AND ($3='' OR EXISTS(SELECT 1 FROM model_quality_tasks t WHERE t.run_id=u.id AND t.benchmark=$3))`

func scanQualityV2Run(row interface{ Scan(...any) error }) (service.ModelQualityV2Run, error) {
	run := service.ModelQualityV2Run{}
	err := row.Scan(&run.ID, &run.Source, &run.GroupID, &run.GroupName, &run.Model, &run.Protocol, &run.ReasoningEffort, &run.Outcome, &run.StartedAt, &run.FinishedAt)
	run.ExpiresAt = run.StartedAt.Add(24 * time.Hour)
	run.Tests = []service.ModelQualityV2Test{}
	return run, err
}

func legacyQualityV2Run(old service.ModelQualityRun) service.ModelQualityV2Run {
	run := service.ModelQualityV2Run{ID: "v1-" + strconv.FormatInt(old.ID, 10), Source: "legacy", GroupID: old.GroupID, GroupName: old.GroupName, Model: old.Model, ReasoningEffort: old.ReasoningEffort, Outcome: old.Outcome, StartedAt: old.StartedAt, FinishedAt: old.FinishedAt, ExpiresAt: old.StartedAt.Add(24 * time.Hour), Tests: []service.ModelQualityV2Test{}}
	for _, test := range old.Tests {
		name, kind := "糖果（旧版）", "text"
		if test.Benchmark == "pelican" {
			name, kind = "鹈鹕（旧版）", "html"
		}
		run.Tests = append(run.Tests, service.ModelQualityV2Test{ModelQualityTest: test, Name: name, OutputType: kind, RuleKind: "legacy", Generation: service.ModelQualityPhase{Status: test.Status, Answer: test.Answer, DurationMS: test.DurationMS, InputTokens: test.InputTokens, OutputTokens: test.OutputTokens, ReasoningTokens: test.ReasoningTokens, FinishedAt: test.FinishedAt}})
	}
	return run
}

func (r *modelQualityRepository) loadQualityV2Tests(ctx context.Context, run *service.ModelQualityV2Run, detail bool) error {
	if run.Source == "legacy" {
		id, err := strconv.ParseInt(strings.TrimPrefix(run.ID, "v1-"), 10, 64)
		if err != nil {
			return err
		}
		legacy := []service.ModelQualityRun{{ID: id}}
		if err = r.loadTests(ctx, legacy, detail); err != nil {
			return err
		}
		run.Tests = legacyQualityV2Run(legacy[0]).Tests
		return nil
	}
	expression := "result"
	if !detail {
		expression = `(result-'answer'-'html'-'previous_judgments')||jsonb_build_object('generation',(result->'generation')-'answer','judgment',CASE WHEN result->'judgment' IS NULL THEN 'null'::jsonb ELSE (result->'judgment')-'answer' END)`
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+expression+` FROM model_quality_v2_tasks WHERE run_id=$1 ORDER BY started_at,id`, strings.TrimPrefix(run.ID, "v2-"))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var raw []byte
		var result service.ModelQualityV2Test
		if err = rows.Scan(&raw); err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &result); err != nil {
			return err
		}
		run.Tests = append(run.Tests, result)
	}
	return rows.Err()
}

func (r *modelQualityRepository) QualityV2Runs(ctx context.Context, f service.ModelQualityFilter) ([]service.ModelQualityV2Run, int64, error) {
	args := []any{f.GroupID, f.Model, f.Method, f.Outcome}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+qualityV2Union+`) combined`, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+qualityV2RunColumns+` FROM (`+qualityV2Union+`) combined ORDER BY started_at DESC,id DESC LIMIT 12 OFFSET $5`, append(args, (f.Page-1)*12)...)
	if err != nil {
		return nil, 0, err
	}
	out := []service.ModelQualityV2Run{}
	for rows.Next() {
		run, e := scanQualityV2Run(rows)
		if e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		out = append(out, run)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		if err = r.loadQualityV2Tests(ctx, &out[i], false); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

func (r *modelQualityRepository) QualityV2Run(ctx context.Context, id string) (*service.ModelQualityV2Run, error) {
	if !strings.HasPrefix(id, "v2-") {
		value, err := strconv.ParseInt(strings.TrimPrefix(id, "v1-"), 10, 64)
		if err != nil {
			return nil, service.ErrModelQualityExpired
		}
		old, err := r.Run(ctx, value)
		if err != nil {
			return nil, err
		}
		run := legacyQualityV2Run(*old)
		return &run, nil
	}
	run, err := scanQualityV2Run(r.db.QueryRowContext(ctx, `SELECT 'v2-'||u.id,'local',q.group_id,q.group_name,u.model,u.protocol,u.reasoning_effort,u.outcome,u.started_at,u.finished_at
 FROM model_quality_v2_runs u JOIN model_quality_v2_rounds q ON q.id=u.round_id WHERE u.id=$1 AND u.started_at>NOW()-INTERVAL '24 hours'`, strings.TrimPrefix(id, "v2-")))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrModelQualityExpired
	}
	if err != nil {
		return nil, err
	}
	if err = r.loadQualityV2Tests(ctx, &run, true); err != nil {
		return nil, err
	}
	return &run, nil
}
