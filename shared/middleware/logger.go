package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// RequestLogger returns a middleware that logs HTTP requests
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		if query != "" {
			path = path + "?" + query
		}

		logger.Info("%s %s %d %s %s",
			method,
			path,
			status,
			latency.String(),
			clientIP,
		)
	}
}
