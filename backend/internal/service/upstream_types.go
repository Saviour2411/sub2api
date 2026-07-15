package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UpstreamPlatformSub2API = "sub2api"
	UpstreamPlatformNewAPI  = "newapi"

	UpstreamAuthPassword = "password"
	UpstreamAuthToken    = "token"

	UpstreamStatusPending = "pending"
	UpstreamStatusSyncing = "syncing"
	UpstreamStatusHealthy = "healthy"
	UpstreamStatusError   = "error"

	// UpstreamCostBasisActual 表示 cost_usd 来自上游实际扣费，而非标准成本。
	UpstreamCostBasisActual = 2
)

var (
	ErrUpstreamNotFound = infraerrors.NotFound(
		"UPSTREAM_NOT_FOUND", "上游站点不存在",
	)
	ErrUpstreamInvalidInput = infraerrors.BadRequest(
		"UPSTREAM_INVALID_INPUT", "上游站点参数无效",
	)
	ErrUpstreamConnectionFailed = infraerrors.BadRequest(
		"UPSTREAM_CONNECTION_FAILED", "无法连接或认证上游站点",
	)
	ErrUpstreamTurnstileRequired = infraerrors.BadRequest(
		"UPSTREAM_TURNSTILE_REQUIRED", "目标站点开启了 Cloudflare Turnstile，请使用访问令牌认证",
	).WithMetadata(map[string]string{"recommended_auth_mode": UpstreamAuthToken})
	ErrUpstreamCredentialDecrypt = infraerrors.InternalServer(
		"UPSTREAM_CREDENTIAL_DECRYPT_FAILED", "上游凭证解密失败，请重新编辑站点凭证",
	)
	ErrUpstreamGroupNotFound = infraerrors.NotFound(
		"UPSTREAM_GROUP_NOT_FOUND", "上游分组不存在",
	)
	ErrUpstreamGroupUnavailable = infraerrors.BadRequest(
		"UPSTREAM_GROUP_UNAVAILABLE", "暂不可用的上游分组不能添加展示",
	)
)

// UpstreamCredential 是 AES-GCM 加密信封中的明文结构，仅在服务内部流转。
type UpstreamCredential struct {
	Password     string `json:"password,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Cookie       string `json:"cookie,omitempty"`
}

// UpstreamSite 是独立上游管理领域的站点模型。
type UpstreamSite struct {
	ID                    int64
	SortOrder             int
	Name                  string
	BaseURL               string
	Platform              string
	AuthMode              string
	Account               string
	CredentialEncrypted   string
	Enabled               bool
	Status                string
	ErrorMessage          *string
	BalanceUSD            *float64
	TodayTokens           int64
	TodayCostUSD          float64
	TotalTokens           int64
	TotalCostUSD          float64
	TrackingStartedAt     time.Time
	LastSyncedAt          *time.Time
	NextSyncAt            *time.Time
	CreatedBy             int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CredentialDecryptFail bool
	DisplayedGroupCount   int
}

// UpstreamSiteView 是管理 API 的脱敏响应。
type UpstreamSiteView struct {
	ID                  int64      `json:"id"`
	SortOrder           int        `json:"sort_order"`
	Name                string     `json:"name"`
	BaseURL             string     `json:"base_url"`
	Platform            string     `json:"platform"`
	AuthMode            string     `json:"auth_mode"`
	Account             string     `json:"account"`
	Enabled             bool       `json:"enabled"`
	Status              string     `json:"status"`
	ErrorMessage        *string    `json:"error_message"`
	BalanceUSD          *float64   `json:"balance_usd"`
	TodayTokens         int64      `json:"today_tokens"`
	TodayCostUSD        float64    `json:"today_cost_usd"`
	TotalTokens         int64      `json:"total_tokens"`
	TotalCostUSD        float64    `json:"total_cost_usd"`
	TrackingStartedAt   time.Time  `json:"tracking_started_at"`
	LastSyncedAt        *time.Time `json:"last_synced_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	HasPassword         bool       `json:"has_password"`
	HasToken            bool       `json:"has_token"`
	DisplayedGroupCount int        `json:"displayed_group_count"`
}

type UpstreamListParams struct {
	Page          int
	PageSize      int
	Search        string
	Platform      string
	Enabled       *bool
	GroupPlatform string
	SortBy        string
	SortOrder     string
}

type UpstreamSortOrderUpdate struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}

type UpstreamProbeInput struct {
	BaseURL  string `json:"base_url"`
	Platform string `json:"platform"`
}

type UpstreamCapabilities struct {
	BaseURL              string `json:"base_url"`
	Platform             string `json:"platform"`
	TurnstileEnabled     bool   `json:"turnstile_enabled"`
	TokenAuthRecommended bool   `json:"token_auth_recommended"`
}

