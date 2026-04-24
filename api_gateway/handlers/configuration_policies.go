package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// Escalation Policies
// ==========================================

func GetPolicies(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.EscalationPolicy{})
		return
	}

	pg, paginated := parseConfigPagination(c)
	if !paginated {
		policies, err := repo.ListPolicies()
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		c.JSON(http.StatusOK, policies)
		return
	}

	policies, total, err := repo.ListPoliciesPaginated(pg)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  policies,
		"total": total,
	})
}

func GetPolicyByID(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		apiErr := errors.NewNotFound("policy " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	pol, err := repo.GetPolicyByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if pol == nil {
		apiErr := errors.NewNotFound("policy " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, pol)
}

func CreatePolicy(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	var req models.CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate name is non-empty
	if strings.TrimSpace(req.Name) == "" {
		apiErr := errors.NewValidation("Policy name is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate at least one escalation level (steps >= 1)
	if req.Steps < 1 {
		apiErr := errors.NewValidation("Escalation policy must have at least one level (steps >= 1)")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	id, _ := repo.GeneratePolicyID()
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	pol := models.EscalationPolicy{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Steps:       req.Steps,
		Active:      active,
	}
	if err := repo.CreatePolicy(&pol); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	logger.Info("Escalation policy %s created", id)
	c.JSON(http.StatusCreated, gin.H{"message": "Policy created", "policy": pol})
}

func UpdatePolicy(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	id := c.Param("id")
	var req models.UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Steps > 0 {
		updates["steps"] = req.Steps
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	if err := repo.UpdatePolicy(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	updated, _ := repo.GetPolicyByID(id)
	c.JSON(http.StatusOK, gin.H{"message": "Policy updated", "policy": updated})
}

func DeletePolicy(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	if err := repo.DeletePolicy(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Policy deleted", "id": id})
}

// ==========================================
// Maintenance Windows
// ==========================================

func GetWindows(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.MaintenanceWindow{})
		return
	}

	pg, paginated := parseConfigPagination(c)
	if !paginated {
		windows, err := repo.ListWindows()
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		c.JSON(http.StatusOK, windows)
		return
	}

	windows, total, err := repo.ListWindowsPaginated(pg)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  windows,
		"total": total,
	})
}

func GetWindowByID(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		apiErr := errors.NewNotFound("window " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	win, err := repo.GetWindowByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if win == nil {
		apiErr := errors.NewNotFound("window " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, win)
}

func CreateWindow(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	var req models.CreateWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate schedule format if provided
	if msg := validateMaintenanceSchedule(req.Schedule); msg != "" {
		apiErr := errors.NewValidation(msg)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate duration is positive if provided
	if !validateDuration(req.Duration) {
		apiErr := errors.NewValidation("Duration must be a positive value")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	id, _ := repo.GenerateWindowID()
	status := "scheduled"
	if req.Status != "" {
		status = req.Status
	}
	win := models.MaintenanceWindow{
		ID:       id,
		Name:     req.Name,
		Schedule: req.Schedule,
		Duration: req.Duration,
		Status:   status,
	}
	if err := repo.CreateWindow(&win); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	logger.Info("Maintenance window %s created", id)

	// Send maintenance-upcoming notification email (non-blocking)
	if services.Email != nil {
		window := win
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = ctx
			custom := map[string]interface{}{
				"WindowName":  window.Name,
				"StartTime":   window.Schedule,
				"Duration":    window.Duration,
				"Status":      window.Status,
				"Description": window.Name,
			}
			if err := services.Email.SendNotification("team@sentrix.local", "Maintenance Team", "Maintenance Window Created", "maintenance-upcoming", custom); err != nil {
				logger.Error("Failed to send maintenance-upcoming email: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Window created", "window": win})
}

func UpdateWindow(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	id := c.Param("id")
	var req models.UpdateWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Schedule != "" {
		updates["schedule"] = req.Schedule
	}
	if req.Duration != "" {
		updates["duration"] = req.Duration
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	if err := repo.UpdateWindow(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	updated, _ := repo.GetWindowByID(id)
	c.JSON(http.StatusOK, gin.H{"message": "Window updated", "window": updated})
}

func DeleteWindow(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	if err := repo.DeleteWindow(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Window deleted", "id": id})
}
