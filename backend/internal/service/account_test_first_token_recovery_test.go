//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFirstToken恢复探测拒绝空白后卡死(t *testing.T) {
	probe := &firstTokenRecoveryProbe{model: "gpt-test", timeout: 30 * time.Millisecond}
	ctx := context.WithValue(context.Background(), firstTokenRecoveryProbeKey{}, probe)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	resp, err := withFirstTokenRecoveryProbe(req, &Account{ID: 1}, func(guarded *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(io.MultiReader(
			strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\n"),
			&contextReadCloser{ctx: guarded.Context()},
		))}, nil
	})
	require.NoError(t, err)
	_, err = io.ReadAll(resp.Body)
	require.ErrorIs(t, err, errFirstTokenAttemptTimedOut)
	require.NoError(t, resp.Body.Close())
	require.NotEmpty(t, probe.failure())
}

func TestFirstToken恢复探测首输出达速后不限制总时长(t *testing.T) {
	probe := &firstTokenRecoveryProbe{model: "gpt-test", timeout: 50 * time.Millisecond}
	ctx := context.WithValue(context.Background(), firstTokenRecoveryProbeKey{}, probe)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	reader, writer := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer writer.Close()
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n")
		time.Sleep(100 * time.Millisecond)
		_, _ = io.WriteString(writer, "data: {\"type\":\"response.completed\"}\n\n")
	}()
	resp, err := withFirstTokenRecoveryProbe(req, &Account{ID: 1}, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: reader}, nil
	})
	require.NoError(t, err)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(data), "response.completed")
	require.NoError(t, resp.Body.Close())
	<-done
	require.Empty(t, probe.failure())
}

type firstTokenRecoveryTesterStub struct {
	model  string
	accept bool
}

type firstTokenRecoveryResultRepoStub struct {
	scheduledTestResultRepoStub
}

func (r *firstTokenRecoveryResultRepoStub) Create(_ context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	r.results = append([]*ScheduledTestResult{result}, r.results...)
	return result, nil
}

func (r *firstTokenRecoveryResultRepoStub) PruneOldResults(context.Context, int64, int) error {
	return nil
}

type firstTokenRecoveryPlanRepoStub struct {
	scheduledTestPlanRepoStub
	nextRunAt time.Time
}

func (r *firstTokenRecoveryPlanRepoStub) UpdateAfterRun(_ context.Context, _ int64, _ time.Time, nextRunAt time.Time) error {
	r.nextRunAt = nextRunAt
	return nil
}

func (s *firstTokenRecoveryTesterStub) RunTestBackground(ctx context.Context, _ int64, model string, _ ...string) (*ScheduledTestResult, error) {
	s.model = model
	if s.accept {
		probe := firstTokenRecoveryProbeFromContext(ctx)
		a := probe.begin(ctx, nil, &Account{ID: 42})
		a.start()
		a.markReceived()
		a.cleanup()
	}
	return &ScheduledTestResult{Status: "success"}, nil
}

func TestFirstToken恢复计划验证事故模型与达速门禁(t *testing.T) {
	for _, accept := range []bool{false, true} {
		t.Run(map[bool]string{false: "不能仅凭最终成功恢复", true: "达速后才恢复"}[accept], func(t *testing.T) {
			account := &Account{ID: 42, Status: StatusActive, Extra: map[string]any{
				accountFailureStrategyUnscheduledKey: map[string]any{
					accountFailureStrategyUnscheduledSourceKey:         "first_token_timeout",
					accountFailureStrategyUnscheduledIncidentIDKey:     "incident-speed",
					accountFailureStrategyUnscheduledStrictOutputKey:   true,
					accountFailureStrategyUnscheduledTimeoutSecondsKey: 20,
					accountFailureStrategyUnscheduledModelKey:          "incident-model",
				},
			}}
			planRepo := &firstTokenRecoveryPlanRepoStub{}
			tester := &firstTokenRecoveryTesterStub{accept: accept}
			recovery := &scheduledAccountRecoveryStub{}
			results := &firstTokenRecoveryResultRepoStub{}
			runner := &ScheduledTestRunnerService{
				accountRepo: &firstTokenRecoveryRepoStub{account: account},
				planRepo:    planRepo, accountTestSvc: tester, rateLimitSvc: recovery,
				scheduledSvc: NewScheduledTestService(planRepo, results),
			}
			runner.runOnePlan(context.Background(), &ScheduledTestPlan{
				ID: 7, AccountID: 42, ModelID: "unrelated-plan-model", AutoRecover: true, AutoManaged: true, MaxResults: 20,
			})
			require.Equal(t, "incident-model", tester.model)
			if accept {
				require.Equal(t, []string{"incident-speed"}, recovery.incidentIDs)
			} else {
				require.Empty(t, recovery.incidentIDs)
				require.Equal(t, "failed", results.results[0].Status)
				require.Empty(t, planRepo.disabled)
				require.True(t, planRepo.nextRunAt.After(time.Now()))
			}
		})
	}
}

