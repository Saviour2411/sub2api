package service

import (
	"sync"

	"github.com/gin-gonic/gin"
)

const openAIRequestBodyReleaseKey = "openai_request_body_release"

// SetOpenAIRequestBodyRelease 注册请求体释放回调。回调只会执行一次。
func SetOpenAIRequestBodyRelease(c *gin.Context, release func()) {
	if c == nil || release == nil {
		return
	}
	c.Set(openAIRequestBodyReleaseKey, sync.OnceFunc(release))
}

// ClearOpenAIRequestBodyRelease 清除尚未执行的请求体释放回调。
func ClearOpenAIRequestBodyRelease(c *gin.Context) {
	if c == nil {
		return
	}
	c.Set(openAIRequestBodyReleaseKey, nil)
}

func releaseOpenAIRequestBody(c *gin.Context) {
	if c == nil {
		return
	}
	value, ok := c.Get(openAIRequestBodyReleaseKey)
	if !ok {
		return
	}
	release, ok := value.(func())
	if ok && release != nil {
		release()
	}
}
