package service

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/manxue"
	"github.com/stretchr/testify/require"
)

type qualitySettingsStub struct {
	SettingRepository
	endpoint string
}

func (s qualitySettingsStub) GetValue(context.Context, string) (string, error) {
	return s.endpoint, nil
}

type qualityKeysStub struct {
	APIKeyRepository
	key *APIKey
}

func (s *qualityKeysStub) GetByID(context.Context, int64) (*APIKey, error) { return s.key, nil }
func (s *qualityKeysStub) ListAllByUserID(context.Context, int64, APIKeyListFilters) ([]APIKey, error) {
	if s.key == nil {
		return nil, nil
	}
	return []APIKey{*s.key}, nil
}

type qualityGroupsStub struct {
	GroupRepository
	group *Group
}

func (s qualityGroupsStub) GetByID(context.Context, int64) (*Group, error) { return s.group, nil }

type qualityRepoStub struct {
	ModelQualityRepository
	mu                         sync.Mutex
	config                     ModelQualityConfig
	tasks                      []ModelQualityTask
	cleaned, started, finished int
}

func (s *qualityRepoStub) Config(context.Context) (*ModelQualityConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.config)
	var c ModelQualityConfig
	_ = json.Unmarshal(raw, &c)
	return &c, nil
}
func (s *qualityRepoStub) SaveConfig(_ context.Context, c *ModelQualityConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, &s.config)
}
func (s *qualityRepoStub) Cleanup(context.Context) error      { s.cleaned++; return nil }
func (s *qualityRepoStub) FinishRounds(context.Context) error { s.finished++; return nil }
func (s *qualityRepoStub) States(context.Context) (map[int64]ModelQualityGroupState, error) {
	return map[int64]ModelQualityGroupState{}, nil
}
func (s *qualityRepoStub) StartRound(_ context.Context, c *ModelQualityConfig, g ModelQualityGroupConfig, name, endpoint string, manual bool) error {
	if s.started > 0 {
		return ErrModelQualityConflict
	}
	s.started++
	for _, model := range g.Models {
		for _, method := range g.Methods {
			s.tasks = append(s.tasks, ModelQualityTask{ID: int64(len(s.tasks) + 1), Group: g, Endpoint: endpoint, Model: model, StartedAt: time.Now(), IdempotencyKey: "stable", ModelQualityTest: ModelQualityTest{Benchmark: method, Status: "pending", Verdict: "unknown"}})
		}
	}
	return nil
}
func (s *qualityRepoStub) Tasks(context.Context) ([]ModelQualityTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []ModelQualityTask{}
	for _, t := range s.tasks {
		if t.Status == "pending" || t.Status == "running" || t.Status == "submitting" || t.Status == "uncertain" {
			out = append(out, t)
		}
	}
	return out, nil
}
func (s *qualityRepoStub) SaveTask(_ context.Context, task *ModelQualityTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID == task.ID {
			s.tasks[i] = *task
			return nil
		}
	}
	return errors.New("任务不存在")
}
func (s *qualityRepoStub) PrepareTask(_ context.Context, task *ModelQualityTask, version int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, t := range s.tasks {
		if t.Status == "submitting" || t.Status == "running" || t.Status == "uncertain" {
			count++
		}
	}
	if count >= 2 || !s.config.Enabled {
		return false, nil
	}
	for i := range s.tasks {
		if s.tasks[i].ID == task.ID && s.tasks[i].Status == "pending" {
			s.tasks[i] = *task
			return true, nil
		}
	}
	return false, nil
}

type qualityClientStub struct {
	submitted, polled int
	submitErr         error
	result            *manxue.Result
}

