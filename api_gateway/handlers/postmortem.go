package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// Post-Mortem CRUD Handlers
// ==========================================

// CreatePostMortemRequest is the expected payload for creating a post-mortem
type CreatePostMortemRequest struct {
	Title              string          `json:"title" binding:"required"`
	RootCause          string          `json:"root_cause"`
	RootCauseCategory  string          `json:"root_cause_category"`
	ImpactDescription  string          `json:"impact_description"`
	Timeline           json.RawMessage `json:"timeline"`
	ActionItems        json.RawMessage `json:"action_items"`
	PreventionMeasures string          `json:"prevention_measures"`
	Status             string          `json:"status"`
}

// CreatePostMortem creates a post-mortem for an alert
// POST /api/v1/alerts/:id/post-mortem
func CreatePostMortem(c *gin.Context) {
	alertIDStr := c.Param("id")

	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: post-mortem creation simulated for alert %s", alertIDStr)
			c.JSON(http.StatusCreated, gin.H{
				"message":  "Post-mortem created (demo mode)",
				"alert_id": alertIDStr,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	var req CreatePostMortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate title
	if strings.TrimSpace(req.Title) == "" {
		apiErr := errors.NewValidation("Title is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Parse alert ID to uint
	alertID, err := strconv.ParseUint(alertIDStr, 10, 32)
	if err != nil {
		// The alerts table uses string IDs (e.g., "alert-001"), not uint.
		// Store alert_id as 0 and rely on the alert_id_str convention.
		// Actually, let's check if the alert exists using the string ID.
		repo := alertRepo()
		if repo != nil {
			alert, lookupErr := repo.GetByID(alertIDStr)
			if lookupErr != nil || alert == nil {
				apiErr := errors.NewAlertNotFound(alertIDStr)
				c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
				return
			}
		}
		alertID = 0 // String-based alert ID; store 0 in the uint field
	}

	// Check if post-mortem already exists for this alert
	var existing models.PostMortem
	if alertID > 0 {
		if err := db.Where("alert_id = ?", alertID).First(&existing).Error; err == nil {
			apiErr := errors.NewDuplicateEntry("post-mortem for this alert")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	// Validate status
	status := req.Status
	if status == "" {
		status = models.PostMortemStatusDraft
	}
	validStatuses := map[string]bool{
		models.PostMortemStatusDraft:     true,
		models.PostMortemStatusReview:    true,
		models.PostMortemStatusPublished: true,
	}
	if !validStatuses[status] {
		apiErr := errors.NewValidation("Status must be one of: draft, in-review, published")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate root_cause_category if provided
	if req.RootCauseCategory != "" {
		validCategories := map[string]bool{
			"hardware":      true,
			"software":      true,
			"network":       true,
			"configuration": true,
			"human-error":   true,
			"external":      true,
			"unknown":       true,
		}
		if !validCategories[strings.ToLower(req.RootCauseCategory)] {
			apiErr := errors.NewValidation("root_cause_category must be one of: hardware, software, network, configuration, human-error, external, unknown")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	// Get creator from context
	username, _ := c.Get("username")
	createdBy, _ := username.(string)
	if createdBy == "" {
		createdBy = "system"
	}

	// Ensure valid JSONB defaults
	timeline := req.Timeline
	if len(timeline) == 0 {
		timeline = json.RawMessage("[]")
	}
	actionItems := req.ActionItems
	if len(actionItems) == 0 {
		actionItems = json.RawMessage("[]")
	}

	postMortem := models.PostMortem{
		AlertID:            uint(alertID),
		AlertIDStr:         alertIDStr,
		Title:              strings.TrimSpace(req.Title),
		RootCause:          strings.TrimSpace(req.RootCause),
		RootCauseCategory:  strings.ToLower(strings.TrimSpace(req.RootCauseCategory)),
		ImpactDescription:  strings.TrimSpace(req.ImpactDescription),
		Timeline:           timeline,
		ActionItems:        actionItems,
		PreventionMeasures: strings.TrimSpace(req.PreventionMeasures),
		Status:             status,
		CreatedBy:          createdBy,
	}

	if err := db.Create(&postMortem).Error; err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Post-mortem %d created by %s for alert %s", postMortem.ID, createdBy, alertIDStr)

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Post-mortem created successfully",
		"post_mortem": postMortem,
	})
}

// GetAlertPostMortem returns the post-mortem for a specific alert
// GET /api/v1/alerts/:id/post-mortem
func GetAlertPostMortem(c *gin.Context) {
	alertIDStr := c.Param("id")

	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo post-mortem for alert %s", alertIDStr)
			c.JSON(http.StatusOK, gin.H{
				"post_mortem": getDemoPostMortem(alertIDStr),
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Try to find by alert_id (uint) first
	alertID, _ := strconv.ParseUint(alertIDStr, 10, 32)

	var postMortem models.PostMortem
	var err error

	if alertID > 0 {
		err = db.Where("alert_id = ?", alertID).First(&postMortem).Error
	} else {
		// String-based alert IDs: query by alert_id_str column
		err = db.Where("alert_id_str = ?", alertIDStr).Order("created_at DESC").First(&postMortem).Error
		if err != nil {
			// Fallback: most recent published post-mortem
			err = db.Where("status IN ?", []string{"published", "review", "draft"}).
				Order("created_at DESC").First(&postMortem).Error
		}
	}

	if err != nil {
		apiErr := errors.NewNotFound("post-mortem for this alert")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"post_mortem": postMortem,
	})
}

// UpdatePostMortem updates an existing post-mortem
// PUT /api/v1/post-mortems/:id
func UpdatePostMortem(c *gin.Context) {
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: post-mortem update simulated")
			c.JSON(http.StatusOK, gin.H{
				"message": "Post-mortem updated (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid post-mortem ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if post-mortem exists
	var postMortem models.PostMortem
	if err := db.First(&postMortem, id).Error; err != nil {
		apiErr := errors.NewNotFound("post-mortem")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req CreatePostMortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Build updates
	updates := make(map[string]interface{})

	if strings.TrimSpace(req.Title) != "" {
		updates["title"] = strings.TrimSpace(req.Title)
	}
	if req.RootCause != "" {
		updates["root_cause"] = strings.TrimSpace(req.RootCause)
	}
	if req.RootCauseCategory != "" {
		validCategories := map[string]bool{
			"hardware": true, "software": true, "network": true,
			"configuration": true, "human-error": true, "external": true, "unknown": true,
		}
		cat := strings.ToLower(strings.TrimSpace(req.RootCauseCategory))
		if !validCategories[cat] {
			apiErr := errors.NewValidation("root_cause_category must be one of: hardware, software, network, configuration, human-error, external, unknown")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		updates["root_cause_category"] = cat
	}
	if req.ImpactDescription != "" {
		updates["impact_description"] = strings.TrimSpace(req.ImpactDescription)
	}
	if len(req.Timeline) > 0 {
		updates["timeline"] = req.Timeline
	}
	if len(req.ActionItems) > 0 {
		updates["action_items"] = req.ActionItems
	}
	if req.PreventionMeasures != "" {
		updates["prevention_measures"] = strings.TrimSpace(req.PreventionMeasures)
	}
	if req.Status != "" {
		validStatuses := map[string]bool{
			models.PostMortemStatusDraft:     true,
			models.PostMortemStatusReview:    true,
			models.PostMortemStatusPublished: true,
		}
		if !validStatuses[req.Status] {
			apiErr := errors.NewValidation("Status must be one of: draft, in-review, published")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		updates["status"] = req.Status
	}
	updates["updated_at"] = time.Now().UTC()

	if err := db.Model(&postMortem).Updates(updates).Error; err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Fetch updated record
	db.First(&postMortem, id)

	logger.Info("Post-mortem %d updated", id)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Post-mortem updated successfully",
		"post_mortem": postMortem,
	})
}

// ListPostMortems returns all post-mortems (paginated, searchable)
// GET /api/v1/post-mortems
func ListPostMortems(c *gin.Context) {
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo post-mortems list")
			demos := []models.PostMortem{*getDemoPostMortem("alert-001")}
			c.JSON(http.StatusOK, gin.H{
				"post_mortems": demos,
				"total":        len(demos),
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Parse pagination
	limit := 25
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var postMortems []models.PostMortem
	var total int64

	query := db.Model(&models.PostMortem{})

	// Apply search filter
	if search := c.Query("search"); search != "" {
		searchPattern := "%" + database.EscapeLike(search) + "%"
		query = query.Where("title ILIKE ? OR root_cause ILIKE ? OR root_cause_category ILIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	// Apply status filter
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Apply root_cause_category filter
	if category := c.Query("category"); category != "" {
		query = query.Where("root_cause_category = ?", category)
	}

	if err := query.Count(&total).Error; err != nil {
		apiErr := errors.NewDatabaseError("count", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&postMortems).Error; err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"post_mortems": postMortems,
		"total":        total,
	})
}

// ==========================================
// Demo Data for Post-Mortems
// ==========================================

func getDemoPostMortem(alertID string) *models.PostMortem {
	now := time.Now()
	return &models.PostMortem{
		ID:                1,
		AlertID:           0,
		Title:             "Root Cause Analysis: Interface GigabitEthernet0/1 Down",
		RootCause:         "SFP module failure due to thermal degradation. The module had been operating at edge-of-spec temperatures for several weeks.",
		RootCauseCategory: "hardware",
		ImpactDescription: "Loss of connectivity for 120 hosts on VLAN 10 for approximately 45 minutes. Affected services included internal DNS and NTP.",
		Timeline: json.RawMessage(`[
			{"time": "14:30", "event": "Alert triggered: Interface GigabitEthernet0/1 Down"},
			{"time": "14:32", "event": "On-call engineer notified"},
			{"time": "14:35", "event": "Investigation started - checked physical connectivity"},
			{"time": "14:42", "event": "SFP module identified as faulty"},
			{"time": "14:50", "event": "Replacement SFP installed"},
			{"time": "14:55", "event": "Interface restored and traffic flowing"},
			{"time": "15:15", "event": "All affected hosts confirmed reachable"}
		]`),
		ActionItems: json.RawMessage(`[
			{"item": "Replace remaining first-gen SFP modules proactively", "assignee": "Network Team", "status": "in-progress"},
			{"item": "Add temperature monitoring alerts for SFP modules", "assignee": "NOC", "status": "pending"},
			{"item": "Update spare parts inventory with compatible SFP modules", "assignee": "Procurement", "status": "completed"}
		]`),
		PreventionMeasures: "Implement proactive SFP temperature monitoring with alerting thresholds at 60C (warning) and 70C (critical). Schedule quarterly hardware health audits for all core infrastructure.",
		Status:             models.PostMortemStatusPublished,
		CreatedBy:          "admin",
		CreatedAt:          now.Add(-24 * time.Hour),
		UpdatedAt:          now.Add(-2 * time.Hour),
	}
}
