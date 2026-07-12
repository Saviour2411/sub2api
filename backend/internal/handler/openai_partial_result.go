package handler

import (
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 部分图片只能容忍本地收尾错误；上游已经明确返回失败结果时仍须按失败结算。
func canAcceptOpenAIPartialImageResult(result *service.OpenAIForwardResult, err error) bool {
	if result == nil || result.ImageCount <= 0 || err == nil {
		return false
	}
	var outcomeErr *service.UpstreamOutcomeError
	return !errors.As(err, &outcomeErr)
}