func (s *qualityClientStub) Submit(_ context.Context, r manxue.Request, key string) (*manxue.Result, error) {
	s.submitted++
	if s.submitErr != nil {
		return nil, s.submitErr
	}
	return &manxue.Result{ID: "remote", Status: "running"}, nil
}
func (s *qualityClientStub) Poll(context.Context, string) (*manxue.Result, error) {
	s.polled++
	if s.result != nil {
		return s.result, nil
	}
	return &manxue.Result{Status: "running"}, nil
}
func qualityFixture() (*ModelQualityService, *qualityRepoStub, *qualityKeysStub, *ModelQualityRunner, *qualityClientStub) {
	id := int64(5)
	groupID := int64(7)
	g := ModelQualityGroupConfig{GroupID: 7, KeyID: &id, OwnerID: 3, IntervalMinutes: 60, Methods: []string{"candy", "pelican"}, Models: []string{"model-a", "model-b"}, ReasoningEffort: "low"}
	repo := &qualityRepoStub{config: ModelQualityConfig{Enabled: true, Version: 1, Groups: []ModelQualityGroupConfig{g}}}
	keys := &qualityKeysStub{key: &APIKey{ID: 5, UserID: 3, GroupID: &groupID, Key: "sk-test-secret", Status: StatusActive, User: &User{ID: 3, Status: StatusActive, Role: RoleAdmin}}}
	s := NewModelQualityService(repo, &SettingService{settingRepo: qualitySettingsStub{endpoint: "https://quality.example.net"}}, keys, qualityGroupsStub{group: &Group{ID: 7, Name: "测试分组", Platform: PlatformOpenAI, Status: StatusActive}})
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	s.lookup = lookup
	r := NewModelQualityRunner(s)
	r.lookup = lookup
	client := &qualityClientStub{}
	r.client = client
	return s, repo, keys, r, client
}
func TestModelQualityEndpoint(t *testing.T) {
	for _, raw := range []string{"http://public.net", "https://127.0.0.1", "https://10.0.0.5", "https://[::1]", "https://localhost", "https://host.internal", "https://host.internal.", "https://user:pass@public.net", "https://public.net?q=1", "https://public.net:0", "https://public.net:65536", "https://[2001:db8::1]"} {
		_, err := NormalizeModelQualityEndpoint(raw)
		require.Error(t, err, raw)
	}
	for _, raw := range []string{"https://api.example.net/custom", "https://api.example.net/custom/v1/", "https://api.example.net/custom/v1/v1"} {
		got, err := NormalizeModelQualityEndpoint(raw)
		require.NoError(t, err)
		require.Equal(t, "https://api.example.net/custom/v1", got)
	}
	got, err := NormalizeModelQualityEndpoint("https://[2606:4700:4700::1111]:8443")
	require.NoError(t, err)
	require.Equal(t, "https://[2606:4700:4700::1111]:8443/v1", got)
	s, _, _, _, _ := qualityFixture()
	s.lookup = func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("192.168.0.1")}}, nil
	}
	c, err := s.Config(context.Background())
	require.NoError(t, err)
	require.Contains(t, c.PauseReason, "非公网")
}
func TestModelQualityConfigAndKeyOwnership(t *testing.T) {
	s, repo, _, _, _ := qualityFixture()
	ctx := context.Background()
	c, _ := repo.Config(ctx)
	c.Groups[0].Models = []string{" model-a ", "model-a"}
	c.Groups[0].KeyID = nil
	require.NoError(t, s.SaveConfig(ctx, 3, c))
	require.Equal(t, []string{"model-a"}, c.Groups[0].Models)
	states, err := s.Groups(ctx)
	require.NoError(t, err)
	require.Equal(t, "待绑定密钥", states[0].PauseReason)
	c.Groups[0].Models = nil
	require.Error(t, s.SaveConfig(ctx, 3, c))
	s, repo, keys, _, _ := qualityFixture()
	c, _ = repo.Config(ctx)
	require.Error(t, s.SaveConfig(ctx, 99, c))
	c, _ = repo.Config(ctx)
	past := time.Now().Add(-time.Hour)
	keys.key.ExpiresAt = &past
	require.Error(t, s.SaveConfig(ctx, 3, c))
	keys.key.ExpiresAt = nil
	other := int64(9)
	keys.key.GroupID = &other
	require.Error(t, s.SaveConfig(ctx, 3, c))
	keys.key.GroupID = &c.Groups[0].GroupID
	keys.key.Status = StatusAPIKeyDisabled
	require.Error(t, s.SaveConfig(ctx, 3, c))
	keys.key.Status = StatusActive
	keys.key.Quota = 1
	keys.key.QuotaUsed = 1
	require.Error(t, s.SaveConfig(ctx, 3, c))
}
func TestModelQualityDisableWithExpiredKey(t *testing.T) {
	s, repo, keys, _, _ := qualityFixture()
	ctx := context.Background()
	past := time.Now().Add(-time.Hour)
	keys.key.ExpiresAt = &past
	c, err := repo.Config(ctx)
	require.NoError(t, err)
	c.Enabled = false
	require.NoError(t, s.SaveConfig(ctx, 99, c))
	require.False(t, repo.config.Enabled)
	require.EqualValues(t, 3, repo.config.Groups[0].OwnerID)
	c.Enabled = true
	require.Error(t, s.SaveConfig(ctx, 99, c))
	c.Enabled = false
	c.Groups[0].Models = []string{"new-model"}
	require.Error(t, s.SaveConfig(ctx, 99, c))
}

