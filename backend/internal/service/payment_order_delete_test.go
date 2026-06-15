//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAdminDeleteOrderDeletesSafeStatusAndWritesAuditLog(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createAdminDeleteOrderTestOrder(t, ctx, client, OrderStatusFailed)
	svc := &PaymentService{entClient: client}

	err := svc.AdminDeleteOrder(ctx, order.ID, false, "admin:42")
	require.NoError(t, err)

	_, err = client.PaymentOrder.Get(ctx, order.ID)
	require.True(t, dbent.IsNotFound(err))

	logs, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "ORDER_DELETED", logs[0].Action)
	require.Equal(t, "admin:42", logs[0].Operator)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].Detail), &detail))
	require.Equal(t, false, detail["force"])
	require.Equal(t, OrderStatusFailed, detail["status"])
	require.Equal(t, order.OutTradeNo, detail["out_trade_no"])
}

func TestAdminDeleteOrderRequiresForceForCompletedOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createAdminDeleteOrderTestOrder(t, ctx, client, OrderStatusCompleted)
	svc := &PaymentService{entClient: client}

	err := svc.AdminDeleteOrder(ctx, order.ID, false, "admin:42")
	require.Error(t, err)
	require.Equal(t, "ORDER_DELETE_REQUIRES_FORCE", infraerrors.Reason(err))

	_, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	count, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10))).
		Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestAdminDeleteOrderForceDeletesCompletedOrder(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createAdminDeleteOrderTestOrder(t, ctx, client, OrderStatusCompleted)
	svc := &PaymentService{entClient: client}

	err := svc.AdminDeleteOrder(ctx, order.ID, true, "")
	require.NoError(t, err)

	_, err = client.PaymentOrder.Get(ctx, order.ID)
	require.True(t, dbent.IsNotFound(err))
	logs, err := client.PaymentAuditLog.Query().
		Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10))).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "admin", logs[0].Operator)

	var detail map[string]any
	require.NoError(t, json.Unmarshal([]byte(logs[0].Detail), &detail))
	require.Equal(t, true, detail["force"])
	require.Equal(t, OrderStatusCompleted, detail["status"])
}

func createAdminDeleteOrderTestOrder(t *testing.T, ctx context.Context, client *dbent.Client, status string) *dbent.PaymentOrder {
	t.Helper()

	user, err := client.User.Create().
		SetEmail("delete-order@example.com").
		SetPasswordHash("hash").
		SetUsername("delete-order-user").
		Save(ctx)
	require.NoError(t, err)

	builder := client.PaymentOrder.Create().
		SetUserID(user.ID).
		SetUserEmail(user.Email).
		SetUserName(user.Username).
		SetAmount(88).
		SetPayAmount(88).
		SetFeeRate(0).
		SetRechargeCode("DELETE-ORDER").
		SetOutTradeNo("sub2_delete_order_" + strconv.FormatInt(time.Now().UnixNano(), 10)).
		SetPaymentType(payment.TypeAlipay).
		SetPaymentTradeNo("trade-delete-order").
		SetOrderType(payment.OrderTypeBalance).
		SetStatus(status).
		SetExpiresAt(time.Now().Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("api.example.com")
	if status == OrderStatusCompleted {
		now := time.Now()
		builder.SetPaidAt(now).SetCompletedAt(now)
	}

	order, err := builder.Save(ctx)
	require.NoError(t, err)
	return order
}
