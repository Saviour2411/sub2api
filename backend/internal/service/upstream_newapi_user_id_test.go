package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamUserIDRepo struct {
	UpstreamRepository
	site *UpstreamSite
}

func (r *upstreamUserIDRepo) Create(_ context.Context, site *UpstreamSite) error {
	site.ID = 1
	copy := *site
	r.site = &copy
	return nil
}

func (r *upstreamUserIDRepo) GetByID(context.Context, int64) (*UpstreamSite, error) {
	copy := *r.site
	return &copy, nil
}

func (r *upstreamUserIDRepo) Update(_ context.Context, site *UpstreamSite) error {
	copy := *site
	r.site = &copy
	return nil
}

func TestUpstreamServiceNewAPIUserIDCreateUpdateAndSync(t *testing.T) {
	wantToken := "test-personal-token"
	calls := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "270", r.Header.Get("New-Api-User"), "首次认证及后续查询都必须携带用户 ID")
		require.Equal(t, "Bearer "+wantToken, r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/api/user/self":
			writeUpstreamJSON(t, w, map[string]any{"id": 270, "quota": 500000})
		case "/api/status":
			writeUpstreamJSON(t, w, map[string]any{"quota_per_unit": 500000})
		case "/api/user/self/groups":
			writeUpstreamJSON(t, w, map[string]any{"test": map[string]any{"ratio": 0.3}})
		case "/api/pricing":
			writeUpstreamJSON(t, w, map[string]any{})
		case "/api/log/self/stat":
			writeUpstreamJSON(t, w, map[string]any{"quota": 100})
		default:
			t.Errorf("不应调用认证刷新或登录接口：%s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := newTestUpstreamHTTPClient(t)
	provider := newNewAPIUpstreamProvider(client)
	repo := &upstreamUserIDRepo{}
	svc := &UpstreamService{
		repo: repo, encryptor: upstreamPlainEncryptor{}, http: client,
		providers: map[string]UpstreamProvider{UpstreamPlatformNewAPI: provider},
	}
	var input UpstreamCreateInput
	require.NoError(t, json.Unmarshal([]byte("{\"name\":\"测试上游\",\"platform\":\"newapi\",\"auth_mode\":\"token\",\"newapi_user_id\":\" 270 \"}"), &input))
	input.BaseURL = server.URL
	input.AccessToken = wantToken
	view, err := svc.Create(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "270", view.NewAPIUserID)
	require.True(t, view.HasToken)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.Contains(t, string(raw), "\"newapi_user_id\":\"270\"")
	require.NotContains(t, string(raw), wantToken)
	credential, err := svc.decryptCredential(repo.site.CredentialEncrypted)
	require.NoError(t, err)
	require.Equal(t, "270", credential.NewAPIUserID)

	name, empty := "修改名称", ""
	view, err = svc.Update(context.Background(), 1, UpstreamUpdateInput{Name: &name, AccessToken: &empty})
	require.NoError(t, err)
	require.Equal(t, "270", view.NewAPIUserID, "未提交 ID 时保留现有值")

	wantToken = "replacement-personal-token"
	userID := " 270 "
	view, err = svc.Update(context.Background(), 1, UpstreamUpdateInput{AccessToken: &wantToken, NewAPIUserID: &userID})
	require.NoError(t, err)
	require.Equal(t, "270", view.NewAPIUserID, "更换令牌后使用显式提交的 ID")
	credential, err = svc.decryptCredential(repo.site.CredentialEncrypted)
	require.NoError(t, err)
	require.Equal(t, wantToken, credential.AccessToken)
	require.Equal(t, "270", credential.NewAPIUserID)

	result, err := provider.Sync(context.Background(), UpstreamSyncRequest{
		Site: repo.site, Credential: credential,
		Dates: []time.Time{time.Now()}, Location: time.Local,
	})
	require.NoError(t, err)
	require.Equal(t, "270", result.Credential.NewAPIUserID)
	require.InDelta(t, 1, *result.BalanceUSD, 1e-9)
	require.Len(t, result.Groups, 1)
	require.InDelta(t, 0.0002, result.Groups[0].TodayCostUSD, 1e-9)
	require.Equal(t, 4, calls["/api/user/self"])
	require.Equal(t, 2, calls["/api/log/self/stat"])
}

func TestUpstreamServiceValidatesExplicitNewAPIUserID(t *testing.T) {
	svc := &UpstreamService{http: newTestUpstreamHTTPClient(t)}
	for _, value := range []string{"", "270", " 270 ", "9223372036854775807"} {
		t.Run("有效_"+value, func(t *testing.T) {
			_, credential, err := svc.prepareCreate(UpstreamCreateInput{
				Name: "测试", BaseURL: "https://example.com", Platform: UpstreamPlatformNewAPI,
				AuthMode: UpstreamAuthToken, AccessToken: "test-token", NewAPIUserID: value,
			})
			require.NoError(t, err)
			require.NotContains(t, credential.NewAPIUserID, " ")
		})
	}
	for _, value := range []string{"0", "-1", "+270", "0270", "2.7", "1e3", "abc", "9223372036854775808", "270\r\nX-Test: yes"} {
		t.Run("无效_"+value, func(t *testing.T) {
			_, _, err := svc.prepareCreate(UpstreamCreateInput{
				Name: "测试", BaseURL: "https://example.com", Platform: UpstreamPlatformNewAPI,
				AuthMode: UpstreamAuthToken, AccessToken: "test-token", NewAPIUserID: value,
			})
			require.ErrorIs(t, err, ErrUpstreamInvalidInput)
		})
	}
}

func TestMergeUpstreamUpdateExplicitNewAPIUserID(t *testing.T) {
	for _, tt := range []struct {
		name    string
		payload string
		want    string
	}{
		{"省略保留", "{}", "9"},
		{"空值清除", "{\"newapi_user_id\":\"\"}", ""},
		{"更新并去除空格", "{\"newapi_user_id\":\" 270 \"}", "270"},
		{"更换令牌清除旧ID", "{\"access_token\":\"new-token\"}", ""},
		{"更换令牌并指定ID", "{\"access_token\":\"new-token\",\"newapi_user_id\":\"270\"}", "270"},
		{"切换密码时不使用显式ID", "{\"auth_mode\":\"password\",\"newapi_user_id\":\"270\"}", ""},
		{"切换平台时不使用显式ID", "{\"platform\":\"sub2api\",\"newapi_user_id\":\"270\"}", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			site := &UpstreamSite{Platform: UpstreamPlatformNewAPI, AuthMode: UpstreamAuthToken}
			credential := UpstreamCredential{AccessToken: "old-token", NewAPIUserID: "9"}
			var input UpstreamUpdateInput
			require.NoError(t, json.Unmarshal([]byte(tt.payload), &input))
			mergeUpstreamUpdate(site, &credential, input)
			sanitizeNewAPIPersonalAccessTokenCredential(site, &credential)
			require.Equal(t, tt.want, credential.NewAPIUserID)
		})
	}
}

