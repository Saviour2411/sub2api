package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const TemporaryCreditType = "temporary_credit"
const userCustomizationPaymentKey = "user_customization_payment_methods"

var (
	ErrUserCustomizationInvalid  = infraerrors.BadRequest("USER_CUSTOMIZATION_INVALID", "用户定制配置无效")
	ErrUserCustomizationConflict = infraerrors.Conflict("USER_CUSTOMIZATION_CONFLICT", "配置已变化或本轮授信尚未使用，请刷新后重试")
	ErrBalanceQueryUnavailable   = infraerrors.NotFound("BALANCE_QUERY_UNAVAILABLE", "查询链接不可用")
)

type UserPaymentMethod struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UserCustomizationInput struct {
	AutoCreditEnabled bool    `json:"auto_credit_enabled"`
	CreditThreshold   float64 `json:"credit_threshold"`
	CreditAmount      float64 `json:"credit_amount"`
}

type UserCustomization struct {
	UserID      int64   `json:"user_id"`
	Username    string  `json:"username"`
	Email       string  `json:"email"`
	Balance     float64 `json:"balance"`
	Status      string  `json:"status"`
	HasLink     bool    `json:"has_link"`
	LinkEnabled bool    `json:"link_enabled"`
	LinkVersion int64   `json:"link_version"`
	UserCustomizationInput
	CreditGeneration int64      `json:"credit_generation"`
	CreditUsedAt     *time.Time `json:"credit_used_at"`
}

type PublicBalanceUser struct {
	ID       int64   `json:"id"`
	Username string  `json:"username"`
	Balance  float64 `json:"balance"`
}

type PublicRechargeRecord struct {
	CreatedAt time.Time `json:"created_at"`
	Amount    float64   `json:"amount"`
	Type      string    `json:"type"`
	Note      string    `json:"note"`
}

type PublicBalanceQuery struct {
	User           PublicBalanceUser      `json:"user"`
	Records        []PublicRechargeRecord `json:"records"`
	PaymentMethods []UserPaymentMethod    `json:"payment_methods"`
	Total          int64                  `json:"total"`
	Page           int                    `json:"page"`
	PageSize       int                    `json:"page_size"`
}

type CreditCacheInvalidation struct {
	ID     int64
	UserID int64
}

type UserCustomizationRepository interface {
	List(context.Context, string, int, int) ([]UserCustomization, int64, error)
	Get(context.Context, int64) (*UserCustomization, error)
	Save(context.Context, int64, UserCustomizationInput) error
	RotateLink(context.Context, int64, int64, string, string) error
	SetLinkEnabled(context.Context, int64, int64, bool) error
	GetLinkSecret(context.Context, int64) (string, error)
	ResolvePublic(context.Context, string) (*PublicBalanceUser, error)
	PublicHistory(context.Context, int64, int, int) ([]PublicRechargeRecord, int64, error)
	RestoreCredit(context.Context, int64, int64) error
	CreditCandidates(context.Context, int64, int) ([]int64, error)
	TryGrantCredit(context.Context, int64) (bool, error)
	PendingCreditInvalidations(context.Context, int64, int) ([]CreditCacheInvalidation, error)
	CompleteCreditInvalidation(context.Context, int64) error
}

type creditBalanceCache interface {
	InvalidateUserBalance(context.Context, int64) error
}

type UserCustomizationService struct {
	repo      UserCustomizationRepository
	settings  SettingRepository
	encryptor SecretEncryptor
}

func NewUserCustomizationService(repo UserCustomizationRepository, settings SettingRepository, encryptor SecretEncryptor) *UserCustomizationService {
	return &UserCustomizationService{repo: repo, settings: settings, encryptor: encryptor}
}

func (s *UserCustomizationService) List(ctx context.Context, search string, page, size int) ([]UserCustomization, int64, error) {
	return s.repo.List(ctx, strings.TrimSpace(search), page, size)
}

func (s *UserCustomizationService) Get(ctx context.Context, id int64) (*UserCustomization, error) {
	return s.repo.Get(ctx, id)
}

