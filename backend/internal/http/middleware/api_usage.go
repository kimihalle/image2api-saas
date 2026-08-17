package middleware

import (
	"strings"
	"time"

	"backend/internal/service"
	"github.com/gin-gonic/gin"
)

func APIUsage(platform *service.APIPlatformService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if platform == nil || !strings.HasPrefix(c.Request.URL.Path, "/v1/") {
			c.Next()
			return
		}
		started := time.Now()
		c.Next()
		platform.RecordUsage(service.APIUsageRecord{
			UserID: stringContext(c, "api_user_id"), APIKeyID: stringContext(c, "api_key_id"),
			Path: c.FullPath(), Model: stringContext(c, "api_model"), Status: c.Writer.Status(),
			LatencyMS: int(time.Since(started).Milliseconds()), Credits: floatContext(c, "api_credits"),
		})
	}
}

func stringContext(c *gin.Context, key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	text, _ := value.(string)
	return text
}

func floatContext(c *gin.Context, key string) float64 {
	value, ok := c.Get(key)
	if !ok {
		return 0
	}
	switch number := value.(type) {
	case float64:
		return number
	case float32:
		return float64(number)
	case int:
		return float64(number)
	}
	return 0
}
