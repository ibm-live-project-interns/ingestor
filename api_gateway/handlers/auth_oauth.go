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

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// oauthStateCookieName is the name of the cookie used to store OAuth state for CSRF validation.
const oauthStateCookieName = "oauth_state"

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
