package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
)

// RateLimiterConfig holds rate limiter settings
type RateLimiterConfig struct {
	// Rate is the number of requests allowed per period
	Rate int
	// Period is the time window for the rate limit
	Period time.Duration
	// BurstSize is the maximum burst size
	BurstSize int
	// KeyFunc extracts the rate limit key from request (default: IP)
	KeyFunc func(*gin.Context) string
}

// DefaultRateLimiterConfig returns default rate limiter settings
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:      config.GetEnvInt("RATE_LIMIT_RATE", 100),
		Period:    time.Duration(config.GetEnvInt("RATE_LIMIT_PERIOD_SECONDS", 60)) * time.Second,
		BurstSize: config.GetEnvInt("RATE_LIMIT_BURST", 10),
		KeyFunc:   func(c *gin.Context) string { return c.ClientIP() },
	}
}

// MemoryRateLimiter is an in-memory rate limiter
type MemoryRateLimiter struct {
	config  RateLimiterConfig
	buckets map[string]*bucket
	mu      sync.RWMutex
	stopCh  chan struct{}
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

// NewMemoryRateLimiter creates a new memory-based rate limiter
func NewMemoryRateLimiter(cfg RateLimiterConfig) *MemoryRateLimiter {
	rl := &MemoryRateLimiter{
		config:  cfg,
		buckets: make(map[string]*bucket),
		stopCh:  make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// cleanup removes expired buckets periodically
func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.Period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for key, b := range rl.buckets {
				if time.Since(b.lastReset) > rl.config.Period*2 {
					delete(rl.buckets, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the rate limiter cleanup goroutine
func (rl *MemoryRateLimiter) Stop() {
	close(rl.stopCh)
}

// Allow checks if a request is allowed
func (rl *MemoryRateLimiter) Allow(key string) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]

	if !exists || now.Sub(b.lastReset) >= rl.config.Period {
		// Reset the bucket
		rl.buckets[key] = &bucket{
			tokens:    rl.config.Rate - 1,
			lastReset: now,
		}
		return true, rl.config.Rate - 1, rl.config.Period
	}

	if b.tokens > 0 {
		b.tokens--
		return true, b.tokens, rl.config.Period - now.Sub(b.lastReset)
	}

	return false, 0, rl.config.Period - now.Sub(b.lastReset)
}

// Middleware returns a Gin middleware for rate limiting
func (rl *MemoryRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.config.KeyFunc(c)
		allowed, remaining, resetIn := rl.Allow(key)

		c.Header("X-RateLimit-Limit", intToString(rl.config.Rate))
		c.Header("X-RateLimit-Remaining", intToString(remaining))
		c.Header("X-RateLimit-Reset", intToString(int(resetIn.Seconds())))

		if !allowed {
			apiErr := errors.NewRateLimited(resetIn.String())
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		c.Next()
	}
}

// RateLimit returns a rate limiting middleware with default config
func RateLimit() gin.HandlerFunc {
	limiter := NewMemoryRateLimiter(DefaultRateLimiterConfig())
	return limiter.Middleware()
}

// RateLimitWithConfig returns a rate limiting middleware with custom config
func RateLimitWithConfig(cfg RateLimiterConfig) gin.HandlerFunc {
	limiter := NewMemoryRateLimiter(cfg)
	return limiter.Middleware()
}

func intToString(n int) string {
	if n < 0 {
		return "0"
	}
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
