package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
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

// ==========================================
// Alert History
// ==========================================

// AlertHistoryEntry is the response shape for a resolved alert history row.
type AlertHistoryEntry struct {
	ID         int       `json:"id"`
	AlertID    string    `json:"alert_id"`
	Title      string    `json:"title"`
	Resolution string    `json:"resolution"`
	Severity   string    `json:"severity"`
	ResolvedAt time.Time `json:"resolved_at"`
}

// GetAlertHistory returns resolved alert history records.
// Supports ?severity=critical&limit=50&offset=0
// GET /api/v1/alert-history
func GetAlertHistory(c *gin.Context) {
	severity := c.Query("severity")
	limit := 50
	offset := 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	db := database.Get()
	if db == nil || db.DB == nil {
		// Demo fallback
		demo := getDemoAlertHistory()
		if severity != "" {
			filtered := demo[:0]
			for _, e := range demo {
				if e.Severity == severity {
					filtered = append(filtered, e)
				}
			}
			demo = filtered
		}
		total := len(demo)
		if offset < len(demo) {
			end := offset + limit
			if end > len(demo) {
				end = len(demo)
			}
			demo = demo[offset:end]
		} else {
			demo = []AlertHistoryEntry{}
		}
		c.JSON(http.StatusOK, gin.H{"history": demo, "total": total})
		return
	}

	type row struct {
		ID         int       `gorm:"column:id"`
		AlertID    string    `gorm:"column:alert_id"`
		Title      string    `gorm:"column:title"`
		Resolution string    `gorm:"column:resolution"`
		Severity   string    `gorm:"column:severity"`
		CreatedAt  time.Time `gorm:"column:created_at"`
	}

	q := db.DB.Table("alert_history").Select("id, alert_id, title, resolution, severity, created_at").
		Order("created_at DESC").Limit(limit).Offset(offset)
	if severity != "" {
		q = q.Where("severity = ?", severity)
	}

	var rows []row
	if err := q.Scan(&rows).Error; err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var total int64
	countQ := db.DB.Table("alert_history")
	if severity != "" {
		countQ = countQ.Where("severity = ?", severity)
	}
	countQ.Count(&total)

	entries := make([]AlertHistoryEntry, len(rows))
	for i, r := range rows {
		entries[i] = AlertHistoryEntry{
			ID:         r.ID,
			AlertID:    r.AlertID,
			Title:      r.Title,
			Resolution: r.Resolution,
			Severity:   r.Severity,
			ResolvedAt: r.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"history": entries, "total": total})
}

func getDemoAlertHistory() []AlertHistoryEntry {
	now := time.Now()
	return []AlertHistoryEntry{
		{ID: 1, AlertID: "alert-008", Title: "OSPF Neighbor Adjacency Restored", Resolution: "Auto-resolved: adjacency re-established after link flap recovery", Severity: "info", ResolvedAt: now.Add(-2 * time.Hour)},
		{ID: 2, AlertID: "alert-001", Title: "Interface GigabitEthernet0/1 Down", Resolution: "SFP module replaced, interface came back up", Severity: "critical", ResolvedAt: now.Add(-5 * time.Hour)},
		{ID: 3, AlertID: "alert-003", Title: "BGP Peer Session Flapping", Resolution: "MTU mismatch corrected on peering interface", Severity: "high", ResolvedAt: now.Add(-12 * time.Hour)},
		{ID: 4, AlertID: "alert-005", Title: "Spanning Tree Topology Change Storm", Resolution: "Offending port isolated, downstream switch reconfigured", Severity: "critical", ResolvedAt: now.Add(-24 * time.Hour)},
		{ID: 5, AlertID: "alert-004", Title: "High Client Association Failures", Resolution: "RADIUS timeout corrected, channel reassigned", Severity: "medium", ResolvedAt: now.Add(-36 * time.Hour)},
	}
}
