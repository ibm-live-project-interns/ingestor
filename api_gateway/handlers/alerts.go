package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// alertRepo returns the alert repository using the global database
// Returns nil if database is not available (demo mode)
func alertRepo() *database.AlertRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewAlertRepository(db.DB)
}

// isDemoMode checks if we're running without a database
func isDemoMode() bool {
	return database.Get() == nil
}

// getDemoAlerts returns empty alert data when database is not connected
func getDemoAlerts() []models.Alert {
	// Return empty array - no fake data when database is not available
	return []models.Alert{}
}

// getDemoAlertSummary returns demo alert summary
func getDemoAlertSummary() gin.H {
	// When database is not connected, return 0 counts instead of fake data
	return gin.H{
		"activeCount":   0,
		"criticalCount": 0,
		"majorCount":    0,
		"minorCount":    0,
		"infoCount":     0,
		"resolvedToday": 0,
	}
}

// getDemoSeverityDistribution returns demo severity distribution
func getDemoSeverityDistribution() []gin.H {
	// When database is not connected, return empty data
	return []gin.H{}
}

// GetAlerts returns all alerts with optional filtering
func GetAlerts(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		// Demo mode - return demo data
		logger.Info("Demo mode: returning demo alerts")
		c.JSON(http.StatusOK, gin.H{
			"alerts": getDemoAlerts(),
			"total":  len(getDemoAlerts()),
		})
		return
	}

	filter := database.AlertFilter{
		Severity: c.Query("severity"),
		Status:   c.Query("status"),
		Category: c.Query("category"),
		Device:   c.Query("device"),
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

	alerts, total, err := repo.List(filter)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  total,
	})
}

// GetAlertByID returns a single alert by ID
func GetAlertByID(c *gin.Context) {
	id := c.Param("id")

	repo := alertRepo()
	if repo == nil {
		// Demo mode - find in demo data
		for _, alert := range getDemoAlerts() {
			if alert.ID == id {
				c.JSON(http.StatusOK, alert)
				return
			}
		}
		apiErr := errors.NewAlertNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
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

	c.JSON(http.StatusOK, alert)
}

// GetAlertsSummary returns aggregated alert statistics
func GetAlertsSummary(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		// Demo mode
		c.JSON(http.StatusOK, getDemoAlertSummary())
		return
	}

	summary, err := repo.GetSummary()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetSeverityDistribution returns alert distribution by severity
func GetSeverityDistribution(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		// Demo mode
		c.JSON(http.StatusOK, getDemoSeverityDistribution())
		return
	}

	distribution, err := repo.GetSeverityDistribution()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, distribution)
}

// getDemoAlertsOverTime returns demo time series data
func getDemoAlertsOverTime(hours int) []models.TimeSeriesPoint {
	points := make([]models.TimeSeriesPoint, 0, hours)
	now := time.Now()
	for i := hours - 1; i >= 0; i-- {
		t := now.Add(time.Duration(-i) * time.Hour)
		// Generate some realistic-looking values
		value := 5 + (i%3)*2 + (i % 5)
		points = append(points, models.TimeSeriesPoint{
			Timestamp: t,
			Label:     t.Format("15:04"),
			Value:     value,
		})
	}
	return points
}

// GetAlertsOverTime returns alert counts over time
func GetAlertsOverTime(c *gin.Context) {
	hours := 24
	if hoursStr := c.Query("hours"); hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	repo := alertRepo()
	if repo == nil {
		// Demo mode
		c.JSON(http.StatusOK, getDemoAlertsOverTime(hours))
		return
	}

	points, err := repo.GetAlertsOverTime(hours)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, points)
}