func TestModelQualityMultipleModelsGlobalSlotsAndRecovery(t *testing.T) {
	s, repo, _, runner, client := qualityFixture()
	ctx := context.Background()
	require.NoError(t, runner.step(ctx))
	require.Len(t, repo.tasks, 4)
	require.Equal(t, 2, client.submitted)
	restarted := NewModelQualityRunner(s)
	restarted.lookup = runner.lookup
	restarted.client = client
	for i := range repo.tasks {
		repo.tasks[i].NextPollAt = time.Time{}
	}
	require.NoError(t, restarted.step(ctx))
	require.Equal(t, 2, client.submitted)
	require.Equal(t, 2, client.polled)
	repo.config.Enabled = false
	for i := range repo.tasks {
		repo.tasks[i].NextPollAt = time.Time{}
	}
	require.NoError(t, restarted.step(ctx))
	require.Equal(t, 2, client.submitted)
	require.Equal(t, 4, client.polled)
	require.Equal(t, "cancelled", repo.tasks[2].Status)
	require.Positive(t, repo.cleaned)
}
func TestModelQualityUncertainSubmissionNeverResent(t *testing.T) {
	_, repo, _, r, c := qualityFixture()
	c.submitErr = &manxue.Error{Ambiguous: true}
	require.NoError(t, r.step(context.Background()))
	require.Equal(t, 2, c.submitted)
	require.Equal(t, "uncertain", repo.tasks[0].Status)
	require.NoError(t, r.step(context.Background()))
	require.Equal(t, 2, c.submitted)
	require.Equal(t, "unknown", repo.tasks[0].Verdict)
}
func TestModelQualityRejectedSubmissionPreservesSafeReason(t *testing.T) {
	for _, tc := range []struct {
		name, body, reason string
	}{
		{"参数不兼容", `{"error":"Invalid API request parameters"}`, "检测接口参数不兼容"},
		{"鹈鹕模型限制", `{"error":"model is required and must be gpt-6-astra for pelican, creative and html"}`, "鹈鹕检测目前仅支持 gpt-6-astra"},
		{"未知错误脱敏", `{"error":"sk-test-secret https://quality.example.net/v1"}`, "请检查模型和端点配置"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, repo, _, runner, _ := qualityFixture()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				var body map[string]any
				require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
				require.NotContains(t, body, "protocol")
				require.Contains(t, []string{"model-a", "model-b"}, body["model"])
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			client := manxue.NewClient()
			client.BaseURL = server.URL
			runner.client = client
			require.NoError(t, runner.step(context.Background()))
			require.Len(t, repo.tasks, 4)
			for _, task := range repo.tasks {
				require.Equal(t, "failed", task.Status)
				require.Equal(t, "unknown", task.Verdict)
				require.Contains(t, task.Reason, "HTTP 400")
				require.Contains(t, task.Reason, tc.reason)
				require.NotContains(t, task.Reason, "sk-test-secret")
				require.NotContains(t, task.Reason, "quality.example.net")
				require.NotNil(t, task.FinishedAt)
			}
		})
	}
}

