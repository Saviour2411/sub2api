package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelQualityAccountsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		viewer  int64
		allowed bool
		err     error
	}{
		{"普通用户", 82, false, nil}, {"其他管理员", 83, false, nil}, {"管理员已撤权", 1, false, nil}, {"数据库异常", 1, false, errors.New("数据库异常")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			if tc.viewer == 1 {
				query := mock.ExpectQuery("SELECT EXISTS.*role='admin'.*status='active'.*deleted_at IS NULL.*split_part").WithArgs(int64(1))
				if tc.err != nil {
					query.WillReturnError(tc.err)
				} else {
					query.WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(tc.allowed))
				}
			}
			hits, allowed, err := (&modelQualityRepository{db: db}).QualityV2AccountHits(context.Background(), tc.viewer, []string{"v2-run"})
			require.Equal(t, tc.err, err)
			require.False(t, allowed)
			require.Nil(t, hits)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestModelQualityAccountsPostgresCorrelation(t *testing.T) {
	db := qualityV2Postgres(t, true)
	_, err := db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,email TEXT,username TEXT,role TEXT,status TEXT,deleted_at TIMESTAMPTZ);
CREATE TABLE accounts(id BIGINT PRIMARY KEY,name TEXT);
CREATE TABLE usage_logs(id BIGSERIAL PRIMARY KEY,request_id TEXT,api_key_id BIGINT,user_id BIGINT,group_id BIGINT,account_id BIGINT,created_at TIMESTAMPTZ DEFAULT NOW());
CREATE TABLE ops_error_logs(id BIGSERIAL PRIMARY KEY,client_request_id TEXT,api_key_id BIGINT,user_id BIGINT,group_id BIGINT,account_id BIGINT,created_at TIMESTAMPTZ DEFAULT NOW());
INSERT INTO users VALUES(1,'saviour2411@example.test','','admin','active',NULL),(82,'user@example.test','saviour2411','user','active',NULL),(83,'saviour2411@other.test','saviour2411','admin','active',NULL);
INSERT INTO accounts VALUES(101,'生成命中'),(102,'本次判题'),(103,'历史失败'),(999,'无关私有账号');`)
	require.NoError(t, err)
	r := &modelQualityRepository{db: db}
	ctx := context.Background()
	config := qualityV2ConfigForTest(t, r)
	require.NoError(t, r.StartQualityV2Round(ctx, config, config.Groups[0], "分组", true))
	var runID, taskID string
	require.NoError(t, db.QueryRow(`SELECT run_id,id FROM model_quality_v2_tasks WHERE question_id='art'`).Scan(&runID, &taskID))
	result := service.ModelQualityV2Test{ID: taskID, Generation: service.ModelQualityPhase{RequestID: "gen"}, Judgment: &service.ModelQualityPhase{RequestID: "judge"}, PreviousJudgments: []service.ModelQualityPhase{{RequestID: "previous"}, {RequestID: "wrong-owner"}, {RequestID: "wrong-key"}, {RequestID: "wrong-group"}, {}}}
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE model_quality_v2_tasks SET result=$2 WHERE id=$1`, taskID, raw)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs(request_id,api_key_id,user_id,group_id,account_id) VALUES
('client:gen',3,1,7,101),('client:judge',3,1,7,102),('client:wrong-owner',3,82,7,999),('client:wrong-key',4,1,7,999),('client:wrong-group',3,1,8,999);
INSERT INTO ops_error_logs(client_request_id,api_key_id,user_id,group_id,account_id) VALUES
('gen',3,1,7,999),('previous',3,1,7,999),('previous',3,1,7,103),('judge',4,1,7,999),('wrong-owner',3,82,7,999),('wrong-key',4,1,7,999),('wrong-group',3,1,8,999);`)
	require.NoError(t, err)
	hits, allowed, err := r.QualityV2AccountHits(ctx, 1, []string{"v2-" + runID, "v1-123"})
	require.NoError(t, err)
	require.True(t, allowed)
	require.ElementsMatch(t, []service.ModelQualityAccountHit{
		{TestID: taskID, Phase: "generation", AccountID: 101, AccountName: "生成命中", Source: "usage"},
		{TestID: taskID, Phase: "judgment", AccountID: 102, AccountName: "本次判题", Source: "usage"},
		{TestID: taskID, Phase: "previous_judgment", JudgmentIndex: 1, AccountID: 103, AccountName: "历史失败", Source: "error"},
	}, hits["v2-"+runID])
	require.Len(t, hits, 1)
	for _, viewer := range []int64{82, 83} {
		hits, allowed, err = r.QualityV2AccountHits(ctx, viewer, []string{"v2-" + runID})
		require.NoError(t, err)
		require.False(t, allowed)
		require.Nil(t, hits)
	}
	for _, update := range []string{"role='user'", "status='disabled'", "deleted_at=NOW()", "email='someone@example.test'"} {
		_, err = db.Exec(`UPDATE users SET role='admin',status='active',deleted_at=NULL,email='saviour2411@example.test' WHERE id=1`)
		require.NoError(t, err)
		_, err = db.Exec(`UPDATE users SET ` + update + ` WHERE id=1`)
		require.NoError(t, err)
		hits, allowed, err = r.QualityV2AccountHits(ctx, 1, []string{"v2-" + runID})
		require.NoError(t, err)
		require.False(t, allowed)
		require.Nil(t, hits)
	}
}
