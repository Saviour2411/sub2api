package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAccountFailureStreakPolicyFingerprint(t *testing.T) {
	settings := DefaultGatewaySettings()
	settings.FirstTokenTimeoutSeconds = 30
	settings.FirstTokenTimeoutConsecutiveThreshold = 5
	settings.UpstreamErrorStatusCodes = []int{504, 502, 504}
	settings.UpstreamErrorConsecutiveThreshold = 12
	settings.FailurePolicyRevision = 9

	require.Equal(
		t,
		"timeout_seconds=30;threshold=5",
		BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceFirstTokenTimeout, settings),
	)
	require.Equal(
		t,
		"status_codes=502,504;threshold=12",
		BuildAccountFailureStreakPolicyFingerprint(AccountFailureStreakSourceUpstreamError, settings),
	)
	require.Equal(t, []int{504, 502, 504}, settings.UpstreamErrorStatusCodes)
	require.Equal(t, AccountFailureStreakPolicy{
		Revision:    9,
		Fingerprint: "timeout_seconds=30;threshold=5",
	}, BuildAccountFailureStreakPolicy(AccountFailureStreakSourceFirstTokenTimeout, settings))
	require.Equal(
		t,
		"first_token={timeout_seconds=30;threshold=5};upstream={status_codes=502,504;threshold=12}",
		BuildGatewayFailurePolicyFingerprint(settings),
	)
}
