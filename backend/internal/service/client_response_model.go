package service

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

// ClientResponseModelMode 读取现有网关配置快照。过期快照在后台刷新，首帧不等数据库。
func (s *SettingService) ClientResponseModelMode() string {
	if s == nil {
		return ""
	}
	cached, _ := s.gatewaySettingsCache.Load().(*cachedGatewaySettings)
	if (cached == nil || time.Now().UnixNano() >= cached.expiresAt) && s.settingRepo != nil && s.responseModelRefreshing.CompareAndSwap(false, true) {
		go func() {
			defer s.responseModelRefreshing.Store(false)
			s.GetGatewayRuntime(context.Background())
		}()
	}
	if cached != nil && cached.settings.ResponseModelRewriteEnabled {
		return cached.settings.ResponseModelRewriteMode
	}
	return ""
}

// ClientResponseModel 只处理客户端协议声明的型号。零值直接透传。
type ClientResponseModel struct {
	target string
	quoted []byte
}

func NewClientResponseModel(mode, requested string) ClientResponseModel {
	switch mode {
	case ResponseModelModeEmpty:
		return ClientResponseModel{quoted: []byte(`""`)}
	case ResponseModelModeRequest:
		if requested != "" {
			quoted, _ := json.Marshal(requested)
			return ClientResponseModel{target: requested, quoted: quoted}
		}
	}
	return ClientResponseModel{}
}

// Rewrite 只修改指定的已存在字段，审计、用量和工具参数始终使用原始字节。
// 调用方可以用返回的 changed 判断是否需要重组 SSE 外层。
func (m ClientResponseModel) Rewrite(body []byte, fields ...string) ([]byte, bool) {
	if m.quoted == nil || (!bytes.Contains(body, []byte(`"model`)) && !bytes.Contains(body, []byte(`\u`))) {
		return body, false
	}
	if trimmed := bytes.TrimSpace(body); len(trimmed) > 0 && trimmed[0] == '[' {
		// Gemini 也能一次返回一组生成分块，按每个顶层对象的字段处理。
		if !gjson.ValidBytes(body) {
			return body, false
		}
		var out []byte
		start := 0
		gjson.ParseBytes(body).ForEach(func(_, item gjson.Result) bool {
			if item.IsObject() {
				end := item.Index + len(item.Raw)
				if b, changed := m.RewriteObjectInFrame(body[item.Index:end], 0, end-item.Index, fields...); changed {
					out = append(out, body[start:item.Index]...)
					out = append(out, b...)
					start = end
				}
			}
			return true
		})
		if out != nil {
			return append(out, body[start:]...), true
		}
		return body, false
	}
	return m.RewriteObjectInFrame(body, 0, len(body), fields...)
}

// RewriteObjectInFrame 对完整 JSON 对象使用字面字段路径，在保留 SSE 前后缀时也只复制一次。
func (m ClientResponseModel) RewriteObjectInFrame(frame []byte, start, stop int, fields ...string) ([]byte, bool) {
	if m.quoted == nil {
		return frame, false
	}
	changed := false
	for _, field := range fields {
		body := frame[start:stop]
		value := gjson.GetBytes(body, field)
		if !value.Exists() || (value.Type == gjson.String && value.Str == m.target) {
			continue
		}
		if !changed && !gjson.ValidBytes(body) {
			return frame, false
		}
		begin := start + value.Index
		end := begin + len(value.Raw)
		diff := len(m.quoted) - len(value.Raw)
		out := make([]byte, 0, len(frame)+diff)
		out = append(out, frame[:begin]...)
		out = append(out, m.quoted...)
		frame = append(out, frame[end:]...)
		stop += diff
		changed = true
	}
	return frame, changed
}

// ResponseModelBufferLimits 复用既有传输上限，仅用于跨 Write 的不完整响应。
func ResponseModelBufferLimits(cfg *config.Config) (int64, int64) {
	lineSize := 0
	if cfg != nil {
		lineSize = cfg.Gateway.MaxLineSize
	}
	return resolveUpstreamResponseReadLimit(cfg), newSSEEventLimits(cfg, lineSize).maxBytes
}
