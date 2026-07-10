//go:build unit

package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type publicPricingSettingRepo struct {
	values map[string]string
}

func (r publicPricingSettingRepo) Get(context.Context, string) (*service.Setting, error) {
	return nil, nil
}

func (r publicPricingSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r publicPricingSettingRepo) Set(context.Context, string, string) error {
	return nil
}

func (r publicPricingSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = r.values[key]
	}
	return out, nil
}

func (r publicPricingSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}

func (r publicPricingSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r publicPricingSettingRepo) Delete(context.Context, string) error {
	return nil
}

func newPublicPricingTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:public_pricing?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestPaymentHandler_GetPublicModelPricingSanitizesAndFiltersPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	client := newPublicPricingTestClient(t)

	activeGroup, err := client.Group.Create().
		SetName("OpenAI Public").
		SetStatus(service.StatusActive).
		SetPlatform(service.PlatformOpenAI).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		SetRateMultiplier(0.58).
		SetWeeklyLimitUsd(250).
		SetMonthlyLimitUsd(1000).
		Save(ctx)
	require.NoError(t, err)
	inactiveGroup, err := client.Group.Create().
		SetName("Inactive").
		SetStatus("inactive").
		SetPlatform(service.PlatformOpenAI).
		SetSubscriptionType(service.SubscriptionTypeSubscription).
		Save(ctx)
	require.NoError(t, err)
	standardGroup, err := client.Group.Create().
		SetName("Standard").
		SetStatus(service.StatusActive).
		SetPlatform(service.PlatformOpenAI).
		SetSubscriptionType(service.SubscriptionTypeStandard).
		Save(ctx)
	require.NoError(t, err)

	originalPrice := 128.0
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(activeGroup.ID).
		SetName("ZeroApi Core").
		SetDescription("中度使用套餐").
		SetPrice(58).
		SetOriginalPrice(originalPrice).
		SetValidityDays(30).
		SetValidityUnit("day").
		SetFeatures("快速响应\n优先支持").
		SetForSale(true).
		SetSortOrder(1).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(inactiveGroup.ID).
		SetName("Inactive Plan").
		SetPrice(99).
		SetForSale(true).
		SetSortOrder(2).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(standardGroup.ID).
		SetName("Standard Plan").
		SetPrice(88).
		SetForSale(true).
		SetSortOrder(3).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.SubscriptionPlan.Create().
		SetGroupID(activeGroup.ID).
		SetName("Hidden Plan").
		SetPrice(188).
		SetForSale(false).
		SetSortOrder(4).
		Save(ctx)
	require.NoError(t, err)

	configSvc := service.NewPaymentConfigService(client, publicPricingSettingRepo{values: map[string]string{
		service.SettingPaymentEnabled:      "true",
		service.SettingBalanceRechargeMult: "8",
		service.SettingHelpText:            "请扫码联系客服",
		service.SettingHelpImageURL:        "https://example.com/help.png",
	}}, nil)
	h := NewPaymentHandler(nil, configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-pricing", nil)

	h.GetPublicModelPricing(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Enabled                   bool    `json:"enabled"`
			GeneratedAt               string  `json:"generated_at"`
			BalanceRechargeMultiplier float64 `json:"balance_recharge_multiplier"`
			HelpText                  string  `json:"help_text"`
			HelpImageURL              string  `json:"help_image_url"`
			Plans                     []struct {
				ID              int64    `json:"id"`
				GroupPlatform   string   `json:"group_platform"`
				GroupName       string   `json:"group_name"`
				RateMultiplier  float64  `json:"rate_multiplier"`
				WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
				MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`
				Name            string   `json:"name"`
				Description     string   `json:"description"`
				Price           float64  `json:"price"`
				OriginalPrice   *float64 `json:"original_price"`
				ValidityDays    int      `json:"validity_days"`
				Features        []string `json:"features"`
			} `json:"plans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.True(t, resp.Data.Enabled)
	require.NotEmpty(t, resp.Data.GeneratedAt)
	require.Equal(t, 8.0, resp.Data.BalanceRechargeMultiplier)
	require.Equal(t, "请扫码联系客服", resp.Data.HelpText)
	require.Equal(t, "https://example.com/help.png", resp.Data.HelpImageURL)
	require.Len(t, resp.Data.Plans, 1)

	plan := resp.Data.Plans[0]
	require.Equal(t, "ZeroApi Core", plan.Name)
	require.Equal(t, "OpenAI Public", plan.GroupName)
	require.Equal(t, service.PlatformOpenAI, plan.GroupPlatform)
	require.Equal(t, 0.58, plan.RateMultiplier)
	require.Equal(t, []string{"快速响应", "优先支持"}, plan.Features)
	require.NotNil(t, plan.OriginalPrice)

	rawPlan, err := json.Marshal(plan)
	require.NoError(t, err)
	require.NotContains(t, string(rawPlan), "group_id")
	require.NotContains(t, string(rawPlan), "payment_type")
	require.NotContains(t, string(rawPlan), "provider")
	require.NotContains(t, recorder.Body.String(), "group_id")
	require.NotContains(t, recorder.Body.String(), "stripe_publishable_key")
	require.NotContains(t, recorder.Body.String(), "methods")
}

func TestPaymentHandler_GetPublicModelPricingDisabledReturnsEmptyPlans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newPublicPricingTestClient(t)
	configSvc := service.NewPaymentConfigService(client, publicPricingSettingRepo{values: map[string]string{
		service.SettingPaymentEnabled: "false",
	}}, nil)
	h := NewPaymentHandler(nil, configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-pricing", nil)

	h.GetPublicModelPricing(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Enabled bool              `json:"enabled"`
			Plans   []json.RawMessage `json:"plans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.False(t, resp.Data.Enabled)
	require.Empty(t, resp.Data.Plans)
}

func TestPaymentHandler_GetCheckoutInfoExposesBalanceRechargeBonusRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newPublicPricingTestClient(t)
	configSvc := service.NewPaymentConfigService(client, publicPricingSettingRepo{values: map[string]string{
		service.SettingPaymentEnabled:    "true",
		service.SettingBalanceBonusRules: `[{"min_amount":10,"max_amount":50,"bonus_rate":5},{"min_amount":50,"max_amount":200,"bonus_rate":8},{"min_amount":200,"bonus_rate":10}]`,
	}}, nil)
	h := NewPaymentHandler(nil, configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/checkout-info", nil)

	h.GetCheckoutInfo(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Code int `json:"code"`
		Data struct {
			BalanceRechargeBonusRules []service.PaymentBonusRule `json:"balance_recharge_bonus_rules"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 0, resp.Code)
	require.Len(t, resp.Data.BalanceRechargeBonusRules, 3)
	require.Equal(t, 5.0, resp.Data.BalanceRechargeBonusRules[0].BonusRate)
	require.Equal(t, 50.0, resp.Data.BalanceRechargeBonusRules[1].MinAmount)
	require.Nil(t, resp.Data.BalanceRechargeBonusRules[2].MaxAmount)
}

func TestPaymentHandler_GetCheckoutInfoReturnsEmptyBalanceRechargeBonusRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newPublicPricingTestClient(t)
	configSvc := service.NewPaymentConfigService(client, publicPricingSettingRepo{values: map[string]string{}}, nil)
	h := NewPaymentHandler(nil, configSvc, nil)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/payment/checkout-info", nil)

	h.GetCheckoutInfo(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Data struct {
			BalanceRechargeBonusRules []service.PaymentBonusRule `json:"balance_recharge_bonus_rules"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.BalanceRechargeBonusRules)
	require.Empty(t, resp.Data.BalanceRechargeBonusRules)
}
