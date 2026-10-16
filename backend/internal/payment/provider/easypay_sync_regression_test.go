package provider

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestEasyPayRejectsNonFiniteAmounts(t *testing.T) {
	for _, money := range []string{"NaN", "Inf", "-Inf", "1e309"} {
		t.Run(money, func(t *testing.T) {
			e, _ := newConfirmTestEasyPay(t, `{"code":1,"status":1,"money":"`+money+`"}`, http.StatusOK)
			_, err := e.QueryOrder(context.Background(), "sub2_ORDER1")
			require.Error(t, err, "查单不得返回非有限金额")
			require.Error(t, e.ConfirmNotification(context.Background(), confirmTestNotification()), "非有限金额不得通过回调确认")

			params := map[string]string{
				"pid": "1001", "out_trade_no": "sub2_ORDER1", "trade_no": "2026101600001",
				"type": "alipay", "name": "测试充值", "money": money, "trade_status": tradeStatusSuccess,
			}
			params["sign"] = easyPaySign(params, e.config["pkey"])
			values := url.Values{}
			for key, value := range params {
				values.Set(key, value)
			}
			_, err = e.VerifyNotification(context.Background(), values.Encode(), nil)
			require.Error(t, err, "合法签名不能使非有限金额通过验签入口")
		})
	}
}

func TestEasyPayConfirmRejectsNonFiniteNotificationAmount(t *testing.T) {
	e, _ := newConfirmTestEasyPay(t, `{"code":1,"status":1,"money":"10.00"}`, http.StatusOK)
	for _, amount := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		n := confirmTestNotification()
		n.Amount = amount
		require.Error(t, e.ConfirmNotification(context.Background(), n))
	}
}

func TestEasyPayQueryRetainsNestedCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   string
		status string
	}{
		{"空顶层采用嵌套状态及金额", `{"code":1,"trade_status":"","money":"","trade_no":"","data":{"trade_status":" trade_success ","money":"10.00","trade_no":"nested"}}`, payment.ProviderStatusPaid},
		{"旧数字默认值采用嵌套字段", `{"code":1,"status":0,"data":{"status":"1","money":10,"trade_no":"nested"}}`, payment.ProviderStatusPaid},
		{"等待状态优先于数字已付", `{"code":1,"trade_status":" WAITING ","status":1,"data":{"money":"10.00","trade_no":"nested"}}`, payment.ProviderStatusPending},
		{"显式空状态不采用数字已付", `{"code":1,"trade_status":"","status":"1","data":{"money":"10.00","trade_no":"nested"}}`, payment.ProviderStatusPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := newConfirmTestEasyPay(t, tc.body, http.StatusOK)
			response, err := e.QueryOrder(context.Background(), "sub2_ORDER1")
			require.NoError(t, err)
			require.Equal(t, tc.status, response.Status)
			require.Equal(t, 10.0, response.Amount)
			require.Equal(t, "nested", response.TradeNo)
		})
	}
}
