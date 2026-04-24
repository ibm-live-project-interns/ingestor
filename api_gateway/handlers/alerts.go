package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// requireJSONContentType ensures write requests carry application/json.
// Returns true if the Content-Type is acceptable (or irrelevant), and false
// if a 415 was written and the handler must return.
func requireJSONContentType(c *gin.Context) bool {
	if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
		ct := c.ContentType()
		if !strings.HasPrefix(ct, "application/json") {
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "Content-Type must be application/json"})
			return false
		}
	}
	return true
}

// alertRepo returns the alert repository using the global database
// Returns nil if database is not available (demo mode)
func alertRepo() *database.AlertRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewAlertRepository(db.DB)
}

// isDemoMode checks if demo mode is explicitly enabled via environment variable.
// This prevents accidental activation of demo mode when the database is
// temporarily unavailable (e.g., during a connection hiccup in production).
func isDemoMode() bool {
	return os.Getenv("DEMO_MODE") == "true"
}

// GetAlerts returns all alerts with optional filtering
func GetAlerts(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo alerts")
			c.JSON(http.StatusOK, gin.H{
				"alerts": getDemoAlerts(),
				"total":  len(getDemoAlerts()),
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	filter := database.AlertFilter{
		Severity: c.Query("severity"),
		Status:   c.Query("status"),
		Category: c.Query("category"),
		Device:   c.Query("device"),
	}

	// Validate query parameter lengths (max 255 chars)
	for _, v := range []string{filter.Severity, filter.Status, filter.Category, filter.Device} {
		if len(v) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter too long"})
			return
		}
	}

	// Parse time range filters
	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = &t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = &t
		}
	}

	// Validate time range
	if filter.From != nil && filter.To != nil {
		if filter.From.After(*filter.To) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to"})
			return
		}
		if filter.To.Sub(*filter.From) > 90*24*time.Hour {
			c.JSON(http.StatusBadRequest, gin.H{"error": "time range cannot exceed 90 days"})
			return
		}
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

	// Apply pagination defaults/caps
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 500 {
		filter.Limit = 500
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
		if isDemoMode() {
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

	c.JSON(http.StatusOK, alert)
}

// GetAlertsSummary returns aggregated alert statistics
func GetAlertsSummary(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			c.JSON(http.StatusOK, getDemoAlertSummary())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
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
		if !isDemoMode() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
			return
		}
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

// ==========================================
// Alert-Ticket Bidirectional Linking
// ==========================================

// GetAlertTickets returns all tickets linked to a specific alert
// GET /api/v1/alerts/:id/tickets
func GetAlertTickets(c *gin.Context) {
	alertID := c.Param("id")

	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			// Demo mode: return demo tickets linked to this alert
			logger.Info("Demo mode: returning linked tickets for alert %s", alertID)
			demoLinked := getDemoLinkedTickets(alertID)
			c.JSON(http.StatusOK, gin.H{
				"tickets":  demoLinked,
				"total":    len(demoLinked),
				"alert_id": alertID,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Query tickets where alert_id matches
	type LinkedTicket struct {
		ID        string    `json:"id"`
		Title     string    `json:"title" gorm:"column:title"`
		Status    string    `json:"status"`
		Priority  string    `json:"priority"`
		CreatedAt time.Time `json:"created_at"`
	}

	var tickets []LinkedTicket
	if err := db.Table("tickets").
		Select("id, title, status, priority, created_at").
		Where("alert_id = ? AND deleted_at IS NULL", alertID).
		Order("created_at DESC").
		Find(&tickets).Error; err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Ensure non-null JSON array
	if tickets == nil {
		tickets = []LinkedTicket{}
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets":  tickets,
		"total":    len(tickets),
		"alert_id": alertID,
	})
}
