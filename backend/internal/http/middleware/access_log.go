package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// AccessLog emits one compact line after every non-health request. It includes
// the request ID returned to the client so an operator can correlate an error
// report with backend logs without recording query strings or request bodies.
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/health/live" {
			return
		}
		requestID, _ := c.Get(requestIDKey)
		log.Printf("http request_id=%v method=%s path=%s status=%d latency_ms=%d ip=%s",
			requestID,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(started).Milliseconds(),
			c.ClientIP(),
		)
	}
}
