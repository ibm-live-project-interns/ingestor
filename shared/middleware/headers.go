package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"
)

// SecurityHeaders returns a middleware that adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		c.Next()
	}
}

// CORS returns a CORS middleware
func CORS() gin.HandlerFunc {
	allowOrigin := config.GetEnv("CORS_ALLOWED_ORIGINS", "*")
	allowMethods := config.GetEnv("CORS_ALLOWED_METHODS", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	allowHeaders := config.GetEnv("CORS_ALLOWED_HEADERS", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
	exposeHeaders := config.GetEnv("CORS_EXPOSE_HEADERS", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")

	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		c.Header("Access-Control-Expose-Headers", exposeHeaders)
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
