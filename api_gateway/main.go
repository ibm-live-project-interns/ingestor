package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"api_gateway/handlers"
	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/middleware"
)

func main() {
	// Initialize structured logger
	logCfg := logger.DefaultLoggerConfig()
	logCfg.ServiceName = "api-gateway"
	logger.Init(logCfg)

	logger.Info("API Gateway starting...")

	// Ensure JWT_SECRET exists (generate random for dev if missing)
	if os.Getenv("JWT_SECRET") == "" {
		logger.Warn("JWT_SECRET not set, generating random secret. Set JWT_SECRET in production!")
		bytes := make([]byte, 32)
		rand.Read(bytes)
		os.Setenv("JWT_SECRET", hex.EncodeToString(bytes))
	}

	// Initialize database (non-fatal: operates in demo mode without DB)
	dbCfg := database.DefaultDBConfig()
	if err := dbCfg.Validate(); err != nil {
		logger.Warn("Database config incomplete (%v), running in demo mode", err)
	} else {
		if _, err := database.Init(dbCfg); err != nil {
			logger.Warn("Failed to connect to database: %v. Running in demo mode", err)
		} else {
			logger.Info("Database connected: %s@%s:%s/%s", dbCfg.User, dbCfg.Host, dbCfg.Port, dbCfg.DBName)
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
	ginMode := config.GetEnv("GIN_MODE", "debug")
	gin.SetMode(ginMode)
	router := gin.New()

	// Global middleware
	router.Use(middleware.Recovery())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.SecurityHeaders())

	// Rate limiting
	if config.GetEnvBool("RATE_LIMIT_ENABLED", true) {
		router.Use(middleware.RateLimit())
		logger.Info("Rate limiting enabled")
	}

	// CORS configuration
	corsOrigins := config.GetEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:5174,http://localhost:3000")
	router.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Split(corsOrigins, ","),
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Internal-API-Key"},
		ExposeHeaders:    []string{"Content-Length", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
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
		// Public routes (no auth required)
		v1.POST("/login", handlers.Login)
		v1.POST("/register", handlers.Register)
		v1.GET("/health", handlers.GetHealth)

		// Google OAuth routes (public)
		v1.GET("/auth/google/login", handlers.GoogleLogin)
		v1.GET("/auth/google/callback", handlers.GoogleCallback)

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(authMiddleware())
		{
			// Auth
			protected.POST("/logout", handlers.Logout)
			protected.GET("/me", handlers.GetCurrentUser)
			protected.GET("/auth/me", handlers.GetCurrentUser)
			protected.POST("/auth/verify-email", handlers.VerifyEmail)
			protected.POST("/auth/forgot-password", handlers.ForgotPassword)
			protected.POST("/auth/reset-password", handlers.ResetPassword)
			protected.POST("/auth/resend-verification", handlers.ResendVerification)

			// Alerts (specific routes BEFORE parameterized :id)
			protected.GET("/alerts", handlers.GetAlerts)
			protected.GET("/alerts/summary", handlers.GetAlertsSummary)
			protected.GET("/alerts/severity-distribution", handlers.GetSeverityDistribution)
			protected.GET("/alerts/over-time", handlers.GetAlertsOverTime)
			protected.GET("/alerts/recurring", handlers.GetRecurringAlerts)
			protected.GET("/alerts/distribution/time", handlers.GetAlertDistributionTime)
			protected.GET("/alerts/:id", handlers.GetAlertByID)

			// Alert Actions
			protected.POST("/alerts/:id/acknowledge", handlers.AcknowledgeAlert)
			protected.POST("/alerts/:id/dismiss", handlers.DismissAlert)
			protected.POST("/alerts/:id/reanalyze", handlers.ReanalyzeAlert)

			// Tickets (specific routes BEFORE parameterized :id)
			protected.GET("/tickets", handlers.GetTickets)
			protected.GET("/tickets/stats", handlers.GetTicketStats)
			protected.GET("/tickets/export", handlers.ExportTickets)
			protected.GET("/tickets/:id", handlers.GetTicketByID)
			protected.POST("/tickets", handlers.CreateTicket)
			protected.PUT("/tickets/:id", handlers.UpdateTicket)
			protected.PATCH("/tickets/:id", handlers.UpdateTicket)

			// Trends
			protected.GET("/trends/kpi", handlers.GetTrendsKPI)

			// Devices (specific routes BEFORE parameterized :id)
			protected.GET("/devices", handlers.GetDevices)
			protected.GET("/devices/noisy", handlers.GetNoisyDevices)
			protected.GET("/devices/:id", handlers.GetDeviceByID)

			// AI
			protected.GET("/ai/metrics", handlers.GetAIMetrics)
			protected.GET("/ai/insights", handlers.GetAIInsights)
			protected.GET("/ai/impact-over-time", handlers.GetAIImpactOverTime)

			// Reports
			protected.GET("/reports/export", handlers.ExportReport)

			// Configuration - Threshold Rules
			protected.GET("/configuration/rules", handlers.GetRules)
			protected.POST("/configuration/rules", handlers.CreateRule)
			protected.GET("/configuration/rules/:id", handlers.GetRuleByID)
			protected.PUT("/configuration/rules/:id", handlers.UpdateRule)
			protected.DELETE("/configuration/rules/:id", handlers.DeleteRule)

			// Configuration - Notification Channels
			protected.GET("/configuration/channels", handlers.GetChannels)
			protected.POST("/configuration/channels", handlers.CreateChannel)
			protected.GET("/configuration/channels/:id", handlers.GetChannelByID)
			protected.PUT("/configuration/channels/:id", handlers.UpdateChannel)
			protected.DELETE("/configuration/channels/:id", handlers.DeleteChannel)

			// Configuration - Escalation Policies
			protected.GET("/configuration/policies", handlers.GetPolicies)
			protected.POST("/configuration/policies", handlers.CreatePolicy)
			protected.GET("/configuration/policies/:id", handlers.GetPolicyByID)
			protected.PUT("/configuration/policies/:id", handlers.UpdatePolicy)
			protected.DELETE("/configuration/policies/:id", handlers.DeletePolicy)

			// Configuration - Maintenance Windows
			protected.GET("/configuration/maintenance", handlers.GetWindows)
			protected.POST("/configuration/maintenance", handlers.CreateWindow)
			protected.GET("/configuration/maintenance/:id", handlers.GetWindowByID)
			protected.PUT("/configuration/maintenance/:id", handlers.UpdateWindow)
			protected.DELETE("/configuration/maintenance/:id", handlers.DeleteWindow)

			// Ingest (also available internally)
			protected.POST("/events", handlers.IngestEvent)
		}
	}

	// Start server
	port := config.GetEnv("API_GATEWAY_PORT", config.GetEnv("PORT", "8080"))
	logger.Info("API Gateway running on :%s (mode=%s, cors=%s)", port, ginMode, corsOrigins)

	if err := router.Run(":" + port); err != nil {
		logger.Fatal("Failed to start API Gateway: %v", err)
	}
}

// authMiddleware validates JWT tokens using the services.Auth package
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
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

// internalAPIKeyMiddleware protects service-to-service endpoints
func internalAPIKeyMiddleware() gin.HandlerFunc {
	apiKey := config.GetEnv("INTERNAL_API_KEY", "")

	return func(c *gin.Context) {
		// If no API key is configured, allow all internal requests (dev mode)
		if apiKey == "" {
			c.Next()
			return
		}

		providedKey := c.GetHeader("X-Internal-API-Key")
		if providedKey == "" {
			providedKey = c.Query("api_key")
		}

		if providedKey != apiKey {
			logger.Warn("Unauthorized internal API access from %s to %s", c.ClientIP(), c.Request.URL.Path)
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid or missing internal API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}
