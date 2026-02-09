package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// auditRepo returns the audit repository using the global database
// Returns nil if database is not available (demo mode)
func auditRepo() *database.AuditRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewAuditRepository(db.DB)
}

// getDemoAuditLogs returns demo audit log entries for when database is unavailable
func getDemoAuditLogs() []models.AuditLogResponse {
	now := time.Now()
	return []models.AuditLogResponse{
		{
			ID:         1,
			CreatedAt:  now.Add(-10 * time.Minute),
			UserID:     1,
			Username:   "admin",
			Action:     "user.create",
			Resource:   "user",
			ResourceID: "5",
			Details:    map[string]interface{}{"email": "new.user@example.com", "role": "network-ops"},
			IPAddress:  "192.168.1.100",
			Result:     "success",
		},
		{
			ID:         2,
			CreatedAt:  now.Add(-25 * time.Minute),
			UserID:     2,
			Username:   "jsmith",
			Action:     "alert.acknowledge",
			Resource:   "alert",
			ResourceID: "ALT-2026-1234",
			Details:    map[string]interface{}{"severity": "critical", "title": "High CPU on Core-SW-01"},
			IPAddress:  "192.168.1.101",
			Result:     "success",
		},
		{
			ID:         3,
			CreatedAt:  now.Add(-45 * time.Minute),
			UserID:     1,
			Username:   "admin",
			Action:     "config.update",
			Resource:   "config",
			ResourceID: "threshold-rule-3",
			Details:    map[string]interface{}{"field": "threshold", "old_value": "80", "new_value": "90"},
			IPAddress:  "192.168.1.100",
			Result:     "success",
		},
		{
			ID:         4,
			CreatedAt:  now.Add(-1 * time.Hour),
			UserID:     3,
			Username:   "jdoe",
			Action:     "ticket.create",
			Resource:   "ticket",
			ResourceID: "TKT-2026-0042",
			Details:    map[string]interface{}{"priority": "high", "title": "Network latency investigation"},
			IPAddress:  "192.168.1.102",
			Result:     "success",
		},
		{
			ID:         5,
			CreatedAt:  now.Add(-2 * time.Hour),
			UserID:     2,
			Username:   "jsmith",
			Action:     "user.login",
			Resource:   "session",
			ResourceID: "2",
			Details:    map[string]interface{}{"method": "password"},
			IPAddress:  "10.0.0.50",
			Result:     "success",
		},
		{
			ID:         6,
			CreatedAt:  now.Add(-3 * time.Hour),
			UserID:     4,
			Username:   "bwilson",
			Action:     "user.login",
			Resource:   "session",
			ResourceID: "4",
			Details:    map[string]interface{}{"method": "password", "reason": "invalid credentials"},
			IPAddress:  "10.0.0.75",
			Result:     "failure",
		},
		{
			ID:         7,
			CreatedAt:  now.Add(-4 * time.Hour),
			UserID:     1,
			Username:   "admin",
			Action:     "user.update",
			Resource:   "user",
			ResourceID: "3",
			Details:    map[string]interface{}{"field": "role", "old_value": "network-ops", "new_value": "sre"},
			IPAddress:  "192.168.1.100",
			Result:     "success",
		},
		{
			ID:         8,
			CreatedAt:  now.Add(-5 * time.Hour),
			UserID:     3,
			Username:   "jdoe",
			Action:     "alert.resolve",
			Resource:   "alert",
			ResourceID: "ALT-2026-1100",
			Details:    map[string]interface{}{"severity": "major", "resolution": "Rebooted switch module"},
			IPAddress:  "192.168.1.102",
			Result:     "success",
		},
		{
			ID:         9,
			CreatedAt:  now.Add(-6 * time.Hour),
			UserID:     1,
			Username:   "admin",
			Action:     "user.delete",
			Resource:   "user",
			ResourceID: "6",
			Details:    map[string]interface{}{"username": "testuser", "reason": "Account cleanup"},
			IPAddress:  "192.168.1.100",
			Result:     "success",
		},
		{
			ID:         10,
			CreatedAt:  now.Add(-8 * time.Hour),
			UserID:     2,
			Username:   "jsmith",
			Action:     "ticket.update",
			Resource:   "ticket",
			ResourceID: "TKT-2026-0038",
			Details:    map[string]interface{}{"field": "status", "old_value": "open", "new_value": "in_progress"},
			IPAddress:  "192.168.1.101",
			Result:     "success",
		},
		{
			ID:         11,
			CreatedAt:  now.Add(-10 * time.Hour),
			UserID:     1,
			Username:   "admin",
			Action:     "config.create",
			Resource:   "config",
			ResourceID: "notification-channel-5",
			Details:    map[string]interface{}{"type": "email", "name": "NOC Team Email"},
			IPAddress:  "192.168.1.100",
			Result:     "success",
		},
		{
			ID:         12,
			CreatedAt:  now.Add(-12 * time.Hour),
			UserID:     3,
			Username:   "jdoe",
			Action:     "report.export",
			Resource:   "report",
			ResourceID: "",
			Details:    map[string]interface{}{"format": "csv", "type": "alert_summary"},
			IPAddress:  "192.168.1.102",
			Result:     "success",
		},
	}
}

