package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
	"golang.org/x/crypto/bcrypt"
)

// CustomClaims extends JWT claims with custom fields
type CustomClaims struct {
	UserID      uint              `json:"user_id"`
	Username    string            `json:"username"`
	Email       string            `json:"email"`
	Role        string            `json:"role"`
	Permissions []rbac.Permission `json:"permissions"`
	jwt.RegisteredClaims
}

// Validate implements jwt.ClaimsValidator interface for custom validation
func (c CustomClaims) Validate() error {
	// Validate role using shared rbac
	if !rbac.IsValidRole(c.Role) {
		return fmt.Errorf("invalid role: %s", c.Role)
	}

	// Validate username
	if c.Username == "" {
		return errors.New("username is required in claims")
	}

	// Validate email
	if c.Email == "" {
		return errors.New("email is required in claims")
	}

	return nil
}

// AuthService handles authentication operations
type AuthService struct {
	jwtSecret []byte
	jwtExpiry time.Duration
}

var Auth *AuthService

// InitAuthService initializes the authentication service
func InitAuthService() error {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return errors.New("JWT_SECRET environment variable is required")
	}

	jwtExpiryHours := 24
	Auth = &AuthService{
		jwtSecret: []byte(jwtSecret),
		jwtExpiry: time.Duration(jwtExpiryHours) * time.Hour,
	}

	return nil
}

// HashPassword hashes a password using bcrypt with cost 12
func (a *AuthService) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// VerifyPassword verifies a password against a hash
func (a *AuthService) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// GenerateToken generates a JWT token for a user
func (a *AuthService) GenerateToken(user *models.User) (string, error) {
	now := time.Now()

	jti, err := generateJTI()
	if err != nil {
		return "", fmt.Errorf("failed to generate token id: %w", err)
	}

	claims := CustomClaims{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Role:        user.Role,
		Permissions: rbac.GetRolePermissions(rbac.RoleID(user.Role)),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "noc-dashboard",
			Subject:   fmt.Sprintf("%d", user.ID),
			Audience:  []string{"noc-dashboard-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.jwtExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(a.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (a *AuthService) ValidateToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&CustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Validate signing method
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return a.jwtSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("noc-dashboard"),
		jwt.WithAudience("noc-dashboard-api"),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		// Handle specific JWT errors
		if errors.Is(err, jwt.ErrTokenMalformed) {
			return nil, errors.New("malformed token")
		}
		if errors.Is(err, jwt.ErrTokenSignatureInvalid) {
			return nil, errors.New("invalid signature")
		}
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		if errors.Is(err, jwt.ErrTokenNotValidYet) {
			return nil, errors.New("token not valid yet")
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

// GenerateVerificationToken generates a random token for email verification
func (a *AuthService) GenerateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateResetToken generates a random token for password reset
func (a *AuthService) GenerateResetToken() (string, error) {
	return a.GenerateVerificationToken()
}

// generateJTI generates a unique JWT ID.
// Returns an error if the underlying crypto/rand read fails so callers
// never silently generate a weak or zero-valued JTI.
func generateJTI() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate JTI: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GetRolePermissionsStrings returns permissions for a role as strings (for API responses)
func GetRolePermissionsStrings(role string) []string {
	perms := rbac.GetRolePermissions(rbac.RoleID(role))
	result := make([]string, len(perms))
	for i, p := range perms {
		result[i] = string(p)
	}
	return result
}

// StoreSession stores a session in the database
func (a *AuthService) StoreSession(userID uint, token, ipAddress, userAgent string) error {
	session := models.Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(a.jwtExpiry),
		IPAddress: ipAddress,
		UserAgent: userAgent,
		IsActive:  true,
	}

	db := database.Get()
	if db == nil {
		return errors.New("database not initialized")
	}

	if err := db.Create(&session).Error; err != nil {
		return fmt.Errorf("failed to store session: %w", err)
	}

	return nil
}

// InvalidateSession invalidates a session (logout)
func (a *AuthService) InvalidateSession(token string) error {
	db := database.Get()
	if db == nil {
		return errors.New("database not initialized")
	}

	result := db.Model(&models.Session{}).
		Where("token = ?", token).
		Update("is_active", false)

	if result.Error != nil {
		return fmt.Errorf("failed to invalidate session: %w", result.Error)
	}

	return nil
}
