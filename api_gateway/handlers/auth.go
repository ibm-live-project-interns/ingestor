package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// getDemoPassword returns the required password for demo mode authentication.
// Returns an empty string when DEMO_MODE is not enabled. When DEMO_MODE=true,
// DEMO_PASSWORD must be set or the process will abort.
func getDemoPassword() string {
	if config.GetEnv("DEMO_MODE", "") != "true" {
		return ""
	}
	pw := config.GetEnv("DEMO_PASSWORD", "")
	if pw == "" {
		logger.Fatal("DEMO_PASSWORD must be set when DEMO_MODE=true")
	}
	return pw
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token       string              `json:"token"`
	ExpiresAt   time.Time           `json:"expires_at"`
	User        models.UserResponse `json:"user"`
	Permissions []string            `json:"permissions"`
}

// writeAuditLog writes an audit log entry to the database.
// It silently fails if the database is unavailable (demo mode).
func writeAuditLog(c *gin.Context, action, resource, resourceID, detail string) {
	db := database.Get()
	if db == nil {
		return
	}

	userID := uint(0)
	username := "system"

	if uid, exists := c.Get("userID"); exists {
		if id, ok := uid.(uint); ok {
			userID = id
		}
	}
	if uname, exists := c.Get("username"); exists {
		if name, ok := uname.(string); ok {
			username = name
		}
	}

	entry := &models.AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    models.JSONB{"detail": detail},
		IPAddress:  c.ClientIP(),
		Result:     "success",
	}

	repo := database.NewAuditRepository(db.DB)
	if err := repo.Create(entry); err != nil {
		logger.Warn("Failed to write audit log: %v", err)
	}
}

// writeAuditLogWithResult writes an audit log entry with a custom result (success/failure).
func writeAuditLogWithResult(c *gin.Context, action, resource, resourceID, detail, result string) {
	db := database.Get()
	if db == nil {
		return
	}

	userID := uint(0)
	username := "system"

	if uid, exists := c.Get("userID"); exists {
		if id, ok := uid.(uint); ok {
			userID = id
		}
	}
	if uname, exists := c.Get("username"); exists {
		if name, ok := uname.(string); ok {
			username = name
		}
	}

	entry := &models.AuditLog{
		UserID:     userID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    models.JSONB{"detail": detail},
		IPAddress:  c.ClientIP(),
		Result:     result,
	}

	repo := database.NewAuditRepository(db.DB)
	if err := repo.Create(entry); err != nil {
		logger.Warn("Failed to write audit log: %v", err)
	}
}

