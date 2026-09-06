package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
)

type firstTokenRecoveryProbeKey struct{}

// 恢复探测独立使用事故快照，不更新业务请求计数，也不依赖当前分组选项。
type firstTokenRecoveryProbe struct {
	model    string
	timeout  time.Duration
	mu       sync.Mutex
	attempts []*firstTokenAttempt
}

func (s *ScheduledTestRunnerService) prepareFirstTokenRecoveryProbe(ctx context.Context, plan *ScheduledTestPlan, incidentID string) (context.Context, string, *firstTokenRecoveryProbe, error) {
	if !plan.AutoRecover || incidentID == "" || s.accountRepo == nil {
		return ctx, plan.ModelID, nil, nil
	}
	account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil {
		return ctx, plan.ModelID, nil, err
	}
	marker := failureStrategyUnscheduledMarker(account)
	strict, _ := marker[accountFailureStrategyUnscheduledStrictOutputKey].(bool)
	if !strict || failureMarkerString(marker, accountFailureStrategyUnscheduledSourceKey) != string(AccountFailureStreakSourceFirstTokenTimeout) ||
		failureMarkerString(marker, accountFailureStrategyUnscheduledIncidentIDKey) != incidentID {
		return ctx, plan.ModelID, nil, nil
	}
	seconds := failureMarkerInt(marker, accountFailureStrategyUnscheduledTimeoutSecondsKey)
	model := failureMarkerString(marker, accountFailureStrategyUnscheduledModelKey)
	if seconds <= 0 || seconds > MaxGatewayFirstTokenTimeoutSeconds || model == "" {
		return ctx, plan.ModelID, nil, fmt.Errorf("首 Token 恢复事故快照无效")
	}
	probe := &firstTokenRecoveryProbe{model: model, timeout: time.Duration(seconds) * time.Second}
	return context.WithValue(ctx, firstTokenRecoveryProbeKey{}, probe), model, probe, nil
}

func firstTokenRecoveryProbeFromContext(ctx context.Context) *firstTokenRecoveryProbe {
	if ctx == nil {
		return nil
	}
	probe, _ := ctx.Value(firstTokenRecoveryProbeKey{}).(*firstTokenRecoveryProbe)
	return probe
}

func (p *firstTokenRecoveryProbe) begin(ctx context.Context, c *gin.Context, account *Account) *firstTokenAttempt {
	a := newFirstTokenAttemptWithTimeout(ctx, c, nil, account, p.model, p.timeout)
	a.strictOutput = true
	p.mu.Lock()
	p.attempts = append(p.attempts, a)
	p.mu.Unlock()
	return a
}

func (p *firstTokenRecoveryProbe) failure() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.attempts) > 0 {
		accepted := true
		for _, a := range p.attempts {
			accepted = accepted && a.currentState() == firstTokenAttemptReceived
		}
		if accepted {
			return ""
		}
	}
	return fmt.Sprintf("恢复探测未在 %g 秒内收到有效首 Token", p.timeout.Seconds())
}

// 事故模型已经过上游映射；只在探测副本中固定它，避免再次命中通配符或链式映射。
func (p *firstTokenRecoveryProbe) accountForTest(account *Account) *Account {
	// Antigravity 首 Token 守卫保存的是原始请求模型，由其网关执行映射。
	if account.Platform == PlatformAntigravity {
		return account
	}
	return accountWithExactTestModel(account, p.model)
}

func withFirstTokenRecoveryProbe(req *http.Request, account *Account, send func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	probe := firstTokenRecoveryProbeFromContext(req.Context())
	if probe == nil {
		return send(req)
	}
	attempt := probe.begin(req.Context(), nil, account)
	attempt.start()
	resp, err := send(attempt.bindRequest(req))
	if err != nil || resp == nil || resp.Body == nil {
		attempt.cleanup()
		return resp, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		attempt.stopBeforeStreaming(resp)
		return resp, nil
	}
	protocol := firstTokenProtocolSSE
	if strings.Contains(resp.Header.Get("Content-Type"), "amazon.eventstream") {
		protocol = firstTokenProtocolBedrock
	}
	attempt.wrapResponse(resp, nil, protocol)
	resp.Body = &firstTokenCleanupReadCloser{upstream: resp.Body, cleanup: attempt.cleanup}
	return resp, nil
}

func (s *AccountTestService) doAccountTestUpstreamWithTLS(req *http.Request, proxyURL string, account *Account, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return withFirstTokenRecoveryProbe(req, account, func(guarded *http.Request) (*http.Response, error) {
		return s.httpUpstream.DoWithTLS(guarded, proxyURL, account.ID, account.Concurrency, profile)
	})
}

func (s *AccountTestService) doAccountTestUpstream(req *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	return withFirstTokenRecoveryProbe(req, account, func(guarded *http.Request) (*http.Response, error) {
		return s.httpUpstream.Do(guarded, proxyURL, account.ID, account.Concurrency)
	})
}

// 复用现有 EventStream 解码器，将恢复探测交给统一的 Claude 流式结果检查。
type bedrockRecoverySSEReader struct {
	decoder *bedrockEventStreamDecoder
	pending []byte
}

func (r *bedrockRecoverySSEReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.pending) == 0 {
		payload, err := r.decoder.Decode()
		if err != nil {
			return 0, err
		}
		data := extractBedrockChunkData(payload)
		if len(data) == 0 {
			return 0, fmt.Errorf("恢复探测返回无效 Bedrock 事件")
		}
		r.pending = []byte(fmt.Sprintf("data: %s\n\n", data))
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (s *AccountTestService) processBedrockRecoveryStream(c *gin.Context, body io.Reader, account *Account) error {
	return s.processClaudeStream(c, &bedrockRecoverySSEReader{decoder: newBedrockEventStreamDecoder(body)}, account)
}
