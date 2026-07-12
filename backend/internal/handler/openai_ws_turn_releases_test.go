package handler

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIWSTurnReleaseTrackerReleasesOnlyMatchingTurn(t *testing.T) {
	tracker := newOpenAIWSTurnReleaseTracker()
	var mu sync.Mutex
	released := make([]string, 0, 4)
	add := func(name string) func() {
		return func() {
			mu.Lock()
			released = append(released, name)
			mu.Unlock()
		}
	}

	tracker.setUser(1, add("user-1"))
	tracker.setAccount(1, add("account-1"))
	tracker.setUser(2, add("user-2"))
	tracker.setAccount(2, add("account-2"))
	tracker.releaseTurn(1)

	require.ElementsMatch(t, []string{"account-1", "user-1"}, released)
	require.True(t, tracker.hasUser(2))

	tracker.releaseAll()
	require.ElementsMatch(t, []string{"account-1", "user-1", "account-2", "user-2"}, released)
}

func TestOpenAIWSTurnReleaseTrackerAccountReleaseKeepsUser(t *testing.T) {
	tracker := newOpenAIWSTurnReleaseTracker()
	userReleases := 0
	accountReleases := 0
	tracker.setUser(1, func() { userReleases++ })
	tracker.setAccount(1, func() { accountReleases++ })

	tracker.releaseAccount(1)
	require.Equal(t, 1, accountReleases)
	require.Zero(t, userReleases)
	require.True(t, tracker.hasUser(1))

	tracker.releaseTurn(1)
	require.Equal(t, 1, userReleases)
}