func (s *UserCustomizationService) Save(ctx context.Context, id int64, in UserCustomizationInput) (*UserCustomization, error) {
	if !validCustomizationMoney(in.CreditThreshold) || !validCustomizationMoney(in.CreditAmount) || (in.AutoCreditEnabled && in.CreditAmount <= 0) {
		return nil, ErrUserCustomizationInvalid
	}
	if err := s.repo.Save(ctx, id, in); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func validCustomizationMoney(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < 1e12 && (v == 0 || v >= 0.00000001)
}

func (s *UserCustomizationService) RotateLink(ctx context.Context, id, version int64) (*UserCustomization, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	encrypted, err := s.encryptor.Encrypt(token)
	if err != nil {
		return nil, fmt.Errorf("加密查询链接失败: %w", err)
	}
	if err := s.repo.RotateLink(ctx, id, version, BalanceQueryTokenHash(token), encrypted); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *UserCustomizationService) SetLinkEnabled(ctx context.Context, id, version int64, enabled bool) (*UserCustomization, error) {
	if err := s.repo.SetLinkEnabled(ctx, id, version, enabled); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *UserCustomizationService) LinkToken(ctx context.Context, id int64) (string, error) {
	encrypted, err := s.repo.GetLinkSecret(ctx, id)
	if err != nil {
		return "", err
	}
	return s.encryptor.Decrypt(encrypted)
}

func BalanceQueryTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *UserCustomizationService) ResolvePublic(ctx context.Context, token string) (*PublicBalanceUser, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return nil, ErrBalanceQueryUnavailable
	}
	return s.repo.ResolvePublic(ctx, BalanceQueryTokenHash(token))
}

func (s *UserCustomizationService) Query(ctx context.Context, user PublicBalanceUser, page, size int) (*PublicBalanceQuery, error) {
	records, total, err := s.repo.PublicHistory(ctx, user.ID, page, size)
	if err != nil {
		return nil, err
	}
	for i := range records {
		switch records[i].Type {
		case TemporaryCreditType:
			records[i].Note = TemporaryCreditNote(records[i].Amount)
		case "admin_balance":
			records[i].Note = "管理员充值"
		default:
			records[i].Note = "余额充值"
		}
	}
	methods, err := s.PaymentMethods(ctx)
	if err != nil {
		return nil, err
	}
	return &PublicBalanceQuery{User: user, Records: records, Total: total, PaymentMethods: methods, Page: page, PageSize: size}, nil
}

func TemporaryCreditNote(amount float64) string {
	return "临时授信" + strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.8f", amount), "0"), ".")
}

func (s *UserCustomizationService) PaymentMethods(ctx context.Context) ([]UserPaymentMethod, error) {
	values, err := s.settings.GetMultiple(ctx, []string{userCustomizationPaymentKey})
	if err != nil {
		return nil, err
	}
	methods := []UserPaymentMethod{}
	if raw := values[userCustomizationPaymentKey]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &methods); err != nil {
			return nil, fmt.Errorf("读取充值方式失败: %w", err)
		}
	}
	if methods == nil {
		methods = []UserPaymentMethod{}
	}
	return methods, nil
}

func (s *UserCustomizationService) SavePaymentMethods(ctx context.Context, methods []UserPaymentMethod) ([]UserPaymentMethod, error) {
	if len(methods) > 20 {
		return nil, ErrUserCustomizationInvalid
	}
	for i := range methods {
		methods[i].Key, methods[i].Value = strings.TrimSpace(methods[i].Key), strings.TrimSpace(methods[i].Value)
		if methods[i].Key == "" || methods[i].Value == "" || utf8.RuneCountInString(methods[i].Key) > 50 || utf8.RuneCountInString(methods[i].Value) > 500 {
			return nil, ErrUserCustomizationInvalid
		}
	}
	if methods == nil {
		methods = []UserPaymentMethod{}
	}
	raw, err := json.Marshal(methods)
	if err != nil {
		return nil, err
	}
	if err := s.settings.Set(ctx, userCustomizationPaymentKey, string(raw)); err != nil {
		return nil, err
	}
	return methods, nil
}

func (s *UserCustomizationService) RestoreCredit(ctx context.Context, id, generation int64) (*UserCustomization, error) {
	if err := s.repo.RestoreCredit(ctx, id, generation); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}
