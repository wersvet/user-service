package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"user-service/internal/observability"
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		observability.RecordHTTPRequest(
			c.Request.Method,
			route,
			c.Writer.Status(),
			time.Since(start),
		)
	}
}
