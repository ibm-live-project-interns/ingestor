package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// GetHealth returns health check status
func GetHealth(c *gin.Context) {
	status := "healthy"
	dbStatus := "connected"

	// Check database connectivity
	if db := database.Get(); db != nil {
		if err := db.Ping(); err != nil {
			dbStatus = "disconnected"
			status = "degraded"
		}
	} else {
		dbStatus = "not initialized"
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"service":   "api-gateway",
		"version":   "1.0.0",
		"database":  dbStatus,
		"timestamp": time.Now().UTC(),
	})
}

// IngestEventRequest represents an incoming event from ingestor core
type IngestEventRequest struct {
	EventType      string                 `json:"event_type" binding:"required"`
	SourceHost     string                 `json:"source_host" binding:"required"`
	SourceIP       string                 `json:"source_ip" binding:"required"`
	Severity       string                 `json:"severity" binding:"required"`
	Category       string                 `json:"category" binding:"required"`
	Message        string                 `json:"message" binding:"required"`
	RawPayload     string                 `json:"raw_payload,omitempty"`
	EventTimestamp time.Time              `json:"event_timestamp"`
	AIAnalysis     map[string]interface{} `json:"ai_analysis,omitempty"`
}

// IngestEvent receives events from ingestor core and creates alerts
func IngestEvent(c *gin.Context) {
	var req IngestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if len(req.RawPayload) > 1_000_000 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large (max 1MB)"})
		return
	}

	repo := alertRepo()
	if repo == nil {
		// Demo mode - just acknowledge the event
		alertID := fmt.Sprintf("ALT-DEMO-%d", time.Now().Unix())
		logger.Info("Demo mode: Alert %s would be created from ingest event: %s", alertID, req.Category)
		c.JSON(http.StatusCreated, gin.H{
			"message":  "Event ingested successfully (demo mode)",
			"alert_id": alertID,
		})
		return
	}

	// Generate alert ID
	alertID, err := repo.GenerateAlertID()
	if err != nil {
		apiErr := errors.NewDatabaseError("generate id", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Handle timestamp
	timestamp := req.EventTimestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	alert := models.Alert{
		ID:          alertID,
		Title:       fmt.Sprintf("[%s] %s from %s", req.Severity, req.Category, req.SourceHost),
		Description: req.Message,
		Severity:    req.Severity,
		Category:    req.Category,
		Status:      models.AlertStatusOpen,
		Source:      req.EventType,
		SourceIP:    req.SourceIP,
		Device:      req.SourceHost,
		Timestamp:   timestamp,
		RawPayload:  req.RawPayload,
	}

	// Add AI analysis if present
	if req.AIAnalysis != nil {
		logger.Info("Alert %s has AI analysis data, extracting fields", alertID)
		if summary, ok := req.AIAnalysis["explanation"].(string); ok && summary != "" {
			alert.AIAnalysisSummary = summary
		}
		if rootCause, ok := req.AIAnalysis["root_cause"].(string); ok && rootCause != "" {
			alert.AIAnalysisRootCause = rootCause
		}
		if impact, ok := req.AIAnalysis["impact"].(string); ok && impact != "" {
			alert.AIAnalysisImpact = impact
		}
		if recommendation, ok := req.AIAnalysis["recommended_action"].(string); ok && recommendation != "" {
			alert.AIAnalysisRecommendation = recommendation
		}
		if confidence, ok := req.AIAnalysis["confidence"].(float64); ok {
			alert.AIConfidence = confidence
		}
		// Also try severity from AI analysis to override if present
		if aiSeverity, ok := req.AIAnalysis["severity"].(string); ok && aiSeverity != "" && aiSeverity != "unknown" {
			alert.Severity = aiSeverity
			logger.Info("Alert %s severity overridden by AI to: %s", alertID, aiSeverity)
		}
	} else {
		logger.Debug("Alert %s has no AI analysis data", alertID)
	}

	// Check if the device is under an active maintenance window.
	// The maintenance_windows table has: name, schedule, duration, status.
	// If status='active' and the device name matches part of the window name,
	// suppress the alert instead of creating it in the default state.
	suppressedReason := checkMaintenanceWindowSuppression(database.Get(), alert.Device)
	if suppressedReason != "" {
		alert.Status = "suppressed"
		logger.Info("Alert %s suppressed: %s", alertID, suppressedReason)
	}

	if err := repo.Create(&alert); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s created from ingest event: severity=%s category=%s device=%s has_ai=%v suppressed=%v", alertID, alert.Severity, req.Category, req.SourceHost, req.AIAnalysis != nil, suppressedReason != "")

	// Send email notifications asynchronously (skip if suppressed)
	if suppressedReason == "" {
		go sendAlertEmailNotifications(alert)
	}

	response := gin.H{
		"message":  "Event ingested successfully",
		"alert_id": alertID,
	}
	if suppressedReason != "" {
		response["suppressed"]        = true
		response["suppressed_reason"] = suppressedReason
	}

	c.JSON(http.StatusCreated, response)
}

// ---------------------------------------------------------------------------
// Shared utility functions used across multiple handler files
// ---------------------------------------------------------------------------

// deterministicHash produces a stable non-negative integer from a string,
// used to seed deterministic-but-varied values per device.
func deterministicHash(s string) int64 {
	var h int64
	for _, ch := range s {
		h = h*31 + int64(ch)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// toInt64 safely extracts an int64 from an interface{} value.
// Handles int64 and int types that may be stored in gin.H maps.
func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

// sendAlertEmailNotifications sends email notifications to users who have email alerts enabled
func sendAlertEmailNotifications(alert models.Alert) {
	if services.Email == nil {
		return
	}

	db := database.Get()
	if db == nil {
		return
	}

	// Find users with email alerts enabled
	var users []models.User
	query := db.Where("email_alerts = ? AND is_active = ?", true, true)

	// If critical_only, only match critical/high alerts
	// We send to all email_alerts users, but skip users who have critical_only=true
	// unless the alert is critical or high severity
	if err := query.Find(&users).Error; err != nil {
		logger.Error("Failed to query users for alert notifications: %v", err)
		return
	}

	isCriticalOrHigh := alert.Severity == "critical" || alert.Severity == "high"

	emailData := services.AlertEmailData{
		AlertID:   alert.ID,
		Title:     alert.Title,
		Severity:  alert.Severity,
		Device:    alert.Device,
		SourceIP:  alert.SourceIP,
		Category:  alert.Category,
		AISummary: alert.AIAnalysisSummary,
		Timestamp: alert.Timestamp.UTC().Format("2006-01-02 15:04:05"),
	}

	sent := 0
	for _, user := range users {
		// Skip users with critical_only if alert isn't critical/high
		if user.CriticalOnly && !isCriticalOrHigh {
			continue
		}

		username := user.Username
		if user.FirstName != "" {
			username = user.FirstName
		}

		if err := services.Email.SendAlertNotification(user.Email, username, emailData); err != nil {
			logger.Error("Failed to send alert notification to %s: %v", user.Email, err)
		} else {
			sent++
		}
	}

	if sent > 0 {
		logger.Info("Sent alert notification emails for %s to %d users", alert.ID, sent)
	}
}

// checkMaintenanceWindowSuppression checks whether the given device falls
// under an active maintenance window. Returns a non-empty reason string if
// the alert should be suppressed, or an empty string if no match.
//
// The maintenance_windows table schema uses:
//   - name:     descriptive name (e.g., "Weekly Switch Maintenance")
//   - schedule: human-readable schedule string
//   - duration: human-readable duration string
//   - status:   "active", "scheduled", etc.
//
// Since the table does not have structured device_pattern/start_time/end_time
// columns, we match by checking if status='active' and the window name contains
// keywords that overlap with the device identifier. This is a heuristic that
// works well with the existing seed data and naming conventions.
func checkMaintenanceWindowSuppression(db *database.Database, device string) string {
	if db == nil || db.DB == nil || device == "" {
		return ""
	}

	// Normalize the device name for matching
	deviceLower := strings.ToLower(device)

	// Query all active maintenance windows
	type mwRow struct {
		ID     string `gorm:"column:id"`
		Name   string `gorm:"column:name"`
		Status string `gorm:"column:status"`
	}
	var windows []mwRow
	if err := db.Table("maintenance_windows").
		Select("id, name, status").
		Where("status = 'active' AND deleted_at IS NULL").
		Find(&windows).Error; err != nil {
		logger.Warn("Failed to query maintenance windows for suppression check: %v", err)
		return ""
	}

	for _, w := range windows {
		// Heuristic matching: check if the window name contains a device type
		// keyword that also appears in the device identifier. For example,
		// "Weekly Switch Maintenance" would match device "Core-SW-01" or any
		// device containing "switch" in its name.
		nameLower := strings.ToLower(w.Name)

		// Direct substring match
		if strings.Contains(nameLower, deviceLower) || strings.Contains(deviceLower, nameLower) {
			return fmt.Sprintf("Alert suppressed: device under maintenance window '%s'", w.Name)
		}

		// Keyword-based matching: extract device type hints
		deviceTypeKeywords := []string{"switch", "router", "firewall", "server", "ap", "ups", "lb"}
		for _, keyword := range deviceTypeKeywords {
			if strings.Contains(nameLower, keyword) && strings.Contains(deviceLower, keyword[:2]) {
				return fmt.Sprintf("Alert suppressed: device under maintenance window '%s'", w.Name)
			}
		}
	}

	return ""
}
