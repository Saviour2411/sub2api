package service

import (
	"fmt"
	"net/http"
)

// UpstreamOutcomeError 表示已经形成明确上游结果、但不应触发重试或切号的错误。
// StatusCode 仅用于账号连续错误结算，客户端响应仍由原协议处理逻辑负责。
type UpstreamOutcomeError struct {
	StatusCode       int
	ResponseBody     []byte
	ClientDisconnect bool
	Cause            error
}

func (e *UpstreamOutcomeError) Error() string {
	if e == nil {
		return "上游结果错误"
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("上游结果错误: %d", e.StatusCode)
}

func (e *UpstreamOutcomeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewUpstreamOutcomeError 保留明确的上游状态，同时避免把普通最终错误误当成 failover。
func NewUpstreamOutcomeError(statusCode int, responseBody []byte, cause error) *UpstreamOutcomeError {
	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusBadGateway
	}
	return &UpstreamOutcomeError{
		StatusCode:   statusCode,
		ResponseBody: append([]byte(nil), responseBody...),
		Cause:        cause,
	}
}
