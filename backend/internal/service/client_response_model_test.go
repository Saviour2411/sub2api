//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type delayedResponseModelSettings struct {
	SettingRepository
	started chan struct{}
	release chan struct{}
}

func (r *delayedResponseModelSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	close(r.started)
	<-r.release
	return map[string]string{SettingKeyGatewayResponseModelRewriteEnabled: "true", SettingKeyGatewayResponseModelRewriteMode: ResponseModelModeRequest}, nil
}

func TestClientResponseModelLateRefreshPreservesAdminSave(t *testing.T) {
	repo := &delayedResponseModelSettings{started: make(chan struct{}), release: make(chan struct{})}
	settings := NewSettingService(repo, nil)
	done := make(chan error, 1)
	go func() {
		_, err := settings.GetGatewaySettings(context.Background())
		done <- err
	}()
	<-repo.started
	saved := DefaultGatewaySettings()
	saved.ResponseModelRewriteEnabled = true
	saved.ResponseModelRewriteMode = ResponseModelModeEmpty
	settings.storeGatewaySettingsCache(saved, time.Minute)
	close(repo.release)
	require.NoError(t, <-done)
	require.Equal(t, ResponseModelModeEmpty, settings.ClientResponseModelMode())
}

func TestClientResponseModelGeminiArrayAndEscapedNames(t *testing.T) {
	input := []byte(` [ {"modelVersion":"a","parts":[{"modelVersion":"inside"}]}, {"response":{"modelVersion":"b"}} ] `)
	snapshot := string(input)
	model := NewClientResponseModel(ResponseModelModeRequest, "quoted\"alias")
	out, changed := model.Rewrite(input, "modelVersion", "response.modelVersion")
	require.True(t, changed)
	require.Equal(t, ` [ {"modelVersion":"quoted\"alias","parts":[{"modelVersion":"inside"}]}, {"response":{"modelVersion":"quoted\"alias"}} ] `, string(out))
	require.Equal(t, snapshot, string(input))
	invalid := []byte(`{"model":"up", broken}`)
	out, changed = model.Rewrite(invalid, "model")
	require.False(t, changed)
	require.Equal(t, invalid, out)
}
