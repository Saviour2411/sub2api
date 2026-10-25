//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestScopedAdminRecoveryKeepsFailureProtectionUntilCommit(t *testing.T) {
	for _, mode := range []string{"commit", "rollback", "commit_failure", "redis_failure", "concurrent_incident"} {
		t.Run(mode, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			incident := func(id string) *Account {
				return &Account{ID: 31, Platform: PlatformOpenAI, Status: StatusError, Extra: map[string]any{
					accountFailureStrategyUnscheduledKey: map[string]any{
						accountFailureStrategyUnscheduledIncidentIDKey: id,
						accountFailureStrategyUnscheduledSourceKey:     "upstream_error",
					},
				}}
			}
			repo := &accountRepoStubForClearAccountError{account: incident("old")}
			if mode == "concurrent_incident" {
				repo.accountAfterFailureClear = incident("new")
			}
			streak := &accountFailureStreakCacheStubForAdminReset{}
			if mode == "redis_failure" {
				streak.errorSource = AccountFailureStreakSourceUpstreamError
			}
			blocker := &runtimeBlockRecorder{}
			svc := &adminServiceImpl{accountRepo: repo, accountFailureStreakCache: streak, runtimeBlocker: blocker}
			mock.ExpectBegin()
			tx, err := client.Tx(context.Background())
			require.NoError(t, err)
			updated, err := svc.ClearAccountError(dbent.NewTxContext(context.Background(), tx), 31)
			require.NoError(t, err)
			require.Empty(t, streak.calls, "提交前不得清理 Redis")
			require.Empty(t, blocker.clearedIDs, "提交前不得放行调度")
			require.Zero(t, repo.clearFailureStateCalls, "提交前保留持久化事故")
			if mode == "rollback" {
				mock.ExpectRollback()
				require.NoError(t, tx.Rollback())
			} else if mode == "commit_failure" {
				mock.ExpectCommit().WillReturnError(errors.New("提交失败"))
				require.ErrorContains(t, tx.Commit(), "提交失败")
			} else {
				mock.ExpectCommit()
				err = tx.Commit()
				if mode == "redis_failure" {
					require.ErrorContains(t, err, "upstream_error")
					require.True(t, repo.account.HasFailureStrategyUnscheduledMarker())
					require.Empty(t, blocker.clearedIDs)
					require.Zero(t, repo.clearFailureStateCalls)
				} else {
					require.NoError(t, err)
					require.Equal(t, []int64{31}, blocker.clearedIDs)
					require.Equal(t, []string{"old"}, repo.clearFailureIncidentIDs)
					if mode == "concurrent_incident" {
						id, _ := updated.FailureStrategyUnscheduledIncident()
						require.Equal(t, "new", id)
						require.Len(t, blocker.accounts, 1)
					} else {
						require.False(t, updated.HasFailureStrategyUnscheduledMarker())
						require.True(t, updated.Schedulable)
					}
				}
				require.Len(t, streak.calls, 2)
			}
			if mode == "rollback" || mode == "commit_failure" {
				require.Empty(t, streak.calls)
				require.Empty(t, blocker.clearedIDs)
				require.True(t, updated.HasFailureStrategyUnscheduledMarker())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
