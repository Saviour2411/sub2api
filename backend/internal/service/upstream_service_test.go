package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamPlainEncryptor struct{}

func (upstreamPlainEncryptor) Encrypt(plaintext string) (string, error) {
	return "ENC:" + plaintext, nil
}

func (upstreamPlainEncryptor) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "ENC:") {
		return "", fmt.Errorf("密文无效")
	}
	return strings.TrimPrefix(ciphertext, "ENC:"), nil
}

type upstreamCredentialFailureRepo struct {
	UpstreamRepository
	site                *UpstreamSite
	updatedCredential   string
	markedFailureReason string
}

func (r *upstreamCredentialFailureRepo) GetByID(context.Context, int64) (*UpstreamSite, error) {
	copy := *r.site
	return &copy, nil
}

func (r *upstreamCredentialFailureRepo) MarkSyncing(context.Context, int64) error { return nil }

func (r *upstreamCredentialFailureRepo) MissingDates(context.Context, int64, time.Time, time.Time, *time.Location) ([]time.Time, error) {
	return []time.Time{time.Now()}, nil
}

func (r *upstreamCredentialFailureRepo) UpdateCredential(_ context.Context, _ int64, encrypted string) error {
	r.updatedCredential = encrypted
	return nil
}

func (r *upstreamCredentialFailureRepo) MarkSyncFailed(_ context.Context, _ int64, message string, _ *time.Time) error {
	r.markedFailureReason = message
	return nil
}

type upstreamCredentialFailureProvider struct {
	credential UpstreamCredential
}

func (p upstreamCredentialFailureProvider) Platform() string { return UpstreamPlatformSub2API }
func (p upstreamCredentialFailureProvider) Validate(context.Context, *UpstreamSite, UpstreamCredential) (*UpstreamCredential, error) {
	return nil, nil
}
func (p upstreamCredentialFailureProvider) Sync(context.Context, UpstreamSyncRequest) (*UpstreamSyncResult, error) {
	return &UpstreamSyncResult{Credential: &p.credential}, errors.New("刷新后统计请求失败")
}

type upstreamMultiplierHistoryRangeRepo struct {
	UpstreamRepository
	called  int
	from    time.Time
	through time.Time
}

type upstreamGroupDisplayRepo struct {
	UpstreamRepository
	remoteID  string
	displayed bool
}

type upstreamSortOrderRepo struct {
	UpstreamRepository
	updates []UpstreamSortOrderUpdate
}

type upstreamGroupBindingsRepo struct {
	UpstreamRepository
	called          int
	siteID          int64
	upstreamGroupID int64
	inputs          []UpstreamGroupAccountBindingInput
	result          *UpstreamGroup
	err             error
}

func (r *upstreamSortOrderRepo) UpdateSortOrder(_ context.Context, updates []UpstreamSortOrderUpdate) error {
	r.updates = append([]UpstreamSortOrderUpdate(nil), updates...)
	return nil
}

func (r *upstreamGroupBindingsRepo) ReplaceGroupBindings(
	_ context.Context,
	siteID, upstreamGroupID int64,
	inputs []UpstreamGroupAccountBindingInput,
) (*UpstreamGroup, error) {
	r.called++
	r.siteID = siteID
	r.upstreamGroupID = upstreamGroupID
	r.inputs = append([]UpstreamGroupAccountBindingInput(nil), inputs...)
	return r.result, r.err
}

func (r *upstreamGroupDisplayRepo) SetGroupDisplayed(_ context.Context, _ int64, remoteID string, displayed bool) (*UpstreamGroupDisplayResult, error) {
	r.remoteID = remoteID
	r.displayed = displayed
	return &UpstreamGroupDisplayResult{}, nil
}

func (r *upstreamMultiplierHistoryRangeRepo) ListMultiplierHistory(
	_ context.Context,
	_ int64,
	from time.Time,
	through time.Time,
) ([]UpstreamGroupMultiplierHistory, error) {
	r.called++
	r.from = from
	r.through = through
	return nil, nil
}

