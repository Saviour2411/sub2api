package service

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const SettingKeyModelQualityEnabled = "model_quality_enabled"

var (
	ErrModelQualityInvalid  = infraerrors.BadRequest("MODEL_QUALITY_INVALID", "降智检测配置无效")
	ErrModelQualityConflict = infraerrors.Conflict("MODEL_QUALITY_CONFLICT", "配置已更新或分组正在检测，请刷新后重试")
	ErrModelQualityDisabled = infraerrors.NotFound("MODEL_QUALITY_DISABLED", "降智检测未启用")
	ErrModelQualityExpired  = infraerrors.NotFound("MODEL_QUALITY_EXPIRED", "检测记录不存在、已过期或分组已移除")
)

type ModelQualityGroupConfig struct {
	GroupID         int64    `json:"group_id"`
	KeyID           *int64   `json:"key_id"`
	OwnerID         int64    `json:"owner_id,omitempty"`
	IntervalMinutes int      `json:"interval_minutes"`
	Methods         []string `json:"methods"`
	Models          []string `json:"models"`
	ReasoningEffort string   `json:"reasoning_effort"`
}

type ModelQualityConfig struct {
	Enabled     bool                      `json:"enabled"`
	Version     int                       `json:"version"`
	Groups      []ModelQualityGroupConfig `json:"groups"`
	Endpoint    string                    `json:"endpoint,omitempty"`
	PauseReason string                    `json:"pause_reason,omitempty"`
}

type ModelQualityGroupState struct {
	GroupID       int64      `json:"group_id"`
	Name          string     `json:"name"`
	Models        []string   `json:"models"`
	Methods       []string   `json:"methods"`
	Status        string     `json:"status"`
	PauseReason   string     `json:"pause_reason,omitempty"`
	Running       bool       `json:"running"`
	LastStartedAt *time.Time `json:"last_started_at"`
	NextRunAt     *time.Time `json:"next_run_at"`
}

type ModelQualityTest struct {
	Benchmark       string     `json:"benchmark"`
	Status          string     `json:"status"`
	Verdict         string     `json:"verdict"`
	Reason          string     `json:"reason"`
	Answer          string     `json:"answer,omitempty"`
	HTML            string     `json:"html,omitempty"`
	HasHTML         bool       `json:"has_html"`
	DurationMS      int64      `json:"duration_ms"`
	InputTokens     *int64     `json:"input_tokens,omitempty"`
	OutputTokens    *int64     `json:"output_tokens,omitempty"`
	ReasoningTokens *int64     `json:"reasoning_tokens,omitempty"`
	FinishedAt      *time.Time `json:"finished_at"`
}

type ModelQualityRun struct {
	ID              int64              `json:"id"`
	GroupID         int64              `json:"group_id"`
	GroupName       string             `json:"group_name"`
	Model           string             `json:"model"`
	ReasoningEffort string             `json:"reasoning_effort"`
	Outcome         string             `json:"outcome"`
	StartedAt       time.Time          `json:"started_at"`
	FinishedAt      *time.Time         `json:"finished_at"`
	ExpiresAt       time.Time          `json:"expires_at"`
	Tests           []ModelQualityTest `json:"tests"`
}

// 内部任务与公开结果分开，凭据引用及第三方 ID 不进入用户接口。
type ModelQualityTask struct {
	ModelQualityTest
	ID, RunID                                 int64
	Group                                     ModelQualityGroupConfig
	Model, Endpoint, RemoteID, IdempotencyKey string
	StartedAt                                 time.Time
	NextPollAt                                time.Time
	SubmittedAt, DeadlineAt                   *time.Time
	Attempts                                  int
}

type ModelQualityFilter struct {
	GroupID                int64
	Model, Method, Outcome string
	Page                   int
}

type ModelQualityRepository interface {
	Config(context.Context) (*ModelQualityConfig, error)
	SaveConfig(context.Context, *ModelQualityConfig) error
	States(context.Context) (map[int64]ModelQualityGroupState, error)
	StartRound(context.Context, *ModelQualityConfig, ModelQualityGroupConfig, string, string, bool) error
	Tasks(context.Context) ([]ModelQualityTask, error)
	PrepareTask(context.Context, *ModelQualityTask, int) (bool, error)
	SaveTask(context.Context, *ModelQualityTask) error
	FinishRounds(context.Context) error
	Cleanup(context.Context) error
	Runs(context.Context, ModelQualityFilter) ([]ModelQualityRun, int64, error)
	Run(context.Context, int64) (*ModelQualityRun, error)
}

