package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
		ID: 1, Name: "站点", AuthMode: UpstreamAuthPassword, CredentialEncrypted: encrypted,
	})
	require.True(t, view.HasPassword)
	require.False(t, view.HasToken)
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
