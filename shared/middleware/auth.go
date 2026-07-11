package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// RequirePermission middleware checks if the user has the required permission
// Must be used AFTER AuthMiddleware which sets "role" in context
func RequirePermission(permission rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get role from context (set by AuthMiddleware)
		roleInterface, exists := c.Get("role")
		if !exists {
			apiErr := errors.NewUnauthorized("No role in context")
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		// Role can be a string or a struct with ID field
		var roleID rbac.RoleID
		switch r := roleInterface.(type) {
		case string:
			roleID = rbac.RoleID(r)
		case map[string]interface{}:
			if id, ok := r["id"].(string); ok {
				roleID = rbac.RoleID(id)
			}
		default:
			// Try to get ID from struct
			roleID = rbac.RoleID("")
		}

		if roleID == "" {
			apiErr := errors.NewUnauthorized("Invalid role format")
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		// Check permission
		if !rbac.HasPermission(roleID, permission) {
			apiErr := errors.NewPermissionDenied(string(permission))
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		c.Next()
	}
}

// RequireAnyPermission middleware checks if the user has any of the required permissions
func RequireAnyPermission(permissions ...rbac.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleInterface, exists := c.Get("role")
		if !exists {
			apiErr := errors.NewUnauthorized("No role in context")
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		var roleID rbac.RoleID
		switch r := roleInterface.(type) {
		case string:
			roleID = rbac.RoleID(r)
		case map[string]interface{}:
			if id, ok := r["id"].(string); ok {
				roleID = rbac.RoleID(id)
			}
		}

		for _, perm := range permissions {
			if rbac.HasPermission(roleID, perm) {
				c.Next()
				return
			}
		}

		apiErr := errors.NewPermissionDenied(joinPermissions(permissions))
		c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
	}
}

// RequireRole middleware checks if the user has one of the required roles
func RequireRole(roles ...rbac.RoleID) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleInterface, exists := c.Get("role")
		if !exists {
			apiErr := errors.NewUnauthorized("No role in context")
			c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}

		var roleID rbac.RoleID
		switch r := roleInterface.(type) {
		case string:
			roleID = rbac.RoleID(r)
		case map[string]interface{}:
			if id, ok := r["id"].(string); ok {
				roleID = rbac.RoleID(id)
			}
		}

		for _, allowedRole := range roles {
			if roleID == allowedRole {
				c.Next()
				return
			}
		}

		apiErr := errors.NewInsufficientRole(joinRoles(roles))
		c.AbortWithStatusJSON(apiErr.HTTPStatus, apiErr.ToResponse())
	}
}

func joinPermissions(perms []rbac.Permission) string {
	strs := make([]string, len(perms))
	for i, p := range perms {
		strs[i] = string(p)
	}
	return strings.Join(strs, " or ")
}

func joinRoles(roles []rbac.RoleID) string {
	strs := make([]string, len(roles))
	for i, r := range roles {
		strs[i] = string(r)
	}
	return strings.Join(strs, " or ")
}

// GetUserRole extracts the role from gin context
func GetUserRole(c *gin.Context) (rbac.RoleID, bool) {
	roleInterface, exists := c.Get("role")
	if !exists {
		return "", false
	}

	switch r := roleInterface.(type) {
	case string:
		return rbac.RoleID(r), true
	case map[string]interface{}:
		if id, ok := r["id"].(string); ok {
			return rbac.RoleID(id), true
		}
	}
	return "", false
}

// ErrorHandler is a centralized error handler that returns consistent error responses
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Check if it's our APIError
			if apiErr, ok := err.Err.(*errors.APIError); ok {
				c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
				return
			}

			// Generic error
			apiErr := errors.NewInternal(err.Error())
			c.JSON(http.StatusInternalServerError, apiErr.ToResponse())
		}
	}
}
