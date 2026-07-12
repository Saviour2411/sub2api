package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type handlerImageSuccessRateRepoStub struct {
	aggregates []service.ImageGroupSuccessRateAggregate
	records    []handlerImageSuccessRateRecord
}

type handlerImageSuccessRateRecord struct {
	groupID   int64
	successes int64
	failures  int64
}

func (r *handlerImageSuccessRateRepoStub) Record(_ context.Context, groupID, successes, failures int64, _ time.Time) error {
	r.records = append(r.records, handlerImageSuccessRateRecord{groupID: groupID, successes: successes, failures: failures})
	return nil
}

func (r *handlerImageSuccessRateRepoStub) RecordOnce(context.Context, string, int64, int64, int64, time.Time) error {
	return nil
}

func (r *handlerImageSuccessRateRepoStub) ListCurrent(context.Context) ([]service.ImageGroupSuccessRateAggregate, error) {
	return r.aggregates, nil
}

func (r *handlerImageSuccessRateRepoStub) Reset(context.Context) (time.Time, error) {
	return time.Time{}, nil
}

func TestTrackImageGroupRequestResult只按最终结果记录一次(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name      string
		status    int
		streamErr bool
		selected  bool
		wantFail  bool
		wantCount int
	}{
		{name: "成功", status: http.StatusOK, selected: true, wantCount: 1},
		{name: "HTTP 失败", status: http.StatusBadGateway, selected: true, wantFail: true, wantCount: 1},
		{name: "流内失败", status: http.StatusOK, streamErr: true, selected: true, wantFail: true, wantCount: 1},
		{name: "未选中账号", status: http.StatusServiceUnavailable, wantCount: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &handlerImageSuccessRateRepoStub{}
			h := &ChannelMonitorUserHandler{successRateService: service.NewImageGroupSuccessRateService(repo)}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				groupID := int64(9)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID})
				c.Next()
			})
			router.POST("/v1/messages", h.TrackImageGroupRequestResult(), func(c *gin.Context) {
				if tt.selected {
					setOpsSelectedAccount(c, 101)
				}
				if tt.streamErr {
					service.MarkOpsStreamError(c, "upstream_error", "failed", http.StatusBadGateway)
				}
				c.Status(tt.status)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
			router.ServeHTTP(recorder, req)

			require.Len(t, repo.records, tt.wantCount)
			if tt.wantCount == 1 {
				require.Equal(t, int64(9), repo.records[0].groupID)
				if tt.wantFail {
					require.Equal(t, int64(1), repo.records[0].failures)
				} else {
					require.Equal(t, int64(1), repo.records[0].successes)
				}
			}
		})
	}
}

func TestImageGroupSuccessRateEndpoint排除非生成请求(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		path string
		want bool
	}{
		{path: "/v1/messages", want: true},
		{path: "/v1/responses/compact", want: true},
		{path: "/v1/images/generations", want: true},
		{path: "/v1beta/models/gemini-3:streamGenerateContent", want: true},
		{path: "/v1/messages/count_tokens"},
		{path: "/v1/embeddings"},
		{path: "/v1/images/batches"},
		{path: "/v1beta/models/gemini-3:countTokens"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tt.path, nil)
			require.Equal(t, tt.want, isImageGroupSuccessRateEndpoint(c))
		})
	}
}

func TestImageGroupSuccessRatesResponse不泄露内部计数(t *testing.T) {
	repo := &handlerImageSuccessRateRepoStub{aggregates: []service.ImageGroupSuccessRateAggregate{{
		GroupID: 3, GroupName: "GPT Image", RequestCount: 8, FailureCount: 2,
	}}}
	h := &ChannelMonitorUserHandler{successRateService: service.NewImageGroupSuccessRateService(repo)}

	result, err := h.imageGroupSuccessRates(context.Background())
	require.NoError(t, err)
	body, err := json.Marshal(result)
	require.NoError(t, err)

	require.JSONEq(t, `{"visible":true,"items":[{"group_id":3,"group_name":"GPT Image","success_rate":75,"last_success_at":null}]}`, string(body))
	require.NotContains(t, string(body), "request_count")
	require.NotContains(t, string(body), "failure_count")
}