func TestNewAPIUpstreamProviderMissingUserIDError(t *testing.T) {
	for _, tt := range []struct {
		name         string
		userID       string
		status       int
		message      string
		wantRequired bool
	}{
		{"缺少ID明确提示", "", 401, "Unauthorized, New-Api-User header not provided", true},
		{"无效令牌保持原错误", "", 401, "Unauthorized, invalid access token", false},
		{"已填ID保持原错误", "270", 401, "Unauthorized, New-Api-User header not provided", false},
		{"其他状态不误判", "", 403, "Unauthorized, New-Api-User header not provided", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/api/user/self", r.URL.Path, "失败后不应尝试登录或刷新")
				http.Error(w, tt.message, tt.status)
			}))
			defer server.Close()
			provider := newNewAPIUpstreamProvider(newTestUpstreamHTTPClient(t))
			_, err := provider.Validate(context.Background(), &UpstreamSite{
				BaseURL: server.URL, Platform: UpstreamPlatformNewAPI, AuthMode: UpstreamAuthToken,
			}, UpstreamCredential{AccessToken: "test-token", NewAPIUserID: tt.userID})
			require.Error(t, err)
			if tt.wantRequired {
				require.ErrorIs(t, err, ErrUpstreamNewAPIUserIDRequired)
				require.ErrorIs(t, upstreamValidationError(err), ErrUpstreamNewAPIUserIDRequired)
			} else {
				require.NotErrorIs(t, err, ErrUpstreamNewAPIUserIDRequired)
				require.ErrorIs(t, upstreamValidationError(err), ErrUpstreamConnectionFailed)
			}
		})
	}
}
