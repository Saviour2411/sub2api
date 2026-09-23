//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"github.com/stretchr/testify/require"
)

type upstreamReleaseReaderFunc func(context.Context, string) (*GitHubRelease, error)

func (f upstreamReleaseReaderFunc) FetchLatestRelease(ctx context.Context, repo string) (*GitHubRelease, error) {
	return f(ctx, repo)
}

func upstreamTestBaseline() buildmeta.UpstreamSync {
	return buildmeta.UpstreamSync{Repository: buildmeta.UpstreamRepository, Version: "0.2.7"}
}

func TestUpstreamVersionStableComparison(t *testing.T) {
	for _, tc := range []struct {
		name    string
		release *GitHubRelease
		status  string
		newer   bool
	}{
		{"新版", &GitHubRelease{TagName: "v0.2.8"}, "ok", true},
		{"无前缀", &GitHubRelease{TagName: "0.2.8"}, "ok", true},
		{"数值比较", &GitHubRelease{TagName: "v0.2.10"}, "ok", true},
		{"同版", &GitHubRelease{TagName: "v0.2.7"}, "ok", false},
		{"旧版", &GitHubRelease{TagName: "v0.2.6"}, "ok", false},
		{"无效标签", &GitHubRelease{TagName: "latest"}, "unavailable", false},
		{"预发布标记", &GitHubRelease{TagName: "v0.3.0", Prerelease: true}, "unavailable", false},
		{"预发布标签", &GitHubRelease{TagName: "v0.3.0-rc.1"}, "unavailable", false},
		{"草稿", &GitHubRelease{TagName: "v0.3.0", Draft: true}, "unavailable", false},
		{"空结果", nil, "unavailable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var repository string
			s := NewUpstreamVersionService(upstreamTestBaseline(), upstreamReleaseReaderFunc(func(_ context.Context, repo string) (*GitHubRelease, error) {
				repository = repo
				return tc.release, nil
			}))
			result := s.GetLatest(context.Background())
			require.Equal(t, buildmeta.UpstreamRepository, repository)
			require.Equal(t, tc.status, result.Status)
			if tc.status == "ok" {
				require.NotNil(t, result.HasUpdate)
				require.Equal(t, tc.newer, *result.HasUpdate)
				require.Contains(t, result.ReleaseURL, "https://github.com/Wei-Shaw/sub2api/releases/tag/")
			} else {
				require.Nil(t, result.HasUpdate)
				require.Empty(t, result.ReleaseURL)
			}
		})
	}
}

func TestUpstreamVersionCacheBackoffAndStale(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	calls := 0
	fail := false
	s := NewUpstreamVersionService(upstreamTestBaseline(), upstreamReleaseReaderFunc(func(context.Context, string) (*GitHubRelease, error) {
		calls++
		if fail {
			return nil, errors.New("GitHub API returned 403")
		}
		return &GitHubRelease{TagName: "v0.2.8", HTMLURL: "https://untrusted.invalid/"}, nil
	}))
	s.now = func() time.Time { return now }
	first := s.GetLatest(context.Background())
	require.Equal(t, 1, calls)
	require.Equal(t, first, s.GetLatest(context.Background()))
	require.Equal(t, 1, calls)
	now = now.Add(upstreamVersionTTL)
	fail = true
	stale := s.GetLatest(context.Background())
	require.Equal(t, 2, calls)
	require.Equal(t, "unavailable", stale.Status)
	require.True(t, stale.Stale)
	require.True(t, *stale.HasUpdate)
	require.Equal(t, first.CheckedAt, stale.CheckedAt)
	require.Equal(t, first.ReleaseURL, stale.ReleaseURL)
	_ = s.GetLatest(context.Background())
	require.Equal(t, 2, calls)
	now = now.Add(upstreamVersionBackoff)
	fail = false
	recovered := s.GetLatest(context.Background())
	require.Equal(t, 3, calls)
	require.Equal(t, "ok", recovered.Status)
	require.False(t, recovered.Stale)
	require.Equal(t, now, *recovered.CheckedAt)
}

func TestUpstreamVersionUnknownBaselineAndUnavailable(t *testing.T) {
	s := NewUpstreamVersionService(buildmeta.UpstreamSync{}, upstreamReleaseReaderFunc(func(context.Context, string) (*GitHubRelease, error) {
		return &GitHubRelease{TagName: "v0.2.8"}, nil
	}))
	result := s.GetLatest(context.Background())
	require.Equal(t, "ok", result.Status)
	require.Nil(t, result.HasUpdate)
	missing := NewUpstreamVersionService(upstreamTestBaseline(), nil).GetLatest(context.Background())
	require.Equal(t, "unavailable", missing.Status)
	require.Nil(t, missing.HasUpdate)
	require.Nil(t, missing.CheckedAt)
}

func TestUpstreamVersionRateLimitWithoutHistory(t *testing.T) {
	for _, message := range []string{"GitHub API returned 403", "GitHub API returned 429"} {
		t.Run(message, func(t *testing.T) {
			calls := 0
			s := NewUpstreamVersionService(upstreamTestBaseline(), upstreamReleaseReaderFunc(func(context.Context, string) (*GitHubRelease, error) {
				calls++
				return nil, errors.New(message)
			}))
			first := s.GetLatest(context.Background())
			require.Equal(t, "unavailable", first.Status)
			require.Nil(t, first.HasUpdate)
			require.Empty(t, first.LatestVersion)
			require.Equal(t, first, s.GetLatest(context.Background()))
			require.Equal(t, 1, calls)
		})
	}
}

func TestUpstreamVersionTimeout(t *testing.T) {
	s := NewUpstreamVersionService(upstreamTestBaseline(), upstreamReleaseReaderFunc(func(ctx context.Context, _ string) (*GitHubRelease, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	require.Equal(t, 8*time.Second, s.timeout)
	s.timeout = 10 * time.Millisecond
	result := s.GetLatest(context.Background())
	require.Equal(t, "unavailable", result.Status)
	require.Nil(t, result.HasUpdate)
}

func TestUpstreamVersionConcurrentAndCallerCancellation(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	s := NewUpstreamVersionService(upstreamTestBaseline(), upstreamReleaseReaderFunc(func(ctx context.Context, _ string) (*GitHubRelease, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return &GitHubRelease{TagName: "v0.2.8"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan UpstreamVersionInfo, 1)
	go func() { first <- s.GetLatest(ctx) }()
	<-started
	cancel()
	require.Equal(t, "unavailable", (<-first).Status)
	var wait sync.WaitGroup
	results := make(chan UpstreamVersionInfo, 16)
	for range 16 {
		wait.Add(1)
		go func() { defer wait.Done(); results <- s.GetLatest(context.Background()) }()
	}
	close(release)
	wait.Wait()
	close(results)
	for result := range results {
		require.Equal(t, "ok", result.Status)
	}
	require.Equal(t, int32(1), calls.Load())
}
