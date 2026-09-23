package buildmeta

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const UpstreamRepository = "Wei-Shaw/sub2api"

//go:embed upstream-sync.json
var upstreamSyncJSON []byte

// UpstreamSync 固定构建时已集成的上游来源，不依赖运行目录或 Git 历史。
type UpstreamSync struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	SyncedAt   string `json:"synced_at"`
}

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func ParseUpstreamSync(data []byte) (UpstreamSync, error) {
	var info UpstreamSync
	if err := json.Unmarshal(data, &info); err != nil {
		return UpstreamSync{}, fmt.Errorf("上游同步元数据格式无效")
	}
	info.Version = strings.TrimPrefix(strings.TrimSpace(info.Version), "v")
	if info.Repository != UpstreamRepository || !semver.IsValid("v"+info.Version) ||
		semver.Prerelease("v"+info.Version) != "" || !commitPattern.MatchString(info.Commit) {
		return UpstreamSync{}, fmt.Errorf("上游同步元数据字段无效")
	}
	if _, err := time.Parse(time.RFC3339, info.SyncedAt); err != nil {
		return UpstreamSync{}, fmt.Errorf("上游同步时间无效")
	}
	return info, nil
}

func EmbeddedUpstreamSync() (UpstreamSync, error) {
	return ParseUpstreamSync(upstreamSyncJSON)
}
