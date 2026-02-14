package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
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
// Reads from DEMO_PASSWORD env var; falls back to a default for development only.
func getDemoPassword() string {
	if pw := config.GetEnv("DEMO_PASSWORD", ""); pw != "" {
		return pw
	}
	return "admin123"
}

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents the reset password request body
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// VerifyEmailRequest represents the verify email request body
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
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

// Register handles user registration
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Check if username already exists
	if err := db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
		return
	}

	// Hash password
	hashedPassword, err := services.Auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Generate verification token
	verificationToken, err := services.Auth.GenerateVerificationToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	// Create user with verification token valid for 24 hours
	verificationTokenExp := time.Now().Add(24 * time.Hour)
	user := models.User{
		Email:                req.Email,
		Username:             req.Username,
		Password:             hashedPassword,
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		Role:                 "network-ops", // Default role
		IsActive:             false,
		EmailVerified:        false,
		VerificationToken:    verificationToken,
		VerificationTokenExp: &verificationTokenExp,
	}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Send verification email
	if services.Email != nil {
		if err := services.Email.SendVerificationEmail(user.Email, user.Username, verificationToken); err != nil {
			logger.Warn("Failed to send verification email: %v", err)
			// Don't fail registration, just log the error
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Registration successful. Please check your email to verify your account.",
		"user":    user.ToResponse(),
	})
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
		// Demo mode: still require the demo password to prevent open access
		if req.Password != getDemoPassword() {
			// Log failed demo login attempt
			logger.Warn("Demo mode: failed login attempt for %s (wrong password)", req.Email)
			writeAuditLogWithResult(c, "auth.login", "user", req.Email,
				fmt.Sprintf("Failed demo login for %s: invalid password", req.Email), "failure")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		logger.Info("Demo mode: logging in as demo user for %s", req.Email)
		now := time.Now()

		// Map common emails to roles for demo convenience
		demoRole := "sysadmin"
		demoName := "Demo Admin"
		if strings.Contains(req.Email, "ops") || strings.Contains(req.Email, "noc") {
			demoRole = "network-ops"
			demoName = "NOC Operator"
		} else if strings.Contains(req.Email, "sre") {
			demoRole = "sre"
			demoName = "SRE Engineer"
		} else if strings.Contains(req.Email, "network") {
			demoRole = "network-admin"
			demoName = "Network Admin"
		} else if strings.Contains(req.Email, "senior") || strings.Contains(req.Email, "eng") {
			demoRole = "senior-eng"
			demoName = "Senior Engineer"
		}

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
			if updated.FailedAttempts >= 5 {
				lockedUntil := time.Now().Add(15 * time.Minute)
				db.Model(&models.User{}).Where("id = ?", user.ID).
					Update("locked_until", &lockedUntil)

				// Send account-locked security email (non-blocking)
				if services.Email != nil {
					go func() {
						custom := map[string]interface{}{
							"AttemptCount":    fmt.Sprintf("%d", updated.FailedAttempts),
							"IPAddress":       c.ClientIP(),
							"Timestamp":       time.Now().UTC().Format("2006-01-02 15:04:05"),
							"UnlockTime":      lockedUntil.UTC().Format("2006-01-02 15:04:05"),
							"LockoutDuration": "15 minutes",
							"ActionURL":       services.Email.FrontendURL() + "/forgot-password",
						}
						if err := services.Email.SendNotification(user.Email, user.Username, "Your Sentrix Account Has Been Locked", "security-account-locked", custom); err != nil {
							logger.Warn("Failed to send account-locked email to %s: %v", user.Email, err)
						}
					}()
				}
			} else if updated.FailedAttempts >= 3 {
				// Send failed-logins warning email at 3+ attempts (non-blocking)
				if services.Email != nil {
					go func() {
						custom := map[string]interface{}{
							"AttemptCount":     fmt.Sprintf("%d", updated.FailedAttempts),
							"IPAddress":        c.ClientIP(),
							"Timestamp":        time.Now().UTC().Format("2006-01-02 15:04:05"),
							"Location":         "Unknown",
							"LockoutThreshold": "5",
							"ActionURL":        services.Email.FrontendURL() + "/settings",
						}
						if err := services.Email.SendNotification(user.Email, user.Username, "Suspicious Login Activity on Your Sentrix Account", "security-failed-logins", custom); err != nil {
							logger.Warn("Failed to send failed-logins email to %s: %v", user.Email, err)
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

// VerifyEmail handles email verification
func VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		var req VerifyEmailRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required"})
			return
		}
		token = req.Token
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Find user by verification token
	var user models.User
	if err := db.Where("verification_token = ?", token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification token"})
		return
	}

	// Check if verification token has expired
	if user.VerificationTokenExp != nil && user.VerificationTokenExp.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token has expired. Please request a new one."})
		return
	}

	// Verify user
	user.EmailVerified = true
	user.IsActive = true
	user.VerificationToken = ""
	verifiedAt := time.Now()
	user.VerifiedAt = &verifiedAt

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	// Send welcome email
	if services.Email != nil {
		if err := services.Email.SendWelcomeEmail(user.Email, user.Username); err != nil {
			logger.Warn("Failed to send welcome email: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully. You can now log in.",
		"user":    user.ToResponse(),
	})
}

// ForgotPassword initiates password reset flow
func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Find user by email
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a password reset link will be sent."})
		return
	}

	// Generate reset token
	resetToken, err := services.Auth.GenerateResetToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Save reset token
	user.ResetToken = resetToken
	resetTokenExp := time.Now().Add(1 * time.Hour)
	user.ResetTokenExp = &resetTokenExp
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save reset token"})
		return
	}

	// Send password reset email
	if services.Email != nil {
		if err := services.Email.SendPasswordResetEmail(user.Email, user.Username, resetToken); err != nil {
			logger.Warn("Failed to send password reset email: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a password reset link will be sent."})
}

// ResetPassword handles password reset
func ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Find user by reset token
	var user models.User
	if err := db.Where("reset_token = ?", req.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	// Check if token is expired
	if user.ResetTokenExp == nil || user.ResetTokenExp.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reset token has expired"})
		return
	}

	// Hash new password
	hashedPassword, err := services.Auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	// Update password and clear reset token
	user.Password = hashedPassword
	user.ResetToken = ""
	user.ResetTokenExp = nil
	user.FailedAttempts = 0
	user.LockedUntil = nil

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	writeAuditLog(c, "auth.password_reset", "user", fmt.Sprintf("%d", user.ID),
		fmt.Sprintf("Password reset via token for %s", user.Email))

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully. You can now log in with your new password."})
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

// ResendVerification resends the verification email
func ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	// Find user by email
	var user models.User
	if err := db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists and is unverified, a verification link will be sent."})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email is already verified"})
		return
	}

	// Generate new verification token
	verificationToken, err := services.Auth.GenerateVerificationToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification token"})
		return
	}

	user.VerificationToken = verificationToken
	verificationTokenExp := time.Now().Add(24 * time.Hour)
	user.VerificationTokenExp = &verificationTokenExp
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save verification token"})
		return
	}

	// Send verification email
	if services.Email != nil {
		if err := services.Email.SendVerificationEmail(user.Email, user.Username, verificationToken); err != nil {
			logger.Warn("Failed to send verification email: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists and is unverified, a verification link will be sent."})
}

