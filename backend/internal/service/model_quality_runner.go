package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/manxue"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
)

type modelQualityClient interface {
	Submit(context.Context, manxue.Request, string) (*manxue.Result, error)
	Poll(context.Context, string) (*manxue.Result, error)
}

type ModelQualityRunner struct {
	service               *ModelQualityService
	client                modelQualityClient
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	startOnce, stopOnce   sync.Once
	lastScan, lastCleanup time.Time
	lookup                func(context.Context, string) ([]net.IPAddr, error)
}

func NewModelQualityRunner(s *ModelQualityService) *ModelQualityRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModelQualityRunner{service: s, client: manxue.NewClient(), ctx: ctx, cancel: cancel, lookup: net.DefaultResolver.LookupIPAddr}
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
	release, accepted := trySharedBackgroundJob(ctx, "model-quality-dispatch")
	if !accepted {
		return
	}
	defer release()
	if err := r.step(ctx); err != nil && r.ctx.Err() == nil {
		slog.Warn("降智检测：本轮调度未完成", "error", err)
	}
}

func (r *ModelQualityRunner) step(ctx context.Context) error {
	repo := r.service.repo
	if time.Since(r.lastCleanup) >= time.Minute {
		if err := repo.Cleanup(ctx); err != nil {
			return err
		}
		r.lastCleanup = time.Now()
	}
	if err := repo.FinishRounds(ctx); err != nil {
		return err
	}
	c, err := r.service.Config(ctx)
	if err != nil {
		return err
	}
	if c.Enabled && time.Since(r.lastScan) >= 30*time.Second {
		for _, g := range c.Groups {
			name, reason := r.service.readiness(ctx, c, g)
			if reason != "" {
				continue
			}
			if err = repo.StartRound(ctx, c, g, name, c.Endpoint, false); err != nil && !errors.Is(err, ErrModelQualityConflict) {
				return err
			}
		}
		r.lastScan = time.Now()
	}
	tasks, err := repo.Tasks(ctx)
	if err != nil {
		return err
	}
	for i := range tasks {
		t := &tasks[i]
		if t.Status == "pending" {
			if !c.Enabled {
				finishQualityTask(t, "cancelled", "功能已关闭")
				if err = repo.SaveTask(ctx, t); err != nil {
					return err
				}
			} else if time.Since(t.StartedAt) > time.Hour {
				finishQualityTask(t, "failed", "排队超过一小时，已结束本轮检测")
				if err = repo.SaveTask(ctx, t); err != nil {
					return err
				}
			}
			continue
		}
		if time.Now().Before(t.NextPollAt) {
			if t.DeadlineAt != nil && time.Now().After(*t.DeadlineAt) {
				finishQualityTask(t, "failed", "检测超时或第三方持续限流，未判定为降智")
				if err = repo.SaveTask(ctx, t); err != nil {
					return err
				}
			}
			continue
		}
		if t.Status == "submitting" || t.Status == "uncertain" {
			// 提交后宕机而未保存 ID 时不重发，保留并发槽直到外部最长执行期结束。
			t.Status = "uncertain"
			t.Reason = "提交结果不明确，为避免重复扣费未自动重试"
			if t.DeadlineAt == nil || time.Now().After(*t.DeadlineAt) {
				finishQualityTask(t, "failed", t.Reason)
			}
		} else {
			r.poll(ctx, t)
		}
		if err = repo.SaveTask(ctx, t); err != nil {
			return err
		}
	}
	for i := range tasks {
		t := &tasks[i]
		if t.Status != "pending" || time.Now().Before(t.NextPollAt) {
			continue
		}
		if time.Since(t.StartedAt) > time.Hour {
			finishQualityTask(t, "failed", "排队超过一小时，已结束本轮检测")
			if err = repo.SaveTask(ctx, t); err != nil {
				return err
			}
			continue
		}
		// 每次提交重新读取开关及绑定，已创建的外部任务仍会在关闭后继续回收。
		current, e := r.service.Config(ctx)
		if e != nil {
			return e
		}
		if !qualityTaskStillEnabled(current, t) {
			finishQualityTask(t, "cancelled", "功能已关闭或检测配置已变更")
			if err = repo.SaveTask(ctx, t); err != nil {
				return err
			}
			continue
		}
		_, reason := r.service.readiness(ctx, current, t.Group)
		if reason != "" {
			finishQualityTask(t, "failed", reason)
			if err = repo.SaveTask(ctx, t); err != nil {
				return err
			}
			continue
		}
		key, e := r.service.validKey(ctx, t.Group)
		if e != nil {
			return e
		}
		if e = r.validateDNS(ctx, t.Endpoint); e != nil {
			finishQualityTask(t, "failed", "API 端点地址无法解析为公网地址，检测已暂停")
			if err = repo.SaveTask(ctx, t); err != nil {
				return err
			}
			continue
		}
		now := time.Now()
		deadline := now.Add(11 * time.Minute)
		t.SubmittedAt = &now
		t.DeadlineAt = &deadline
		t.Status = "submitting"
		t.Attempts++
		claimed, e := repo.PrepareTask(ctx, t, current.Version)
		if e != nil {
			return e
		}
		if !claimed {
			break
		}
		result, e := r.client.Submit(ctx, manxue.Request{Benchmark: t.Benchmark, BaseURL: t.Endpoint, APIKey: key.Key, Model: t.Model, ReasoningEffort: t.Group.ReasoningEffort}, t.IdempotencyKey)
		if e != nil {
			var remote *manxue.Error
			if errors.As(e, &remote) && (remote.Status == 429 || remote.Status == 503) {
				t.Status = "pending"
				t.SubmittedAt = nil
				t.DeadlineAt = nil
				t.NextPollAt = time.Now().Add(qualityBackoff(t.Attempts, remote.RetryAfter))
				if t.Attempts >= 5 {
					finishQualityTask(t, "failed", "第三方服务持续限流或繁忙")
				}
			} else if errors.As(e, &remote) && !remote.Ambiguous {
				finishQualityTask(t, "failed", remote.RejectionReason())
			} else {
				t.Status = "uncertain"
				t.Reason = "提交结果不明确，为避免重复扣费未自动重试"
			}
		} else {
			t.RemoteID = result.ID
			t.Status = "running"
			t.NextPollAt = time.Now().Add(3 * time.Second)
			applyQualityResult(t, result, key.Key)
		}
		if err = repo.SaveTask(ctx, t); err != nil {
			return err
		}
	}
	return repo.FinishRounds(ctx)
}

