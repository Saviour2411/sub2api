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

func TestTrackImageGroupRequestResult排除业务拒绝(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name           string
		status         int
		message        string
		code           string
		upstreamStatus int
		upstreamDetail string
		events         []*service.OpsUpstreamErrorEvent
		streamError    *service.OpsStreamError
		sse            bool
		cancel         bool
		wantCount      int
		wantSuccess    bool
	}{
		{name: "余额403", status: 403, message: "用户余额不足"},
		{name: "403包装为502", status: 502, message: "Upstream access forbidden, please contact administrator", upstreamStatus: 403, upstreamDetail: "error code: 1010"},
		{name: "最后一次403事件", status: 502, events: []*service.OpsUpstreamErrorEvent{{UpstreamStatusCode: 403}}},
		{name: "内容审核拒绝", status: 400, message: "生成的图片可能违反了我们的内容政策"},
		{name: "输入参数错误", status: 400, message: "Missing required parameter: 'input'."},
		{name: "尺寸超过限制", status: 400, message: "图片尺寸无效：最大支持 8K 分辨率"},
		{name: "空提示词", status: 400, message: "缺少有效的 prompt，请提供非空的图片描述"},
		{name: "包装后的参数错误", status: 502, upstreamStatus: 400, upstreamDetail: "{\"error\":{\"code\":\"invalid_image\",\"message\":\"unreadable\"}}"},
		{name: "400超时保留失败", status: 400, message: "生图任务超时", wantCount: 1},
		{name: "434超时保留失败", status: 434, message: "生图超时，请重新生成", wantCount: 1},
		{name: "未知400保留失败", status: 400, message: "图片生成没有返回可用的图片文件", wantCount: 1},
		{name: "上游401保留失败", status: 502, upstreamStatus: 401, wantCount: 1},
		{name: "上游429保留失败", status: 429, upstreamStatus: 429, wantCount: 1},
		{name: "重试后成功仍计成功", status: 200, upstreamStatus: 403, wantCount: 1, wantSuccess: true},
		{name: "最后一次500不受早先403影响", status: 502, upstreamStatus: 403, events: []*service.OpsUpstreamErrorEvent{{UpstreamStatusCode: 403}, {UpstreamStatusCode: 500}}, wantCount: 1},
		{name: "最后网络失败不继承403", status: 502, upstreamStatus: 403, events: []*service.OpsUpstreamErrorEvent{{UpstreamStatusCode: 403}, {Kind: "request_error", Message: "connection reset"}}, wantCount: 1},
		{name: "流中余额不足", status: 200, streamError: &service.OpsStreamError{IntendedStatus: 403, Code: "INSUFFICIENT_BALANCE", Message: "余额不足"}},
		{name: "流中审核拒绝", status: 200, streamError: &service.OpsStreamError{IntendedStatus: 400, Code: "content_policy_violation", Message: "blocked"}},
		{name: "流中包装403", status: 200, upstreamStatus: 403, streamError: &service.OpsStreamError{IntendedStatus: 502, Message: "Upstream access forbidden"}},
		{name: "请求级错误不继承403", status: 200, upstreamStatus: 403, streamError: &service.OpsStreamError{IntendedStatus: 500, Message: "unknown failure", RequestScoped: true}, wantCount: 1},
		{name: "流中断仍计失败", status: 200, streamError: &service.OpsStreamError{IntendedStatus: 502, Message: "Upstream response stream was interrupted"}, wantCount: 1},
		{name: "未显式标记的流中审核拒绝", status: 200, code: "content_policy_violation", message: "blocked", sse: true},
		{name: "未显式标记的流中故障", status: 200, code: "server_error", message: "upstream failed", sse: true, wantCount: 1},
		{name: "主动取消", status: 502, message: "cancelled", cancel: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &handlerImageSuccessRateRepoStub{}
			h := &ChannelMonitorUserHandler{successRateService: service.NewImageGroupSuccessRateService(repo)}
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(nil))
			router.Use(func(c *gin.Context) {
				groupID := int64(9)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{GroupID: &groupID})
				c.Next()
			})
			router.POST("/v1/images/generations", h.TrackImageGroupRequestResult(), func(c *gin.Context) {
				setOpsSelectedAccount(c, 101)
				service.SetOpsUpstreamError(c, tt.upstreamStatus, "", tt.upstreamDetail)
				if tt.events != nil {
					c.Set(service.OpsUpstreamErrorsKey, tt.events)
				}
				if tt.streamError != nil {
					service.MarkOpsStreamErrorValue(c, *tt.streamError)
				}
				if tt.cancel {
					ctx, cancel := context.WithCancel(c.Request.Context())
					cancel()
					c.Request = c.Request.WithContext(ctx)
				}
				errorBody := gin.H{"type": "invalid_request_error", "code": tt.code, "message": tt.message}
				if tt.sse {
					body, err := json.Marshal(gin.H{"type": "response.failed", "response": gin.H{"error": errorBody}})
					require.NoError(t, err)
					// 不带末尾换行，覆盖捕获器在请求结束时补齐终态帧。
					c.Data(tt.status, "text/event-stream", append([]byte("event: response.failed\ndata: "), body...))
					return
				}
				c.JSON(tt.status, gin.H{"error": errorBody})
			})

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader("{}")))
			require.Equal(t, tt.status, recorder.Code)
			require.Len(t, repo.records, tt.wantCount)
			if tt.wantCount > 0 {
				require.Equal(t, int64(1), repo.records[0].successes+repo.records[0].failures)
				require.Equal(t, tt.wantSuccess, repo.records[0].successes == 1)
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
		{path: "/v1/videos/edits", want: true},
		{path: "/videos/extensions", want: true},
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