type UpstreamCreateInput struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	Platform     string `json:"platform"`
	AuthMode     string `json:"auth_mode"`
	Account      string `json:"account"`
	Password     string `json:"password"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Enabled      bool   `json:"enabled"`
	CreatedBy    int64  `json:"-"`
}

// UpstreamUpdateInput 使用指针区分未提交字段；凭证空字符串表示保留旧值。
type UpstreamUpdateInput struct {
	Name         *string `json:"name"`
	BaseURL      *string `json:"base_url"`
	Platform     *string `json:"platform"`
	AuthMode     *string `json:"auth_mode"`
	Account      *string `json:"account"`
	Password     *string `json:"password"`
	AccessToken  *string `json:"access_token"`
	RefreshToken *string `json:"refresh_token"`
	Enabled      *bool   `json:"enabled"`
}

type UpstreamGroup struct {
	ID           int64     `json:"id"`
	SiteID       int64     `json:"site_id"`
	RemoteID     string    `json:"remote_id"`
	Name         string    `json:"name"`
	Platform     string    `json:"platform"`
	Description  string    `json:"description"`
	Multiplier   *float64  `json:"multiplier"`
	TodayTokens  int64     `json:"today_tokens"`
	TodayCostUSD float64   `json:"today_cost_usd"`
	Displayed    bool      `json:"displayed"`
	Available    bool      `json:"available"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

type UpstreamGroupDisplayInput struct {
	RemoteID  string `json:"remote_id"`
	Displayed *bool  `json:"displayed"`
}

type UpstreamGroupDisplayResult struct {
	Group               UpstreamGroup `json:"group"`
	DisplayedGroupCount int           `json:"displayed_group_count"`
}

type UpstreamDailyStat struct {
	ID               int64     `json:"id"`
	SiteID           int64     `json:"site_id"`
	Date             time.Time `json:"date"`
	BalanceUSD       *float64  `json:"balance_usd"`
	Tokens           int64     `json:"tokens"`
	CostUSD          float64   `json:"cost_usd"`
	CostBasisVersion int       `json:"cost_basis_version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type UpstreamSyncRequest struct {
	Site       *UpstreamSite
	Credential UpstreamCredential
	Dates      []time.Time
	Location   *time.Location
}

type UpstreamGroupSnapshot struct {
	RemoteID     string
	Name         string
	Platform     string
	Description  string
	Multiplier   *float64
	TodayTokens  int64
	TodayCostUSD float64
}

type UpstreamDailySnapshot struct {
	Date       time.Time
	BalanceUSD *float64
	Tokens     int64
	CostUSD    float64
}

type UpstreamGroupMultiplierPoint struct {
	RecordedAt time.Time `json:"recorded_at"`
	Multiplier *float64  `json:"multiplier"`
}

type UpstreamGroupMultiplierHistory struct {
	RemoteID          string                         `json:"remote_id"`
	Name              string                         `json:"name"`
	Platform          string                         `json:"platform"`
	Description       string                         `json:"description"`
	CurrentMultiplier *float64                       `json:"current_multiplier"`
	Points            []UpstreamGroupMultiplierPoint `json:"points"`
}

// UpstreamSyncResult 必须在适配器完成全部分页后一次性返回。
type UpstreamSyncResult struct {
	BalanceUSD *float64
	Groups     []UpstreamGroupSnapshot
	Daily      []UpstreamDailySnapshot
	Credential *UpstreamCredential
}

type UpstreamProvider interface {
	Platform() string
	Validate(ctx context.Context, site *UpstreamSite, credential UpstreamCredential) (*UpstreamCredential, error)
	Sync(ctx context.Context, req UpstreamSyncRequest) (*UpstreamSyncResult, error)
}

type UpstreamRepository interface {
	Create(ctx context.Context, site *UpstreamSite) error
	GetByID(ctx context.Context, id int64) (*UpstreamSite, error)
	Update(ctx context.Context, site *UpstreamSite) error
	ListAll(ctx context.Context, params UpstreamListParams) ([]*UpstreamSite, error)
	SetEnabled(ctx context.Context, id int64, enabled bool, nextSyncAt *time.Time) error
	UpdateSortOrder(ctx context.Context, updates []UpstreamSortOrderUpdate) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params UpstreamListParams) ([]*UpstreamSite, int64, error)
	ListIDs(ctx context.Context, enabledOnly bool) ([]int64, error)
	ListDue(ctx context.Context, now time.Time, limit int) ([]int64, error)
	MarkPending(ctx context.Context, id int64, nextSyncAt *time.Time) error
	MarkSyncing(ctx context.Context, id int64) error
	MarkSyncFailed(ctx context.Context, id int64, message string, nextSyncAt *time.Time) error
	UpdateCredential(ctx context.Context, id int64, encryptedCredential string) error
	MissingDates(ctx context.Context, id int64, from, through time.Time, loc *time.Location) ([]time.Time, error)
	CommitSync(ctx context.Context, id int64, result *UpstreamSyncResult, encryptedCredential string, syncedAt time.Time, nextSyncAt *time.Time) error
	ListGroups(ctx context.Context, siteID int64) ([]UpstreamGroup, error)
	SetGroupDisplayed(ctx context.Context, siteID int64, remoteID string, displayed bool) (*UpstreamGroupDisplayResult, error)
	ListHistory(ctx context.Context, siteID int64, from, through time.Time) ([]UpstreamDailyStat, error)
	ListMultiplierHistory(ctx context.Context, siteID int64, from, through time.Time) ([]UpstreamGroupMultiplierHistory, error)
}

type UpstreamSyncScheduler interface {
	Enqueue(id int64)
}
