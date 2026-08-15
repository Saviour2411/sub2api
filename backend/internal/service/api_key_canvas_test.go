package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCanvasAPIFormat(t *testing.T) {
	tests := []struct {
		platform string
		format   string
		allowed  bool
	}{
		{PlatformOpenAI, "openai", true},
		{PlatformGrok, "openai", true},
		{PlatformGemini, "gemini", true},
		{PlatformAntigravity, "gemini", true},
		{PlatformAnthropic, "", false},
	}
	for _, test := range tests {
		format, allowed := canvasAPIFormat(test.platform)
		assert.Equal(t, test.format, format)
		assert.Equal(t, test.allowed, allowed)
	}
}

func TestCanvasKeyUsable(t *testing.T) {
	groupID := int64(7)
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	base := APIKey{GroupID: &groupID, Status: StatusActive, ExpiresAt: &future}

	t.Run("可用普通密钥", func(t *testing.T) {
		key := base
		assert.True(t, canvasKeyUsable(&key, "192.0.2.10"))
	})
	t.Run("拒绝过期和额度耗尽", func(t *testing.T) {
		expired := base
		expired.ExpiresAt = &past
		assert.False(t, canvasKeyUsable(&expired, "192.0.2.10"))
		exhausted := base
		exhausted.Quota, exhausted.QuotaUsed = 1, 1
		assert.False(t, canvasKeyUsable(&exhausted, "192.0.2.10"))
	})
	t.Run("拒绝达到速率限制和不匹配IP", func(t *testing.T) {
		limited := base
		limited.RateLimit5h, limited.Usage5h = 1, 1
		now := time.Now()
		limited.Window5hStart = &now
		assert.False(t, canvasKeyUsable(&limited, "192.0.2.10"))
		restricted := base
		restricted.IPWhitelist = []string{"198.51.100.0/24"}
		assert.False(t, canvasKeyUsable(&restricted, "192.0.2.10"))
	})
}
