package buildmeta

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedUpstreamSync(t *testing.T) {
	info, err := EmbeddedUpstreamSync()
	require.NoError(t, err)
	require.Equal(t, UpstreamRepository, info.Repository)
	require.NotEmpty(t, info.Version)
	require.Len(t, info.Commit, 40)
}

func TestParseUpstreamSyncRejectsInvalidMetadata(t *testing.T) {
	baseline, err := EmbeddedUpstreamSync()
	require.NoError(t, err)
	for name, change := range map[string]func(*UpstreamSync){
		"仓库错误": func(v *UpstreamSync) { v.Repository = "another/repository" },
		"版本缺失": func(v *UpstreamSync) { v.Version = "" },
		"版本无效": func(v *UpstreamSync) { v.Version = "latest" },
		"预发布":  func(v *UpstreamSync) { v.Version = "0.3.0-rc.1" },
		"提交缺失": func(v *UpstreamSync) { v.Commit = "" },
		"提交缩写": func(v *UpstreamSync) { v.Commit = "7c700729c" },
		"时间无效": func(v *UpstreamSync) { v.SyncedAt = "2026-09-20" },
	} {
		t.Run(name, func(t *testing.T) {
			info := baseline
			change(&info)
			data, marshalErr := json.Marshal(info)
			require.NoError(t, marshalErr)
			parsed, parseErr := ParseUpstreamSync(data)
			require.Error(t, parseErr)
			require.Empty(t, parsed)
		})
	}
	_, err = ParseUpstreamSync([]byte("{"))
	require.Error(t, err)
}
