package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type upstreamPlainEncryptor struct{}

func (upstreamPlainEncryptor) Encrypt(plaintext string) (string, error) {
	return "ENC:" + plaintext, nil
}

func (upstreamPlainEncryptor) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "ENC:") {
		return "", fmt.Errorf("密文无效")
	}
	return strings.TrimPrefix(ciphertext, "ENC:"), nil
}

func TestUpstreamServiceCredentialEnvelopeAndMaskedView(t *testing.T) {
	service := &UpstreamService{encryptor: upstreamPlainEncryptor{}}
	encrypted, err := service.encryptCredential(UpstreamCredential{Password: "secret", AccessToken: "sensitive-access-value"})
	require.NoError(t, err)
	require.NotContains(t, encrypted, `"has_password"`)

	view := service.toView(&UpstreamSite{
		ID: 1, Name: "站点", AuthMode: UpstreamAuthPassword, CredentialEncrypted: encrypted,
	})
	require.True(t, view.HasPassword)
	require.False(t, view.HasToken)
	raw, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "secret")
	require.NotContains(t, string(raw), "sensitive-access-value")
}

func TestMergeUpstreamUpdateKeepsBlankCredential(t *testing.T) {
	site := &UpstreamSite{Name: "旧名称", BaseURL: "https://example.com", Platform: UpstreamPlatformSub2API, AuthMode: UpstreamAuthPassword, Account: "admin"}
	credential := UpstreamCredential{Password: "old-password", AccessToken: "old-token"}
	empty := ""
	newName := "新名称"
	changed := mergeUpstreamUpdate(site, &credential, UpstreamUpdateInput{
		Name: &newName, Password: &empty, AccessToken: &empty, RefreshToken: &empty,
	})
	require.True(t, changed)
	require.Equal(t, "old-password", credential.Password)
	require.Equal(t, "old-token", credential.AccessToken)
}

func TestUpstreamServiceRejectsNewAPITokenMode(t *testing.T) {
	service := &UpstreamService{}
	err := service.validateSite(&UpstreamSite{
		Name: "New API", BaseURL: "https://example.com", Platform: UpstreamPlatformNewAPI,
		AuthMode: UpstreamAuthToken,
	}, UpstreamCredential{AccessToken: "token"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "new API 仅支持密码认证")
}
