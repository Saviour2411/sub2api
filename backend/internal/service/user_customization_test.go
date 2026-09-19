package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type customizationTestRepo struct {
	UserCustomizationRepository
	item      UserCustomization
	saves     int
	hash      string
	encrypted string
	resolves  int
	complete  int
	grants    int
	search    string
	page      int
	size      int
}

func (r *customizationTestRepo) List(_ context.Context, search string, page, size int) ([]UserCustomization, int64, error) {
	r.search, r.page, r.size = search, page, size
	return []UserCustomization{r.item}, 1, nil
}

func (r *customizationTestRepo) Get(context.Context, int64) (*UserCustomization, error) {
	return &r.item, nil
}
func (r *customizationTestRepo) Save(_ context.Context, _ int64, in UserCustomizationInput) error {
	r.saves++
	r.item.UserCustomizationInput = in
	return nil
}
func (r *customizationTestRepo) RotateLink(_ context.Context, _, _ int64, hash, encrypted string) error {
	r.hash = hash
	r.encrypted = encrypted
	return nil
}
func (r *customizationTestRepo) GetLinkSecret(context.Context, int64) (string, error) {
	return r.encrypted, nil
}
func (r *customizationTestRepo) ResolvePublic(context.Context, string) (*PublicBalanceUser, error) {
	r.resolves++
	return &PublicBalanceUser{ID: 1, Username: "用户甲", Balance: 90}, nil
}
func (r *customizationTestRepo) PublicHistory(context.Context, int64, int, int) ([]PublicRechargeRecord, int64, error) {
	return []PublicRechargeRecord{{Type: TemporaryCreditType, Amount: 50, Note: "不应公开的内部备注"}, {Type: "admin_balance", Amount: 12, Note: "内部订单信息"}}, 2, nil
}
func (r *customizationTestRepo) PendingCreditInvalidations(_ context.Context, after int64, _ int) ([]CreditCacheInvalidation, error) {
	if after > 0 {
		return nil, nil
	}
	return []CreditCacheInvalidation{{ID: 1, UserID: 1}}, nil
}
func (r *customizationTestRepo) CompleteCreditInvalidation(context.Context, int64) error {
	r.complete++
	return nil
}
func (r *customizationTestRepo) TryGrantCredit(context.Context, int64) (bool, error) {
	r.grants++
	return true, nil
}

type customizationSettingsStub struct {
	SettingRepository
	values map[string]string
}