type ModelQualityService struct {
	repo        ModelQualityRepository
	settings    *SettingService
	keys        APIKeyRepository
	groups      GroupRepository
	dnsMu       sync.Mutex
	dnsEndpoint string
	dnsChecked  time.Time
	dnsReason   string
	lookup      func(context.Context, string) ([]net.IPAddr, error)
}

func NewModelQualityService(repo ModelQualityRepository, settings *SettingService, keys APIKeyRepository, groups GroupRepository) *ModelQualityService {
	return &ModelQualityService{repo: repo, settings: settings, keys: keys, groups: groups, lookup: net.DefaultResolver.LookupIPAddr}
}

func NormalizeModelQualityEndpoint(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("API 端点地址必须是公网 HTTPS 基础地址，不能含凭据、查询参数或片段")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if ip := net.ParseIP(host); ip != nil {
		if !modelQualityPublicIP(ip) {
			return "", fmt.Errorf("API 端点地址必须使用公网 IP")
		}
	} else if host == "localhost" || !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".localhost") {
		return "", fmt.Errorf("API 端点地址不能使用本机或内网域名")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("API 端点地址端口无效")
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	for strings.HasSuffix(u.Path, "/v1/v1") {
		u.Path = strings.TrimSuffix(u.Path, "/v1")
	}
	if !strings.HasSuffix(u.Path, "/v1") {
		u.Path += "/v1"
	}
	u.RawPath = ""
	return u.String(), nil
}

func modelQualityPublicIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, cidr := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}