// getDemoAuditStats returns demo statistics for when database is unavailable
func getDemoAuditStats() map[string]interface{} {
	return map[string]interface{}{
		"total_actions_24h":        42,
		"failed_actions_24h":       3,
		"active_users_24h":         4,
		"most_active_user":         "admin",
		"most_active_user_actions": 18,
	}
}

// GetAuditLogs returns audit logs with optional filtering and pagination
// GET /api/v1/audit-logs
func GetAuditLogs(c *gin.Context) {
	// Verify admin access - sysadmin only
	if !isAdminRole(c) {
		apiErr := errors.NewInsufficientRole(string(rbac.RoleSysAdmin))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := auditRepo()
	if repo == nil {
		// Demo mode - return demo audit logs
		demoLogs := getDemoAuditLogs()
		demoStats := getDemoAuditStats()
		logger.Info("Demo mode: returning demo audit logs")

		// Apply client-side filtering for demo mode
		filtered := filterDemoLogs(demoLogs, c)

		// Apply pagination for demo mode
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

		total := len(filtered)
		if offset > len(filtered) {
			filtered = []models.AuditLogResponse{}
		} else if offset+limit > len(filtered) {
			filtered = filtered[offset:]
		} else {
			filtered = filtered[offset : offset+limit]
		}

		c.JSON(http.StatusOK, gin.H{
			"audit_logs": filtered,
			"total":      total,
			"stats":      demoStats,
		})
		return
	}

	// Build filter from query parameters
	filter := database.AuditFilter{
		Search:   c.Query("search"),
		Action:   c.Query("action"),
		Resource: c.Query("resource"),
		Username: c.Query("username"),
		Result:   c.Query("result"),
	}

	// Parse user_id
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			filter.UserID = uint(uid)
		}
	}

	// Parse date range
	if startStr := c.Query("start_date"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			filter.StartDate = &t
		} else if t, err := time.Parse("2006-01-02", startStr); err == nil {
			filter.StartDate = &t
		}
	}
	if endStr := c.Query("end_date"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			filter.EndDate = &t
		} else if t, err := time.Parse("2006-01-02", endStr); err == nil {
			// Set to end of day
			endOfDay := t.Add(24*time.Hour - time.Second)
			filter.EndDate = &endOfDay
		}
	}

	// Parse pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	// Query audit logs
	logs, total, err := repo.List(filter)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Convert to response format
	logResponses := make([]models.AuditLogResponse, 0, len(logs))
	for _, l := range logs {
		logResponses = append(logResponses, l.ToResponse())
	}

	// Get stats
	stats, err := repo.GetStats()
	if err != nil {
		// Stats failure is non-fatal - log and continue with empty stats
		logger.Error("Failed to get audit stats: %v", err)
		stats = map[string]interface{}{
			"total_actions_24h":        0,
			"failed_actions_24h":       0,
			"active_users_24h":         0,
			"most_active_user":         "N/A",
			"most_active_user_actions": 0,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": logResponses,
		"total":      total,
		"stats":      stats,
	})
}

// GetAuditLogActions returns distinct action types for filter dropdowns
// GET /api/v1/audit-logs/actions
func GetAuditLogActions(c *gin.Context) {
	// Verify admin access
	if !isAdminRole(c) {
		apiErr := errors.NewInsufficientRole(string(rbac.RoleSysAdmin))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := auditRepo()
	if repo == nil {
		// Demo mode - return demo actions
		c.JSON(http.StatusOK, gin.H{
			"actions": []string{
				"user.create", "user.update", "user.delete", "user.login",
				"alert.acknowledge", "alert.resolve",
				"ticket.create", "ticket.update",
				"config.create", "config.update",
				"report.export",
			},
		})
		return
	}

	actions, err := repo.GetDistinctActions()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"actions": actions,
	})
}

// filterDemoLogs applies query parameter filters to demo audit logs
func filterDemoLogs(logs []models.AuditLogResponse, c *gin.Context) []models.AuditLogResponse {
	filtered := make([]models.AuditLogResponse, 0, len(logs))

	search := c.Query("search")
	action := c.Query("action")
	resource := c.Query("resource")
	username := c.Query("username")
	result := c.Query("result")

	for _, log := range logs {
		// Search filter
		if search != "" {
			searchLower := search
			if !containsInsensitive(log.Username, searchLower) &&
				!containsInsensitive(log.Action, searchLower) &&
				!containsInsensitive(log.Resource, searchLower) &&
				!containsInsensitive(log.ResourceID, searchLower) {
				continue
			}
		}

		// Action filter
		if action != "" && log.Action != action {
			continue
		}

		// Resource filter
		if resource != "" && log.Resource != resource {
			continue
		}

		// Username filter
		if username != "" && log.Username != username {
			continue
		}

		// Result filter
		if result != "" && log.Result != result {
			continue
		}

		filtered = append(filtered, log)
	}

	return filtered
}

// containsInsensitive checks if str contains substr (case-insensitive)
func containsInsensitive(str, substr string) bool {
	if substr == "" {
		return true
	}
	// Simple case-insensitive contains using lowercase comparison
	strLower := toLower(str)
	substrLower := toLower(substr)
	return len(strLower) >= len(substrLower) && containsStr(strLower, substrLower)
}

// toLower converts a string to lowercase (ASCII-only, sufficient for demo filtering)
func toLower(s string) string {
	bytes := []byte(s)
	for i, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[i] = b + 32
		}
	}
	return string(bytes)
}

// containsStr checks if s contains substr
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