func TestUpstreamServiceCredentialEnvelopeAndMaskedView(t *testing.T) {
	service := &UpstreamService{encryptor: upstreamPlainEncryptor{}}
	encrypted, err := service.encryptCredential(UpstreamCredential{Password: "secret", AccessToken: "sensitive-access-value"})
	require.NoError(t, err)
	require.NotContains(t, encrypted, `"has_password"`)

	view := service.toView(&UpstreamSite{
		ID: 1, Name: "站点", AuthMode: UpstreamAuthPassword, CredentialEncrypted: encrypted, BindingCount: 3,
	})
	require.True(t, view.HasPassword)
	require.False(t, view.HasToken)
	require.Equal(t, 3, view.BindingCount)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "sensitive-access-value")
}

func TestMergeUpstreamUpdateKeepsBlankCredential(t *testing.T) {
	site := &UpstreamSite{Name: "旧名称", BaseURL: "https://example.com", Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthPassword, Account: "admin"}
	credential := UpstreamCredential{Password: "old-password", AccessToken: "old-token"}
	empty := ""
	newName := "新名称"
	changed := mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{
		Name: &newName, Password: &empty, AccessToken: &empty, RefreshToken: &empty,
	})
	require.True(t, changed)
	require.Equal(t, "old-password", credential.Password)
	require.Equal(t, "old-token", credential.AccessToken)
}

func TestMergeUpstreamUpdateClearsCredentialFromPreviousAuthMode(t *testing.T) {
	t.Run("密码切换为令牌", func(t *testing.T) {
		site := &UpstreamSite{AuthMode: UpstreamAuthPassword, Account: "admin"}
		credential := UpstreamCredential{Password: "old-password", Cookie: "old-cookie"}
		authMode := UpstreamAuthToken
		accessToken := "new-token"
		changed := mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{
			AuthMode: &authMode, AccessToken: &accessToken,
		})

		require.True(t, changed)
		require.Empty(t, credential.Password)
		require.Empty(t, credential.Cookie)
		require.Equal(t, "new-token", credential.AccessToken)
	})

	t.Run("令牌切换为密码", func(t *testing.T) {
		site := &UpstreamSite{AuthMode: UpstreamAuthToken}
		credential := UpstreamCredential{AccessToken: "old-access", RefreshToken: "old-refresh", Cookie: "old-cookie"}
		authMode := UpstreamAuthPassword
		password := "new-password"
		changed := mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{
			AuthMode: &authMode, Password: &password,
		})

		require.True(t, changed)
		require.Empty(t, credential.AccessToken)
		require.Empty(t, credential.RefreshToken)
		require.Empty(t, credential.Cookie)
		require.Equal(t, "new-password", credential.Password)
	})
}

func TestMergeUpstreamUpdateInvalidatesCachedSessionWhenCredentialScopeChanges(t *testing.T) {
	assertSessionCleared := func(t *testing.T, credential UpstreamCredential) {
		t.Helper()
		require.Empty(t, credential.AccessToken)
		require.Empty(t, credential.RefreshToken)
		require.Empty(t, credential.Cookie)
		require.Empty(t, credential.NewAPIUserID)
	}
	credentialFixture := func() UpstreamCredential {
		return UpstreamCredential{
			Password: "old-password", AccessToken: "old-access", RefreshToken: "old-refresh",
			Cookie: "old-cookie", NewAPIUserID: "9",
		}
	}

	t.Run("账号变化", func(t *testing.T) {
		site := &UpstreamSite{BaseURL: "https://example.com", Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthPassword, Account: "old"}
		credential := credentialFixture()
		account := "new"
		mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{Account: &account})
		assertSessionCleared(t, credential)
		require.Equal(t, "old-password", credential.Password)
	})

	t.Run("上游地址变化", func(t *testing.T) {
		site := &UpstreamSite{BaseURL: "https://old.example.com", Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthToken}
		credential := credentialFixture()
		baseURL := "https://new.example.com"
		mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{BaseURL: &baseURL})
		assertSessionCleared(t, credential)
	})

	t.Run("显式更新密码", func(t *testing.T) {
		site := &UpstreamSite{BaseURL: "https://example.com", Platform: UpstreamPlatformNewAPI, AuthMode: UpstreamAuthPassword, Account: "admin"}
		credential := credentialFixture()
		password := "new-password"
		mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{Password: &password})
		assertSessionCleared(t, credential)
		require.Equal(t, "new-password", credential.Password)
	})
}

