package lifecycle

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDrainWaitsForEveryKindAndSession(t *testing.T) {
	m := New("a")
	var releases []func()
	for _, kind := range []string{HTTP, SSE, WS, WSTurn, Upstream, Usage, UsagePending, Producer} {
		done, err := m.Begin(kind)
		require.NoError(t, err)
		releases = append(releases, done)
	}
	m.HoldSession("session", time.Now().Add(80*time.Millisecond))
	m.Drain()
	require.ErrorIs(t, m.Seal(), ErrBusy)
	for _, done := range releases {
		done()
		done()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, m.Wait(ctx))
	require.Equal(t, Drained, m.Snapshot().State)
	require.NoError(t, m.Seal())
	_, err := m.Begin(HTTP)
	require.ErrorIs(t, err, ErrRetired)
}
func TestDrainRetainsResidualRequestsAndWaitsForProducer(t *testing.T) {
	m := New("a")
	finish, accepted := m.BeginBackground()
	require.True(t, accepted)
	m.Drain()
	done, err := m.Begin(HTTP)
	require.NoError(t, err)
	require.ErrorIs(t, m.Seal(), ErrBusy)
	finish()
	done()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, m.Wait(ctx))
}
func TestControl(t *testing.T) {
	m := New("b")
	m.Starting("v1")
	m.Initialized(true)
	socket := filepath.Join(t.TempDir(), "c.sock")
	closeControl, err := m.Control(socket)
	require.NoError(t, err)
	defer closeControl()
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	defer client.CloseIdleConnections()
	get := func(path string) int {
		req, _ := http.NewRequest(http.MethodPost, "http://local"+path, nil)
		r, e := client.Do(req)
		require.NoError(t, e)
		defer func() { _ = r.Body.Close() }()
		_, _ = io.Copy(io.Discard, r.Body)
		return r.StatusCode
	}
	business := m.HTTP(http.NotFoundHandler())
	rr := httptest.NewRecorder()
	business.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	require.Equal(t, 503, rr.Code)
	m.SetProbe(func(context.Context) error { return errors.New("db") })
	require.Equal(t, 409, get("/activate"))
	m.SetProbe(nil)
	require.Equal(t, 204, get("/activate"))
	require.True(t, m.Snapshot().Ready)
	rr = httptest.NewRecorder()
	business.ServeHTTP(rr, httptest.NewRequest("POST", "/drain", nil))
	require.Equal(t, 404, rr.Code)
	require.Equal(t, 202, get("/drain"))
}

func TestDrainRollbackReopensBackgroundWithoutCancellingWork(t *testing.T) {
	m := New("rollback")
	request, err := m.Begin(SSE)
	require.NoError(t, err)
	for range 20 {
		m.Drain()
		_, ok := m.BeginBackground()
		require.False(t, ok)
		require.ErrorIs(t, m.Seal(), ErrBusy)
		require.NoError(t, m.Activate(context.Background()))
		finish, ok := m.BeginBackground()
		require.True(t, ok)
		finish()
		require.Equal(t, int64(1), m.Snapshot().Work[SSE])
	}
	request()
	m.Drain()
	require.Equal(t, Drained, m.Snapshot().State)
	require.NoError(t, m.Activate(context.Background()))
	m.Drain()
	require.NoError(t, m.Seal())
	require.Error(t, m.Activate(context.Background()), "已封闭实例不能回滚")
}