// Login handles user authentication
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil || db.DB == nil {
		// Demo mode: require DEMO_MODE=true + DEMO_PASSWORD to prevent open access.
		demoPw := getDemoPassword()
		if demoPw == "" || req.Password != demoPw {
			logger.Warn("Demo mode: failed login attempt (wrong password or demo disabled)")
			writeAuditLogWithResult(c, "auth.login", "user", req.Email,
				"Failed demo login: invalid password or demo mode disabled", "failure")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		logger.Warn("DEMO AUTH: user authenticated without DB, assigned role=network-ops, email=%s", req.Email)
		now := time.Now()

		// All demo-mode logins are assigned the least-privileged role.
		demoRole := "network-ops"
		demoName := "NOC Operator"

		// Safely split name and check length before indexing to avoid out-of-bounds panic
		nameParts := strings.Split(demoName, " ")
		firstName := nameParts[0]
		lastName := ""
		if len(nameParts) > 1 {
			lastName = nameParts[1]
		}

		demoUser := &models.User{
			Email:         req.Email,
			Username:      strings.Split(req.Email, "@")[0],
			FirstName:     firstName,
			LastName:      lastName,
			Role:          demoRole,
			IsActive:      true,
			EmailVerified: true,
			LastLogin:     &now,
			CreatedAt:     now.Add(-30 * 24 * time.Hour),
		}
		demoUser.ID = 1

		token, err := services.Auth.GenerateToken(demoUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		writeAuditLog(c, "auth.login", "user", req.Email,
			fmt.Sprintf("Demo login successful for %s (role: %s)", req.Email, demoRole))

		c.JSON(http.StatusOK, AuthResponse{
			Token:       token,
			ExpiresAt:   now.Add(24 * time.Hour),
			User:        demoUser.ToResponse(),
			Permissions: services.GetRolePermissionsStrings(demoRole),
		})
		return
	}

	// Find user by email
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		writeAuditLogWithResult(c, "auth.login", "user", req.Email,
			fmt.Sprintf("Login failed: user not found for email %s", req.Email), "failure")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		writeAuditLogWithResult(c, "auth.login", "user", fmt.Sprintf("%d", user.ID),
			fmt.Sprintf("Login rejected: account locked for %s", user.Email), "failure")
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is locked. Try again later."})
		return
	}

	// Verify password
	if err := services.Auth.VerifyPassword(user.Password, req.Password); err != nil {
		// Use atomic database increment to prevent race conditions with concurrent login attempts
		if err := db.Model(&models.User{}).
			Where("id = ?", user.ID).
			Update("failed_attempts", gorm.Expr("failed_attempts + 1")).Error; err != nil {
			logger.Error("Failed to increment login attempts for user %d: %v", user.ID, err)
		}

		// Re-read to get updated count and lock if threshold reached
		var updated models.User
		if err := db.First(&updated, user.ID).Error; err == nil {
			// Extract client IP BEFORE any goroutines (Gin context is not goroutine-safe)
			clientIP := c.ClientIP()

			if updated.FailedAttempts >= 5 {
				lockedUntil := time.Now().Add(15 * time.Minute)
				db.Model(&models.User{}).Where("id = ?", user.ID).
					Update("locked_until", &lockedUntil)

				// Send account-locked security email (non-blocking)
				if services.Email != nil {
					go func() {
						custom := map[string]interface{}{
							"AttemptCount":    fmt.Sprintf("%d", updated.FailedAttempts),
							"IPAddress":       clientIP,
							"Timestamp":       time.Now().UTC().Format("2006-01-02 15:04:05"),
							"UnlockTime":      lockedUntil.UTC().Format("2006-01-02 15:04:05"),
							"LockoutDuration": "15 minutes",
							"ActionURL":       services.Email.FrontendURL() + "/forgot-password",
						}
						if err := services.Email.SendNotification(user.Email, user.Username, "Your Sentrix Account Has Been Locked", "security-account-locked", custom); err != nil {
							logger.Error("Failed to send account-locked email to %s: %v", user.Email, err)
						}
					}()
				}
			} else if updated.FailedAttempts >= 3 {
				// Send failed-logins warning email at 3+ attempts (non-blocking)
				if services.Email != nil {
					go func() {
						custom := map[string]interface{}{
							"AttemptCount":     fmt.Sprintf("%d", updated.FailedAttempts),
							"IPAddress":        clientIP,
							"Timestamp":        time.Now().UTC().Format("2006-01-02 15:04:05"),
							"Location":         "Unknown",
							"LockoutThreshold": "5",
							"ActionURL":        services.Email.FrontendURL() + "/settings",
						}
						if err := services.Email.SendNotification(user.Email, user.Username, "Suspicious Login Activity on Your Sentrix Account", "security-failed-logins", custom); err != nil {
							logger.Error("Failed to send failed-logins email to %s: %v", user.Email, err)
						}
					}()
				}
			}
		}

		writeAuditLogWithResult(c, "auth.login", "user", fmt.Sprintf("%d", user.ID),
			fmt.Sprintf("Login failed: invalid password for %s", user.Email), "failure")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not verified. Please check your email."})
		return
	}

	// Check if account is active
	if !user.IsActive {
		writeAuditLogWithResult(c, "auth.login", "user", fmt.Sprintf("%d", user.ID),
			fmt.Sprintf("Login rejected: account deactivated for %s", user.Email), "failure")
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is not active"})
		return
	}

	// Reset failed attempts atomically
	db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"failed_attempts": 0,
		"locked_until":    nil,
		"last_login":      time.Now(),
	})

	// Generate JWT token
	token, err := services.Auth.GenerateToken(&user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Store session
	services.Auth.StoreSession(
		user.ID,
		token,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)

	writeAuditLog(c, "auth.login", "user", fmt.Sprintf("%d", user.ID),
		fmt.Sprintf("Login successful for %s", user.Email))

	c.JSON(http.StatusOK, AuthResponse{
		Token:       token,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		User:        user.ToResponse(),
		Permissions: services.GetRolePermissionsStrings(user.Role),
	})
}

// Logout handles user logout
func Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid authorization header"})
		return
	}

	if err := services.Auth.InvalidateSession(parts[1]); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetCurrentUser returns the current authenticated user
func GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":        user.ToResponse(),
		"permissions": services.GetRolePermissionsStrings(user.Role),
	})
}
