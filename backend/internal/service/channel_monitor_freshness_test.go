//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMonitorAvailabilityDistinguishesNoSamplesAndFailure(t *testing.T) {
	missing := buildStatusSummary(nil, nil, "model", nil)
	require.Nil(t, missing.Availability7d)
	failed := buildStatusSummary(nil, map[string]*ChannelMonitorAvailability{
		"model": {Model: "model", TotalChecks: 3, AvailabilityPct: 0},
	}, "model", nil)
	require.NotNil(t, failed.Availability7d)
	require.Zero(t, *failed.Availability7d)
	require.Equal(t, 3, failed.Samples7d)
	available := buildStatusSummary(nil, map[string]*ChannelMonitorAvailability{
		"model": {Model: "model", TotalChecks: 1, AvailabilityPct: 100},
	}, "model", nil)
	require.Equal(t, 1, available.Samples7d)
	require.Equal(t, 100.0, *available.Availability7d)
}

func TestMonitorResultStalePreservesSlowProbeBudget(t *testing.T) {
	now := time.Now()
	m := &ChannelMonitor{IntervalSeconds: 300, JitterSeconds: 60}
	grace := 720*time.Second + monitorRequestTimeout + monitorPingTimeout + monitorRunOneBuffer
	boundary := now.Add(-grace)
	old := boundary.Add(-time.Second)
	require.True(t, MonitorResultStale(m, nil, now))
	require.False(t, MonitorResultStale(m, &boundary, now))
	require.True(t, MonitorResultStale(m, &old, now))
	latest := &ChannelMonitorLatest{Model: "model", Status: "operational", CheckedAt: old}
	summary := buildStatusSummary(map[string]*ChannelMonitorLatest{"model": latest}, nil, "model", nil)
	view := buildUserViewFromSummary(m, summary, latest, nil)
	require.True(t, view.Stale)
	require.Equal(t, old, *view.LastCheckedAt)
	require.Nil(t, view.Availability7d)
}

func TestMonitorDetailAvailabilityWindows(t *testing.T) {
	m := &ChannelMonitor{PrimaryModel: "model", IntervalSeconds: 300}
	rows := mergeModelDetails(m, nil, map[int]map[string]*ChannelMonitorAvailability{
		15: {"model": {Model: "model", TotalChecks: 2, AvailabilityPct: 50}},
		30: {"model": {Model: "model", TotalChecks: 3, AvailabilityPct: 0}},
	})
	require.Len(t, rows, 1)
	require.Nil(t, rows[0].Availability7d)
	require.Equal(t, 50.0, *rows[0].Availability15d)
	require.Equal(t, 0.0, *rows[0].Availability30d)
	require.Equal(t, 2, rows[0].Samples15d)
	require.Equal(t, 3, rows[0].Samples30d)
	require.True(t, rows[0].Stale)
}
