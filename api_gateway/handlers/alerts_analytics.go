package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// GetSeverityDistribution returns alert distribution by severity
func GetSeverityDistribution(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			c.JSON(http.StatusOK, getDemoSeverityDistribution())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
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

// GetAlertsOverTime returns alert counts over time
func GetAlertsOverTime(c *gin.Context) {
	hours := 24
	// Support period parameter (24h, 7d, 30d, 90d) sent by frontend
	if period := c.Query("period"); period != "" {
		switch period {
		case "7d":
			hours = 168
		case "30d":
			hours = 720
		case "90d":
			hours = 2160
		default:
			// "24h" or fallback
			hours = 24
		}
	} else if hoursStr := c.Query("hours"); hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			c.JSON(http.StatusOK, getDemoAlertsOverTime(hours))
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	points, err := repo.GetAlertsOverTime(hours)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// When the DB returns no data (e.g. seed alerts have stale timestamps),
	// fall back to demo data so the chart is never empty.
	if len(points) == 0 {
		points = getDemoAlertsOverTime(hours)
	}
	c.JSON(http.StatusOK, points)
}

// GetRecurringAlerts returns patterns of recurring alerts
func GetRecurringAlerts(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if !isDemoMode() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
			return
		}
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
		if isDemoMode() {
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
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
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
