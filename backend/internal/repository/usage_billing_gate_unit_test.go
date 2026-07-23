//go:build unit

package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUserBillingGate_SerializesSameUser(t *testing.T) {
	var gate userBillingGate
	releaseFirst, err := gate.acquire(context.Background(), 42)
	require.NoError(t, err)

	acquired := make(chan func(), 1)
	go func() {
		release, acquireErr := gate.acquire(context.Background(), 42)
		if acquireErr == nil {
			acquired <- release
		}
	}()

	select {
	case release := <-acquired:
		release()
		t.Fatal("同一用户的第二个扣费任务不应并行进入")
	case <-time.After(50 * time.Millisecond):
	}

	releaseFirst()
	select {
	case release := <-acquired:
		release()
	case <-time.After(time.Second):
		t.Fatal("首个任务释放后，等待任务未能进入")
	}
	require.Eventually(t, func() bool { return billingGateEntryCount(&gate) == 0 }, time.Second, 10*time.Millisecond)
}

func TestUserBillingGate_AllowsDifferentUsersConcurrently(t *testing.T) {
	var gate userBillingGate
	releaseFirst, err := gate.acquire(context.Background(), 42)
	require.NoError(t, err)
	defer releaseFirst()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	releaseSecond, err := gate.acquire(ctx, 43)
	require.NoError(t, err)
	releaseSecond()
}

func TestUserBillingGate_CancelledWaitCleansEntry(t *testing.T) {
	var gate userBillingGate
	release, err := gate.acquire(context.Background(), 42)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = gate.acquire(ctx, 42)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	release()
	require.Equal(t, 0, billingGateEntryCount(&gate))
}

func TestUsageBillingRepositoryApply_GateTimeoutDoesNotBeginTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &usageBillingRepository{db: db}
	release, err := repo.balanceGates.acquire(context.Background(), 42)
	require.NoError(t, err)
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err = repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:   "gate-timeout",
		APIKeyID:    7,
		UserID:      42,
		BalanceCost: 1,
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_ReleasesGateOnTransactionExit(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "开启事务失败",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("begin failed"))
			},
			wantErr: true,
		},
		{
			name: "事务内查询失败并回滚",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)")).
					WillReturnError(errors.New("claim failed"))
				mock.ExpectRollback()
			},
			wantErr: true,
		},
		{
			name: "提交成功",
			setup: func(mock sqlmock.Sqlmock) {
				expectUsageBillingBalanceApply(mock, nil)
			},
		},
		{
			name: "提交失败",
			setup: func(mock sqlmock.Sqlmock) {
				expectUsageBillingBalanceApply(mock, errors.New("commit failed"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			tt.setup(mock)

			repo := &usageBillingRepository{db: db}
			_, err = repo.Apply(context.Background(), &service.UsageBillingCommand{
				RequestID:   "transaction-exit",
				APIKeyID:    7,
				UserID:      42,
				BalanceCost: 1,
			})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 0, billingGateEntryCount(&repo.balanceGates))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func expectUsageBillingBalanceApply(mock sqlmock.Sqlmock, commitErr error) {
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)")).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT request_fingerprint FROM usage_billing_dedup_archive")).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(9.0))
	if commitErr != nil {
		mock.ExpectCommit().WillReturnError(commitErr)
		return
	}
	mock.ExpectCommit()
}

func billingGateEntryCount(gate *userBillingGate) int {
	gate.mu.Lock()
	defer gate.mu.Unlock()
	return len(gate.entries)
}
