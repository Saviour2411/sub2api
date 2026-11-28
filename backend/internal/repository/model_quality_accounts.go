package repository

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *modelQualityRepository) QualityV2AccountHits(ctx context.Context, viewerID int64, runIDs []string) (map[string][]service.ModelQualityAccountHit, bool, error) {
	if viewerID != service.ModelQualityAccountViewerID {
		return nil, false, nil
	}
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND role='admin' AND status='active' AND deleted_at IS NULL AND split_part(email,'@',1)='saviour2411')`, viewerID).Scan(&allowed)
	if err != nil || !allowed {
		return nil, false, err
	}
	ids := []string{}
	for _, id := range runIDs {
		if strings.HasPrefix(id, "v2-") {
			ids = append(ids, strings.TrimPrefix(id, "v2-"))
		}
	}
	out := map[string][]service.ModelQualityAccountHit{}
	if len(ids) == 0 {
		return out, true, nil
	}
	// 优先使用真实用量的最终账号，失败请求使用最后一条错误记录。
	// 请求 ID 必须同时匹配冻结配置的密钥、用户及分组，不能按模型或时间猜测。
	rows, err := r.db.QueryContext(ctx, qualityAccountHitsSQL, pq.Array(ids))
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		var hit service.ModelQualityAccountHit
		if err := rows.Scan(&id, &hit.TestID, &hit.Phase, &hit.JudgmentIndex, &hit.AccountID, &hit.AccountName, &hit.Source); err != nil {
			return nil, false, err
		}
		out["v2-"+id] = append(out["v2-"+id], hit)
	}
	return out, true, rows.Err()
}

const qualityAccountHitsSQL = `SELECT t.run_id,t.id,p.phase,p.judgment_index,hit.account_id,COALESCE(a.name,'已删除账号'),hit.source
FROM model_quality_v2_tasks t
CROSS JOIN LATERAL (
 SELECT 'generation'::text AS phase,0::bigint AS judgment_index,t.result->'generation' AS result,t.specification->'target' AS call
 UNION ALL SELECT 'judgment',0,t.result->'judgment',t.specification->'question'->'rule'->'judge'
 UNION ALL SELECT 'previous_judgment',h.ordinality,h.value,t.specification->'question'->'rule'->'judge'
 FROM jsonb_array_elements(COALESCE(NULLIF(t.result->'previous_judgments','null'::jsonb),'[]'::jsonb)) WITH ORDINALITY h
) p
CROSS JOIN LATERAL (
 SELECT account_id,source FROM (
  SELECT u.account_id,'usage'::text AS source,0 AS priority,u.created_at,u.id
  FROM usage_logs u
  WHERE u.request_id='client:'||(p.result->>'request_id') AND u.api_key_id=(p.call->>'key_id')::bigint
    AND u.user_id=(p.call->>'owner_id')::bigint AND u.group_id=(p.call->>'group_id')::bigint
    AND u.account_id>0 AND u.created_at>=t.started_at-INTERVAL '1 minute'
  UNION ALL
  SELECT e.account_id,'error',1,e.created_at,e.id FROM ops_error_logs e
  WHERE e.client_request_id=p.result->>'request_id' AND e.api_key_id=(p.call->>'key_id')::bigint
    AND e.user_id=(p.call->>'owner_id')::bigint AND e.group_id=(p.call->>'group_id')::bigint
    AND e.account_id>0 AND e.created_at>=t.started_at-INTERVAL '1 minute'
 ) matched ORDER BY priority,created_at DESC,id DESC LIMIT 1
) hit
LEFT JOIN accounts a ON a.id=hit.account_id
WHERE t.run_id=ANY($1) AND t.started_at>NOW()-INTERVAL '24 hours'
  AND COALESCE(p.result->>'request_id','')<>''
ORDER BY t.run_id,t.started_at,t.id,p.phase,p.judgment_index`
