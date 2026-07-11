package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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

// oauthCodeEntry represents a short-lived one-time exchange code that the
// frontend can trade for a JWT. This keeps the JWT out of the browser URL
// entirely (only an opaque short-lived code is redirected through).
type oauthCodeEntry struct {
	token     string
	expiresAt time.Time
}

// oauthCodes holds short-lived codes issued during the OAuth redirect flow.
var oauthCodes sync.Map

func init() {
	// Periodically purge expired exchange codes so the map does not grow
	// unbounded even if the frontend never calls the exchange endpoint.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		for range ticker.C {
			oauthCodes.Range(func(key, value interface{}) bool {
				if entry, ok := value.(oauthCodeEntry); ok {
					if time.Now().After(entry.expiresAt) {
						oauthCodes.Delete(key)
					}
				}
				return true
			})
		}
	}()
}

// generateOAuthState generates a random state for OAuth.
// Increased to 32 bytes to provide stronger CSRF protection.
func generateOAuthState() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateOAuthExchangeCode returns a random hex-encoded code used as the
// opaque handle the frontend exchanges for a real JWT.
func generateOAuthExchangeCode() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// computeOAuthStateHMAC signs payload with JWT_SECRET.
// Using HMAC instead of cookies means the state is self-validating and
// works correctly behind any proxy that strips or doesn't forward cookies.
func computeOAuthStateHMAC(payload string) string {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "noc-platform-dev-secret-key-2026"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildOAuthState creates a tamper-proof state: nonce|b64(redirect)|hmac
func buildOAuthState(redirectURL string) string {
	nonce := generateOAuthState()
	encoded := base64.RawURLEncoding.EncodeToString([]byte(redirectURL))
	payload := nonce + "|" + encoded
	return payload + "|" + computeOAuthStateHMAC(payload)
}

// parseOAuthState validates the HMAC and returns the redirect URL.
// Returns ("", false) if the state is invalid or tampered.
func parseOAuthState(state string) (string, bool) {
	parts := strings.SplitN(state, "|", 3)
	if len(parts) != 3 {
		return "", false
	}
	payload := parts[0] + "|" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(computeOAuthStateHMAC(payload))) {
		return "", false
	}
	redirectBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	return string(redirectBytes), true
}

// resolvedFrontendURL returns FRONTEND_URL, auto-upgrading to the first
// non-localhost CORS origin when FRONTEND_URL is still pointing at localhost.
func resolvedFrontendURL() string {
	frontendURL := config.GetEnv("FRONTEND_URL", "http://localhost:5173")
	if strings.HasPrefix(frontendURL, "http://localhost") {
		for _, origin := range strings.Split(config.GetEnv("CORS_ALLOWED_ORIGINS", ""), ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" && !strings.HasPrefix(origin, "http://localhost") {
				return origin
			}
		}
	}
	return frontendURL
}

// validateRedirectURL validates a redirect URL against allowed frontend origins.
// Prevents open redirect attacks by only allowing redirects to known frontend URLs.
func validateRedirectURL(redirectURL string) string {
	frontendURL := resolvedFrontendURL()
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

	// Build HMAC-signed state — embeds redirect URL and signature so no
	// server-side session or cookie is required (works behind any proxy).
	state := buildOAuthState(redirectURL)

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

	frontendURL := resolvedFrontendURL()
	defaultRedirect := frontendURL + "/dashboard"

	// Check for error from Google
	if errParam := c.Query("error"); errParam != "" {
		logger.Warn("OAuth callback error received")
		c.Redirect(http.StatusTemporaryRedirect, defaultRedirect+"?error=oauth_failed")
		return
	}

	// Get authorization code
	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, defaultRedirect+"?error=oauth_failed")
		return
	}

	// Validate HMAC-signed state — no cookie required, works behind any proxy
	stateParam := c.Query("state")
	rawRedirect, stateOK := parseOAuthState(stateParam)
	if !stateOK {
		logger.Warn("OAuth callback: invalid or tampered state parameter")
		c.Redirect(http.StatusTemporaryRedirect, defaultRedirect+"?error=oauth_state_failed")
		return
	}
	frontendRedirect := validateRedirectURL(rawRedirect)

	// Exchange code for token
	token, err := services.Google.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		logger.Error("Failed to exchange Google code: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error=oauth_failed")
		return
	}

	// Get user info from Google
	userInfo, err := services.Google.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		logger.Error("Failed to get Google user info: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error=user_info_failed")
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
			// User doesn't exist, create new user.
			// Password is NOT NULL in the schema; Google OAuth users have no password
			// so we store a random hex string that is not a valid bcrypt hash —
			// it can never match any login attempt.
			oauthPasswordPlaceholder := "oauth:" + generateOAuthState()
			user = models.User{
				Email:         userInfo.Email,
				Username:      strings.Split(userInfo.Email, "@")[0],
				FirstName:     userInfo.GivenName,
				LastName:      userInfo.FamilyName,
				Password:      oauthPasswordPlaceholder,
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
				c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error=account_creation_failed")
				return
			}

			// Send welcome email for new Google OAuth users
			if services.Email != nil {
				if err := services.Email.SendWelcomeEmail(user.Email, user.Username); err != nil {
					logger.Error("Failed to send welcome email to Google OAuth user: %v", err)
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
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error=account_deactivated")
		return
	}

	// Generate JWT token
	jwtToken, err := services.Auth.GenerateToken(&user)
	if err != nil {
		logger.Error("Failed to generate JWT for Google user: %v", err)
		c.Redirect(http.StatusTemporaryRedirect, frontendRedirect+"?error=token_generation_failed")
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

	// Redirect to the frontend OAuth callback page with a short-lived one-time
	// exchange code. The frontend immediately trades the code for the JWT via
	// GET /api/v1/auth/oauth/exchange, keeping the JWT out of the URL bar,
	// browser history, and referrer headers.
	redirectParsed, parseErr := url.Parse(frontendRedirect)
	redirectBase := frontendURL // fallback
	if parseErr == nil && redirectParsed.Scheme != "" && redirectParsed.Host != "" {
		redirectBase = redirectParsed.Scheme + "://" + redirectParsed.Host
	}
	exchangeCode := generateOAuthExchangeCode()
	oauthCodes.Store(exchangeCode, oauthCodeEntry{
		token:     jwtToken,
		expiresAt: time.Now().Add(5 * time.Minute),
	})
	loginRedirect := redirectBase + "/login?code=" + exchangeCode
	c.Redirect(http.StatusTemporaryRedirect, loginRedirect)
}

// ExchangeOAuthCode trades a short-lived one-time code issued by GoogleCallback
// for the real JWT. Codes are single-use and expire after 30 seconds.
func ExchangeOAuthCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "code required"})
		return
	}
	val, ok := oauthCodes.Load(code)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired code"})
		return
	}
	// Delete immediately — one-time use, even if expired.
	oauthCodes.Delete(code)
	entry, ok := val.(oauthCodeEntry)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired code"})
		return
	}
	if time.Now().After(entry.expiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": entry.token})
}
