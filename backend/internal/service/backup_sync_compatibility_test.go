//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/lifecycle"
	"github.com/stretchr/testify/require"
)

func TestBackupRecordLockRetainsLegacyDatabaseKey(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "显式数据库", true: "生命周期共享数据库"}[shared], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			db.SetMaxOpenConns(2)
			cache := &fakeLeaderLockCache{}
			svc := newTestBackupService(newMockSettingRepo(), &mockDumper{}, newMockObjectStore())
			if shared {
				previous := lifecycle.Process.SharedDB()
				lifecycle.Process.SetSharedDB(db)
				defer lifecycle.Process.SetSharedDB(previous)
				svc.SetLeaderLock(cache, nil)
			} else {
				svc.SetLeaderLock(cache, db)
			}
			key := hashAdvisoryLockID("backup:records")
			mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(key).
				WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(key).
				WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, svc.saveRecord(context.Background(), archiveRecord("compat", "2026-09-24T00:00:00Z")))
			require.Empty(t, cache.heldBy("backup:records:writer"))
			require.NoError(t, mock.ExpectationsWereMet())
			// 数据库异常必须失败关闭，不能绕到另一个锁后端继续写入。
			mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(key).
				WillReturnError(errors.New("隔离测试锁错误"))
			require.ErrorContains(t, svc.saveRecord(context.Background(), archiveRecord("rejected", "2026-09-24T00:00:00Z")), "隔离测试锁错误")
			require.Empty(t, cache.heldBy("backup:records:writer"))
			_, err = svc.GetBackupRecord(context.Background(), "rejected")
			require.ErrorIs(t, err, ErrBackupNotFound)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBackupRecoveryExpiredLiveAndUnknownOwnersKeepProtection(t *testing.T) {
	for _, owner := range []string{"", "live-peer", "self"} {
		t.Run("所有者-"+owner, func(t *testing.T) {
			repo := newMockSettingRepo()
			seedS3Config(t, repo)
			store := newMockObjectStore()
			svc := newTestBackupService(repo, &mockDumper{}, store)
			svc.lockCache = deadBackupOwner{dead: false}
			if owner == "self" {
				owner = svc.instanceID
			}
			old := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
			for _, id := range []string{"backup", "restore"} {
				record := archiveRecord(id, old)
				record.OwnerInstance = owner
				record.RestoreOwnerInstance = owner
				record.S3Key = id
				store.objects[id] = []byte("必须保留")
				if id == "backup" {
					record.Status = "running"
				} else {
					record.RestoreStatus = "running"
					record.RestoreStartedAt = old
				}
				require.NoError(t, svc.saveRecord(context.Background(), record))
			}
			svc.recoverStaleRecords()
			backup, err := svc.GetBackupRecord(context.Background(), "backup")
			require.NoError(t, err)
			require.Equal(t, "running", backup.Status)
			restored, err := svc.GetBackupRecord(context.Background(), "restore")
			require.NoError(t, err)
			require.Equal(t, "running", restored.RestoreStatus)
			require.Equal(t, old, restored.RestoreStartedAt)
			require.Empty(t, store.deletedKeys)
			require.ErrorIs(t, svc.DeleteBackup(context.Background(), "restore"), ErrRestoreInProgress)
		})
	}
}