func (s *customizationSettingsStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return s.values, nil
}
func (s *customizationSettingsStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

type customizationEncryptor struct{}

func (customizationEncryptor) Encrypt(value string) (string, error) { return "密文:" + value, nil }
func (customizationEncryptor) Decrypt(value string) (string, error) {
	return strings.TrimPrefix(value, "密文:"), nil
}

func TestUserCustomizationMoneyValidation(t *testing.T) {
	for _, value := range []float64{-1, math.NaN(), math.Inf(1), 1e12, 1e-9} {
		repo := &customizationTestRepo{}
		s := NewUserCustomizationService(repo, nil, nil)
		_, err := s.Save(context.Background(), 1, UserCustomizationInput{AutoCreditEnabled: true, CreditThreshold: 1000, CreditAmount: value})
		require.ErrorIs(t, err, ErrUserCustomizationInvalid)
		require.Zero(t, repo.saves)
	}
	repo := &customizationTestRepo{}
	s := NewUserCustomizationService(repo, nil, nil)
	_, err := s.Save(context.Background(), 1, UserCustomizationInput{AutoCreditEnabled: true, CreditThreshold: 1000, CreditAmount: 0})
	require.ErrorIs(t, err, ErrUserCustomizationInvalid)
	_, err = s.Save(context.Background(), 1, UserCustomizationInput{CreditThreshold: 1000})
	require.NoError(t, err)
	_, err = s.Save(context.Background(), 1, UserCustomizationInput{AutoCreditEnabled: true, CreditThreshold: 1000, CreditAmount: 0.00000001})
	require.NoError(t, err)
}

func TestUserCustomizationListTrimsSearchAndPreservesAdminEmail(t *testing.T) {
	for _, search := range []string{" box ", "\tBOX\n", " 查询 ", " 1054 ", "  "} {
		t.Run(search, func(t *testing.T) {
			repo := &customizationTestRepo{item: UserCustomization{UserID: 1054, Email: "boxinsmart@example.test"}}
			s := NewUserCustomizationService(repo, nil, nil)
			items, total, err := s.List(context.Background(), search, 2, 20)
			require.NoError(t, err)
			require.Equal(t, strings.TrimSpace(search), repo.search)
			require.Equal(t, 2, repo.page)
			require.Equal(t, 20, repo.size)
			require.EqualValues(t, 1, total)
			require.Equal(t, "boxinsmart@example.test", items[0].Email)
		})
	}
}

func TestUserCustomizationPaymentMethods(t *testing.T) {
	settings := &customizationSettingsStub{values: map[string]string{}}
	s := NewUserCustomizationService(nil, settings, nil)
	methods, err := s.PaymentMethods(context.Background())
	require.NoError(t, err)
	require.NotNil(t, methods)
	for _, methods := range [][]UserPaymentMethod{
		make([]UserPaymentMethod, 21), {{Key: "", Value: "x"}}, {{Key: "ID", Value: " "}},
		{{Key: strings.Repeat("名", 51), Value: "x"}}, {{Key: "ID", Value: strings.Repeat("值", 501)}},
	} {
		_, err := s.SavePaymentMethods(context.Background(), methods)
		require.ErrorIs(t, err, ErrUserCustomizationInvalid)
	}
	methods, err = s.SavePaymentMethods(context.Background(), []UserPaymentMethod{{Key: " 币安 ID ", Value: " 123 "}, {Key: "备注", Value: "<script>不可执行</script>"}})
	require.NoError(t, err)
	require.Equal(t, "币安 ID", methods[0].Key)
	loaded, err := s.PaymentMethods(context.Background())
	require.NoError(t, err)
	require.Equal(t, methods, loaded)
}

func TestUserCustomizationLinkAndMinimalPublicDTO(t *testing.T) {
	repo := &customizationTestRepo{}
	s := NewUserCustomizationService(repo, &customizationSettingsStub{values: map[string]string{}}, customizationEncryptor{})
	_, err := s.RotateLink(context.Background(), 1, 0)
	require.NoError(t, err)
	token, err := s.LinkToken(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, token, 43)
	require.Equal(t, BalanceQueryTokenHash(token), repo.hash)
	require.NotEqual(t, token, repo.encrypted)
	for _, invalid := range []string{"", "1", strings.Repeat("A", 44), strings.Repeat("!", 43)} {
		_, err := s.ResolvePublic(context.Background(), invalid)
		require.ErrorIs(t, err, ErrBalanceQueryUnavailable)
	}
	require.Zero(t, repo.resolves)
	user, err := s.ResolvePublic(context.Background(), token)
	require.NoError(t, err)
	data, err := s.Query(context.Background(), *user, 1, 20)
	require.NoError(t, err)
	require.Equal(t, "临时授信50", data.Records[0].Note)
	require.Equal(t, "管理员充值", data.Records[1].Note)
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	for _, secret := range []string{token, "email", "api_key", "内部", "redeem_code", "credit_generation"} {
		require.NotContains(t, string(raw), secret)
	}
	require.Zero(t, repo.saves)
	require.Zero(t, repo.grants)
}

type creditCacheStub struct {
	err   error
	calls int
}

func (c *creditCacheStub) InvalidateUserBalance(context.Context, int64) error {
	c.calls++
	return c.err
}

func TestTemporaryCreditCacheRetryDoesNotGrantAgain(t *testing.T) {
	repo := &customizationTestRepo{}
	cache := &creditCacheStub{err: errors.New("Redis 不可用")}
	w := NewTemporaryCreditWorker(repo, nil)
	w.balanceCache = cache
	t.Cleanup(w.Stop)
	w.retryInvalidations(context.Background())
	require.Zero(t, repo.complete)
	cache.err = nil
	w.retryInvalidations(context.Background())
	require.Equal(t, 1, repo.complete)
	require.Equal(t, 2, cache.calls)
	require.Zero(t, repo.grants)
}

func TestTemporaryCreditNote(t *testing.T) {
	require.Equal(t, "临时授信1000", TemporaryCreditNote(1000))
	require.Equal(t, "临时授信0.12345678", TemporaryCreditNote(0.12345678))
}
