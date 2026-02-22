package handlers

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

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

// getDemoLinkedTickets returns demo tickets linked to an alert for demo mode
func getDemoLinkedTickets(alertID string) []gin.H {
	// Return tickets that reference this alert from the demo data
	ticketMap := map[string][]gin.H{
		"alert-001": {
			{
				"id":         "TKT-001",
				"title":      "Investigate Core Switch Interface Down",
				"status":     "open",
				"priority":   "critical",
				"created_at": time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			},
		},
		"alert-003": {
			{
				"id":         "TKT-003",
				"title":      "BGP Peering Link Investigation",
				"status":     "open",
				"priority":   "high",
				"created_at": time.Now().Add(-5 * time.Hour).Format(time.RFC3339),
			},
		},
		"alert-005": {
			{
				"id":         "TKT-005",
				"title":      "STP Storm Root Cause Analysis",
				"status":     "in-progress",
				"priority":   "critical",
				"created_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			},
		},
	}

	if tickets, ok := ticketMap[alertID]; ok {
		return tickets
	}
	return []gin.H{}
}
