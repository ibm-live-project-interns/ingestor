package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

func configRepo() *database.ConfigRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewConfigRepository(db.DB)
}

// ==========================================
// Threshold Rules
// ==========================================

func GetRules(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.ThresholdRule{})
		return
	}
	rules, err := repo.ListRules()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, rules)
}

func GetRuleByID(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		apiErr := errors.NewNotFound("rule " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	rule, err := repo.GetRuleByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if rule == nil {
		apiErr := errors.NewNotFound("rule " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, rule)
}

func CreateRule(c *gin.Context) {
	var req models.CreateRuleRequest
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
	id, _ := repo.GenerateRuleID()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := models.ThresholdRule{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Condition:   req.Condition,
		Duration:    req.Duration,
		Severity:    req.Severity,
		Enabled:     enabled,
	}
	if err := repo.CreateRule(&rule); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	logger.Info("Threshold rule %s created", id)
	c.JSON(http.StatusCreated, gin.H{"message": "Rule created", "rule": rule})
}

func UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var req models.UpdateRuleRequest
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
	if req.Condition != "" {
		updates["condition"] = req.Condition
	}
	if req.Duration != "" {
		updates["duration"] = req.Duration
	}
	if req.Severity != "" {
		updates["severity"] = req.Severity
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	if err := repo.UpdateRule(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	updated, _ := repo.GetRuleByID(id)
	c.JSON(http.StatusOK, gin.H{"message": "Rule updated", "rule": updated})
}

func DeleteRule(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	if err := repo.DeleteRule(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted", "id": id})
}

// ==========================================
// Notification Channels
// ==========================================

func GetChannels(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.NotificationChannel{})
		return
	}
	channels, err := repo.ListChannels()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, channels)
}

func GetChannelByID(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		apiErr := errors.NewNotFound("channel " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	ch, err := repo.GetChannelByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if ch == nil {
		apiErr := errors.NewNotFound("channel " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, ch)
}

func CreateChannel(c *gin.Context) {
	var req models.CreateChannelRequest
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
	id, _ := repo.GenerateChannelID()
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	ch := models.NotificationChannel{
		ID:     id,
		Name:   req.Name,
		Type:   req.Type,
		Meta:   req.Meta,
		Active: active,
	}
	if err := repo.CreateChannel(&ch); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	logger.Info("Notification channel %s created", id)
	c.JSON(http.StatusCreated, gin.H{"message": "Channel created", "channel": ch})
}

func UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var req models.UpdateChannelRequest
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
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Meta != "" {
		updates["meta"] = req.Meta
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	if err := repo.UpdateChannel(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	updated, _ := repo.GetChannelByID(id)
	c.JSON(http.StatusOK, gin.H{"message": "Channel updated", "channel": updated})
}

func DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	if err := repo.DeleteChannel(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Channel deleted", "id": id})
}

// ==========================================
// Escalation Policies
// ==========================================

func GetPolicies(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.EscalationPolicy{})
		return
	}
	policies, err := repo.ListPolicies()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, policies)
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
	var req models.CreatePolicyRequest
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
	windows, err := repo.ListWindows()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, windows)
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
	var req models.CreateWindowRequest
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
	c.JSON(http.StatusCreated, gin.H{"message": "Window created", "window": win})
}

func UpdateWindow(c *gin.Context) {
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
