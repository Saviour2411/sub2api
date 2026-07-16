package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type customFeatureUpstreamBindingsRepo struct {
	service.UpstreamRepository
	called          int
	siteID          int64
	upstreamGroupID int64
	inputs          []service.UpstreamGroupAccountBindingInput
	result          *service.UpstreamGroup
	err             error
}

func (r *customFeatureUpstreamBindingsRepo) ReplaceGroupBindings(
	_ context.Context,
	siteID, upstreamGroupID int64,
	inputs []service.UpstreamGroupAccountBindingInput,
) (*service.UpstreamGroup, error) {
	r.called++
	r.siteID = siteID
	r.upstreamGroupID = upstreamGroupID
	r.inputs = append([]service.UpstreamGroupAccountBindingInput(nil), inputs...)
	return r.result, r.err
}

func newCustomFeatureUpstreamBindingsRouter(t *testing.T, repo service.UpstreamRepository) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	upstreamService, err := service.NewUpstreamService(repo, nil, &config.Config{})
	require.NoError(t, err)
	handler := NewCustomFeatureHandler(nil, nil)
	handler.SetUpstreamService(upstreamService)
	router := gin.New()
	router.PUT("/api/v1/admin/custom-features/upstreams/:id/groups/:groupID/bindings", handler.ReplaceUpstreamGroupBindings)
	return router
}

func TestCustomFeatureHandlerReplaceUpstreamGroupBindings(t *testing.T) {
	now := time.Date(2026, time.July, 16, 12, 0, 0, 0, time.UTC)
	repo := &customFeatureUpstreamBindingsRepo{result: &service.UpstreamGroup{
		ID: 7, SiteID: 3, Name: "vip", Bindings: []service.UpstreamGroupAccountBinding{{
			ID: 21, UpstreamGroupID: 7, LocalGroupID: 10, LocalGroupName: "默认分组",
			AccountID: 100, AccountName: "账号 A", AccountPlatform: "openai",
			AccountStatus: "active", AccountPriority: 10, CreatedAt: now,
		}},
	}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/custom-features/upstreams/3/groups/7/bindings",
		bytes.NewBufferString(`{"bindings":[{"local_group_id":10,"account_id":100}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	newCustomFeatureUpstreamBindingsRouter(t, repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, repo.called)
	require.Equal(t, int64(3), repo.siteID)
	require.Equal(t, int64(7), repo.upstreamGroupID)
	require.Equal(t, []service.UpstreamGroupAccountBindingInput{{LocalGroupID: 10, AccountID: 100}}, repo.inputs)
	var body struct {
		Data service.UpstreamGroup `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Len(t, body.Data.Bindings, 1)
	require.Equal(t, "默认分组", body.Data.Bindings[0].LocalGroupName)
	require.Equal(t, 10, body.Data.Bindings[0].AccountPriority)
}

func TestCustomFeatureHandlerReplaceUpstreamGroupBindingsRejectsInvalidRequest(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "站点 ID 无效", path: "/api/v1/admin/custom-features/upstreams/0/groups/7/bindings", body: `{"bindings":[]}`},
		{name: "分组 ID 无效", path: "/api/v1/admin/custom-features/upstreams/3/groups/nope/bindings", body: `{"bindings":[]}`},
		{name: "请求格式无效", path: "/api/v1/admin/custom-features/upstreams/3/groups/7/bindings", body: `{`},
		{name: "缺少 bindings", path: "/api/v1/admin/custom-features/upstreams/3/groups/7/bindings", body: `{}`},
		{name: "账号重复", path: "/api/v1/admin/custom-features/upstreams/3/groups/7/bindings", body: `{"bindings":[{"local_group_id":1,"account_id":9},{"local_group_id":2,"account_id":9}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &customFeatureUpstreamBindingsRepo{}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPut, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			newCustomFeatureUpstreamBindingsRouter(t, repo).ServeHTTP(recorder, request)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Zero(t, repo.called)
		})
	}
}

func TestCustomFeatureHandlerReplaceUpstreamGroupBindingsReturnsConflict(t *testing.T) {
	repo := &customFeatureUpstreamBindingsRepo{err: service.ErrUpstreamBindingConflict}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/custom-features/upstreams/3/groups/7/bindings",
		bytes.NewBufferString(`{"bindings":[{"local_group_id":10,"account_id":100}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	newCustomFeatureUpstreamBindingsRouter(t, repo).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusConflict, recorder.Code)
	var body struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "UPSTREAM_BINDING_CONFLICT", body.Reason)
}