func TestUpstreamServiceRejectsNewAPITokenMode(t *testing.T) {
	service := &UpstreamService{}
	err := service.validateSite(&UpstreamSite{
		Name: "New API", BaseURL: "https://example.com", Platform: UpstreamPlatformNewAPI,
		AuthMode: UpstreamAuthToken,
	}, UpstreamCredential{AccessToken: "token"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "new API 仅支持密码认证")
}

func TestUpstreamServicePersistsRotatedCredentialWhenSyncDataFails(t *testing.T) {
	repo := &upstreamCredentialFailureRepo{}
	svc := &UpstreamService{
		repo:      repo,
		encryptor: upstreamPlainEncryptor{},
		providers: map[string]UpstreamProvider{
			UpstreamPlatformSub2API: upstreamCredentialFailureProvider{credential: UpstreamCredential{
				AccessToken: "new-access", RefreshToken: "new-refresh",
			}},
		},
		location: time.FixedZone("Asia/Shanghai", 8*60*60),
	}
	initial, err := svc.encryptCredential(UpstreamCredential{AccessToken: "old-access", RefreshToken: "old-refresh"})
	require.NoError(t, err)
	repo.site = &UpstreamSite{
		ID: 1, Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthToken,
		CredentialEncrypted: initial, Enabled: true, TrackingStartedAt: time.Now(),
	}

	err = svc.RunSync(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, repo.markedFailureReason, "统计请求失败")
	require.NotEmpty(t, repo.updatedCredential)
	updated, err := svc.decryptCredential(repo.updatedCredential)
	require.NoError(t, err)
	require.Equal(t, "new-access", updated.AccessToken)
	require.Equal(t, "new-refresh", updated.RefreshToken)
}

func TestUpstreamServiceListMultiplierHistoryDateRangeBoundary(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	from := time.Date(2026, time.January, 1, 12, 30, 0, 0, loc)
	repo := &upstreamMultiplierHistoryRangeRepo{}
	service := &UpstreamService{repo: repo, location: loc}

	items, err := service.ListMultiplierHistory(context.Background(), 1, from, from.AddDate(0, 0, 365))
	require.NoError(t, err)
	require.Nil(t, items)
	require.Equal(t, 1, repo.called)
	require.Equal(t, time.Date(2026, time.January, 1, 0, 0, 0, 0, loc), repo.from)
	require.Equal(t, time.Date(2027, time.January, 2, 0, 0, 0, 0, loc).Add(-time.Nanosecond), repo.through)

	items, err = service.ListMultiplierHistory(context.Background(), 1, from, from.AddDate(0, 0, 366))
	require.Error(t, err)
	require.Nil(t, items)
	require.Contains(t, err.Error(), "1 到 366 天")
	require.Equal(t, 1, repo.called, "367 天范围不得查询仓储")
}

func TestUpstreamServiceSetGroupDisplayedValidatesInput(t *testing.T) {
	repo := &upstreamGroupDisplayRepo{}
	svc := &UpstreamService{repo: repo}
	show := true

	_, err := svc.SetGroupDisplayed(context.Background(), 1, UpstreamGroupDisplayInput{RemoteID: " vip ", Displayed: &show})
	require.NoError(t, err)
	require.Equal(t, "vip", repo.remoteID)
	require.True(t, repo.displayed)

	for _, input := range []UpstreamGroupDisplayInput{
		{RemoteID: "", Displayed: &show},
		{RemoteID: "vip", Displayed: nil},
		{RemoteID: strings.Repeat("x", 101), Displayed: &show},
	} {
		_, err = svc.SetGroupDisplayed(context.Background(), 1, input)
		require.ErrorIs(t, err, ErrUpstreamInvalidInput)
	}
}

func TestUpstreamServiceUpdateSortOrderValidatesInput(t *testing.T) {
	repo := &upstreamSortOrderRepo{}
	svc := &UpstreamService{repo: repo}
	updates := []UpstreamSortOrderUpdate{{ID: 2, SortOrder: 0}, {ID: 1, SortOrder: 10}}
	require.NoError(t, svc.UpdateSortOrder(context.Background(), updates))
	require.Equal(t, updates, repo.updates)

	for _, invalid := range [][]UpstreamSortOrderUpdate{
		nil,
		{{ID: 0, SortOrder: 0}},
		{{ID: 1, SortOrder: -1}},
		{{ID: 1, SortOrder: 0}, {ID: 1, SortOrder: 10}},
	} {
		repo.updates = nil
		require.ErrorIs(t, svc.UpdateSortOrder(context.Background(), invalid), ErrUpstreamInvalidInput)
		require.Empty(t, repo.updates)
	}
}

func TestUpstreamServiceReplaceGroupBindingsValidatesInput(t *testing.T) {
	repo := &upstreamGroupBindingsRepo{result: &UpstreamGroup{ID: 7, SiteID: 3}}
	svc := &UpstreamService{repo: repo}
	inputs := []UpstreamGroupAccountBindingInput{
		{LocalGroupID: 10, AccountID: 100},
		{LocalGroupID: 10, AccountID: 101},
	}

	group, err := svc.ReplaceGroupBindings(context.Background(), 3, 7, inputs)
	require.NoError(t, err)
	require.Same(t, repo.result, group)
	require.Equal(t, 1, repo.called)
	require.Equal(t, int64(3), repo.siteID)
	require.Equal(t, int64(7), repo.upstreamGroupID)
	require.Equal(t, inputs, repo.inputs)

	group, err = svc.ReplaceGroupBindings(context.Background(), 3, 7, nil)
	require.NoError(t, err, "空绑定集合应表示全部解绑")
	require.Same(t, repo.result, group)
	require.NotNil(t, group.Bindings, "响应应稳定序列化为空数组，而不是 null")
	require.Equal(t, 2, repo.called)
	require.Empty(t, repo.inputs)

	tooMany := make([]UpstreamGroupAccountBindingInput, maxUpstreamGroupBindings+1)
	for i := range tooMany {
		tooMany[i] = UpstreamGroupAccountBindingInput{LocalGroupID: 1, AccountID: int64(i + 1)}
	}
	invalidCases := []struct {
		name            string
		siteID          int64
		upstreamGroupID int64
		inputs          []UpstreamGroupAccountBindingInput
	}{
		{name: "站点 ID 非正数", siteID: 0, upstreamGroupID: 7},
		{name: "上游分组 ID 非正数", siteID: 3, upstreamGroupID: -1},
		{name: "本地分组 ID 非正数", siteID: 3, upstreamGroupID: 7, inputs: []UpstreamGroupAccountBindingInput{{LocalGroupID: 0, AccountID: 1}}},
		{name: "账号 ID 非正数", siteID: 3, upstreamGroupID: 7, inputs: []UpstreamGroupAccountBindingInput{{LocalGroupID: 1, AccountID: 0}}},
		{name: "请求内账号重复", siteID: 3, upstreamGroupID: 7, inputs: []UpstreamGroupAccountBindingInput{{LocalGroupID: 1, AccountID: 9}, {LocalGroupID: 2, AccountID: 9}}},
		{name: "绑定数量超限", siteID: 3, upstreamGroupID: 7, inputs: tooMany},
	}
	for _, tt := range invalidCases {
		t.Run(tt.name, func(t *testing.T) {
			called := repo.called
			group, err := svc.ReplaceGroupBindings(context.Background(), tt.siteID, tt.upstreamGroupID, tt.inputs)
			require.Nil(t, group)
			require.ErrorIs(t, err, ErrUpstreamInvalidInput)
			require.Equal(t, called, repo.called, "无效参数不得调用仓储")
		})
	}
}

func TestUpstreamServiceProbeCapabilitiesDetectsTurnstile(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/settings/public", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		writeUpstreamJSON(t, w, map[string]any{
			"turnstile_enabled":  true,
			"turnstile_site_key": "site-key",
		})
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	svc := &UpstreamService{http: newTestUpstreamHTTPClient(t)}
	capabilities, err := svc.ProbeCapabilities(context.Background(), UpstreamProbeInput{
		BaseURL: server.URL + "/", Platform: " SUB2API ",
	})
	require.NoError(t, err)
	require.Equal(t, server.URL, capabilities.BaseURL)
	require.Equal(t, UpstreamPlatformSub2API, capabilities.Platform)
	require.True(t, capabilities.TurnstileEnabled)
	require.True(t, capabilities.TokenAuthRecommended)
}
