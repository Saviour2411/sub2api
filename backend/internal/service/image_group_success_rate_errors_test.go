package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageGroupExcludedError业务拒绝与真实故障(t *testing.T) {
	tests := []struct {
		name, code, errorType, message string
		status                         int
		excluded                       bool
	}{
		{name: "余额403", status: 403, excluded: true},
		{name: "余额业务码", status: 402, code: " INSUFFICIENT_BALANCE ", excluded: true},
		{name: "余额中文提示", status: 400, message: "用户余额不足", excluded: true},
		{name: "审核业务码", status: 400, code: "content_policy_violation", excluded: true},
		{name: "审核中文提示", status: 400, message: "生成的图片可能违反了关于与第三方内容相似性的防护限制", excluded: true},
		{name: "审核英文提示", status: 400, message: "Rejected by the safety system", excluded: true},
		{name: "批量安全拒绝", code: "SAFETY_BLOCKED", excluded: true},
		{name: "批量无效参数", code: "INVALID_ARGUMENT", excluded: true},
		{name: "缺少参数", status: 400, message: "Missing required parameter: 'input'.", excluded: true},
		{name: "图片尺寸", status: 400, message: "图片尺寸无效：最大支持 8K 分辨率", excluded: true},
		{name: "错误生图模式", status: 400, message: "当前生图模式不接受参考图或蒙版，请使用图片编辑接口", excluded: true},
		{name: "图片解码失败", status: 400, message: "输入图片无法识别或解码，请重新导出", excluded: true},
		{name: "空提示词", status: 400, message: "缺少有效的 prompt，请提供非空的图片描述", excluded: true},
		{name: "400超时仍失败", status: 400, errorType: "invalid_request_error", message: "生图任务超时"},
		{name: "434超时仍失败", status: 434, message: "生图超时，请重新生成"},
		{name: "批量错误码不能遮掩超时", code: "INVALID_ARGUMENT", message: "upstream request timed out"},
		{name: "429上游限流仍失败", status: 429, code: "PROVIDER_RATE_LIMITED", message: "resource exhausted"},
		{name: "401上游认证仍失败", status: 401, message: "Upstream authentication failed"},
		{name: "500仍失败", status: 500, message: "internal server error"},
		{name: "未知400仍失败", status: 400, errorType: "invalid_request_error", message: "图片生成没有返回可用的图片文件"},
		{name: "缺少目标原因不明仍失败", status: 400, message: "请上传需要编辑的原始图片"},
		{name: "批量未知仍失败", code: "PROVIDER_ITEM_FAILED", message: "unknown failure"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.excluded, IsImageGroupExcludedError(tt.status, tt.code, tt.errorType, tt.message))
		})
	}
	require.False(t, IsImageGroupExcludedError(http.StatusBadGateway, "", "upstream_error", "Upstream response stream was interrupted"))
}
