package middleware

import (
	"os"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders returns a middleware that adds security headers.
// CORS is handled exclusively by gin-contrib/cors in main.go — this middleware
// only sets non-CORS hardening headers to avoid conflicting with the CORS
// middleware's origin negotiation.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'self'")

		// Only send HSTS when TLS is actually active — sending it over plain
		// HTTP encourages browsers to latch onto a non-existent https origin.
		if os.Getenv("TLS_ENABLED") == "true" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}
