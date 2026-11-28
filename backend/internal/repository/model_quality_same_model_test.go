package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelQualitySameModelSnapshots(t *testing.T) {
	db := qualityV2Postgres(t, true)
	repo := &modelQualityRepository{db: db}
	ctx := context.Background()
	config := qualityV2ConfigForTest(t, repo)
	config.Groups[0].Targets[1].QuestionIDs = []string{"art"}
	config.Groups[0].Targets[1].Model = "claude-test"
	config.Groups[0].Targets[1].Protocol = "messages"
	config.Groups[0].Targets[1].TimeoutSeconds = 1200
	require.NoError(t, repo.StartQualityV2Round(ctx, config, config.Groups[0], "分组", true))
	rows, err := db.Query("SELECT specification,result FROM model_quality_v2_tasks WHERE question_id='art'")
	require.NoError(t, err)
	defer rows.Close()
	count := 0
	for rows.Next() {
		var specRaw, resultRaw []byte
		require.NoError(t, rows.Scan(&specRaw, &resultRaw))
		var spec service.ModelQualitySpecification
		var result service.ModelQualityV2Test
		require.NoError(t, json.Unmarshal(specRaw, &spec))
		require.NoError(t, json.Unmarshal(resultRaw, &result))
		require.Equal(t, spec.Target.ModelQualityCall, *spec.Question.Rule.Judge)
		require.Equal(t, spec.Target.Model, result.JudgeModel)
		require.Equal(t, spec.Target.Protocol, result.JudgeProtocol)
		count++
	}
	require.NoError(t, rows.Err())
	require.Equal(t, 2, count)
}