func TestFirstToken恢复探测模型不重复映射且不修改账号(t *testing.T) {
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"model_mapping": map[string]any{"requested": "incident-model", "incident-model": "wrong-model", "*": "fallback"},
	}}
	require.Equal(t, "wrong-model", account.GetMappedModel("incident-model"))
	probe := &firstTokenRecoveryProbe{model: "incident-model", timeout: time.Second}
	copy := probe.accountForTest(account)
	require.Equal(t, "incident-model", copy.GetMappedModel("incident-model"))
	require.Equal(t, "wrong-model", account.GetMappedModel("incident-model"))
	require.Equal(t, "incident-model", account.GetMappedModel("requested"))
}

func TestFirstToken恢复探测及时输出后失败不能恢复(t *testing.T) {
	probe := &firstTokenRecoveryProbe{model: "gpt-test", timeout: time.Second}
	ctx := context.WithValue(context.Background(), firstTokenRecoveryProbeKey{}, probe)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(http.StatusOK,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\ndata: {\"type\":\"response.failed\",\"error\":{\"message\":\"upstream failed\"}}\n\n")}}
	c, _ := newTestContext()
	c.Request = c.Request.WithContext(ctx)
	svc := &AccountTestService{httpUpstream: upstream}
	err := svc.testOpenAIAccountConnection(c, account, probe.model, "", "")
	require.Error(t, err)
	require.Empty(t, probe.failure(), "首输出达标不能代替最终成功")
}

func TestFirstToken恢复探测Bedrock复用流式判定(t *testing.T) {
	var body bytes.Buffer
	for _, event := range []string{
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":" "}}`,
		`{"type":"content_block_delta","delta":{"type":"text_delta","text":"ok"}}`,
		`{"type":"message_stop"}`,
	} {
		headers := append([]byte{byte(len(":event-type"))}, []byte(":event-type")...)
		headers = append(headers, 7, 0, 5)
		headers = append(headers, []byte("chunk")...)
		payload := []byte(fmt.Sprintf(`{"bytes":%q}`, base64.StdEncoding.EncodeToString([]byte(event))))
		frame := make([]byte, 12+len(headers)+len(payload)+4)
		binary.BigEndian.PutUint32(frame[:4], uint32(len(frame)))
		binary.BigEndian.PutUint32(frame[4:8], uint32(len(headers)))
		binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))
		copy(frame[12:], headers)
		copy(frame[12+len(headers):], payload)
		binary.BigEndian.PutUint32(frame[len(frame)-4:], crc32.ChecksumIEEE(frame[:len(frame)-4]))
		_, _ = body.Write(frame)
	}
	probe := &firstTokenRecoveryProbe{model: "claude-test", timeout: time.Second}
	ctx := context.WithValue(context.Background(), firstTokenRecoveryProbeKey{}, probe)
	req := httptest.NewRequest(http.MethodPost, "/invoke-with-response-stream", nil).WithContext(ctx)
	account := &Account{ID: 1}
	resp, err := withFirstTokenRecoveryProbe(req, account, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/vnd.amazon.eventstream"}}, Body: io.NopCloser(&body)}, nil
	})
	require.NoError(t, err)
	defer resp.Body.Close()
	c, recorder := newTestContext()
	c.Request = c.Request.WithContext(ctx)
	require.NoError(t, (&AccountTestService{}).processBedrockRecoveryStream(c, resp.Body, account))
	require.Empty(t, probe.failure())
	require.Contains(t, recorder.Body.String(), "test_complete")
	require.Contains(t, recorder.Body.String(), "ok")
}
