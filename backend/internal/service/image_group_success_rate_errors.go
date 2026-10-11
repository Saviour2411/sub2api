package service

import (
	"errors"
	"net/http"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// IsImageGroupExcludedError 判断不应进入生图成功率分母的业务拒绝。
// 仅改变成功率口径，不改变错误响应、计费、重试或运维错误日志。
func IsImageGroupExcludedError(status int, code, errorType, message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	// 部分上游将超时等服务故障也包装成 400/invalid_request_error。
	for _, pattern := range []string{
		"timeout", "timed out", "deadline exceeded", "overloaded",
		"temporarily unavailable", "service unavailable", "connection reset",
		"connection refused", "stream was interrupted",
		"超时", "过载", "服务不可用", "流中断", "流被中断",
	} {
		if strings.Contains(message, pattern) {
			return false
		}
	}

	// 按 Image 分组约定，403 为余额或业务权限拒绝；使用包装前的上游状态码。
	if status == http.StatusForbidden {
		return true
	}
	for _, value := range []string{code, errorType} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "insufficient_balance", "insufficient_user_balance", "insufficient_quota",
			"api_key_quota_exhausted", "usage_limit_exceeded", "billing_hard_limit_reached",
			"batch_image_insufficient_balance",
			"content_policy_violation", "content_filter", "content_filter_error",
			"moderation_blocked", "safety_blocked", "safety", "image_safety",
			"prohibited_content", "image_prohibited_content", "recitation",
			"invalid_argument", "invalid_parameter", "invalid_value",
			"missing_required_parameter", "unsupported_parameter",
			"invalid_image", "invalid_image_format", "invalid_image_url",
			"image_parse_error", "invalid_prompt", "image_too_large":
			return true
		}
	}
	for _, pattern := range []string{
		"insufficient balance", "insufficient account balance", "insufficient user balance",
		"余额不足", "余额已用完", "余额已耗尽", "额度已用完", "额度已耗尽",
		"content policy", "content_policy", "safety filter", "safety system",
		"policy violation", "moderation blocked", "防护限制", "内容政策", "内容审核拒绝",
		"missing required parameter", "invalid parameter", "invalid argument",
		"invalid image", "unsupported image format", "image is too large",
		"prompt is required", "prompt must not be empty",
		"one of \"input\" or \"previous_response_id\"",
		"参数无效", "参数错误", "参数不合法", "缺少必填", "缺少必要参数",
		"缺少有效的 prompt", "图片尺寸无效", "不支持该分辨率与比例组合",
		"不接受参考图或蒙版", "输入图片无法识别或解码", "图片格式不支持",
	} {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

func isExcludedBatchImageError(err error) bool {
	if err == nil {
		return false
	}
	var geminiError *GeminiAPIError
	if errors.As(err, &geminiError) {
		return IsImageGroupExcludedError(geminiError.StatusCode, geminiError.Code, "", geminiError.Message)
	}
	var vertexError *VertexAPIError
	if errors.As(err, &vertexError) {
		return IsImageGroupExcludedError(vertexError.StatusCode, vertexError.Code, "", vertexError.Message)
	}
	return IsImageGroupExcludedError(infraerrors.Code(err), infraerrors.Reason(err), "", err.Error())
}