// generateOAuthState generates a random state for OAuth
func generateOAuthState() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// validateRedirectURL validates a redirect URL against allowed frontend origins.
// Prevents open redirect attacks by only allowing redirects to known frontend URLs.
func validateRedirectURL(redirectURL string) string {
	frontendURL := config.GetEnv("FRONTEND_URL", "http://localhost:5173")
	defaultRedirect := frontendURL + "/dashboard"

	if redirectURL == "" {
		return defaultRedirect
	}

	// Parse the provided redirect URL
	parsed, err := url.Parse(redirectURL)
	if err != nil {
		logger.Warn("Invalid redirect URL rejected: %s", redirectURL)
		return defaultRedirect
	}

	// Allow relative paths (no scheme/host)
	if parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(redirectURL, "/") {
		return frontendURL + redirectURL
	}

	// Build allowed origins list from FRONTEND_URL and CORS origins
	allowedOrigins := []string{frontendURL}
	corsOrigins := config.GetEnv("CORS_ALLOWED_ORIGINS", "")
	if corsOrigins != "" {
		for _, origin := range strings.Split(corsOrigins, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins = append(allowedOrigins, origin)
			}
		}
	}

	// Check if the redirect URL matches an allowed origin
	redirectOrigin := parsed.Scheme + "://" + parsed.Host
	for _, allowed := range allowedOrigins {
		allowedParsed, err := url.Parse(strings.TrimSpace(allowed))
		if err != nil {
			continue
		}
		allowedOrigin := allowedParsed.Scheme + "://" + allowedParsed.Host
		if redirectOrigin == allowedOrigin {
			return redirectURL
		}
	}

	logger.Warn("Redirect URL rejected (not in allowed origins): %s", redirectURL)
	return defaultRedirect
}

