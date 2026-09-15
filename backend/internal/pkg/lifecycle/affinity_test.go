package lifecycle

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func affinityPair(t *testing.T) (*Affinity, *Affinity) {
	t.Helper()
	r := miniredis.RunT(t)
	db := redis.NewClient(&redis.Options{Addr: r.Addr()})
	t.Cleanup(func() { _ = db.Close() })
	dir := t.TempDir()
	a := &Affinity{Registry: Registry{Redis: db}, Manager: New("a"), SocketDir: dir, Secret: []byte("a-long-shared-secret-for-tests-only")}
	b := &Affinity{Registry: a.Registry, Manager: New("b"), SocketDir: dir, Secret: a.Secret}
	a.Manager.SetAffinity(a)
	b.Manager.SetAffinity(b)
	for slot, peer := range map[string]*Affinity{"blue": a, "green": b} {
		require.NoError(t, peer.Registry.Publish(context.Background(), Instance{ID: peer.Manager.ID(), Socket: filepath.Join(dir, slot+".sock")}))
	}
	return a, b
}
func TestAffinityClaimAndScope(t *testing.T) {
	a, b := affinityPair(t)
	id := Identity{User: 1, Key: 2, Group: 3}
	ctx := context.Background()
	var wg sync.WaitGroup
	owners := make(chan string, 2)
	for _, peer := range []*Affinity{a, b} {
		wg.Add(1)
		go func(p *Affinity) {
			defer wg.Done()
			owner, err := p.Select(ctx, id, "session", "", time.Hour)
			require.NoError(t, err)
			owners <- owner
		}(peer)
	}
	wg.Wait()
	first := <-owners
	require.Equal(t, first, <-owners)
	owner, err := b.Select(ctx, Identity{User: 99, Key: 2, Group: 3}, "session", "", time.Hour)
	require.NoError(t, err)
	require.Equal(t, "b", owner, "不同用户不能继承同名会话")
	require.NoError(t, a.BindResponse(ctx, id, "response", time.Hour))
	owner, err = b.Select(ctx, Identity{User: 1, Key: 44, Group: 3}, "", "response", time.Hour)
	require.NoError(t, err)
	require.Equal(t, "a", owner, "保留同用户跨 API Key 续接")
	a.Manager.Drain()
	require.NotEqual(t, Drained, a.Manager.Snapshot().State)
}
func TestPeer(t *testing.T) {
	a, b := affinityPair(t)
	id := Identity{User: 1, Key: 2, Group: 3}
	control, err := a.Manager.Control(filepath.Join(a.SocketDir, "blue.sock"), a.Manager.HTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := a.Validate(r, id, "a"); err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Test-Remote", r.RemoteAddr)
		w.Header().Set("Test-Forwarded", r.Header.Get("X-Forwarded-For"))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.Copy(w, r.Body)
		_, _ = io.WriteString(w, "event: message_stop\ndata: {}\n\n")
	})))
	require.NoError(t, err)
	defer control()
	request := httptest.NewRequest(http.MethodPost, "http://api.example/v1/responses", strings.NewReader("event: delta\ndata: hello\n\n"))
	request.Header.Set("X-Forwarded-For", "203.0.113.1")
	response := httptest.NewRecorder()
	require.NoError(t, b.Forward(response, request, id, "a"))
	require.Equal(t, 200, response.Code)
	require.Equal(t, request.RemoteAddr, response.Header().Get("Test-Remote"))
	require.Equal(t, "203.0.113.1", response.Header().Get("Test-Forwarded"))
	require.Contains(t, response.Body.String(), "hello")
	require.Contains(t, response.Body.String(), "message_stop")
	// 签名绑定已重新鉴权的用户与 Key，不能替换身份。
	forwarded, transport, err := b.PeerRequest(request, id, "a")
	require.NoError(t, err)
	defer transport.CloseIdleConnections()
	require.Error(t, a.Validate(forwarded, id, "a"), "公网请求不能获得私有通道上下文")
	forwarded = forwarded.WithContext(context.WithValue(forwarded.Context(), peerContextKey{}, true))
	require.NoError(t, a.Validate(forwarded, id, "a"))
	require.Error(t, a.Validate(forwarded, Identity{User: 9, Key: 2, Group: 3}, "a"))
	require.Error(t, a.Validate(forwarded, id, "b"), "禁止转发环路")
	_, _, err = b.PeerRequest(forwarded, id, "a")
	require.Error(t, err)
	require.NoError(t, a.Registry.Publish(context.Background(), Instance{ID: "evil", Socket: "/var/run/docker.sock"}))
	_, _, err = b.PeerRequest(request, id, "evil")
	require.Error(t, err)
}
func TestPublicPeerHeaderRejectedBeforeHandler(t *testing.T) {
	called := false
	m := New("test")
	request := httptest.NewRequest("POST", "http://test/v1/responses", nil)
	request.Header.Set(peerHeader, "forged")
	response := httptest.NewRecorder()
	m.HTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, request)
	require.Equal(t, 403, response.Code)
	require.False(t, called)
}

func TestAffinityKeepSessionRenewsOnlyLiveConnectionOwner(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	defer func() { _ = client.Close() }()
	a := &Affinity{Registry: Registry{Redis: client}, Manager: New("owner")}
	identity := Identity{User: 1, Key: 2, Group: 3}
	ctx := context.Background()
	ttl := 120 * time.Millisecond
	owner, err := a.Select(ctx, identity, "connected", "", ttl)
	require.NoError(t, err)
	require.Equal(t, "owner", owner)
	stop := a.KeepSession(ctx, identity, "connected", ttl)
	defer stop()
	key := affinityKey(identity, "session", "connected")
	server.FastForward(80 * time.Millisecond)
	require.Eventually(t, func() bool { return server.TTL(key) > 40*time.Millisecond }, time.Second, time.Millisecond)
	// 即使错误记录指向另一个拥有者，也不能覆盖对方的绑定。
	require.NoError(t, client.Set(ctx, key, "peer", ttl).Err())
	server.FastForward(80 * time.Millisecond)
	time.Sleep(60 * time.Millisecond)
	value, err := client.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, "peer", value)
	require.Equal(t, 40*time.Millisecond, server.TTL(key))
	stop()
	server.FastForward(ttl)
	require.False(t, server.Exists(key), "连接关闭后沿原 TTL 到期，不再续约")
}
