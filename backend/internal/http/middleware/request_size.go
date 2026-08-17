package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestBodyLimit caps inbound bodies before JSON or multipart parsing can
// allocate unbounded memory. Nginx uses the same 50 MiB ceiling.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"detail": "request body too large"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
