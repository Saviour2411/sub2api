//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestScopedAdminSchedulingRollbackPreservesIncidentAndProbe(t *testing.T) {
	for _, action := range []string{"manual_pause", "clear_incident"} {
		t.Run(action, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client := testEntClient(t)
			repo := newAccountRepositoryWithSQL(client, integrationDB, nil)
			account, err := client.Account.Create().SetName(fmt.Sprintf("scoped-rollback-%s-%d", action, time.Now().UnixNano())).
				SetPlatform(service.PlatformOpenAI).SetType(service.AccountTypeAPIKey).
				SetCredentials(map[string]any{"api_key": "local-test"}).Save(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduled_test_plans WHERE account_id=$1", account.ID)
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM scheduler_outbox WHERE account_id=$1", account.ID)
				_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id=$1", account.ID)
			})
			created, err := repo.PersistFailureSchedulingState(ctx, account.ID, map[string]any{"incident_id": "incident-old", "source": "upstream_error"}, time.Now())
			require.NoError(t, err)
			require.True(t, created)
			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback() }()
			txctx := dbent.NewTxContext(ctx, tx)
			_, err = tx.Client().ExecContext(txctx, "SELECT id FROM accounts WHERE id=$1 FOR UPDATE", account.ID)
			require.NoError(t, err)
			if action == "manual_pause" {
				require.NoError(t, repo.SetAdminSchedulable(txctx, account.ID, false))
			} else {
				cleared, clearErr := repo.ClearFailureSchedulingState(txctx, account.ID, "incident-old")
				require.NoError(t, clearErr)
				require.True(t, cleared)
			}
			require.NoError(t, tx.Rollback())
			fresh, err := repo.GetByID(ctx, account.ID)
			require.NoError(t, err)
			require.False(t, fresh.Schedulable)
			require.False(t, fresh.IsManuallySchedulingPaused())
			require.True(t, fresh.HasFailureStrategyUnscheduledMarker())
			plans, err := (&scheduledTestPlanRepository{db: integrationDB}).ListByAccountID(ctx, account.ID)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			require.True(t, plans[0].Enabled)
			require.NotNil(t, plans[0].NextRunAt)
		})
	}
}
