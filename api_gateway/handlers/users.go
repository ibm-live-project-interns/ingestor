package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// userRepo returns the user repository using the global database
// Returns nil if database is not available (demo mode)
func userRepo() *database.UserRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewUserRepository(db.DB)
}

// getDemoUsers returns demo users for when database is unavailable
func getDemoUsers() []models.UserResponse {
	now := time.Now()
	lastLogin := now.Add(-2 * time.Hour)
	return []models.UserResponse{
		{
			ID:            1,
			Email:         "admin@example.com",
			Username:      "admin",
			FirstName:     "System",
			LastName:      "Administrator",
			Role:          string(rbac.RoleSysAdmin),
			IsActive:      true,
			EmailVerified: true,
			LastLogin:     &lastLogin,
			CreatedAt:     now.Add(-90 * 24 * time.Hour),
		},
		{
			ID:            2,
			Email:         "john.smith@example.com",
			Username:      "jsmith",
			FirstName:     "John",
			LastName:      "Smith",
			Role:          string(rbac.RoleNetworkOps),
			IsActive:      true,
			EmailVerified: true,
			LastLogin:     &lastLogin,
			CreatedAt:     now.Add(-30 * 24 * time.Hour),
		},
		{
			ID:            3,
			Email:         "jane.doe@example.com",
			Username:      "jdoe",
			FirstName:     "Jane",
			LastName:      "Doe",
			Role:          string(rbac.RoleSRE),
			IsActive:      true,
			EmailVerified: true,
			CreatedAt:     now.Add(-15 * 24 * time.Hour),
		},
		{
			ID:            4,
			Email:         "bob.wilson@example.com",
			Username:      "bwilson",
			FirstName:     "Bob",
			LastName:      "Wilson",
			Role:          string(rbac.RoleNetworkAdmin),
			IsActive:      false,
			EmailVerified: false,
			CreatedAt:     now.Add(-5 * 24 * time.Hour),
		},
	}
}

// isAdminRole checks if the current user has the sysadmin role
func isAdminRole(c *gin.Context) bool {
	role, exists := c.Get("role")
	if !exists {
		return false
	}
	roleStr, ok := role.(string)
	if !ok {
		return false
	}
	return rbac.RoleID(roleStr) == rbac.RoleSysAdmin
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role,omitempty"`
	IsActive  *bool  `json:"is_active,omitempty"`
}

// GetUsers returns all users with optional filtering and pagination
func GetUsers(c *gin.Context) {
	// Verify admin access
	if !isAdminRole(c) {
		apiErr := errors.NewInsufficientRole(string(rbac.RoleSysAdmin))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := userRepo()
	if repo == nil {
		if isDemoMode() {
			demoUsers := getDemoUsers()
			logger.Info("Demo mode: returning demo users")
			c.JSON(http.StatusOK, gin.H{
				"users": demoUsers,
				"total": len(demoUsers),
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	filter := database.UserFilter{
		Search: c.Query("search"),
		Role:   c.Query("role"),
	}

	// Parse pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	users, total, err := repo.GetAll(filter)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Convert to safe response format (no passwords, tokens, etc.)
	userResponses := make([]models.UserResponse, 0, len(users))
	for _, u := range users {
		userResponses = append(userResponses, u.ToResponse())
	}

	c.JSON(http.StatusOK, gin.H{
		"users": userResponses,
		"total": total,
	})
}

// GetUserByID returns a single user by ID
func GetUserByID(c *gin.Context) {
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

	repo := userRepo()
	if repo == nil {
		if isDemoMode() {
			for _, user := range getDemoUsers() {
				if user.ID == uint(id) {
					c.JSON(http.StatusOK, user)
					return
				}
			}
			apiErr := errors.NewNotFound(fmt.Sprintf("user %d", id))
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

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

	c.JSON(http.StatusOK, user.ToResponse())
}

// UpdateUser updates an existing user
func UpdateUser(c *gin.Context) {
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

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := userRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: User %d updated", id)
			writeAuditLog(c, "user.update", "user", fmt.Sprintf("%d", id),
				fmt.Sprintf("User %d updated (demo mode)", id))
			c.JSON(http.StatusOK, gin.H{
				"message": "User updated successfully (demo mode)",
				"user": models.UserResponse{
					ID:        uint(id),
					Email:     req.Email,
					FirstName: req.FirstName,
					LastName:  req.LastName,
					Role:      req.Role,
					CreatedAt: time.Now().Add(-24 * time.Hour),
				},
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

	// Prevent admin from deactivating themselves
	currentUserID, _ := c.Get("userID")
	if currentUID, ok := currentUserID.(uint); ok && currentUID == uint(id) {
		if req.IsActive != nil && !*req.IsActive {
			apiErr := errors.NewBadRequest("Cannot deactivate your own account")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		if req.Role != "" && req.Role != user.Role {
			apiErr := errors.NewBadRequest("Cannot change your own role")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	// Validate role if provided
	if req.Role != "" && !rbac.IsValidRole(req.Role) {
		apiErr := errors.NewValidation(fmt.Sprintf("Invalid role: %s. Valid roles are: network-ops, sre, network-admin, senior-eng, sysadmin", req.Role))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check email uniqueness if email is being changed
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

	// Build updates map
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
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		apiErr := errors.NewBadRequest("No fields to update")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := repo.UpdateFields(uint(id), updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get username for logging
	username, _ := c.Get("username")
	logger.Info("User %d updated by %v", id, username)

	// Record this action in the audit log for compliance tracking
	writeAuditLog(c, "user.update", "user", fmt.Sprintf("%d", id),
		fmt.Sprintf("User %d (%s) updated: %v", id, user.Email, updates))

	// Send email notifications for role changes or deactivation (non-blocking)
	if services.Email != nil {
		go func() {
			adminUsername := fmt.Sprintf("%v", username)
			// Role changed
			if req.Role != "" && req.Role != user.Role {
				custom := map[string]interface{}{
					"OldRole":   user.Role,
					"NewRole":   req.Role,
					"ChangedBy": adminUsername,
					"Timestamp": time.Now().Format("Jan 2, 2006 3:04 PM"),
					"ActionURL": fmt.Sprintf("%s/dashboard", services.Email.FrontendURL()),
				}
				subject := fmt.Sprintf("Your role has been updated to %s", req.Role)
				if err := services.Email.SendNotification(user.Email, user.Username, subject, "account-role-changed", custom); err != nil {
					logger.Warn("Failed to send role-change email to %s: %v", user.Email, err)
				}
			}
			// Account deactivated
			if req.IsActive != nil && !*req.IsActive && user.IsActive {
				custom := map[string]interface{}{
					"Reason":        "Deactivated by administrator",
					"DeactivatedBy": adminUsername,
					"Timestamp":     time.Now().Format("Jan 2, 2006 3:04 PM"),
					"ActionURL":     fmt.Sprintf("%s/login", services.Email.FrontendURL()),
				}
				subject := "Your account has been deactivated"
				if err := services.Email.SendNotification(user.Email, user.Username, subject, "account-deactivated", custom); err != nil {
					logger.Warn("Failed to send account-deactivated email to %s: %v", user.Email, err)
				}
			}
		}()
	}

	// Check error from GetByID to avoid nil pointer dereference on .ToResponse()
	updatedUser, err := repo.GetByID(uint(id))
	if err != nil || updatedUser == nil {
		// The update succeeded, but re-fetch failed. Return a generic success.
		logger.Warn("User %d updated successfully but re-fetch failed: %v", id, err)
		c.JSON(http.StatusOK, gin.H{
			"message": "User updated successfully",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    updatedUser.ToResponse(),
	})
}
