package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auth "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type qualityAccountHandlerRepo struct {
	service.ModelQualityRepository
	service.ModelQualityV2Repository
	run          service.ModelQualityV2Run
	allowed      bool
	err          error
	privateReads int
}

func (r *qualityAccountHandlerRepo) QualityV2Config(context.Context) (*service.ModelQualityV2Config, error) {
	return &service.ModelQualityV2Config{Enabled: true, Groups: []service.ModelQualityV2GroupConfig{{GroupID: 7}}}, nil
}
func (r *qualityAccountHandlerRepo) QualityV2Runs(context.Context, service.ModelQualityFilter) ([]service.ModelQualityV2Run, int64, error) {
	return []service.ModelQualityV2Run{r.run}, 1, nil
}
func (r *qualityAccountHandlerRepo) QualityV2Run(context.Context, string) (*service.ModelQualityV2Run, error) {
	return &r.run, nil
}
func (r *qualityAccountHandlerRepo) QualityV2AccountHits(_ context.Context, _ int64, ids []string) (map[string][]service.ModelQualityAccountHit, bool, error) {
	r.privateReads++
	return map[string][]service.ModelQualityAccountHit{ids[0]: {{TestID: "art", Phase: "generation", AccountID: 740, AccountName: "私有渠道账号", Source: "usage"}}}, r.allowed, r.err
}

type qualityAccountSettings struct{ service.SettingRepository }

func (qualityAccountSettings) GetValue(context.Context, string) (string, error) {
	return "", errors.New("未配置端点")
}

func TestModelQualityAccountResponsePrivacy(t *testing.T) {
	runID := "v2-12345678-1234-4234-9234-123456789abc"
	repo := &qualityAccountHandlerRepo{allowed: true, run: service.ModelQualityV2Run{ID: runID, GroupID: 7, StartedAt: time.Now(), Tests: []service.ModelQualityV2Test{}}}
	s := service.NewModelQualityService(repo, service.NewSettingService(qualityAccountSettings{}, nil), nil, nil)
	h := NewModelQualityHandler(s)
	for _, endpoint := range []string{"/runs?group_id=7", "/runs/" + runID} {
		for _, tc := range []struct {
			name             string
			id               int64
			role             string
			allowed, private bool
			dbError          bool
		}{
			{"指定管理员", 1, "admin", true, true, false},
			{"同名普通用户", 82, "user", true, false, false},
			{"其他管理员", 83, "admin", true, false, false},
			{"身份没有管理员角色", 1, "user", true, false, false},
			{"数据库已撤权", 1, "admin", false, false, false},
			{"权限查询失败", 1, "admin", false, false, true},
			{"没有认证身份", 0, "", true, false, false},
		} {
			t.Run(endpoint+tc.name, func(t *testing.T) {
				repo.allowed, repo.err, repo.privateReads = tc.allowed, nil, 0
				if tc.dbError {
					repo.err = errors.New("权限读取失败")
				}
				r := gin.New()
				r.Use(func(c *gin.Context) {
					if tc.id > 0 {
						c.Set(string(auth.ContextKeyUser), auth.AuthSubject{UserID: tc.id})
						c.Set(string(auth.ContextKeyUserRole), tc.role)
					}
				})
				r.GET("/runs", h.RunsV2)
				r.GET("/runs/:id", h.RunV2)
				request := httptest.NewRequest(http.MethodGet, endpoint, nil)
				query := request.URL.Query()
				query.Set("include_accounts", "true")
				query.Set("viewer_id", "1")
				query.Set("username", "saviour2411")
				request.URL.RawQuery = query.Encode()
				request.Header.Set("X-User-Id", "1")
				request.Header.Set("X-User-Role", "admin")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, request)
				if tc.dbError {
					require.Equal(t, 500, w.Code)
				} else {
					require.Equal(t, 200, w.Code)
					require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
				}
				if tc.private {
					require.Contains(t, w.Body.String(), `"account_hits"`)
					require.Contains(t, w.Body.String(), `"account_id":740`)
					require.Contains(t, w.Body.String(), "私有渠道账号")
				} else {
					for _, value := range []string{"account_hits", "account_id", "account_name", "私有渠道账号"} {
						require.NotContains(t, w.Body.String(), value)
					}
				}
				if tc.id != 1 || tc.role != "admin" {
					require.Zero(t, repo.privateReads, "未授权身份不得查询账号关联")
				}
			})
		}
	}
}