// oauthStateCookieName is the name of the cookie used to store OAuth state for CSRF validation.
const oauthStateCookieName = "oauth_state"

// GoogleLogin initiates Google OAuth login
func GoogleLogin(c *gin.Context) {
	if services.Google == nil || !services.Google.IsEnabled() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Google OAuth is not configured"})
		return
	}

	// Get redirect URL from query param, validated against allowed origins
	redirectURL := c.Query("redirect")
	redirectURL = validateRedirectURL(redirectURL)

	// Generate state with redirect URL encoded
	state := generateOAuthState() + ":" + url.QueryEscape(redirectURL)

	// Store OAuth state in a secure HTTP-only cookie to validate on callback and prevent CSRF attacks
	isSecure := strings.HasPrefix(config.GetEnv("FRONTEND_URL", "http://localhost:5173"), "https")
	c.SetCookie(
		oauthStateCookieName, // name
		state,                // value
		600,                  // maxAge: 10 minutes
		"/",                  // path
		"",                   // domain (auto from request)
		isSecure,             // secure
		true,                 // httpOnly
	)

	// Get Google authorization URL
	authURL := services.Google.GetAuthURL(state)

	// Redirect to Google
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// GoogleCallback handles the Google OAuth callback
func GoogleCallback(c *gin.Context) {
	if services.Google == nil || !services.Google.IsEnabled() {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "Google OAuth is not configured"})
		return
	}

	frontendURL := config.GetEnv("FRONTEND_URL", "http://localhost:5173")
	defaultRedirect := frontendURL + "/dashboard"

	// Check for error from Google
	if errParam := c.Query("error"); errParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google OAuth error: " + errParam})
		return
	}

	// Get authorization code
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	// Validate OAuth state against cookie to prevent CSRF attacks
	stateParam := c.Query("state")
	stateCookie, cookieErr := c.Cookie(oauthStateCookieName)
	if cookieErr != nil || stateCookie == "" {
		logger.Warn("OAuth callback: missing state cookie (possible CSRF)")
		c.Redirect(http.StatusTemporaryRedirect, defaultRedirect+"?error="+url.QueryEscape("OAuth state validation failed"))
		return
	}
	if stateParam != stateCookie {
		logger.Warn("OAuth callback: state mismatch (possible CSRF). param=%s cookie=%s", stateParam, stateCookie)
		c.Redirect(http.StatusTemporaryRedirect, defaultRedirect+"?error="+url.QueryEscape("OAuth state validation failed"))
		return
	}

	// Clear the state cookie now that it's been validated
	isSecure := strings.HasPrefix(frontendURL, "https")
	c.SetCookie(oauthStateCookieName, "", -1, "/", "", isSecure, true)

	// Parse state to get redirect URL, then validate against allowed origins
	var rawRedirect string
	if parts := strings.SplitN(stateParam, ":", 2); len(parts) == 2 {
		decoded, err := url.QueryUnescape(parts[1])
		if err == nil {
			rawRedirect = decoded
		}
	}
	frontendRedirect := validateRedirectURL(rawRedirect)

	// Exchange code for token
	token, err := services.Google.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		logger.Error("Failed to exchange Google code: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error="+url.QueryEscape("Failed to authenticate with Google"))
		return
	}

	// Get user info from Google
	userInfo, err := services.Google.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		logger.Error("Failed to get Google user info: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error="+url.QueryEscape("Failed to get user info from Google"))
		return
	}

	var user models.User
	db := database.Get()

	if db == nil {
		// Demo mode - create an in-memory user from Google info
		logger.Info("Demo mode: Creating in-memory user for Google OAuth: %s", userInfo.Email)
		user = models.User{
			ID:            1,
			Email:         userInfo.Email,
			Username:      strings.Split(userInfo.Email, "@")[0],
			FirstName:     userInfo.GivenName,
			LastName:      userInfo.FamilyName,
			GoogleID:      userInfo.ID,
			Role:          "network-ops",
			IsActive:      true,
			EmailVerified: true,
		}
	} else {
		// Find or create user in database
		result := db.Where("email = ?", userInfo.Email).First(&user)

		if result.Error != nil {
			// User doesn't exist, create new user
			user = models.User{
				Email:         userInfo.Email,
				Username:      strings.Split(userInfo.Email, "@")[0],
				FirstName:     userInfo.GivenName,
				LastName:      userInfo.FamilyName,
				GoogleID:      userInfo.ID,
				Role:          "network-ops",
				IsActive:      true,
				EmailVerified: true,
			}

			// Check if username exists, append random suffix if so
			var existing models.User
			if db.Where("username = ?", user.Username).First(&existing).Error == nil {
				user.Username = user.Username + "_" + generateOAuthState()[:6]
			}

			if err := db.Create(&user).Error; err != nil {
				logger.Error("Failed to create Google user: %v", err)
				c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error="+url.QueryEscape("Failed to create user account"))
				return
			}

			// Send welcome email for new Google OAuth users
			if services.Email != nil {
				if err := services.Email.SendWelcomeEmail(user.Email, user.Username); err != nil {
					logger.Warn("Failed to send welcome email to Google OAuth user: %v", err)
				}
			}
		} else {
			// Update existing user with Google info if not already set
			if user.GoogleID == "" {
				user.GoogleID = userInfo.ID
			}
			if user.FirstName == "" {
				user.FirstName = userInfo.GivenName
			}
			if user.LastName == "" {
				user.LastName = userInfo.FamilyName
			}
			user.EmailVerified = true
			lastLogin := time.Now()
			user.LastLogin = &lastLogin
			db.Save(&user)
		}
	}

	// Reject deactivated accounts even when using OAuth login
	if !user.IsActive {
		logger.Warn("OAuth login rejected: account deactivated for %s", user.Email)
		writeAuditLogWithResult(c, "auth.oauth_login", "user", fmt.Sprintf("%d", user.ID),
			fmt.Sprintf("OAuth login rejected: account deactivated for %s", user.Email), "failure")
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error="+url.QueryEscape("Account deactivated. Please contact an administrator."))
		return
	}

	// Generate JWT token
	jwtToken, err := services.Auth.GenerateToken(&user)
	if err != nil {
		logger.Error("Failed to generate JWT for Google user: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error="+url.QueryEscape("Failed to generate authentication token"))
		return
	}

	// Store session (only if db available)
	if db != nil {
		services.Auth.StoreSession(
			user.ID,
			jwtToken,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
	}

	logger.Info("Google OAuth successful for user: %s", user.Email)

	writeAuditLog(c, "auth.oauth_login", "user", fmt.Sprintf("%d", user.ID),
		fmt.Sprintf("Google OAuth login successful for %s", user.Email))

	// Redirect to frontend login page with token as URL parameter.
	// The frontend's LoginPage component reads ?token= and calls setOAuthToken().
	// Use the origin from the validated redirect URL (preserves the port the user came from).
	redirectParsed, parseErr := url.Parse(frontendRedirect)
	redirectBase := frontendURL // fallback
	if parseErr == nil && redirectParsed.Scheme != "" && redirectParsed.Host != "" {
		redirectBase = redirectParsed.Scheme + "://" + redirectParsed.Host
	}
	loginRedirect := redirectBase + "/login?token=" + url.QueryEscape(jwtToken)
	c.Redirect(http.StatusTemporaryRedirect, loginRedirect)
}
