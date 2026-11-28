package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelQualitySameModelConfig(t *testing.T) {
	s, repo, keys, _, _ := qualityFixture()
	delete(keys.keys, 6)
	ctx := context.Background()
	c, err := s.ConfigV2(ctx)
	require.NoError(t, err)
	require.Nil(t, c.Questions[1].Rule.Judge)
	_, reason := s.qualityV2Readiness(ctx, c, c.Groups[0])
	require.Empty(t, reason)
	c.Groups[0].Targets[0].MaxOutputTokens = 0
	c.Groups[0].Targets[0].TimeoutSeconds = 0
	c.Questions[1].Rule.Judge = &ModelQualityCall{Protocol: "obsolete", GroupID: 999}
	require.NoError(t, s.SaveConfigV2(ctx, 3, c))
	require.Nil(t, repo.config.Questions[1].Rule.Judge)
	require.Equal(t, 128000, repo.config.Groups[0].Targets[0].MaxOutputTokens)
	require.Equal(t, 1200, repo.config.Groups[0].Targets[0].TimeoutSeconds)
	c.Groups[0].Targets[0].MaxOutputTokens = 4096
	c.Groups[0].Targets[0].TimeoutSeconds = 90
	require.NoError(t, s.SaveConfigV2(ctx, 3, c))
	require.Equal(t, 4096, repo.config.Groups[0].Targets[0].MaxOutputTokens)
	require.Equal(t, 90, repo.config.Groups[0].Targets[0].TimeoutSeconds)
}

func TestModelQualitySameModelJudgment(t *testing.T) {
	_, repo, _, runner, client := qualityFixture()
	task := qualityFixtureTask(repo, 1)
	call := task.Specification.Target.ModelQualityCall
	task.Specification.Question.Rule.Judge = &call
	client.outputs = []string{"<svg>作品</svg>", "通过"}
	runner.execute(repo, task)
	require.Equal(t, "ready_judge", task.Status)
	task.Status = "judging"
	task.Result.Judgment.Attempts = 1
	runner.execute(repo, task)
	require.Equal(t, "normal", task.Result.Verdict)
	require.Len(t, client.calls, 2)
	require.Equal(t, client.calls[0], client.calls[1])
	require.NotEqual(t, client.prompts[0], client.prompts[1])
}

func TestModelQualityMissingAnswerCannotPass(t *testing.T) {
	for _, answer := range []string{"", "  ", "暂无答案", "正在生成"} {
		verdict, _ := evaluateQualityAnswer(ModelQualityRule{Kind: "number", Expected: "21"}, answer)
		require.Equal(t, "unknown", verdict)
	}
}

func TestModelQualityTimeoutStageAndRequestID(t *testing.T) {
	for _, headers := range []bool{false, true} {
		t.Run(map[bool]string{false: "响应头", true: "正文"}[headers], func(t *testing.T) {
			ids := make(chan string, 1)
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ids <- r.Header.Get("X-Client-Request-Id")
				if headers {
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
				}
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer func() {
				close(release)
				server.Close()
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			client := &qualityProtocolClient{http: server.Client()}
			phase, err := client.Call(ctx, server.URL, "key", ModelQualityCall{Protocol: "messages", Model: "test", MaxOutputTokens: 128000, TimeoutSeconds: 1200}, "", "题目")
			require.ErrorContains(t, err, map[bool]string{false: "等待模型响应头超时", true: "接收模型正文超时"}[headers])
			require.Equal(t, "failed", phase.Status)
			require.NotEmpty(t, phase.RequestID)
			require.Equal(t, <-ids, phase.RequestID)
			require.NotNil(t, phase.StartedAt)
			require.NotNil(t, phase.FinishedAt)
		})
	}
}
