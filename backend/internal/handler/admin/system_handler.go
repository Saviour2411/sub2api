package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// SystemHandler 仅提供当前进程的只读版本信息。
type SystemHandler struct {
	version string
}

func NewSystemHandler(version string) *SystemHandler {
	return &SystemHandler{version: version}
}

// GetVersion 直接读取构建版本，不检查或下载上游发布。
func (h *SystemHandler) GetVersion(c *gin.Context) {
	response.Success(c, gin.H{"version": h.version})
}
