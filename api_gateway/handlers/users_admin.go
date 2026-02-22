package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// DeleteUser soft deletes a user
func DeleteUser(c *gin.Context) {
	// Verify admin access
	if !isAdminRole(c) {
		apiErr := errors.NewInsufficientRole(string(rbac.RoleSysAdmin))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewValidation("Invalid user ID: must be a positive integer")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Prevent admin from deleting themselves
	currentUserID, _ := c.Get("userID")
	if currentUID, ok := currentUserID.(uint); ok && currentUID == uint(id) {
		apiErr := errors.NewBadRequest("Cannot delete your own account")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := userRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: User %d deleted", id)
			writeAuditLog(c, "user.delete", "user", fmt.Sprintf("%d", id),
				fmt.Sprintf("User %d deleted (demo mode)", id))
			c.JSON(http.StatusOK, gin.H{
				"message": "User deleted successfully (demo mode)",
				"user_id": id,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Check if user exists
	user, err := repo.GetByID(uint(id))
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if user == nil {
		apiErr := errors.NewNotFound(fmt.Sprintf("user %d", id))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Soft delete the user
	if err := repo.SoftDelete(uint(id)); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Invalidate all sessions for the deleted user
	db := database.Get()
	if db != nil && db.DB != nil {
		sessionRepo := database.NewSessionRepository(db.DB)
		if err := sessionRepo.InvalidateAllForUser(uint(id)); err != nil {
			logger.Error("Failed to invalidate sessions for deleted user %d: %v", id, err)
		}
	}

	username, _ := c.Get("username")
	logger.Info("User %d deleted by %v", id, username)

	// Record deletion in the audit log for compliance tracking
	writeAuditLog(c, "user.delete", "user", fmt.Sprintf("%d", id),
		fmt.Sprintf("User %d (%s) deleted by %v", id, user.Email, username))

	// Send account-deactivated email notification (non-blocking)
	if services.Email != nil {
		go func() {
			adminUsername := fmt.Sprintf("%v", username)
			custom := map[string]interface{}{
				"Reason":        "Account deleted by administrator",
				"DeactivatedBy": adminUsername,
				"Timestamp":     time.Now().Format("Jan 2, 2006 3:04 PM"),
				"ActionURL":     fmt.Sprintf("%s/login", services.Email.FrontendURL()),
			}
			subject := "Your account has been deactivated"
			if err := services.Email.SendNotification(user.Email, user.Username, subject, "account-deactivated", custom); err != nil {
				logger.Warn("Failed to send account-deactivated email to %s: %v", user.Email, err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
		"user_id": id,
	})
}

// ResetUserPassword resets a user's password (admin action)
func ResetUserPassword(c *gin.Context) {
	// Verify admin access
	if !isAdminRole(c) {
		apiErr := errors.NewInsufficientRole(string(rbac.RoleSysAdmin))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewValidation("Invalid user ID: must be a positive integer")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Optional: accept a new password in the request body
	var req struct {
		NewPassword string `json:"new_password,omitempty"`
	}
	// Bind is optional - if no body, generate a random password
	_ = c.ShouldBindJSON(&req)

	repo := userRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: Password reset for user %d", id)
			writeAuditLog(c, "user.password_reset", "user", fmt.Sprintf("%d", id),
				fmt.Sprintf("Admin password reset for user %d (demo mode)", id))
			c.JSON(http.StatusOK, gin.H{
				"message":            "Password reset successfully (demo mode)",
				"user_id":            id,
				"password_generated": true,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Check if user exists
	user, err := repo.GetByID(uint(id))
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if user == nil {
		apiErr := errors.NewNotFound(fmt.Sprintf("user %d", id))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Generate or use provided password
	newPassword := req.NewPassword
	isGenerated := false
	if newPassword == "" {
		// Generate a random temporary password
		passwordBytes := make([]byte, 16)
		if _, err := rand.Read(passwordBytes); err != nil {
			apiErr := errors.NewInternal("Failed to generate temporary password")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		newPassword = hex.EncodeToString(passwordBytes)[:16]
		isGenerated = true
	}

	// Validate password length
	if len(newPassword) < 8 {
		apiErr := errors.NewValidation("Password must be at least 8 characters long")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Hash the new password
	hashedPassword, err := services.Auth.HashPassword(newPassword)
	if err != nil {
		apiErr := errors.NewInternal("Failed to hash password")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Update the user's password and reset any lockout
	user.Password = hashedPassword
	user.FailedAttempts = 0
	user.LockedUntil = nil
	user.ResetToken = ""
	user.ResetTokenExp = nil

	if err := repo.Update(user); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Invalidate all existing sessions for security
	db := database.Get()
	if db != nil && db.DB != nil {
		sessionRepo := database.NewSessionRepository(db.DB)
		if err := sessionRepo.InvalidateAllForUser(uint(id)); err != nil {
			logger.Error("Failed to invalidate sessions for user %d after password reset: %v", id, err)
		}
	}

	username, _ := c.Get("username")
	logger.Info("Password reset for user %d by %v", id, username)

	// Record password reset in the audit log for compliance tracking
	writeAuditLog(c, "user.password_reset", "user", fmt.Sprintf("%d", id),
		fmt.Sprintf("Admin password reset for user %d (%s) by %v", id, user.Email, username))

	// Send password-reset-by-admin email notification (non-blocking)
	if services.Email != nil {
		go func() {
			adminUsername := fmt.Sprintf("%v", username)
			custom := map[string]interface{}{
				"ResetBy":   adminUsername,
				"Timestamp": time.Now().Format("Jan 2, 2006 3:04 PM"),
				"ActionURL": fmt.Sprintf("%s/login", services.Email.FrontendURL()),
			}
			if isGenerated {
				custom["TempPassword"] = newPassword
			}
			subject := "Your password has been reset by an administrator"
			if err := services.Email.SendNotification(user.Email, user.Username, subject, "password-reset-by-admin", custom); err != nil {
				logger.Warn("Failed to send password-reset-by-admin email to %s: %v", user.Email, err)
			}
		}()
	}

	response := gin.H{
		"message": "Password reset successfully",
		"user_id": id,
	}

	// Never include the password in the API response -- it is sent via email
	// if the email service is configured. Only indicate whether one was generated.
	if isGenerated {
		response["password_generated"] = true
	}

	c.JSON(http.StatusOK, response)
}
