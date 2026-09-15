package lifecycle

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const peerHeader = "X-Sub2api-Peer"
const peerRemoteHeader = "X-Sub2api-Peer-Remote"
const peerForwardedHeader = "X-Sub2api-Peer-Forwarded"
const peerRealIPHeader = "X-Sub2api-Peer-Real-Ip"

type peerContextKey struct{}
type verifiedPeerIdentityKey struct{}

// Affinity 只登记可认证的会话归属，连接仍留在拥有它的实例。
// 两个槽共享受保护的 Unix socket 目录，不允许 Redis 记录任意 TCP 目标。
type Affinity struct {
	Registry  Registry
	Manager   *Manager
	SocketDir string
	Secret    []byte
}

type Identity struct{ User, Key, Group int64 }

func (i Identity) scope() string { return fmt.Sprintf("%d:%d", i.Group, i.User) }
func affinityKey(i Identity, kind, ref string) string {
	sum := sha256.Sum256([]byte(ref))
	return "lifecycle:affinity:" + i.scope() + ":" + kind + ":" + hex.EncodeToString(sum[:])
}

// Select 先解析 response 归属，再以 SET NX 认领会话，避免两实例同时首访各自建连接。
// API Key 在两端重新鉴权；同一用户的不同 Key 沿用既有 HTTP response 互通语义。
func (a *Affinity) Select(ctx context.Context, i Identity, session, previous string, ttl time.Duration) (string, error) {
	if i.User <= 0 || i.Key <= 0 {
		return "", errors.New("会话身份无效")
	}
	owner := ""
	if previous != "" {
		v, err := a.Registry.Redis.Get(ctx, affinityKey(i, "response", previous)).Result()
		if err != nil && err != redis.Nil {
			return "", err
		}
		owner = v
	}
	if owner != "" && owner != a.Manager.ID() {
		return owner, nil
	}
	if session == "" {
		return a.Manager.ID(), nil
	}
	key := affinityKey(i, "session", session)
	// 响应归属优先；没有响应归属时才认领尚未绑定的会话。
	if owner == "" {
		_, err := a.Registry.Redis.SetNX(ctx, key, a.Manager.ID(), ttl).Result()
		if err != nil {
			return "", err
		}
		owner, err = a.Registry.Redis.Get(ctx, key).Result()
		if err != nil {
			return "", err
		}
	}
	if owner == a.Manager.ID() {
		// 不覆盖另一个并发请求已经绑定的会话。
		if err := a.Registry.Redis.Eval(ctx, `if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('PEXPIRE',KEYS[1],ARGV[2]) end return 0`, []string{key}, owner, ttl.Milliseconds()).Err(); err != nil {
			return "", err
		}
		a.Manager.HoldSession(key, time.Now().Add(ttl))
	}
	return owner, nil
}

// KeepSession 在实际 WS 仍存活时续约归属，不能让长轮次跨过缓存 TTL 后漂到新实例。
// 不改变 WS 原有的空闲/读取超时；连接结束后停止续约，记录沿既有 TTL 自然到期。
func (a *Affinity) KeepSession(parent context.Context, i Identity, session string, ttl time.Duration) func() {
	if session == "" || ttl <= 0 {
		return func() {}
	}
	key := affinityKey(i, "session", session)
	ctx, cancel := context.WithCancel(parent)
	finished := make(chan struct{})
	interval := min(ttl/3, LeaseInterval)
	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				call, stop := context.WithTimeout(ctx, 2*time.Second)
				renewed, err := a.Registry.Redis.Eval(call, `if redis.call('GET',KEYS[1])==ARGV[1] then return redis.call('PEXPIRE',KEYS[1],ARGV[2]) end return 0`, []string{key}, a.Manager.ID(), ttl.Milliseconds()).Int()
				stop()
				if err == nil && renewed == 1 {
					a.Manager.HoldSession(key, time.Now().Add(ttl))
				}
			}
		}
	}()
	return sync.OnceFunc(func() { cancel(); <-finished })
}

func (a *Affinity) BindResponse(ctx context.Context, i Identity, response string, ttl time.Duration) error {
	if response == "" || i.User <= 0 || i.Key <= 0 {
		return nil
	}
	key := affinityKey(i, "response", response)
	if err := a.Registry.Redis.Set(ctx, key, a.Manager.ID(), ttl).Err(); err != nil {
		return err
	}
	a.Manager.HoldSession(key, time.Now().Add(ttl))
	return nil
}

