package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayQueryOrderStatusMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		wantStatus  string
		wantTradeNo string
		wantAmount  float64
	}{
		{
			name:        "trade success wins over numeric status",
			body:        `{"code":1,"status":1,"trade_no":"B123","money":"20.00","trade_status":"TRADE_SUCCESS"}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "B123",
			wantAmount:  20,
		},
		{
			name:        "waiting is pending even when numeric status is one",
			body:        `{"code":1,"status":1,"trade_no":"B456","money":"20.00","trade_status":"WAITING"}`,
			wantStatus:  payment.ProviderStatusPending,
			wantTradeNo: "B456",
			wantAmount:  20,
		},
		{
			name:        "nested data fields are accepted",
			body:        `{"code":1,"status":1,"data":{"trade_no":"B789","money":"28.00","trade_status":"TRADE_SUCCESS"}}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "B789",
			wantAmount:  28,
		},
		{
			name:        "legacy numeric paid without trade status remains compatible",
			body:        `{"code":1,"status":1,"money":"10.00"}`,
			wantStatus:  payment.ProviderStatusPaid,
			wantTradeNo: "out-123",
			wantAmount:  10,
		},
		{
			name:        "legacy numeric pending without trade status remains pending",
			body:        `{"code":1,"status":0,"money":"10.00"}`,
			wantStatus:  payment.ProviderStatusPending,
			wantTradeNo: "out-123",
			wantAmount:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := newTestEasyPay(t, "https://easypay.test")
			provider.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/api.php" {
					t.Errorf("query path = %q, want /api.php", r.URL.Path)
				}
				if err := r.ParseForm(); err != nil {
					t.Errorf("ParseForm: %v", err)
				}
				if got := r.PostForm.Get("act"); got != "order" {
					t.Errorf("form[act] = %q, want order", got)
				}
				if got := r.PostForm.Get("out_trade_no"); got != "out-123" {
					t.Errorf("form[out_trade_no] = %q, want out-123", got)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}, nil
			})}

			resp, err := provider.QueryOrder(context.Background(), "out-123")
			if err != nil {
				t.Fatalf("QueryOrder returned error: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", resp.Status, tt.wantStatus)
			}
			if resp.TradeNo != tt.wantTradeNo {
				t.Fatalf("trade no = %q, want %q", resp.TradeNo, tt.wantTradeNo)
			}
			if resp.Amount != tt.wantAmount {
				t.Fatalf("amount = %v, want %v", resp.Amount, tt.wantAmount)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