// GetRecurringAlerts returns patterns of recurring alerts
func GetRecurringAlerts(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		// Demo mode - return demo recurring alerts
		recurring := []models.RecurringAlert{
			{
				Pattern:   "High CPU utilization on network devices",
				Count:     12,
				FirstSeen: time.Now().Add(-72 * time.Hour),
				LastSeen:  time.Now().Add(-1 * time.Hour),
				Devices:   []string{"core-router-01", "core-router-02"},
				Severity:  constants.SeverityHigh,
			},
			{
				Pattern:   "Interface flapping on distribution switches",
				Count:     8,
				FirstSeen: time.Now().Add(-48 * time.Hour),
				LastSeen:  time.Now().Add(-30 * time.Minute),
				Devices:   []string{"sw-dc1-01", "sw-dc1-02"},
				Severity:  constants.SeverityMedium,
			},
		}
		c.JSON(http.StatusOK, recurring)
		return
	}

	// Get noisy devices as a proxy for recurring patterns
	noisyDevices, err := repo.GetNoisyDevices(10)
	if err != nil {
		c.JSON(http.StatusOK, []models.RecurringAlert{})
		return
	}

	var recurring []models.RecurringAlert
	for _, device := range noisyDevices {
		recurring = append(recurring, models.RecurringAlert{
			Pattern:   device.TopIssue,
			Count:     device.AlertCount,
			FirstSeen: time.Now().Add(-72 * time.Hour),
			LastSeen:  time.Now(),
			Devices:   []string{device.DeviceName},
			Severity:  constants.SeverityMedium,
		})
	}

	c.JSON(http.StatusOK, recurring)
}

// GetAlertDistributionTime returns alert distribution by time of day
func GetAlertDistributionTime(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		// Demo mode
		distribution := getDemoAlertsOverTime(24)
		result := make([]gin.H, 0, len(distribution))
		for _, p := range distribution {
			result = append(result, gin.H{
				"hour":  p.Label,
				"count": p.Value,
			})
		}
		c.JSON(http.StatusOK, result)
		return
	}

	points, err := repo.GetAlertsOverTime(24)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Convert to hour distribution format
	distribution := make([]gin.H, 0, len(points))
	for _, p := range points {
		distribution = append(distribution, gin.H{
			"hour":  p.Label,
			"count": p.Value,
		})
	}

	c.JSON(http.StatusOK, distribution)
}

// AcknowledgeAlert marks an alert as acknowledged
func AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	username, _ := c.Get("username")
	usernameStr := fmt.Sprintf("%v", username)

	repo := alertRepo()
	if repo == nil {
		// Demo mode - return success
		logger.Info("Demo mode: Alert %s acknowledged by %s", id, usernameStr)
		c.JSON(http.StatusOK, gin.H{
			"message":         "Alert acknowledged (demo mode)",
			"alert_id":        id,
			"acknowledged_by": usernameStr,
		})
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
		// Demo mode - return success
		logger.Info("Demo mode: Alert %s dismissed by %s", id, usernameStr)
		c.JSON(http.StatusOK, gin.H{
			"message":      "Alert dismissed (demo mode)",
			"alert_id":     id,
			"dismissed_by": usernameStr,
		})
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

	c.JSON(http.StatusOK, gin.H{
		"message":      "Alert dismissed",
		"alert":        updatedAlert,
		"dismissed_by": usernameStr,
	})
}

// ExportAlerts exports alerts as CSV
func ExportAlerts(c *gin.Context) {
	filter := database.AlertFilter{
		Severity: c.Query("severity"),
		Status:   c.Query("status"),
		Category: c.Query("category"),
	}

	repo := alertRepo()
	var alerts []models.Alert

	if repo == nil {
		// Demo mode - use demo data
		demoAlerts := getDemoAlerts()
		for _, da := range demoAlerts {
			alerts = append(alerts, models.Alert{
				ID:        da.ID,
				Title:     da.Title,
				Severity:  da.Severity,
				Status:    da.Status,
				Category:  da.Category,
				Device:    da.Device,
				Source:    da.Source,
				Timestamp: da.Timestamp,
			})
		}
	} else {
		var err error
		alerts, _, err = repo.List(filter)
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=alerts-report-%s.csv", time.Now().Format("2006-01-02")))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Header
	writer.Write([]string{"ID", "Title", "Severity", "Status", "Category", "Device", "Source", "Timestamp"})

	for _, alert := range alerts {
		writer.Write([]string{
			alert.ID,
			alert.Title,
			alert.Severity,
			alert.Status,
			alert.Category,
			alert.Device,
			alert.Source,
			alert.Timestamp.Format(time.RFC3339),
		})
	}
}
