package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/buildmeta"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SystemHandler 仅提供当前进程的只读版本信息。
type SystemHandler struct {
	version  string
	upstream buildmeta.UpstreamSync
	checker  *service.UpstreamVersionService
}

func NewSystemHandler(version string, upstream buildmeta.UpstreamSync, checker *service.UpstreamVersionService) *SystemHandler {
	return &SystemHandler{version: version, upstream: upstream, checker: checker}
}

// GetVersion 直接读取构建版本，不检查或下载上游发布。
func (h *SystemHandler) GetVersion(c *gin.Context) {
	response.Success(c, gin.H{
		"version":            h.version,
		"upstream_repo":      h.upstream.Repository,
		"upstream_version":   h.upstream.Version,
		"upstream_commit":    h.upstream.Commit,
		"upstream_synced_at": h.upstream.SyncedAt,
	})
}

// GetUpstreamVersion 只查询发布信息，不下载文件或修改应用。
func (h *SystemHandler) GetUpstreamVersion(c *gin.Context) {
	if h.checker == nil {
		response.Success(c, service.UpstreamVersionInfo{Status: "unavailable"})
		return
	}
	response.Success(c, h.checker.GetLatest(c.Request.Context()))
}
