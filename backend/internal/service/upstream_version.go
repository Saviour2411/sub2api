package service

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/singleflight"
)

const (
	upstreamVersionTimeout = 8 * time.Second
	upstreamVersionTTL     = 30 * time.Minute
	upstreamVersionBackoff = time.Minute
)

// latestReleaseReader 限定查询能力，版本提示不获得下载或修改程序的能力。
type latestReleaseReader interface {
	FetchLatestRelease(context.Context, string) (*GitHubRelease, error)
}

type UpstreamVersionInfo struct {
	LatestVersion string     `json:"latest_version"`
	ReleaseURL    string     `json:"release_url"`
	HasUpdate     *bool      `json:"has_update"`
	CheckedAt     *time.Time `json:"checked_at"`
	Status        string     `json:"status"`
	Stale         bool       `json:"stale"`
}

type UpstreamVersionService struct {
	client   latestReleaseReader
	baseline string
	now      func() time.Time
	timeout  time.Duration
	flight   singleflight.Group
	mu       sync.Mutex
	latest   string
	tag      string
	checked  time.Time
	next     time.Time
	failed   bool
}

func NewUpstreamVersionService(info buildmeta.UpstreamSync, client latestReleaseReader) *UpstreamVersionService {
	baseline := ""
	if info.Repository == buildmeta.UpstreamRepository && semver.IsValid("v"+info.Version) {
		baseline = "v" + info.Version
	}
	return &UpstreamVersionService{client: client, baseline: baseline, now: time.Now, timeout: upstreamVersionTimeout}
}

func (s *UpstreamVersionService) snapshotLocked() UpstreamVersionInfo {
	result := UpstreamVersionInfo{Status: "unavailable"}
	if s.latest == "" {
		return result
	}
	checked := s.checked
	result.LatestVersion = s.latest
	result.ReleaseURL = "https://github.com/" + buildmeta.UpstreamRepository + "/releases/tag/" + url.PathEscape(s.tag)
	result.CheckedAt = &checked
	result.Stale = s.failed || !s.now().Before(s.checked.Add(upstreamVersionTTL))
	if !result.Stale {
		result.Status = "ok"
	}
	if s.baseline != "" {
		newer := semver.Compare("v"+s.latest, s.baseline) > 0
		result.HasUpdate = &newer
	}
	return result
}

func (s *UpstreamVersionService) GetLatest(ctx context.Context) UpstreamVersionInfo {
	s.mu.Lock()
	if s.now().Before(s.next) {
		result := s.snapshotLocked()
		s.mu.Unlock()
		return result
	}
	s.mu.Unlock()
	result := s.flight.DoChan("latest", func() (any, error) {
		s.mu.Lock()
		if s.now().Before(s.next) {
			cached := s.snapshotLocked()
			s.mu.Unlock()
			return cached, nil
		}
		s.mu.Unlock()

		// 浏览器取消不能中断其他管理员共享的查询；独立超时限制后台任务寿命。
		queryCtx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		var release *GitHubRelease
		if s.client != nil {
			fetched, err := s.client.FetchLatestRelease(queryCtx, buildmeta.UpstreamRepository)
			if err == nil && queryCtx.Err() == nil {
				release = fetched
			}
		}
		valid := release != nil && !release.Draft && !release.Prerelease
		version := ""
		if valid {
			version = strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
			valid = semver.IsValid("v"+version) && semver.Prerelease("v"+version) == ""
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.failed = !valid
		s.next = s.now().Add(upstreamVersionBackoff)
		if valid {
			s.latest, s.tag, s.checked = version, strings.TrimSpace(release.TagName), s.now()
			s.next = s.checked.Add(upstreamVersionTTL)
		}
		return s.snapshotLocked(), nil
	})
	select {
	case value := <-result:
		if info, ok := value.Val.(UpstreamVersionInfo); ok {
			return info
		}
		return UpstreamVersionInfo{Status: "unavailable"}
	case <-ctx.Done():
		s.mu.Lock()
		defer s.mu.Unlock()
		cached := s.snapshotLocked()
		cached.Status = "unavailable"
		cached.Stale = cached.LatestVersion != ""
		return cached
	}
}
