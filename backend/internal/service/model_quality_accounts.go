package service

import "context"

// 绑定已核验的生产管理员 ID，不能用可重复、可修改的昵称授予私有数据权限。
const ModelQualityAccountViewerID int64 = 1

type ModelQualityAccountHit struct {
	TestID        string `json:"test_id"`
	Phase         string `json:"phase"`
	JudgmentIndex int    `json:"judgment_index,omitempty"`
	AccountID     int64  `json:"account_id"`
	AccountName   string `json:"account_name"`
	Source        string `json:"source"`
}

// 私有扩展与公共结果类型分离，普通响应无法序列化出账号字段。
type ModelQualityPrivateRun struct {
	ModelQualityV2Run
	AccountHits []ModelQualityAccountHit `json:"account_hits"`
}

type ModelQualityAccountRepository interface {
	QualityV2AccountHits(context.Context, int64, []string) (map[string][]ModelQualityAccountHit, bool, error)
}

func (s *ModelQualityService) PrivateQualityV2Runs(ctx context.Context, viewerID int64, runs []ModelQualityV2Run) ([]ModelQualityPrivateRun, error) {
	if viewerID != ModelQualityAccountViewerID {
		return nil, nil
	}
	repo, ok := s.repo.(ModelQualityAccountRepository)
	if !ok {
		return nil, nil
	}
	ids := make([]string, len(runs))
	for i := range runs {
		ids[i] = runs[i].ID
	}
	hits, allowed, err := repo.QualityV2AccountHits(ctx, viewerID, ids)
	if err != nil || !allowed {
		return nil, err
	}
	out := make([]ModelQualityPrivateRun, len(runs))
	for i, run := range runs {
		out[i] = ModelQualityPrivateRun{ModelQualityV2Run: run, AccountHits: hits[run.ID]}
		if out[i].AccountHits == nil {
			out[i].AccountHits = []ModelQualityAccountHit{}
		}
	}
	return out, nil
}
