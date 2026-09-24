package service

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func forwardKimiRequestNonce(c *gin.Context, account *Account, dst http.Header) {
	if account == nil || account.Platform != PlatformKimi || c == nil || c.Request == nil {
		return
	}
	for _, nonce := range c.Request.Header.Values("X-Msh-Request-Nonce") {
		dst.Add("X-Msh-Request-Nonce", nonce)
	}
}