func qualityTaskStillEnabled(c *ModelQualityConfig, t *ModelQualityTask) bool {
	if !c.Enabled || c.PauseReason != "" || c.Endpoint != t.Endpoint {
		return false
	}
	for _, g := range c.Groups {
		if g.GroupID == t.Group.GroupID && g.OwnerID == t.Group.OwnerID && g.KeyID != nil && t.Group.KeyID != nil && *g.KeyID == *t.Group.KeyID && g.ReasoningEffort == t.Group.ReasoningEffort && slices.Contains(g.Models, t.Model) && slices.Contains(g.Methods, t.Benchmark) {
			return true
		}
	}
	return false
}
func (r *ModelQualityRunner) validateDNS(ctx context.Context, endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	ips, err := r.lookup(ctx, u.Hostname())
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return errors.New("端点解析失败")
	}
	for _, ip := range ips {
		if !modelQualityPublicIP(ip.IP) {
			return errors.New("端点解析到非公网地址")
		}
	}
	return nil
}
func (r *ModelQualityRunner) poll(ctx context.Context, t *ModelQualityTask) {
	result, err := r.client.Poll(ctx, t.RemoteID)
	if err == nil {
		secret := ""
		if t.Group.KeyID != nil {
			if k, e := r.service.keys.GetByID(ctx, *t.Group.KeyID); e == nil && k != nil {
				secret = k.Key
			}
		}
		applyQualityResult(t, result, secret)
		t.NextPollAt = time.Now().Add(3 * time.Second)
	} else {
		var remote *manxue.Error
		retry := time.Duration(0)
		if errors.As(err, &remote) {
			retry = remote.RetryAfter
			if remote.Status == 404 {
				finishQualityTask(t, "failed", "第三方任务已过期或丢失，未重新提交")
				return
			}
		}
		t.Attempts++
		t.NextPollAt = time.Now().Add(qualityBackoff(t.Attempts, retry))
	}
	if t.Status == "running" && t.DeadlineAt != nil && time.Now().After(*t.DeadlineAt) {
		finishQualityTask(t, "failed", "检测超时或结果暂不可用，未判定为降智")
	}
}
func qualityBackoff(attempt int, retry time.Duration) time.Duration {
	d := time.Duration(5*(1<<min(attempt, 6)))*time.Second + time.Duration(rand.IntN(1000))*time.Millisecond
	if d < retry {
		d = retry
	}
	return d
}
func finishQualityTask(t *ModelQualityTask, status, reason string) {
	now := time.Now()
	t.Status = status
	t.Reason = reason
	t.FinishedAt = &now
	if t.SubmittedAt != nil {
		t.DurationMS = now.Sub(*t.SubmittedAt).Milliseconds()
	}
}

var qualityURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

func qualitySafeText(text, secret, endpoint string) string {
	for _, value := range []string{secret, endpoint} {
		if value != "" {
			text = strings.ReplaceAll(text, value, "[已脱敏]")
		}
	}
	return qualityURLPattern.ReplaceAllString(logredact.RedactText(text), "[地址已隐藏]")
}
func applyQualityResult(t *ModelQualityTask, result *manxue.Result, secret string) {
	switch result.Status {
	case "failed", "cancelled":
		finishQualityTask(t, "failed", "第三方检测失败或被取消，未判定为降智")
		return
	case "succeeded":
	default:
		return
	}
	finishQualityTask(t, "completed", "")
	usage := result.Usage
	if t.Benchmark == "candy" {
		t.Answer = qualitySafeText(result.Candy.Answer, secret, t.Endpoint)
		t.DurationMS = result.Candy.DurationMS
		usage = result.Candy.Usage
		switch result.Candy.Status {
		case "passed":
			t.Verdict = "normal"
			t.Reason = "糖果题通过"
		case "incorrect":
			t.Verdict = "degraded"
			t.Reason = "糖果题答案未通过"
		default:
			t.Status = "failed"
			t.Reason = "糖果检测失败或无法判定"
		}
	} else {
		t.HTML = logredact.RedactText(result.Result.HTML)
		for _, value := range []string{secret, t.Endpoint} {
			if value != "" {
				t.HTML = strings.ReplaceAll(t.HTML, value, "[已脱敏]")
			}
		}
		t.HasHTML = t.HTML != ""
		t.Reason = qualitySafeText(result.Assessment.Reason, secret, t.Endpoint)
		switch result.Assessment.Quality {
		case "normal":
			t.Verdict = "normal"
		case "degraded", "suspicious":
			t.Verdict = "degraded"
		default:
			t.Verdict = "unknown"
		}
		if t.HTML == "" {
			t.Status = "failed"
			t.Verdict = "unknown"
			t.Reason = "检测未返回作品"
		}
		if result.Result.DurationMS > 0 {
			t.DurationMS = result.Result.DurationMS
		}
		if result.Result.Usage.InputTokens != nil {
			usage = result.Result.Usage
		}
	}
	t.InputTokens = usage.InputTokens
	t.OutputTokens = usage.OutputTokens
	t.ReasoningTokens = usage.ReasoningTokens
	if t.ReasoningTokens == nil {
		t.ReasoningTokens = usage.OutputTokensDetails.ReasoningTokens
	}
}
