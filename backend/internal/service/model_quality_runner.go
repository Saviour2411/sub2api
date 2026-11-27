package service

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/google/uuid"
)

type qualityModelClient interface {
	Call(context.Context, string, string, ModelQualityCall, string, string) (ModelQualityPhase, error)
}

type ModelQualityRunner struct {
	service             *ModelQualityService
	client              qualityModelClient
	ctx                 context.Context
	cancel              context.CancelFunc
	wg                  sync.WaitGroup
	startOnce, stopOnce sync.Once
	lastScan            time.Time
}

func NewModelQualityRunner(s *ModelQualityService) *ModelQualityRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModelQualityRunner{service: s, client: newQualityProtocolClient(), ctx: ctx, cancel: cancel}
}
func ProvideModelQualityRunner(s *ModelQualityService) *ModelQualityRunner {
	r := NewModelQualityRunner(s)
	r.Start()
	return r
}
func (r *ModelQualityRunner) Start() {
	r.startOnce.Do(func() {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				r.tick()
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
func (r *ModelQualityRunner) Stop() { r.stopOnce.Do(func() { r.cancel(); r.wg.Wait() }) }

func (r *ModelQualityRunner) tick() {
	ctx, cancel := context.WithTimeout(r.ctx, 90*time.Second)
	defer cancel()
	release, accepted := trySharedBackgroundJob(ctx, "model-quality-v2-dispatch")
	if !accepted {
		return
	}
	defer release()
	if err := r.step(ctx); err != nil && r.ctx.Err() == nil {
		slog.Warn("降智检测调度暂未完成", "error", err)
	}
}

func (r *ModelQualityRunner) step(ctx context.Context) error {
	repo, err := r.service.qualityV2Repo()
	if err != nil {
		return err
	}
	if err = repo.MaintainQualityV2(ctx); err != nil {
		return err
	}
	config, err := r.service.ConfigV2(ctx)
	if err != nil {
		return err
	}
	if !config.Enabled || config.PauseReason != "" {
		return nil
	}
	if time.Since(r.lastScan) >= 30*time.Second {
		for _, group := range config.Groups {
			name, reason := r.service.qualityV2Readiness(ctx, config, group)
			if reason != "" {
				continue
			}
			if err = repo.StartQualityV2Round(ctx, config, group, name, false); err != nil && !errors.Is(err, ErrModelQualityConflict) {
				return err
			}
		}
		r.lastScan = time.Now()
	}
	for range 2 {
		// 每个外部调用独立登记；调度 tick 结束不会提前降低排空计数。
		done, ok := lifecycle.Process.BeginBackground()
		if !ok {
			return nil
		}
		task, e := repo.ClaimQualityV2Task(ctx, config.Version, lifecycle.Process.ID()+":"+uuid.NewString())
		if e != nil {
			done()
			return e
		}
		if task == nil {
			done()
			break
		}
		r.wg.Add(1)
		go func() { defer r.wg.Done(); defer done(); r.execute(repo, task) }()
	}
	return nil
}

func qualityTaskEnabled(c *ModelQualityV2Config, t *ModelQualityV2Task) bool {
	if !c.Enabled {
		return false
	}
	for _, group := range c.Groups {
		if group.GroupID != t.Specification.Target.GroupID {
			continue
		}
		for _, target := range group.Targets {
			old := t.Specification.Target
			if target.ID == old.ID && target.Model == old.Model && target.Protocol == old.Protocol && target.KeyID != nil && old.KeyID != nil && *target.KeyID == *old.KeyID && slices.Contains(target.QuestionIDs, t.Specification.Question.ID) {
				return true
			}
		}
	}
	return false
}

func finishQualityV2Task(t *ModelQualityV2Task, status, verdict, reason string) {
	now := time.Now()
	t.Status = status
	t.Result.Status = status
	t.Result.Verdict = verdict
	t.Result.Reason = reason
	t.Result.FinishedAt = &now
	if t.Result.Generation.Status == "running" {
		t.Result.Generation.Status = status
		t.Result.Generation.Reason = reason
		t.Result.Generation.FinishedAt = &now
	}
	if t.Result.Judgment != nil && t.Result.Judgment.Status == "running" {
		t.Result.Judgment.Status = status
		t.Result.Judgment.Reason = reason
		t.Result.Judgment.FinishedAt = &now
	}
}

func qualityTokenSum(values ...*int64) *int64 {
	var result *int64
	for _, value := range values {
		if value == nil {
			continue
		}
		if result == nil {
			result = new(int64)
		}
		*result += *value
	}
	return result
}

func refreshQualityV2Totals(t *ModelQualityV2Task) {
	phases := []ModelQualityPhase{t.Result.Generation}
	phases = append(phases, t.Result.PreviousJudgments...)
	if t.Result.Judgment != nil {
		phases = append(phases, *t.Result.Judgment)
	}
	t.Result.DurationMS = 0
	t.Result.InputTokens = nil
	t.Result.OutputTokens = nil
	t.Result.ReasoningTokens = nil
	for _, phase := range phases {
		t.Result.DurationMS += phase.DurationMS
		t.Result.InputTokens = qualityTokenSum(t.Result.InputTokens, phase.InputTokens)
		t.Result.OutputTokens = qualityTokenSum(t.Result.OutputTokens, phase.OutputTokens)
		t.Result.ReasoningTokens = qualityTokenSum(t.Result.ReasoningTokens, phase.ReasoningTokens)
	}
}

func (r *ModelQualityRunner) execute(repo ModelQualityV2Repository, t *ModelQualityV2Task) {
	previous := t.Status
	defer func() {
		refreshQualityV2Totals(t)
		for range 3 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := repo.SaveQualityV2Task(ctx, t, previous)
			cancel()
			if err == nil {
				return
			}
		}
		slog.Warn("降智检测结果暂未存档；保留调用占位，恢复时不自动重发", "task_id", t.ID)
	}()
	configCtx, configCancel := context.WithTimeout(r.ctx, 10*time.Second)
	config, err := r.service.ConfigV2(configCtx)
	configCancel()
	if err != nil {
		finishQualityV2Task(t, "failed", "unknown", "读取检测配置失败，未发送请求")
		return
	}
	if !qualityTaskEnabled(config, t) {
		finishQualityV2Task(t, "cancelled", "unknown", "检测已关闭或模型配置已移除，未发送后续请求")
		return
	}
	call := t.Specification.Target.ModelQualityCall
	system, prompt := t.Specification.Question.SystemPrompt, t.Specification.Question.Prompt
	attempts := t.Result.Generation.Attempts
	if previous == "judging" {
		judge := t.Specification.Question.Rule.Judge
		if judge == nil {
			finishQualityV2Task(t, "failed", "unknown", "判题配置缺失")
			return
		}
		call = *judge
		system, prompt = qualityJudgeInput(t.Specification.Question, t.Result.Generation.Answer)
		attempts = t.Result.Judgment.Attempts
	}
	ctx, cancel := context.WithTimeout(r.ctx, time.Duration(call.TimeoutSeconds)*time.Second)
	defer cancel()
	key, err := r.service.validQualityCall(ctx, call)
	if err != nil {
		finishQualityV2Task(t, "failed", "unknown", err.Error())
		return
	}
	lease := *t
	heartbeatDone := make(chan struct{})
	heartbeatStopped := make(chan struct{})
	go func() {
		defer close(heartbeatStopped)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				check, end := context.WithTimeout(ctx, 5*time.Second)
				ok, e := repo.RefreshQualityV2Lease(check, &lease)
				end()
				if e != nil || !ok {
					cancel()
					return
				}
			}
		}
	}()
	phase, err := r.client.Call(ctx, t.Specification.Endpoint, key.Key, call, system, prompt)
	close(heartbeatDone)
	<-heartbeatStopped
	phase.Attempts = attempts
	if previous == "generating" {
		t.Result.Generation = phase
	} else {
		t.Result.Judgment = &phase
	}
	if err != nil {
		var remote *qualityCallError
		if errors.As(err, &remote) && remote.rejected && attempts < 5 && r.ctx.Err() == nil {
			t.Status = "pending"
			if previous == "judging" {
				t.Status = "ready_judge"
			}
			t.Result.Status = t.Status
			t.Result.Reason = "接口明确限流，等待重试"
			t.NextAttemptAt = time.Now().Add(remote.retryAfter)
			return
		}
		finishQualityV2Task(t, "failed", "unknown", err.Error())
		return
	}
	if previous == "generating" {
		t.Result.Answer = phase.Answer
		t.Result.HTML = qualityArtwork(phase.Answer, t.Specification.Question.OutputType)
		t.Result.HasHTML = t.Result.HTML != ""
		if t.Specification.Question.Rule.Kind == "model" {
			t.Status = "ready_judge"
			t.Result.Status = t.Status
			t.Result.Reason = "生成已完成，等待判题"
			t.Result.Judgment = &ModelQualityPhase{Status: "pending"}
			t.NextAttemptAt = time.Now()
			return
		}
		verdict, reason := evaluateQualityAnswer(t.Specification.Question.Rule, phase.Answer)
		finishQualityV2Task(t, "completed", verdict, reason)
		return
	}
	verdict, reason := evaluateQualityJudgment(phase.Answer)
	finishQualityV2Task(t, "completed", verdict, reason)
}
