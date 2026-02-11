package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// Recovery returns a middleware that recovers from panics
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the stack trace
				logger.Error("Panic recovered: %v\n%s", err, debug.Stack())

				apiErr := errors.NewInternal("An unexpected error occurred")
				c.AbortWithStatusJSON(http.StatusInternalServerError, apiErr.ToResponse())
			}
		}()
		c.Next()
	}
}
