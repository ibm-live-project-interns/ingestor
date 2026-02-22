package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Username  string `json:"username" binding:"required,min=3,max=50"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// VerifyEmailRequest represents the verify email request body
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
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
