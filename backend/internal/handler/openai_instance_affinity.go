package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func affinityIdentity(key *service.APIKey, user int64) lifecycle.Identity {
	id := lifecycle.Identity{User: user, Key: key.ID}
	if key.GroupID != nil {
		id.Group = *key.GroupID
	}
	return id
}
func (h *OpenAIGatewayHandler) instanceOwner(c *gin.Context, key *service.APIKey, user int64, body []byte) (string, error) {
	a := lifecycle.Process.Affinity()
	if a == nil {
		return lifecycle.Process.ID(), nil
	}
	ttl := time.Hour
	if h.cfg != nil && h.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		ttl = time.Duration(h.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	id := affinityIdentity(key, user)
	owner, err := a.Select(ctx, id, h.gatewayService.GenerateSessionHash(c, body), strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String()), ttl)
	if err != nil {
		return "", err
	}
	return owner, a.Validate(c.Request, id, owner)
}
func (h *OpenAIGatewayHandler) forwardInstanceHTTP(c *gin.Context, key *service.APIKey, user int64, body []byte) bool {
	owner, err := h.instanceOwner(c, key, user, body)
	if err != nil {
		h.errorResponse(c, http.StatusBadGateway, "session_owner_unavailable", "Session ownership is unavailable")
		return true
	}
	if owner == lifecycle.Process.ID() {
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	if err := lifecycle.Process.Affinity().Forward(c.Writer, c.Request, affinityIdentity(key, user), owner); err != nil {
		h.errorResponse(c, http.StatusBadGateway, "session_owner_unavailable", "Session owner is unavailable")
	}
	return true
}

func (h *OpenAIGatewayHandler) keepInstanceWSSession(c *gin.Context, key *service.APIKey, user int64, body []byte) func() {
	a := lifecycle.Process.Affinity()
	if a == nil {
		return func() {}
	}
	ttl := time.Hour
	if h.cfg != nil && h.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds > 0 {
		ttl = time.Duration(h.cfg.Gateway.OpenAIWS.StickySessionTTLSeconds) * time.Second
	}
	return a.KeepSession(c.Request.Context(), affinityIdentity(key, user), h.gatewayService.GenerateSessionHash(c, body), ttl)
}

// WS 首帧只用于归属解析；中转实例不领取并发租约、不调度账号，也不执行计费。
// 每条消息原样转发一次，不重试生成请求、不缓存完整会话。
func (h *OpenAIGatewayHandler) forwardInstanceWS(c *gin.Context, key *service.APIKey, user int64, client *coderws.Conn, kind coderws.MessageType, first []byte, beforeLegacy func()) bool {
	owner, err := h.instanceOwner(c, key, user, first)
	if err != nil {
		closeOpenAIClientWS(client, coderws.StatusTryAgainLater, "Session ownership is unavailable")
		return true
	}
	if owner == lifecycle.Process.ID() {
		return false
	}
	if owner == lifecycle.LegacyOwner {
		beforeLegacy()
	}
	req, transport, err := lifecycle.Process.Affinity().PeerRequest(c.Request, affinityIdentity(key, user), owner)
	if err != nil {
		closeOpenAIClientWS(client, coderws.StatusTryAgainLater, "Session owner is unavailable")
		return true
	}
	defer transport.CloseIdleConnections()
	req.URL.Scheme = "ws"
	req.URL.Host = c.Request.Host
	headers := req.Header.Clone()
	for _, name := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Extensions", "Sec-WebSocket-Accept"} {
		headers.Del(name)
	}
	upstream, response, err := coderws.Dial(c.Request.Context(), req.URL.String(), &coderws.DialOptions{
		HTTPClient: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("禁止内部重定向") }},
		HTTPHeader: headers,
	})
	if err != nil {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		closeOpenAIClientWS(client, coderws.StatusTryAgainLater, "Session owner is unavailable")
		return true
	}
	defer func() { _ = upstream.CloseNow() }()
	upstream.SetReadLimit(service.ResolveOpenAIWSClientReadLimitBytes(h.cfg))
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	if err := upstream.Write(ctx, kind, first); err != nil {
		return true
	}
	var wg sync.WaitGroup
	pump := func(dst, src *coderws.Conn) {
		defer wg.Done()
		defer cancel()
		for {
			kind, reader, err := src.Reader(ctx)
			if err != nil {
				var closeErr coderws.CloseError
				if errors.As(err, &closeErr) {
					_ = dst.Close(closeErr.Code, closeErr.Reason)
				}
				return
			}
			writer, err := dst.Writer(ctx, kind)
			if err != nil {
				return
			}
			_, err = io.Copy(writer, reader)
			closeErr := writer.Close()
			if err != nil || closeErr != nil {
				return
			}
		}
	}
	wg.Add(2)
	go pump(upstream, client)
	go pump(client, upstream)
	wg.Wait()
	return true
}
