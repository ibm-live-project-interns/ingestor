package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// emailRegex is a basic email validation pattern
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// UpdateProfileRequest represents the request body for updating own profile
type UpdateProfileRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
}

// ChangePasswordRequest represents the request body for changing own password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// getAuthenticatedUserID extracts and returns the authenticated user's ID from the Gin context.
// Returns 0 and false if the user ID is not present or not a valid uint.
func getAuthenticatedUserID(c *gin.Context) (uint, bool) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		return 0, false
	}
	userID, ok := userIDVal.(uint)
	if !ok {
		return 0, false
	}
	return userID, true
}

// getDemoProfileUser returns a demo user for profile endpoints when the database is unavailable
func getDemoProfileUser() models.UserResponse {
	now := time.Now()
	lastLogin := now.Add(-2 * time.Hour)
	return models.UserResponse{
		ID:            1,
		Email:         "admin@example.com",
		Username:      "admin",
		FirstName:     "Demo",
		LastName:      "User",
		Role:          "network-ops",
		IsActive:      true,
		EmailVerified: true,
		LastLogin:     &lastLogin,
		CreatedAt:     now.Add(-90 * 24 * time.Hour),
	}
}

// UpdateProfile handles PUT /api/v1/me - update the authenticated user's own profile
func UpdateProfile(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		apiErr := errors.NewUnauthorized("Not authenticated")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Trim whitespace from input fields
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.TrimSpace(req.Email)

	// Validate email format if provided
	if req.Email != "" && !emailRegex.MatchString(req.Email) {
		apiErr := errors.NewValidation("Invalid email format")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate name lengths
	if req.FirstName != "" && len(req.FirstName) > 100 {
		apiErr := errors.NewValidation("First name must be 100 characters or fewer")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if req.LastName != "" && len(req.LastName) > 100 {
		apiErr := errors.NewValidation("Last name must be 100 characters or fewer")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := userRepo()
	if repo == nil {
		// Demo mode - return success with mock data
		logger.Info("Demo mode: Profile updated for user %d", userID)
		demoUser := getDemoProfileUser()
		if req.FirstName != "" {
			demoUser.FirstName = req.FirstName
		}
		if req.LastName != "" {
			demoUser.LastName = req.LastName
		}
		if req.Email != "" {
			demoUser.Email = req.Email
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "Profile updated successfully (demo mode)",
			"user":    demoUser,
		})
		return
	}

	// Fetch the current user
	user, err := repo.GetByID(userID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if user == nil {
		apiErr := errors.NewNotFound("user")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check email uniqueness if the email is being changed
	if req.Email != "" && req.Email != user.Email {
		existing, err := repo.GetByEmail(req.Email)
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		if existing != nil {
			apiErr := errors.NewDuplicateEntry("email")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	// Build the updates map with only provided fields
	updates := make(map[string]interface{})
	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}

	if len(updates) == 0 {
		apiErr := errors.NewBadRequest("No fields to update")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := repo.UpdateFields(userID, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("User %d updated their own profile", userID)

	// Fetch the updated user to return fresh data
	updatedUser, err := repo.GetByID(userID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user":    updatedUser.ToResponse(),
	})
}

// ChangePassword handles PUT /api/v1/me/password - change the authenticated user's own password
func ChangePassword(c *gin.Context) {
	userID, ok := getAuthenticatedUserID(c)
	if !ok {
		apiErr := errors.NewUnauthorized("Not authenticated")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate that new password and confirm password match
	if req.NewPassword != req.ConfirmPassword {
		apiErr := errors.NewValidation("New password and confirmation do not match")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate password strength (same rules as registration)
	if err := validatePasswordStrength(req.NewPassword); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Prevent reusing the same password
	if req.CurrentPassword == req.NewPassword {
		apiErr := errors.NewValidation("New password must be different from current password")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := userRepo()
	if repo == nil {
		// Demo mode - return success
		logger.Info("Demo mode: Password changed for user %d", userID)
		c.JSON(http.StatusOK, gin.H{
			"message": "Password changed successfully (demo mode)",
		})
		return
	}

	// Fetch the current user (need password hash for verification)
	user, err := repo.GetByID(userID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if user == nil {
		apiErr := errors.NewNotFound("user")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Verify the current password using bcrypt
	if err := services.Auth.VerifyPassword(user.Password, req.CurrentPassword); err != nil {
		apiErr := errors.NewValidation("Current password is incorrect")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Hash the new password
	hashedPassword, err := services.Auth.HashPassword(req.NewPassword)
	if err != nil {
		apiErr := errors.NewInternal("Failed to hash password")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Update the user's password
	user.Password = hashedPassword
	if err := repo.Update(user); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Invalidate all other sessions for this user (security best practice)
	// The current session remains valid; the user stays logged in on this device.
	db := database.Get()
	if db != nil && db.DB != nil {
		sessionRepo := database.NewSessionRepository(db.DB)
		if err := sessionRepo.InvalidateAllForUser(userID); err != nil {
			logger.Error("Failed to invalidate sessions for user %d after password change: %v", userID, err)
			// Non-fatal: password was already changed successfully
		}
	}

	logger.Info("User %d changed their password", userID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}
