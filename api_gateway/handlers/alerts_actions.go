package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// AcknowledgeAlert marks an alert as acknowledged
func AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: Alert %s acknowledged by %s", id, usernameStr)
			c.JSON(http.StatusOK, gin.H{
				"message":         "Alert acknowledged (demo mode)",
				"alert_id":        id,
				"acknowledged_by": usernameStr,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// First check if alert exists
	alert, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if alert == nil {
		apiErr := errors.NewAlertNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if alert can be acknowledged
	if alert.Status == models.AlertStatusResolved || alert.Status == models.AlertStatusDismissed {
		apiErr := errors.NewBadRequest("Cannot acknowledge a resolved or dismissed alert")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Update status
	if err := repo.UpdateStatus(id, models.AlertStatusAcknowledged, usernameStr); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s acknowledged by %s", id, usernameStr)

	// Fetch updated alert
	updatedAlert, _ := repo.GetByID(id)

	// Extract values from Gin context BEFORE the goroutine (c is not goroutine-safe)
	userEmail, _ := c.Get("email")
	userEmailStr := fmt.Sprintf("%v", userEmail)

	// Send alert-acknowledged email notification (non-blocking)
	if services.Email != nil && alert != nil {
		go func() {
			if userEmailStr == "" || userEmailStr == "<nil>" {
				return
			}
			custom := map[string]interface{}{
				"AlertID":   id,
				"Title":     alert.Title,
				"Severity":  alert.Severity,
				"Device":    alert.Device,
				"AckedBy":   usernameStr,
				"Timestamp": time.Now().Format("Jan 2, 2006 3:04 PM"),
				"ActionURL": fmt.Sprintf("%s/alerts/%s", services.Email.FrontendURL(), id),
			}
			subject := fmt.Sprintf("[Acknowledged] %s – %s", alert.Device, alert.Title)
			if err := services.Email.SendNotification(userEmailStr, usernameStr, subject, "alert-acknowledged", custom); err != nil {
				logger.Warn("Failed to send alert-acknowledged email: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Alert acknowledged",
		"alert":           updatedAlert,
		"acknowledged_by": usernameStr,
	})
}

// DismissAlert marks an alert as dismissed
func DismissAlert(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: Alert %s dismissed by %s", id, usernameStr)
			c.JSON(http.StatusOK, gin.H{
				"message":      "Alert dismissed (demo mode)",
				"alert_id":     id,
				"dismissed_by": usernameStr,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// First check if alert exists
	alert, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if alert == nil {
		apiErr := errors.NewAlertNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Update status
	if err := repo.UpdateStatus(id, models.AlertStatusDismissed, usernameStr); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s dismissed by %s", id, usernameStr)

	// Fetch updated alert
	updatedAlert, _ := repo.GetByID(id)

	// Extract values from Gin context BEFORE the goroutine (c is not goroutine-safe)
	userEmail, _ := c.Get("email")
	userEmailStr := fmt.Sprintf("%v", userEmail)

	// Send alert-dismissed email notification (non-blocking)
	if services.Email != nil && alert != nil {
		go func() {
			if userEmailStr == "" || userEmailStr == "<nil>" {
				return
			}
			custom := map[string]interface{}{
				"AlertID":     id,
				"Title":       alert.Title,
				"Severity":    alert.Severity,
				"Device":      alert.Device,
				"DismissedBy": usernameStr,
				"Reason":      "Dismissed by operator",
				"Timestamp":   time.Now().Format("Jan 2, 2006 3:04 PM"),
				"ActionURL":   fmt.Sprintf("%s/alerts/%s", services.Email.FrontendURL(), id),
			}
			subject := fmt.Sprintf("[Dismissed] %s – %s", alert.Device, alert.Title)
			if err := services.Email.SendNotification(userEmailStr, usernameStr, subject, "alert-dismissed", custom); err != nil {
				logger.Warn("Failed to send alert-dismissed email: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Alert dismissed",
		"alert":        updatedAlert,
		"dismissed_by": usernameStr,
	})
}

// ResolveAlert marks an alert as resolved
func ResolveAlert(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: Alert %s resolved by %s", id, usernameStr)
			c.JSON(http.StatusOK, gin.H{
				"message":      "Alert resolved (demo mode)",
				"alert_id":     id,
				"resolved_by": usernameStr,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// First check if alert exists
	alert, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if alert == nil {
		apiErr := errors.NewAlertNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if alert is already resolved
	if alert.Status == models.AlertStatusResolved {
		apiErr := errors.NewBadRequest("Alert is already resolved")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if alert is dismissed (cannot resolve a dismissed alert)
	if alert.Status == models.AlertStatusDismissed {
		apiErr := errors.NewBadRequest("Cannot resolve a dismissed alert")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Update status to resolved
	if err := repo.UpdateStatus(id, models.AlertStatusResolved, usernameStr); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s resolved by %s", id, usernameStr)

	// Fetch updated alert
	updatedAlert, _ := repo.GetByID(id)

	// Extract values from Gin context BEFORE the goroutine (c is not goroutine-safe)
	userEmail, _ := c.Get("email")
	userEmailStr := fmt.Sprintf("%v", userEmail)

	// Send alert-resolved email notification (non-blocking)
	if services.Email != nil && alert != nil {
		go func() {
			if userEmailStr == "" || userEmailStr == "<nil>" {
				return
			}
			custom := map[string]interface{}{
				"AlertID":    id,
				"Title":      alert.Title,
				"Severity":   alert.Severity,
				"Device":     alert.Device,
				"ResolvedBy": usernameStr,
				"Duration":   "N/A",
				"Timestamp":  time.Now().Format("Jan 2, 2006 3:04 PM"),
				"ActionURL":  fmt.Sprintf("%s/alerts/%s", services.Email.FrontendURL(), id),
			}
			subject := fmt.Sprintf("[Resolved] %s – %s", alert.Device, alert.Title)
			if err := services.Email.SendNotification(userEmailStr, usernameStr, subject, "alert-resolved", custom); err != nil {
				logger.Warn("Failed to send alert-resolved email: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Alert resolved",
		"alert":        updatedAlert,
		"resolved_by": usernameStr,
	})
}

// ReanalyzeAlert re-sends an alert to AI-Core for fresh Watson analysis and saves the result
func ReanalyzeAlert(c *gin.Context) {
	id := c.Param("id")

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			c.JSON(http.StatusOK, gin.H{"message": "Re-analysis not available in demo mode", "alert_id": id})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	alert, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if alert == nil {
		apiErr := errors.NewAlertNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Call ai-core for re-analysis
	aiCoreURL := config.GetEnv("AI_CORE_URL", "http://ai-core:9000")
	message := alert.Description
	if message == "" {
		message = alert.Title
	}
	payload := map[string]string{
		"type":        alert.Severity,
		"message":     message,
		"source_host": alert.Device,
		"source_ip":   alert.SourceIP,
		"event_type":  alert.Source,
		"category":    alert.Category,
		"severity":    alert.Severity,
	}

	payloadBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", aiCoreURL+"/events", bytes.NewBuffer(payloadBytes))
	if err != nil {
		logger.Error("Failed to create AI-Core request for alert %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create AI request"})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("AI-Core request failed for alert %s: %v", id, err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI service unavailable", "detail": err.Error()})
		return
	}
	defer resp.Body.Close()

	// Check if AI-Core returned a non-200 status (e.g. 503 when Watson not configured)
	if resp.StatusCode != http.StatusOK {
		var errBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errBody)
		detail := "AI service returned an error"
		if d, ok := errBody["detail"].(string); ok {
			detail = d
		} else if e, ok := errBody["error"].(string); ok {
			detail = e
		}
		logger.Error("AI-Core returned %d for alert %s: %s", resp.StatusCode, id, detail)
		c.JSON(resp.StatusCode, gin.H{"error": detail, "alert": alert})
		return
	}

	var aiResp struct {
		Severity          string `json:"severity"`
		Explanation       string `json:"explanation"`
		RootCause         string `json:"root_cause"`
		Impact            string `json:"impact"`
		RecommendedAction string `json:"recommended_action"`
		Confidence        int    `json:"confidence"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		logger.Error("Failed to decode AI-Core response for alert %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse AI response"})
		return
	}

	// Update alert with new AI analysis
	// Normalize confidence to 0-1 range (Watson returns 0-100)
	confidence := float64(aiResp.Confidence)
	if confidence > 1 {
		confidence = confidence / 100.0
	}
	updates := map[string]interface{}{
		"ai_summary":        aiResp.Explanation,
		"ai_root_cause":     aiResp.RootCause,
		"ai_impact":         aiResp.Impact,
		"ai_recommendation": aiResp.RecommendedAction,
		"ai_confidence":     confidence,
	}

	if err := repo.UpdateFields(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s re-analyzed successfully: severity=%s", id, aiResp.Severity)

	// Return updated alert
	updatedAlert, _ := repo.GetByID(id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Alert re-analyzed successfully",
		"alert":   updatedAlert,
	})
}

// ==========================================
// Bulk Alert Actions
// ==========================================

// BulkActionRequest represents a request to perform an action on multiple alerts
type BulkActionRequest struct {
	Action   string   `json:"action" binding:"required"`
	AlertIDs []string `json:"alert_ids" binding:"required"`
}

// BulkActionResult represents the result of a bulk action
type BulkActionResult struct {
	Succeeded []string          `json:"succeeded"`
	Failed    map[string]string `json:"failed"`
}

// BulkAlertAction performs an action on multiple alerts at once
// POST /api/v1/alerts/bulk-action
func BulkAlertAction(c *gin.Context) {
	var req BulkActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate action
	validActions := map[string]bool{
		"acknowledge": true,
		"resolve":     true,
		"dismiss":     true,
	}
	if !validActions[req.Action] {
		apiErr := errors.NewBadRequest("Invalid action. Must be one of: acknowledge, resolve, dismiss")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Enforce max 100 alert IDs per request
	if len(req.AlertIDs) > 100 {
		apiErr := errors.NewBadRequest("Maximum 100 alert IDs per bulk action request")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if len(req.AlertIDs) == 0 {
		apiErr := errors.NewBadRequest("At least one alert_id is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get username from context
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			// Demo mode: simulate success for all IDs
			logger.Info("Demo mode: bulk %s for %d alerts by %s", req.Action, len(req.AlertIDs), usernameStr)
			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("Bulk %s completed (demo mode)", req.Action),
				"result": BulkActionResult{
					Succeeded: req.AlertIDs,
					Failed:    map[string]string{},
				},
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Determine the target status based on action
	var targetStatus string
	switch req.Action {
	case "acknowledge":
		targetStatus = models.AlertStatusAcknowledged
	case "resolve":
		targetStatus = models.AlertStatusResolved
	case "dismiss":
		targetStatus = models.AlertStatusDismissed
	}

	result := BulkActionResult{
		Succeeded: make([]string, 0, len(req.AlertIDs)),
		Failed:    make(map[string]string),
	}

	// Process each alert
	for _, alertID := range req.AlertIDs {
		// Check if alert exists
		alert, err := repo.GetByID(alertID)
		if err != nil {
			result.Failed[alertID] = "Database error: " + err.Error()
			continue
		}
		if alert == nil {
			result.Failed[alertID] = "Alert not found"
			continue
		}

		// Validate state transitions
		switch req.Action {
		case "acknowledge":
			if alert.Status == models.AlertStatusResolved || alert.Status == models.AlertStatusDismissed {
				result.Failed[alertID] = "Cannot acknowledge a resolved or dismissed alert"
				continue
			}
		case "resolve":
			if alert.Status == models.AlertStatusResolved {
				result.Failed[alertID] = "Alert is already resolved"
				continue
			}
			if alert.Status == models.AlertStatusDismissed {
				result.Failed[alertID] = "Cannot resolve a dismissed alert"
				continue
			}
		case "dismiss":
			if alert.Status == models.AlertStatusResolved {
				result.Failed[alertID] = "Cannot dismiss a resolved alert"
				continue
			}
		}

		// Update status
		if err := repo.UpdateStatus(alertID, targetStatus, usernameStr); err != nil {
			result.Failed[alertID] = "Update failed: " + err.Error()
			continue
		}

		result.Succeeded = append(result.Succeeded, alertID)
	}

	logger.Info("Bulk %s by %s: %d succeeded, %d failed",
		req.Action, usernameStr, len(result.Succeeded), len(result.Failed))

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Bulk %s completed: %d succeeded, %d failed",
			req.Action, len(result.Succeeded), len(result.Failed)),
		"result": result,
	})
}
