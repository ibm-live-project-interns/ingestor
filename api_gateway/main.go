package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"api_gateway/handlers"
	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/middleware"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// splitAndTrimOrigins splits a comma-separated CORS origin list and trims
// whitespace from each entry. Empty entries (e.g. from trailing commas) are
// dropped so gin-contrib/cors never receives a blank origin.
func splitAndTrimOrigins(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			result = append(result, t)
		}
	}
	return result
}

// authIPLimiters holds a per-source-IP token bucket for authentication
// endpoints (login, register, password flows). A per-IP budget blunts
// credential-stuffing and brute-force attempts without needing a shared
// Redis store for the typical single-instance deployment.
var authIPLimiters sync.Map

// authRateLimiter returns a middleware that allows at most 5 auth requests
// per minute per client IP. Every request consumes one token; buckets
// refill at 1 token per 12 seconds (5/minute) with a burst of 5.
func authRateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		val, _ := authIPLimiters.LoadOrStore(ip, rate.NewLimiter(rate.Every(60*time.Second/5), 5))
		limiter := val.(*rate.Limiter)
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests. Please wait before trying again."})
			return
		}
		c.Next()
	}
}

func main() {
	// Initialize structured logger
	logCfg := logger.DefaultLoggerConfig()
	logCfg.ServiceName = "api-gateway"
	logger.Init(logCfg)

	logger.Info("API Gateway starting...")

	// JWT_SECRET is mandatory. Silently generating a random secret on startup
	// would invalidate every existing session on each restart and masks the
	// real misconfiguration — fail fast instead.
	if os.Getenv("JWT_SECRET") == "" {
		logger.Fatal("JWT_SECRET environment variable is required")
	}

	// Initialize database (non-fatal: operates in demo mode without DB)
	dbCfg := database.DefaultDBConfig()
	if err := dbCfg.Validate(); err != nil {
		logger.Warn("Database config incomplete (%v), running in demo mode", err)
	} else {
		if db, err := database.Init(dbCfg); err != nil {
			logger.Warn("Failed to connect to database: %v. Running in demo mode", err)
		} else {
			// Don't log DB host/user/port/dbname to avoid leaking topology to logs.
			logger.Info("Database connected successfully")

			// Migrate legacy text columns to JSONB in-place. Errors are logged
			// but non-fatal so repeated startups on already-migrated schemas do
			// not abort boot.
			if err := db.Exec(`ALTER TABLE device_groups ALTER COLUMN device_ids TYPE JSONB USING CASE WHEN device_ids IS NULL OR device_ids = '' THEN '[]'::jsonb ELSE device_ids::jsonb END`).Error; err != nil {
				logger.Debug("device_groups jsonb migration skipped: %v", err)
			}
			if err := db.Exec(`ALTER TABLE runbooks ALTER COLUMN steps TYPE JSONB USING CASE WHEN steps IS NULL OR steps = '' THEN '[]'::jsonb ELSE steps::jsonb END`).Error; err != nil {
				logger.Debug("runbooks jsonb migration skipped: %v", err)
			}
			if err := db.Exec(`ALTER TABLE post_mortems ADD COLUMN IF NOT EXISTS alert_id_str VARCHAR(50)`).Error; err != nil {
				logger.Debug("post_mortems alert_id_str migration skipped: %v", err)
			}
			// Backfill alert_id_str for known post-mortems based on title matching
			backfills := []struct{ pattern, alertID string }{
				{"%BGP%", "ALT-S001"},
				{"%SFP Module%", "ALT-S004"},
				{"%Out-of-Memory%", "ALT-S006"},
			}
			for _, b := range backfills {
				if err := db.Exec(
					`UPDATE post_mortems SET alert_id_str = ? WHERE title ILIKE ? AND (alert_id_str IS NULL OR alert_id_str = '')`,
					b.alertID, b.pattern,
				).Error; err != nil {
					logger.Debug("post_mortems backfill skipped for %s: %v", b.alertID, err)
				}
			}
			// Remove duplicate on-call schedule entries (keep earliest ID per user+period)
			if err := db.Exec(`
				DELETE FROM on_call_schedules WHERE id NOT IN (
					SELECT MIN(id) FROM on_call_schedules GROUP BY user_id, start_time, end_time
				)`).Error; err != nil {
				logger.Debug("on_call_schedules dedup skipped: %v", err)
			}
			// Remove duplicate post-mortems (keep lowest ID per title)
			if err := db.Exec(`
				DELETE FROM post_mortems WHERE id NOT IN (
					SELECT MIN(id) FROM post_mortems GROUP BY title
				)`).Error; err != nil {
				logger.Debug("post_mortems dedup skipped: %v", err)
			}
			// Fix alert-008: resolved_at is before timestamp due to legacy seed inversion.
			// Set resolved_at to 45 minutes after timestamp so MTTR and SLA calcs include it.
			if err := db.Exec(`
				UPDATE alerts SET resolved_at = timestamp + INTERVAL '45 minutes'
				WHERE id = 'alert-008' AND resolved_at IS NOT NULL AND resolved_at < timestamp
			`).Error; err != nil {
				logger.Debug("alert-008 timestamp fix skipped: %v", err)
			}

			// Auto-migrate: adds new columns to existing tables, never drops columns.
			// User and Session are included so OAuth columns (google_id, etc.) are
			// added when upgrading an existing DB that pre-dates them.
			if err := db.AutoMigrate(
				&models.User{},
				&models.Session{},
				&models.AuditLog{},
				&models.OnCallSchedule{},
				&models.OnCallOverride{},
				&models.PostMortem{},
				&models.DeviceGroup{},
				&models.Runbook{},
				&models.Device{},
			); err != nil {
				logger.Warn("Auto-migration failed: %v", err)
			}

			// Fallback: explicitly add any columns that AutoMigrate may have
			// silently failed to add on existing databases. ADD COLUMN IF NOT EXISTS
			// is idempotent and safe to run on every startup.
			userCols := []string{
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS google_id VARCHAR(100)`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_token TEXT`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_refresh TEXT`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS failed_attempts INTEGER NOT NULL DEFAULT 0`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token VARCHAR(100)`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS verification_token_exp TIMESTAMPTZ`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_token VARCHAR(100)`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS reset_token_exp TIMESTAMPTZ`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS email_alerts BOOLEAN NOT NULL DEFAULT TRUE`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS push_notifications BOOLEAN NOT NULL DEFAULT TRUE`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS sound_enabled BOOLEAN NOT NULL DEFAULT FALSE`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS critical_only BOOLEAN NOT NULL DEFAULT FALSE`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS theme VARCHAR(20) NOT NULL DEFAULT 'system'`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS language VARCHAR(20) NOT NULL DEFAULT 'en'`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT 'UTC'`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS auto_refresh BOOLEAN NOT NULL DEFAULT TRUE`,
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS refresh_interval VARCHAR(10) NOT NULL DEFAULT '30'`,
				`CREATE INDEX IF NOT EXISTS idx_users_google_id ON users(google_id)`,
				`CREATE INDEX IF NOT EXISTS idx_users_verification_token ON users(verification_token)`,
				`CREATE INDEX IF NOT EXISTS idx_users_reset_token ON users(reset_token)`,
			}
			for _, sql := range userCols {
				if err := db.Exec(sql).Error; err != nil {
					logger.Debug("user column migration skipped: %v", err)
				}
			}
		}
	}

	// Initialize auth service
	if err := services.InitAuthService(); err != nil {
		logger.Fatal("Failed to initialize auth service: %v", err)
	}
	logger.Info("Auth service initialized")

	// Initialize email service (non-fatal)
	if err := services.InitEmailService(); err != nil {
		logger.Warn("Email service not available: %v", err)
	} else if services.Email != nil {
		logger.Info("Email service initialized")
	}

	// Initialize Google OAuth (non-fatal)
	if err := services.InitGoogleOAuth(); err != nil {
		logger.Warn("Google OAuth not available: %v", err)
	} else if services.Google != nil {
		logger.Info("Google OAuth initialized")
	}

	// Setup Gin router
	ginMode := config.GetEnv("GIN_MODE", "release")
	gin.SetMode(ginMode)
	router := gin.New()

	// Trust only the proxies declared in TRUSTED_PROXIES (comma-separated CIDRs).
	// Defaults to the full range so HuggingFace Spaces works out of the box.
	// Narrow this to the actual HF proxy CIDR in production to prevent
	// X-Forwarded-For spoofing that could bypass the per-IP rate limiter.
	trustedProxies := splitAndTrimOrigins(config.GetEnv("TRUSTED_PROXIES", "0.0.0.0/0"))
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		logger.Warn("Failed to set trusted proxies: %v", err)
	}

	// 8.3 fix: Set request body size limit for multipart forms (8MB)
	router.MaxMultipartMemory = 8 << 20

	// Global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestBodyLimit(4 << 20)) // 4 MB cap on JSON request bodies

	// Rate limiting
	if config.GetEnvBool("RATE_LIMIT_ENABLED", true) {
		router.Use(middleware.RateLimit())
		logger.Info("Rate limiting enabled")
	}

	// CORS configuration
	// 8.3 fix: Reduced MaxAge from 12 hours to 6 hours
	corsOrigins := config.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:3000")
	router.Use(cors.New(cors.Config{
		AllowOrigins:     splitAndTrimOrigins(corsOrigins),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Internal-API-Key"},
		ExposeHeaders:    []string{"Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           1 * time.Hour,
	}))

	// Internal API routes (service-to-service, API key protected)
	internal := router.Group("/api/internal")
	internal.Use(internalAPIKeyMiddleware())
	{
		internal.POST("/events", handlers.IngestEvent)
		internal.GET("/health", handlers.GetHealth)
	}

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes (no auth required) — gated by per-IP rate limiter to
		// make credential-stuffing and token-guessing costly.
		authLimited := authRateLimiter()
		v1.POST("/login", authLimited, handlers.Login)
		v1.POST("/register", authLimited, handlers.Register)
		v1.GET("/health", handlers.GetHealth)

		// Google OAuth routes (public)
		v1.GET("/auth/google/login", handlers.GoogleLogin)
		v1.GET("/auth/google/callback", handlers.GoogleCallback)
		v1.GET("/auth/oauth/exchange", handlers.ExchangeOAuthCode)

		// Email test endpoint (requires sysadmin role)
		// Moved to protected routes for security — see testAdmin group below

		// Public auth routes (no auth required - used by unauthenticated users)
		v1.POST("/auth/verify-email", handlers.VerifyEmail)
		v1.POST("/auth/forgot-password", authLimited, handlers.ForgotPassword)
		v1.POST("/auth/reset-password", authLimited, handlers.ResetPassword)
		v1.POST("/auth/resend-verification", handlers.ResendVerification)

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(authMiddleware())
		{
			// Auth
			protected.POST("/logout", handlers.Logout)
			protected.GET("/me", handlers.GetCurrentUser)
			protected.GET("/auth/me", handlers.GetCurrentUser)

			// Alerts - GET (view permissions handled by frontend, no extra RBAC)
			protected.GET("/alerts", handlers.GetAlerts)
			protected.GET("/alerts/summary", handlers.GetAlertsSummary)
			protected.GET("/alerts/severity-distribution", handlers.GetSeverityDistribution)
			protected.GET("/alerts/over-time", handlers.GetAlertsOverTime)
			protected.GET("/alerts/recurring", handlers.GetRecurringAlerts)
			protected.GET("/alerts/distribution/time", handlers.GetAlertDistributionTime)
			protected.GET("/alerts/:id", handlers.GetAlertByID)
			protected.GET("/alerts/:id/tickets", handlers.GetAlertTickets)
			protected.GET("/alerts/:id/post-mortem", handlers.GetAlertPostMortem)

			// Only users with acknowledge-alerts permission can modify alert state
			alertActions := protected.Group("")
			alertActions.Use(middleware.RequireAnyPermission(rbac.PermAcknowledgeAlerts))
			{
				alertActions.POST("/alerts/:id/acknowledge", handlers.AcknowledgeAlert)
				alertActions.POST("/alerts/:id/dismiss", handlers.DismissAlert)
				alertActions.POST("/alerts/:id/resolve", handlers.ResolveAlert)
				alertActions.POST("/alerts/:id/reanalyze", handlers.ReanalyzeAlert)
				alertActions.POST("/alerts/bulk-action", handlers.BulkAlertAction)
				alertActions.POST("/alerts/:id/post-mortem", handlers.CreatePostMortem)
			}

			// Tickets - GET (view permissions handled by frontend)
			protected.GET("/tickets", handlers.GetTickets)
			protected.GET("/tickets/stats", handlers.GetTicketStats)
			protected.GET("/tickets/:id", handlers.GetTicketByID)
			protected.GET("/tickets/:id/comments", handlers.GetTicketComments)

			// Ticket export requires export-reports permission
			ticketExport := protected.Group("")
			ticketExport.Use(middleware.RequireAnyPermission(rbac.PermExportReports))
			{
				ticketExport.GET("/tickets/export", handlers.ExportTickets)
			}

			// Only users with create-tickets permission can modify tickets
			ticketActions := protected.Group("")
			ticketActions.Use(middleware.RequireAnyPermission(rbac.PermCreateTickets))
			{
				ticketActions.POST("/tickets", handlers.CreateTicket)
				ticketActions.PUT("/tickets/:id", handlers.UpdateTicket)
				ticketActions.PATCH("/tickets/:id", handlers.UpdateTicket)
				ticketActions.DELETE("/tickets/:id", handlers.DeleteTicket)
				ticketActions.POST("/tickets/:id/comments", handlers.AddTicketComment)
			}

			// Trends
			protected.GET("/trends/kpi", handlers.GetTrendsKPI)

			// Devices (view permissions handled by frontend)
			protected.GET("/devices", handlers.GetDevices)
			protected.GET("/devices/noisy", handlers.GetNoisyDevices)
			protected.GET("/devices/:id", handlers.GetDeviceByID)
			protected.GET("/devices/:id/metrics", handlers.GetDeviceMetrics)

			// AI
			protected.GET("/ai/metrics", handlers.GetAIMetrics)
			protected.GET("/ai/insights", handlers.GetAIInsights)
			protected.GET("/ai/impact-over-time", handlers.GetAIImpactOverTime)

			// CVE feed (network security intelligence)
			protected.GET("/cve/feed", handlers.GetCVEFeed)

			// User Settings (self-service, no extra RBAC)
			protected.GET("/settings/notifications", handlers.GetNotificationPreferences)
			protected.PUT("/settings/notifications", handlers.UpdateNotificationPreferences)
			protected.GET("/settings/ui", handlers.GetUIPreferences)
			protected.PUT("/settings/ui", handlers.UpdateUIPreferences)

			// User management restricted to sysadmin role
			userAdmin := protected.Group("")
			userAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				userAdmin.GET("/users", handlers.GetUsers)
				userAdmin.GET("/users/:id", handlers.GetUserByID)
				userAdmin.PUT("/users/:id", handlers.UpdateUser)
				userAdmin.DELETE("/users/:id", handlers.DeleteUser)
				userAdmin.POST("/users/:id/reset-password", handlers.ResetUserPassword)
			}

			// Profile (self-service, no extra RBAC)
			protected.PUT("/me", handlers.UpdateProfile)
			protected.PUT("/me/password", handlers.ChangePassword)

			// Reports export requires export-reports permission
			reportsAdmin := protected.Group("")
			reportsAdmin.Use(middleware.RequireAnyPermission(rbac.PermExportReports))
			{
				reportsAdmin.GET("/reports/export", handlers.ExportReport)
			}

			// SLA Reports require view-sla permission
			slaAdmin := protected.Group("")
			slaAdmin.Use(middleware.RequireAnyPermission(rbac.PermViewSLA))
			{
				slaAdmin.GET("/reports/sla", handlers.GetSLAOverview)
				slaAdmin.GET("/reports/sla/violations", handlers.GetSLAViolations)
				slaAdmin.GET("/reports/sla/trend", handlers.GetSLATrend)
			}

			// Audit logs restricted to sysadmin role
			auditAdmin := protected.Group("")
			auditAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				auditAdmin.GET("/audit-logs", handlers.GetAuditLogs)
				auditAdmin.GET("/audit-logs/actions", handlers.GetAuditLogActions)
			}

			// On-Call Schedule (read)
			protected.GET("/on-call/current", handlers.GetCurrentOnCall)
			protected.GET("/on-call/schedule", handlers.GetOnCallSchedule)
			protected.GET("/on-call/schedules", handlers.GetOnCallSchedules)

			// On-Call Schedule CRUD (write - sysadmin only)
			onCallAdmin := protected.Group("")
			onCallAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				onCallAdmin.POST("/on-call/schedules", handlers.CreateOnCallSchedule)
				onCallAdmin.PUT("/on-call/schedules/:id", handlers.UpdateOnCallSchedule)
				onCallAdmin.DELETE("/on-call/schedules/:id", handlers.DeleteOnCallSchedule)
				onCallAdmin.POST("/on-call/overrides", handlers.CreateOnCallOverride)
				onCallAdmin.DELETE("/on-call/overrides/:id", handlers.DeleteOnCallOverride)
			}

			// Network Topology
			protected.GET("/topology", handlers.GetTopology)

			// Runbook read operations (all authenticated users)
			protected.GET("/runbooks", handlers.GetRunbooks)
			protected.GET("/runbooks/suggest", handlers.SuggestRunbooks)
			protected.GET("/runbooks/:id", handlers.GetRunbookByID)
			runbookAdmin := protected.Group("")
			runbookAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin, rbac.RoleSeniorEng))
			{
				runbookAdmin.POST("/runbooks", handlers.CreateRunbook)
				runbookAdmin.PUT("/runbooks/:id", handlers.UpdateRunbook)
				runbookAdmin.DELETE("/runbooks/:id", handlers.DeleteRunbook)
			}

			// Device Groups - read access for all authenticated users
			protected.GET("/device-groups", handlers.GetDeviceGroups)
			protected.GET("/device-groups/:id", handlers.GetDeviceGroupByID)

			// Device Groups - write operations require network-admin, senior-eng, or sysadmin role
			deviceGroupAdmin := protected.Group("/device-groups")
			deviceGroupAdmin.Use(middleware.RequireRole(rbac.RoleNetworkAdmin, rbac.RoleSeniorEng, rbac.RoleSysAdmin))
			{
				deviceGroupAdmin.POST("", handlers.CreateDeviceGroup)
				deviceGroupAdmin.PUT("/:id", handlers.UpdateDeviceGroup)
				deviceGroupAdmin.DELETE("/:id", handlers.DeleteDeviceGroup)
				deviceGroupAdmin.POST("/:id/devices", handlers.AddDevicesToGroup)
				deviceGroupAdmin.DELETE("/:id/devices/:deviceId", handlers.RemoveDeviceFromGroup)
			}

			// Configuration management requires sysadmin or senior-eng role
			configAdmin := protected.Group("/configuration")
			configAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin, rbac.RoleSeniorEng))
			{
				// Threshold Rules
				configAdmin.GET("/rules", handlers.GetRules)
				configAdmin.POST("/rules", handlers.CreateRule)
				configAdmin.GET("/rules/:id", handlers.GetRuleByID)
				configAdmin.PUT("/rules/:id", handlers.UpdateRule)
				configAdmin.DELETE("/rules/:id", handlers.DeleteRule)

				// Notification Channels
				configAdmin.GET("/channels", handlers.GetChannels)
				configAdmin.POST("/channels", handlers.CreateChannel)
				configAdmin.GET("/channels/:id", handlers.GetChannelByID)
				configAdmin.PUT("/channels/:id", handlers.UpdateChannel)
				configAdmin.DELETE("/channels/:id", handlers.DeleteChannel)

				// Escalation Policies
				configAdmin.GET("/policies", handlers.GetPolicies)
				configAdmin.POST("/policies", handlers.CreatePolicy)
				configAdmin.GET("/policies/:id", handlers.GetPolicyByID)
				configAdmin.PUT("/policies/:id", handlers.UpdatePolicy)
				configAdmin.DELETE("/policies/:id", handlers.DeletePolicy)

				// Maintenance Windows
				configAdmin.GET("/maintenance", handlers.GetWindows)
				configAdmin.POST("/maintenance", handlers.CreateWindow)
				configAdmin.GET("/maintenance/:id", handlers.GetWindowByID)
				configAdmin.PUT("/maintenance/:id", handlers.UpdateWindow)
				configAdmin.DELETE("/maintenance/:id", handlers.DeleteWindow)
			}

			// Global Settings - GET is open, PUT requires sysadmin
			protected.GET("/configuration/global-settings", handlers.GetGlobalSettings)
			globalSettingsAdmin := protected.Group("")
			globalSettingsAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				globalSettingsAdmin.PUT("/configuration/global-settings", handlers.UpdateGlobalSettings)
			}

			// Email test endpoint (sysadmin only, must not be public)
			testAdmin := protected.Group("")
			testAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				testAdmin.POST("/test/send-all-emails", handlers.SendAllTestEmails)
				testAdmin.GET("/test/send-all-emails", handlers.SendAllTestEmails)
			}

			// Alert history (resolved alerts log)
			protected.GET("/alert-history", handlers.GetAlertHistory)

			// Service Status (application-level health checks)
			protected.GET("/service-status", handlers.GetServiceStatus)

			// Docker Container Status & Logs — restricted to privileged roles
			// because container logs can expose sensitive runtime state.
			dockerAdmin := protected.Group("")
			dockerAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin, rbac.RoleSeniorEng))
			{
				dockerAdmin.GET("/services/status", handlers.GetDockerServiceStatus)
				dockerAdmin.GET("/services/:name/logs", handlers.GetDockerServiceLogs)
			}

			// Post-Mortems (list and update - all authenticated users)
			protected.GET("/post-mortems", handlers.ListPostMortems)
			protected.PUT("/post-mortems/:id", handlers.UpdatePostMortem)

			// System Health (sysadmin only)
			sysHealthAdmin := protected.Group("")
			sysHealthAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				sysHealthAdmin.GET("/system/health", handlers.GetSystemHealth)
			}

			// Ingest via JWT (sysadmin only — internal services use /api/internal/events with API key)
			ingestAdmin := protected.Group("")
			ingestAdmin.Use(middleware.RequireRole(rbac.RoleSysAdmin))
			{
				ingestAdmin.POST("/events", handlers.IngestEvent)
			}
		}
	}

	// 8.3 fix: Use http.Server with timeouts instead of router.Run()
	// Also implements graceful shutdown with signal handling (SIGTERM/SIGINT)
	port := config.GetEnv("API_GATEWAY_PORT", config.GetEnv("PORT", "8080"))
	logger.Info("API Gateway running on :%s (mode=%s)", port, ginMode)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in a goroutine so we can handle shutdown signals
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start API Gateway: %v", err)
		}
	}()

	// Graceful shutdown: wait for SIGTERM or SIGINT
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("Received signal %v, shutting down gracefully...", sig)

	// Give outstanding requests up to 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown: %v", err)
	}

	logger.Info("API Gateway shut down cleanly")
}

// authMiddleware validates JWT tokens using the services.Auth package
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Also check for auth_token cookie (set by OAuth callback as HTTP-only cookie)
		if authHeader == "" {
			if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
				authHeader = "Bearer " + cookie
			}
		}

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format. Use: Bearer <token>"})
			c.Abort()
			return
		}

		claims, err := services.Auth.ValidateToken(parts[1])
		if err != nil {
			logger.Debug("Token validation failed: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user info in context for downstream handlers
		// NOTE: handlers use "userID" (capital D) - must match exactly
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// internalAPIKeyMiddleware protects service-to-service endpoints.
// When INTERNAL_API_KEY is not configured, reject all requests
// to prevent accidental exposure of internal endpoints.
func internalAPIKeyMiddleware() gin.HandlerFunc {
	apiKey := config.GetEnv("INTERNAL_API_KEY", "")

	return func(c *gin.Context) {
		// If no API key is configured, reject all internal requests
		if apiKey == "" {
			logger.Warn("Internal API access rejected: INTERNAL_API_KEY not configured. Set INTERNAL_API_KEY env var to enable internal endpoints.")
			c.JSON(http.StatusForbidden, gin.H{"error": "Internal API is not configured. Set INTERNAL_API_KEY environment variable."})
			c.Abort()
			return
		}

		// Only accept API key via header -- never from query params,
		// which leak into access logs, browser history, and referrer headers.
		providedKey := c.GetHeader("X-Internal-API-Key")

		// Use constant-time comparison to prevent timing attacks
		// that could leak key length or content via response latency.
		if subtle.ConstantTimeCompare([]byte(providedKey), []byte(apiKey)) != 1 {
			logger.Warn("Unauthorized internal API access from %s to %s", c.ClientIP(), c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid or missing internal API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
