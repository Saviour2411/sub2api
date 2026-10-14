package manxue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const BaseURL = "https://manxue.ai"
const MaxResponseBytes = 5 << 20

type Request struct {
	Benchmark       string `json:"benchmark"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	Model           string `json:"model"`
	Protocol        string `json:"protocol"`
	ReasoningEffort string `json:"reasoning_effort"`
}

type Usage struct {
	InputTokens         *int64 `json:"input_tokens"`
	OutputTokens        *int64 `json:"output_tokens"`
	ReasoningTokens     *int64 `json:"reasoning_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens *int64 `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

type Result struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Benchmark  string `json:"benchmark"`
	DurationMS int64  `json:"duration_ms"`
	Candy      struct {
		Status     string `json:"status"`
		Answer     string `json:"answer"`
		DurationMS int64  `json:"duration_ms"`
		Usage
	} `json:"candy"`
	Result struct {
		HTML       string `json:"html"`
		DurationMS int64  `json:"duration_ms"`
		Usage      Usage  `json:"usage"`
	} `json:"result"`
	Assessment struct {
		Quality string `json:"quality"`
		Reason  string `json:"reason"`
	} `json:"assessment"`
	Usage Usage `json:"usage"`
}

// 不包含远端响应体和请求地址，避免错误日志泄漏凭据。
type Error struct {
	Status     int
	RetryAfter time.Duration
	Ambiguous  bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("第三方检测请求失败（HTTP %d）", e.Status)
}

type Client struct {
	HTTP    *http.Client
	BaseURL string
}

func NewClient() *Client {
	return &Client{BaseURL: BaseURL, HTTP: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Submit(ctx context.Context, input Request, key string) (*Result, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return c.call(ctx, http.MethodPost, "/api/v1/tests", raw, key)
}
func (c *Client) Poll(ctx context.Context, id string) (*Result, error) {
	return c.call(ctx, http.MethodGet, "/api/v1/tests/"+url.PathEscape(id), nil, "")
}
func (c *Client) call(ctx context.Context, method, path string, body []byte, key string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{Ambiguous: method == http.MethodPost}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	// 幂等范围依赖出口 IP；禁止传输层在未知提交结果后自动重放 POST。
	if method == http.MethodPost {
		req.GetBody = nil
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, &Error{Ambiguous: method == http.MethodPost}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{Status: resp.StatusCode, RetryAfter: ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()), Ambiguous: method == http.MethodPost && resp.StatusCode != 429 && resp.StatusCode != 503 && resp.StatusCode >= 500}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil || len(raw) > MaxResponseBytes {
		return nil, &Error{Ambiguous: method == http.MethodPost}
	}
	var result Result
	if json.Unmarshal(raw, &result) != nil || result.Status == "" || (method == http.MethodPost && (result.ID == "" || len(result.ID) > 256)) {
		return nil, &Error{Ambiguous: method == http.MethodPost}
	}
	return &result, nil
}

func ParseRetryAfter(raw string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds >= 0 {
		if seconds > int64((time.Duration(1<<63-1))/time.Second) {
			return time.Duration(1<<63 - 1)
		}
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(raw); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}