func (s *ModelQualityService) Config(ctx context.Context) (*ModelQualityConfig, error) {
	c, err := s.repo.Config(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := s.settings.settingRepo.GetValue(ctx, SettingKeyAPIBaseURL)
	if err != nil {
		c.PauseReason = "未配置 API 端点地址"
		return c, nil
	}
	c.Endpoint, err = NormalizeModelQualityEndpoint(raw)
	if err != nil {
		c.Endpoint = raw
		c.PauseReason = err.Error()
	} else {
		c.PauseReason = s.endpointDNSReason(ctx, c.Endpoint)
	}
	return c, nil
}

func (s *ModelQualityService) endpointDNSReason(ctx context.Context, endpoint string) string {
	s.dnsMu.Lock()
	defer s.dnsMu.Unlock()
	if endpoint == s.dnsEndpoint && time.Since(s.dnsChecked) < time.Minute {
		return s.dnsReason
	}
	lookup := s.lookup
	if lookup == nil {
		lookup = net.DefaultResolver.LookupIPAddr
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	u, _ := url.Parse(endpoint)
	ips, err := lookup(ctx, u.Hostname())
	reason := ""
	if err != nil || len(ips) == 0 {
		reason = "API 端点地址暂时无法解析，检测已暂停"
	}
	for _, ip := range ips {
		if !modelQualityPublicIP(ip.IP) {
			reason = "API 端点地址解析到非公网地址，检测已暂停"
		}
	}
	s.dnsEndpoint = endpoint
	s.dnsChecked = time.Now()
	s.dnsReason = reason
	return reason
}

func validateModelQualityConfig(c *ModelQualityConfig) error {
	if len(c.Groups) > 50 {
		return ErrModelQualityInvalid
	}
	seen := map[int64]bool{}
	for i := range c.Groups {
		g := &c.Groups[i]
		if g.GroupID <= 0 || seen[g.GroupID] {
			return ErrModelQualityInvalid
		}
		seen[g.GroupID] = true
		if g.IntervalMinutes == 0 {
			g.IntervalMinutes = 60
		}
		if g.IntervalMinutes < 5 || g.IntervalMinutes > 1440 {
			return ErrModelQualityInvalid
		}
		if g.ReasoningEffort == "" {
			g.ReasoningEffort = "low"
		}
		if !slices.Contains([]string{"low", "medium", "high"}, g.ReasoningEffort) {
			return ErrModelQualityInvalid
		}
		if g.Methods == nil {
			g.Methods = []string{"candy", "pelican"}
		}
		g.Methods = uniqueModelQualityStrings(g.Methods)
		if len(g.Methods) == 0 || len(g.Methods) > 2 {
			return ErrModelQualityInvalid
		}
		for _, m := range g.Methods {
			if m != "candy" && m != "pelican" {
				return ErrModelQualityInvalid
			}
		}
		g.Models = uniqueModelQualityStrings(g.Models)
		if len(g.Models) == 0 || len(g.Models) > 20 {
			return ErrModelQualityInvalid
		}
		for _, m := range g.Models {
			if len(m) > 200 || strings.ContainsAny(m, "\r\n\x00") {
				return ErrModelQualityInvalid
			}
		}
	}
	if c.Groups == nil {
		c.Groups = []ModelQualityGroupConfig{}
	}
	return nil
}

func uniqueModelQualityStrings(values []string) []string {
	out := []string{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

func (s *ModelQualityService) validKey(ctx context.Context, g ModelQualityGroupConfig) (*APIKey, error) {
	if g.KeyID == nil {
		return nil, fmt.Errorf("待绑定密钥")
	}
	k, err := s.keys.GetByID(ctx, *g.KeyID)
	if err != nil || k == nil || k.UserID != g.OwnerID || k.GroupID == nil || *k.GroupID != g.GroupID || !k.IsActive() || k.IsExpired() || k.IsQuotaExhausted() || k.Purpose == APIKeyPurposeInfiniteCanvas {
		return nil, fmt.Errorf("绑定密钥已失效、额度不足或分组已变更")
	}
	if k.User == nil || k.User.Role != RoleAdmin || k.User.Status != StatusActive || k.User.DeletedAt != nil {
		return nil, fmt.Errorf("密钥所属管理员已失效")
	}
	return k, nil
}

func (s *ModelQualityService) SaveConfig(ctx context.Context, adminID int64, c *ModelQualityConfig) error {
	if err := validateModelQualityConfig(c); err != nil {
		return err
	}
	var saved *ModelQualityConfig
	if !c.Enabled {
		var err error
		saved, err = s.repo.Config(ctx)
		if err != nil {
			return err
		}
		if saved.Version != c.Version {
			return ErrModelQualityConflict
		}
	}
	for i := range c.Groups {
		g := &c.Groups[i]
		if saved != nil {
			// 关闭已有配置不受密钥过期或分组停用影响；修改绑定仍须校验。
			unchanged := false
			for _, previous := range saved.Groups {
				if sameModelQualityGroup(*g, previous) {
					g.OwnerID = previous.OwnerID
					unchanged = true
					break
				}
			}
			if unchanged {
				continue
			}
		}
		group, err := s.groups.GetByID(ctx, g.GroupID)
		if err != nil || group.Platform != PlatformOpenAI || group.Status != StatusActive {
			return ErrModelQualityInvalid
		}
		g.OwnerID = adminID
		if g.KeyID != nil {
			if _, err = s.validKey(ctx, *g); err != nil {
				return infraerrors.BadRequest("MODEL_QUALITY_KEY_INVALID", err.Error())
			}
		}
	}
	if err := s.repo.SaveConfig(ctx, c); err != nil {
		return err
	}
	s.settings.notifyCustomFeatureSettingsUpdated()
	return nil
}

func sameModelQualityGroup(a, b ModelQualityGroupConfig) bool {
	sameKey := a.KeyID == nil && b.KeyID == nil || a.KeyID != nil && b.KeyID != nil && *a.KeyID == *b.KeyID
	return a.GroupID == b.GroupID && sameKey && a.IntervalMinutes == b.IntervalMinutes && a.ReasoningEffort == b.ReasoningEffort && slices.Equal(a.Models, b.Models) && slices.Equal(a.Methods, b.Methods)
}

type ModelQualityKeyOption struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	MaskedKey string `json:"masked_key"`
}

func (s *ModelQualityService) KeyOptions(ctx context.Context, adminID, groupID int64) ([]ModelQualityKeyOption, error) {
	group, err := s.groups.GetByID(ctx, groupID)
	if err != nil || group.Platform != PlatformOpenAI {
		return nil, ErrModelQualityInvalid
	}
	lister, ok := s.keys.(apiKeyAllByUserIDLister)
	if !ok {
		return nil, fmt.Errorf("密钥列表暂不可用")
	}
	keys, err := lister.ListAllByUserID(ctx, adminID, APIKeyListFilters{GroupID: &groupID, Status: StatusActive})
	if err != nil {
		return nil, err
	}
	out := []ModelQualityKeyOption{}
	for _, k := range keys {
		if k.UserID != adminID || k.GroupID == nil || *k.GroupID != groupID || !k.IsActive() || k.IsExpired() || k.IsQuotaExhausted() || k.Purpose == APIKeyPurposeInfiniteCanvas {
			continue
		}
		masked := "••••"
		if len(k.Key) > 8 {
			masked = k.Key[:3] + "••••" + k.Key[len(k.Key)-4:]
		}
		out = append(out, ModelQualityKeyOption{k.ID, k.Name, masked})
	}
	return out, nil
}

func (s *ModelQualityService) readiness(ctx context.Context, c *ModelQualityConfig, g ModelQualityGroupConfig) (string, string) {
	group, err := s.groups.GetByID(ctx, g.GroupID)
	if err != nil || group.Platform != PlatformOpenAI || group.Status != StatusActive {
		return "分组已停用", "分组已停用或类型已变更"
	}
	if g.KeyID == nil {
		return group.Name, "待绑定密钥"
	}
	if c.PauseReason != "" {
		return group.Name, c.PauseReason
	}
	if _, err := s.validKey(ctx, g); err != nil {
		return group.Name, err.Error()
	}
	return group.Name, ""
}

func (s *ModelQualityService) Groups(ctx context.Context) ([]ModelQualityGroupState, error) {
	c, err := s.Config(ctx)
	if err != nil {
		return nil, err
	}
	if !c.Enabled {
		return nil, ErrModelQualityDisabled
	}
	states, err := s.repo.States(ctx)
	if err != nil {
		return nil, err
	}
	out := []ModelQualityGroupState{}
	for _, g := range c.Groups {
		state := states[g.GroupID]
		state.GroupID = g.GroupID
		state.Models = g.Models
		state.Methods = g.Methods
		state.Name, state.PauseReason = s.readiness(ctx, c, g)
		if state.Status == "" {
			state.Status = "pending"
		}
		if state.PauseReason != "" {
			state.Status = "paused"
			state.NextRunAt = nil
		}
		if state.Running {
			state.Status = "running"
			state.NextRunAt = nil
		}
		out = append(out, state)
	}
	return out, nil
}

func (s *ModelQualityService) RunNow(ctx context.Context, groupID int64) error {
	c, err := s.Config(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return ErrModelQualityDisabled
	}
	for _, g := range c.Groups {
		if g.GroupID == groupID {
			name, reason := s.readiness(ctx, c, g)
			if reason != "" {
				return infraerrors.BadRequest("MODEL_QUALITY_PAUSED", reason)
			}
			return s.repo.StartRound(ctx, c, g, name, c.Endpoint, true)
		}
	}
	return ErrModelQualityInvalid
}

func (s *ModelQualityService) visible(ctx context.Context, groupID int64) error {
	c, err := s.repo.Config(ctx)
	if err != nil {
		return err
	}
	if !c.Enabled {
		return ErrModelQualityDisabled
	}
	for _, g := range c.Groups {
		if g.GroupID == groupID {
			return nil
		}
	}
	return ErrModelQualityExpired
}

func (s *ModelQualityService) Runs(ctx context.Context, f ModelQualityFilter) ([]ModelQualityRun, int64, error) {
	if err := s.visible(ctx, f.GroupID); err != nil {
		return nil, 0, err
	}
	if f.Page < 1 || f.Page > 10000 || (f.Method != "" && f.Method != "candy" && f.Method != "pelican") || (f.Outcome != "" && !slices.Contains([]string{"normal", "degraded", "failed", "running", "unknown"}, f.Outcome)) {
		return nil, 0, ErrModelQualityInvalid
	}
	return s.repo.Runs(ctx, f)
}

func (s *ModelQualityService) Run(ctx context.Context, id int64) (*ModelQualityRun, error) {
	r, err := s.repo.Run(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = s.visible(ctx, r.GroupID); err != nil {
		return nil, err
	}
	return r, nil
}
