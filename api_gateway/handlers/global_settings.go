package handlers

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// GlobalSettings holds the system-wide configuration toggles that apply
// to all users (maintenance mode, auto-resolve, AI correlation).
// Protected by settingsMu for concurrent access safety.
type GlobalSettings struct {
	MaintenanceMode    bool `json:"maintenance_mode"`
	AutoResolveEnabled bool `json:"auto_resolve_enabled"`
	AICorrelation      bool `json:"ai_correlation_enabled"`
}

var (
	settingsMu      sync.RWMutex
	currentSettings = GlobalSettings{
		MaintenanceMode:    false,
		AutoResolveEnabled: true,
		AICorrelation:      true,
	}
)

// GetGlobalSettings returns the current global configuration settings.
// GET /api/v1/configuration/global-settings
func GetGlobalSettings(c *gin.Context) {
	settingsMu.RLock()
	s := currentSettings
	settingsMu.RUnlock()

	c.JSON(http.StatusOK, s)
}

// UpdateGlobalSettings replaces the global configuration settings.
// PUT /api/v1/configuration/global-settings
// Defense-in-depth: validates role even though middleware should gate access.
func UpdateGlobalSettings(c *gin.Context) {
	// RBAC defense-in-depth: only sysadmin and senior-eng may update global settings.
	role, exists := c.Get("role")
	if !exists {
		c.JSON(http.StatusForbidden, gin.H{"error": "No role found in request context"})
		return
	}
	roleStr, ok := role.(string)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role in request context"})
		return
	}
	rid := rbac.RoleID(roleStr)
	if rid != rbac.RoleSysAdmin && rid != rbac.RoleSeniorEng {
		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient role: requires sysadmin or senior-eng"})
		return
	}

	var req GlobalSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	settingsMu.Lock()
	currentSettings = req
	settingsMu.Unlock()

	logger.Info("Global settings updated: maintenance_mode=%v, auto_resolve=%v, ai_correlation=%v",
		req.MaintenanceMode, req.AutoResolveEnabled, req.AICorrelation)

	c.JSON(http.StatusOK, gin.H{
		"message":  "Global settings updated",
		"settings": req,
	})
}
