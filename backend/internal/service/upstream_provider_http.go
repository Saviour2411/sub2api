package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const upstreamMaxResponseBytes = 4 << 20

type upstreamHTTPStatusError struct {
	StatusCode int
	Message    string
}

func (e *upstreamHTTPStatusError) Error() string {
	return fmt.Sprintf("上游返回 HTTP %d: %s", e.StatusCode, e.Message)
}

type upstreamHTTPClient struct {
	client              *http.Client
	allowInsecureHTTP   bool
	requireAllowlist    bool
	allowPrivateHosts   bool
	allowedUpstreamHost []string
}

func newUpstreamHTTPClient(cfg *config.Config) (*upstreamHTTPClient, error) {
	if cfg == nil {
		return nil, errors.New("配置不能为空")
	}
	shared, err := httpclient.GetClient(httpclient.Options{
		Timeout:               45 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ValidateResolvedIP:    true,
		AllowPrivateHosts:     cfg.Security.URLAllowlist.AllowPrivateHosts,
		MaxConnsPerHost:       4,
	})
	if err != nil {
		return nil, fmt.Errorf("创建上游 HTTP 客户端: %w", err)
	}
	clientCopy := *shared
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("上游重定向次数过多")
		}
		if len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return &upstreamHTTPClient{
		client:              &clientCopy,
		allowInsecureHTTP:   cfg.Security.URLAllowlist.AllowInsecureHTTP,
		requireAllowlist:    cfg.Security.URLAllowlist.Enabled,
		allowPrivateHosts:   cfg.Security.URLAllowlist.AllowPrivateHosts,
		allowedUpstreamHost: append([]string(nil), cfg.Security.URLAllowlist.UpstreamHosts...),
	}, nil
}

func (c *upstreamHTTPClient) normalizeBaseURL(raw string) (string, error) {
	normalized, err := urlvalidator.ValidateHTTPURL(raw, c.allowInsecureHTTP, urlvalidator.ValidationOptions{
		AllowedHosts:     c.allowedUpstreamHost,
		RequireAllowlist: c.requireAllowlist,
		AllowPrivate:     c.allowPrivateHosts,
	})
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("上游地址不能包含用户信息、查询参数或片段")
	}
	return strings.TrimRight(normalized, "/"), nil
}

func (c *upstreamHTTPClient) doJSON(
	ctx context.Context,
	method, baseURL, path string,
	headers map[string]string,
	cookie string,
	body any,
) (map[string]any, string, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, "", fmt.Errorf("编码上游请求: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, "", fmt.Errorf("创建上游请求: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("请求上游失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	limited := io.LimitReader(resp.Body, upstreamMaxResponseBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("读取上游响应: %w", err)
	}
	if len(raw) > upstreamMaxResponseBytes {
		return nil, "", errors.New("上游响应体超过 4 MiB 限制")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(raw))
		if len(message) > 200 {
			message = message[:200]
		}
		return nil, "", &upstreamHTTPStatusError{StatusCode: resp.StatusCode, Message: message}
	}
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("解析上游 JSON: %w", err)
	}
	if err := ensureAPISuccess(payload); err != nil {
		return nil, "", err
	}
	return payload, responseCookie(resp), nil
}

func sameOrigin(a, b *url.URL) bool {
	if a == nil || b == nil {
		return false
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func responseCookie(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	parts := make([]string, 0)
	for _, cookie := range resp.Cookies() {
		if cookie.Name != "" {
			parts = append(parts, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(parts, "; ")
}

func ensureAPISuccess(payload map[string]any) error {
	if success, ok := payload["success"].(bool); ok && !success {
		return fmt.Errorf("上游接口失败: %s", apiMessage(payload))
	}
	if code, ok := numberValue(payload["code"]); ok && code != 0 && code != 200 {
		return fmt.Errorf("上游接口失败: %s", apiMessage(payload))
	}
	return nil
}

func apiMessage(payload map[string]any) string {
	for _, key := range []string{"message", "msg", "error"} {
		if value := stringValue(payload[key]); value != "" {
			return value
		}
	}
	return "未知错误"
}

func apiData(payload map[string]any) any {
	if data, ok := payload["data"]; ok {
		return data
	}
	return payload
}

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func asSlice(value any) []any {
	result, _ := value.([]any)
	return result
}

func valueByKeys(value any, keys ...string) any {
	if value == nil {
		return nil
	}
	if object := asMap(value); object != nil {
		for _, key := range keys {
			if found, ok := object[key]; ok {
				return found
			}
		}
		for _, nestedKey := range []string{"data", "user", "summary", "stats", "overview"} {
			if nested, ok := object[nestedKey]; ok {
				if found := valueByKeys(nested, keys...); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func int64Value(value any) (int64, bool) {
	number, ok := numberValue(value)
	return int64(number), ok
}

func floatPointer(value any) *float64 {
	number, ok := numberValue(value)
	if !ok {
		return nil
	}
	return &number
}

func extractItems(payload map[string]any) []any {
	data := apiData(payload)
	if items := asSlice(data); items != nil {
		return items
	}
	if object := asMap(data); object != nil {
		for _, key := range []string{"items", "list", "records", "groups", "logs"} {
			if items := asSlice(object[key]); items != nil {
				return items
			}
		}
	}
	return nil
}

func isHTTPStatus(err error, status int) bool {
	var statusErr *upstreamHTTPStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == status
}
