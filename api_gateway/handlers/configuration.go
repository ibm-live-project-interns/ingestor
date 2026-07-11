package handlers

import (
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

// validChannelTypes lists the allowed notification channel types.
var validChannelTypes = map[string]bool{
	"email": true, "Email": true,
	"slack": true, "Slack": true,
	"webhook": true, "Webhook": true,
	"pagerduty": true, "PagerDuty": true,
	"Twilio": true, "twilio": true,
}

// validateConditionValue checks if a threshold rule condition contains a
// numeric, positive value. The condition is expected to be a string like
// "cpu > 80" or "memory >= 90". We extract the last token and verify it
// parses as a positive number.
func validateConditionValue(condition string) bool {
	parts := strings.Fields(condition)
	if len(parts) == 0 {
		return false
	}
	// The numeric value is typically the last token
	valueStr := parts[len(parts)-1]
	// Strip trailing % if present (e.g. "80%")
	valueStr = strings.TrimSuffix(valueStr, "%")
	val, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		// Could be a non-numeric condition like "status == down" which is valid
		// but not a threshold. Allow it through since the model binding already
		// validated the field is required.
		return true
	}
	return val > 0
}

// validateDuration checks if a duration string is parseable and positive.
func validateDuration(duration string) bool {
	if duration == "" {
		return true // duration is optional
	}
	// Try Go duration format first (e.g. "5m", "1h30m")
	d, err := time.ParseDuration(duration)
	if err == nil {
		return d > 0
	}
	// Also accept plain positive integer (interpreted as minutes by frontend)
	if val, err := strconv.Atoi(duration); err == nil {
		return val > 0
	}
	// Accept human-readable like "5 minutes", "1 hour" - pass through
	return true
}

// validateMaintenanceSchedule checks that schedule looks like a valid date/time.
// Returns an error message string, or "" if valid.
func validateMaintenanceSchedule(schedule string) string {
	if schedule == "" {
		return ""
	}
	// Try RFC3339 first, then date-only, then datetime without timezone
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if _, err := time.Parse(f, schedule); err == nil {
			return ""
		}
	}
	return "Schedule must be a valid date/time format (RFC3339 or YYYY-MM-DD)"
}

// parseConfigPagination extracts optional page & page_size query params.
// If page is not provided, returns zero-value pagination (no limit applied) for
// backward compatibility. page_size defaults to 100, max 500.
func parseConfigPagination(c *gin.Context) (pg database.ConfigPagination, paginated bool) {
	pageStr := c.Query("page")
	if pageStr == "" {
		return database.ConfigPagination{}, false
	}
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 500 {
		pageSize = 500
	}
	return database.ConfigPagination{
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}, true
}

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

	pg, paginated := parseConfigPagination(c)
	if !paginated {
		// No page param: return all results for backward compatibility
		rules, err := repo.ListRules()
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		c.JSON(http.StatusOK, rules)
		return
	}

	rules, total, err := repo.ListRulesPaginated(pg)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  rules,
		"total": total,
	})
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
	if !requireJSONContentType(c) {
		return
	}
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	var req models.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate condition value is numeric and positive
	if strings.TrimSpace(req.Condition) != "" && !validateConditionValue(req.Condition) {
		apiErr := errors.NewValidation("Condition value must be numeric and positive")
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
	if !requireJSONContentType(c) {
		return
	}
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
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
	if err := repo.DeleteRule(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted", "id": id})
}