func (a *Affinity) target(ctx context.Context, id string) (Instance, error) {
	target, err := a.Registry.Lookup(ctx, id)
	if err != nil {
		return target, err
	}
	path := filepath.Clean(target.Socket)
	name := filepath.Base(path)
	if path != target.Socket || filepath.Dir(path) != filepath.Clean(a.SocketDir) || (name != "blue.sock" && name != "green.sock") || id == a.Manager.ID() {
		return Instance{}, errors.New("非法内部转发目标")
	}
	return target, nil
}
func (a *Affinity) signature(r *http.Request, i Identity, target, expires string) string {
	h := hmac.New(sha256.New, a.Secret)
	_, _ = fmt.Fprintf(h, "%s\n%s\n%d\n%d\n%d\n%s\n%s\n%s\n%s\n%s", r.Method, r.URL.RequestURI(), i.User, i.Key, i.Group, target, expires, r.Header.Get(peerRemoteHeader), r.Header.Get(peerForwardedHeader), r.Header.Get(peerRealIPHeader))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *Affinity) authorize(r *http.Request, i Identity) error {
	if verified, ok := r.Context().Value(verifiedPeerIdentityKey{}).(Identity); ok {
		if verified != i {
			return errors.New("内部身份不匹配")
		}
		// 时间窗只限制进入私有通道，不缩短正常的大请求体或 WS 首帧等待时间。
		return nil
	}
	value := r.Header.Get(peerHeader)
	if value == "" {
		return nil
	}
	if r.Context().Value(peerContextKey{}) != true {
		return errors.New("禁止外部伪造内部转发")
	}
	parts := strings.Split(value, ":")
	if len(parts) != 5 {
		return errors.New("内部签名格式无效")
	}
	expires, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || time.Now().Unix() > expires || expires > time.Now().Add(time.Minute).Unix() {
		return errors.New("内部签名过期")
	}
	expected := a.signature(r, i, a.Manager.ID(), parts[3])
	if !hmac.Equal([]byte(parts[4]), []byte(expected)) {
		return errors.New("内部身份不匹配")
	}
	return nil
}
func (a *Affinity) Validate(r *http.Request, i Identity, owner string) error {
	if err := a.authorize(r, i); err != nil {
		return err
	}
	if r.Header.Get(peerHeader) != "" && owner != a.Manager.ID() {
		return errors.New("内部转发归属不匹配或出现循环")
	}
	return nil
}
func (a *Affinity) transport(socket string) *http.Transport {
	return &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "unix", socket)
	}}
}
func (a *Affinity) PeerRequest(r *http.Request, i Identity, owner string) (*http.Request, *http.Transport, error) {
	if r.Header.Get(peerHeader) != "" {
		return nil, nil, errors.New("禁止内部转发循环")
	}
	target, err := a.target(r.Context(), owner)
	if err != nil {
		return nil, nil, err
	}
	request := r.Clone(r.Context())
	request.URL = &url.URL{Scheme: "http", Host: "peer", Path: r.URL.Path, RawPath: r.URL.RawPath, RawQuery: r.URL.RawQuery}
	request.RequestURI = ""
	request.GetBody = nil // 无论任何方法，禁止重放已发送的请求。
	expires := strconv.FormatInt(time.Now().Add(30*time.Second).Unix(), 10)
	request.Header.Set(peerRemoteHeader, r.RemoteAddr)
	request.Header.Set(peerForwardedHeader, r.Header.Get("X-Forwarded-For"))
	request.Header.Set(peerRealIPHeader, r.Header.Get("X-Real-Ip"))
	request.Header.Set(peerHeader, fmt.Sprintf("%d:%d:%d:%s:%s", i.User, i.Key, i.Group, expires, a.signature(request, i, owner, expires)))
	return request, a.transport(target.Socket), nil
}
func (a *Affinity) Forward(w http.ResponseWriter, r *http.Request, i Identity, owner string) error {
	request, transport, err := a.PeerRequest(r, i, owner)
	if err != nil {
		return err
	}
	defer transport.CloseIdleConnections()
	proxy := httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL = request.URL
			p.Out.Host = request.Host
			p.Out.GetBody = nil
			for _, name := range []string{peerHeader, peerRemoteHeader, peerForwardedHeader, peerRealIPHeader} {
				p.Out.Header.Set(name, request.Header.Get(name))
			}
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "session owner unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, request)
	return nil
}

// Metadata 延续原有 TTL，供可迁移的 turn state 使用；不持久化 TCP 连接对象。
func (a *Affinity) SetMetadata(ctx context.Context, key, value string, ttl time.Duration) error {
	return a.Registry.Redis.Set(ctx, "lifecycle:metadata:"+key, value, ttl).Err()
}
func (a *Affinity) GetMetadata(ctx context.Context, key string) (string, error) {
	return a.Registry.Redis.Get(ctx, "lifecycle:metadata:"+key).Result()
}
func (a *Affinity) DeleteMetadata(ctx context.Context, key string) error {
	return a.Registry.Redis.Del(ctx, "lifecycle:metadata:"+key).Err()
}

func (a *Affinity) AuthenticatedPeer(r *http.Request, i Identity) (bool, error) {
	if err := a.authorize(r, i); err != nil {
		return false, err
	}
	return r.Header.Get(peerHeader) != "", nil
}

// Unix 通道先验证签名并恢复源实例看到的网络身份，然后由原 API Key 中间件重新鉴权。
// 这保证 IP 白名单/可信代理策略不会因中转而被绕过或误拒绝。
func (a *Affinity) AuthenticateTransport(r *http.Request) error {
	parts := strings.Split(r.Header.Get(peerHeader), ":")
	if len(parts) != 5 {
		return errors.New("内部身份格式无效")
	}
	values := make([]int64, 3)
	for index := range values {
		value, err := strconv.ParseInt(parts[index], 10, 64)
		if err != nil {
			return err
		}
		values[index] = value
	}
	identity := Identity{User: values[0], Key: values[1], Group: values[2]}
	if err := a.authorize(r, identity); err != nil {
		return err
	}
	*r = *r.WithContext(context.WithValue(r.Context(), verifiedPeerIdentityKey{}, identity))
	r.RemoteAddr = r.Header.Get(peerRemoteHeader)
	r.Header.Set("X-Forwarded-For", r.Header.Get(peerForwardedHeader))
	r.Header.Set("X-Real-Ip", r.Header.Get(peerRealIPHeader))
	return nil
}
