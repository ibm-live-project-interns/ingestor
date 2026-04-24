package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest represents the reset password request body
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
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

	// Invalidate any pre-existing reset token so only the newest token works.
	db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{
		"reset_token":     "",
		"reset_token_exp": nil,
	})

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
			logger.Error("Failed to send password reset email: %v", err)
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

	// Find user by reset token. Use the same error message for both
	// "not found" and "expired" to avoid leaking token-validity status.
	var user models.User
	if err := db.Where("reset_token = ?", req.Token).First(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired password reset token"})
		return
	}

	// Check if token is expired
	if user.ResetTokenExp == nil || user.ResetTokenExp.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired password reset token"})
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
