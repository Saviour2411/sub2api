package lifecycle

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestLongSSECompletesNaturallyDuringDrain(t *testing.T) {
	m := New("sse")
	finish := make(chan struct{})
	server := httptest.NewServer(m.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/residual" {
			w.WriteHeader(200)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: delta\ndata: beginning\n\n")
		_ = http.NewResponseController(w).Flush()
		<-finish
		_, _ = io.WriteString(w, "event: message_stop\ndata: real-terminal\n\n")
	})))
	defer server.Close()
	response, err := http.Get(server.URL)
	require.NoError(t, err)
	defer func() { _ = response.Body.Close() }()
	reader := bufio.NewReader(response.Body)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "event: delta\n", line)
	m.Drain()
	require.Equal(t, int64(1), m.Snapshot().Work[SSE])
	require.ErrorIs(t, m.Seal(), ErrBusy)
	residual, err := http.Get(server.URL + "/residual")
	require.NoError(t, err)
	require.Equal(t, 200, residual.StatusCode)
	_ = residual.Body.Close()
	close(finish)
	rest, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Contains(t, string(rest), "event: message_stop\ndata: real-terminal\n\n")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, m.Wait(ctx))
	require.NoError(t, m.Seal())
}
func TestHijackedWSBlocksRetirementAfterHandlerReturns(t *testing.T) {
	m := New("ws")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server := httptest.NewServer(m.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			defer func() { _ = conn.CloseNow() }()
			for {
				kind, body, err := conn.Read(ctx)
				if err != nil {
					return
				}
				if conn.Write(ctx, kind, body) != nil {
					return
				}
			}
		}()
	})))
	defer server.Close()
	client, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = client.CloseNow() }()
	m.Drain()
	require.Equal(t, int64(1), m.Snapshot().Work[WS])
	require.ErrorIs(t, m.Seal(), ErrBusy)
	require.NoError(t, client.Write(ctx, websocket.MessageText, []byte("still-alive")))
	_, body, err := client.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, "still-alive", string(body))
	_ = client.Close(websocket.StatusNormalClosure, "done")
	require.NoError(t, m.Wait(ctx))
	require.NoError(t, m.Seal())
}
func TestDetachedProducerKeepsDependenciesAlive(t *testing.T) {
	m := New("upstream")
	handlerDone, err := m.Begin(HTTP)
	require.NoError(t, err)
	upstreamDone, err := m.Begin(Upstream)
	require.NoError(t, err)
	handlerDone()
	m.Drain()
	require.ErrorIs(t, m.Seal(), ErrBusy, "客户端断连后仍需等待上游")
	pendingDone, err := m.Begin(UsagePending)
	require.NoError(t, err)
	upstreamDone()
	require.ErrorIs(t, m.Seal(), ErrBusy, "排队用量不能漏记")
	runningDone, err := m.Begin(Usage)
	require.NoError(t, err)
	pendingDone()
	require.ErrorIs(t, m.Seal(), ErrBusy, "同步/异步落库期间不得关闭依赖")
	runningDone()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, m.Wait(ctx))
	require.NoError(t, m.Seal())
}