func TestModelQualityRetryAfterAndPartialResults(t *testing.T) {
	_, repo, _, r, c := qualityFixture()
	c.submitErr = &manxue.Error{Status: 429, RetryAfter: 120 * time.Second}
	before := time.Now()
	require.NoError(t, r.step(context.Background()))
	for _, task := range repo.tasks {
		require.Equal(t, "pending", task.Status)
		require.True(t, task.NextPollAt.After(before.Add(119*time.Second)))
	}
	result := &manxue.Result{Status: "succeeded"}
	result.Candy.Status = "incorrect"
	result.Candy.Answer = "20 sk-secret https://private.local/v1"
	candy := &ModelQualityTask{Endpoint: "https://private.local/v1", ModelQualityTest: ModelQualityTest{Benchmark: "candy", Status: "running", Verdict: "unknown"}}
	applyQualityResult(candy, result, "sk-secret")
	require.Equal(t, "degraded", candy.Verdict)
	require.NotContains(t, candy.Answer, "sk-secret")
	require.NotContains(t, candy.Answer, "private.local")
	art := &ModelQualityTask{ModelQualityTest: ModelQualityTest{Benchmark: "pelican", Status: "running", Verdict: "unknown"}}
	applyQualityResult(art, &manxue.Result{Status: "failed"}, "")
	require.Equal(t, "failed", art.Status)
	require.Equal(t, "unknown", art.Verdict)
	require.Equal(t, "degraded", candy.Verdict)
	result.Candy.Status = "error"
	candy.Verdict = "unknown"
	applyQualityResult(candy, result, "")
	require.Equal(t, "failed", candy.Status)
	require.Equal(t, "unknown", candy.Verdict)
}
func TestModelQualityExpiredTimeoutAndKeyRebind(t *testing.T) {
	_, repo, keys, r, c := qualityFixture()
	require.NoError(t, r.step(context.Background()))
	past := time.Now().Add(-time.Minute)
	for i := range repo.tasks {
		repo.tasks[i].NextPollAt = time.Time{}
		if repo.tasks[i].Status == "running" {
			repo.tasks[i].DeadlineAt = &past
		}
	}
	keys.key.Status = StatusAPIKeyDisabled
	require.NoError(t, r.step(context.Background()))
	require.Equal(t, 2, c.submitted)
	for _, task := range repo.tasks {
		require.Equal(t, "failed", task.Status)
		require.Equal(t, "unknown", task.Verdict)
	}
}
func TestModelQualityDisabledAndHiddenGroups(t *testing.T) {
	s, repo, _, _, _ := qualityFixture()
	require.ErrorIs(t, s.visible(context.Background(), 999), ErrModelQualityExpired)
	repo.config.Enabled = false
	_, err := s.Groups(context.Background())
	require.ErrorIs(t, err, ErrModelQualityDisabled)
	_, _, err = s.Runs(context.Background(), ModelQualityFilter{GroupID: 7, Page: 1})
	require.ErrorIs(t, err, ErrModelQualityDisabled)
}

func TestModelQualityBackoffDoesNotDelayCloseOrDeadline(t *testing.T) {
	_, repo, _, runner, client := qualityFixture()
	require.NoError(t, runner.step(context.Background()))
	past := time.Now().Add(-time.Minute)
	for i := range repo.tasks {
		repo.tasks[i].NextPollAt = time.Now().Add(24 * time.Hour)
		if repo.tasks[i].Status == "running" {
			repo.tasks[i].DeadlineAt = &past
		}
	}
	repo.config.Enabled = false
	require.NoError(t, runner.step(context.Background()))
	require.Equal(t, 2, client.submitted)
	for _, task := range repo.tasks {
		require.Contains(t, []string{"failed", "cancelled"}, task.Status)
		require.Equal(t, "unknown", task.Verdict)
	}
}
