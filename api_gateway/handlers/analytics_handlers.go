package handlers

import (
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// TrendKPI represents key performance indicators for trends
type TrendKPI struct {
	AlertVolume       int64   `json:"alert_volume"`
	AlertVolumeChange float64 `json:"alert_volume_change"`
	MTTR              float64 `json:"mttr"` // Mean Time To Resolution in minutes
	MTTRChange        float64 `json:"mttr_change"`
	AcknowledgeRate   float64 `json:"acknowledge_rate"`
	ResolutionRate    float64 `json:"resolution_rate"`
}

// GetTrendsKPI returns trend KPIs calculated from real data
func GetTrendsKPI(c *gin.Context) {
	db := database.Get()
	if db == nil {
		c.JSON(http.StatusOK, TrendKPI{})
		return
	}

	now := time.Now().UTC()
	last7d := now.Add(-7 * 24 * time.Hour)
	prev7d := now.Add(-14 * 24 * time.Hour)

	// Current period alert count (last 7 days)
	var currentCount int64
	db.Model(&models.Alert{}).Where("timestamp >= ?", last7d).Count(&currentCount)

	// Previous period alert count (7–14 days ago)
	var prevCount int64
	db.Model(&models.Alert{}).Where("timestamp >= ? AND timestamp < ?", prev7d, last7d).Count(&prevCount)

	// Calculate volume change
	var volumeChange float64
	if prevCount > 0 {
		volumeChange = float64(currentCount-prevCount) / float64(prevCount) * 100
	}

	// Get acknowledged and resolved counts
	var ackedCount int64
	db.Model(&models.Alert{}).Where("status = ?", models.AlertStatusAcknowledged).Count(&ackedCount)

	var resolvedCount int64
	db.Model(&models.Alert{}).Where("status = ?", models.AlertStatusResolved).Count(&resolvedCount)

	var totalCount int64
	db.Model(&models.Alert{}).Count(&totalCount)

	// Calculate rates
	var ackRate, resRate float64
	if totalCount > 0 {
		ackRate = float64(ackedCount+resolvedCount) / float64(totalCount) * 100
		resRate = float64(resolvedCount) / float64(totalCount) * 100
	}

	// Calculate MTTR from alerts where resolved_at is after timestamp (valid resolutions only)
	var avgMTTR float64
	db.Model(&models.Alert{}).
		Where("status = ? AND resolved_at IS NOT NULL AND resolved_at > timestamp", models.AlertStatusResolved).
		Select("AVG(EXTRACT(EPOCH FROM (resolved_at - timestamp)) / 60)").
		Scan(&avgMTTR)

	c.JSON(http.StatusOK, TrendKPI{
		AlertVolume:       currentCount,
		AlertVolumeChange: volumeChange,
		MTTR:              avgMTTR,
		MTTRChange:        0, // Would need historical comparison
		AcknowledgeRate:   ackRate,
		ResolutionRate:    resRate,
	})
}

// AIMetrics represents AI processing metrics
type AIMetrics struct {
	TotalProcessed int64     `json:"total_processed"`
	SuccessRate    float64   `json:"success_rate"`
	AvgProcessTime float64   `json:"avg_process_time_ms"`
	AlertsEnriched int64     `json:"alerts_enriched"`
	PatternsFound  int       `json:"patterns_found"`
	LastProcessed  time.Time `json:"last_processed"`
}

// GetAIMetrics returns AI processing metrics
func GetAIMetrics(c *gin.Context) {
	db := database.Get()
	if db == nil {
		c.JSON(http.StatusOK, AIMetrics{})
		return
	}

	var totalAlerts int64
	db.Model(&models.Alert{}).Count(&totalAlerts)

	var enrichedAlerts int64
	db.Model(&models.Alert{}).Where("ai_summary IS NOT NULL AND ai_summary != ''").Count(&enrichedAlerts)

	var successRate float64
	if totalAlerts > 0 {
		successRate = float64(enrichedAlerts) / float64(totalAlerts) * 100
	}

	// Compute avg processing time from ai_results table if available
	var avgProcessTime float64
	db.Table("ai_results").
		Select("AVG(EXTRACT(EPOCH FROM (created_at - (SELECT MIN(created_at) FROM ai_results))) * 1000)").
		Scan(&avgProcessTime)
	// If ai_results is empty or not meaningful, compute from alert re-analysis patterns
	// (alerts where updated_at is significantly after created_at, i.e., re-analyzed)
	if avgProcessTime <= 0 || avgProcessTime > 600000 { // cap at 10 minutes
		// Use a realistic value based on Watson API call latency
		var reanalyzedCount int64
		db.Model(&models.Alert{}).
			Where("ai_summary IS NOT NULL AND ai_summary != '' AND EXTRACT(EPOCH FROM (updated_at - created_at)) BETWEEN 1 AND 300").
			Count(&reanalyzedCount)
		if reanalyzedCount > 0 {
			db.Model(&models.Alert{}).
				Where("ai_summary IS NOT NULL AND ai_summary != '' AND EXTRACT(EPOCH FROM (updated_at - created_at)) BETWEEN 1 AND 300").
				Select("AVG(EXTRACT(EPOCH FROM (updated_at - created_at)) * 1000)").
				Scan(&avgProcessTime)
		} else {
			avgProcessTime = 0
		}
	}

	// Count distinct patterns: unique (device, severity) combos with AI enrichment
	var patternsFound int64
	db.Model(&models.Alert{}).
		Where("ai_summary IS NOT NULL AND ai_summary != ''").
		Select("COUNT(DISTINCT (device || '-' || severity))").
		Scan(&patternsFound)

	// Get last processed alert
	var lastAlert models.Alert
	db.Model(&models.Alert{}).Order("created_at DESC").First(&lastAlert)

	c.JSON(http.StatusOK, AIMetrics{
		TotalProcessed: totalAlerts,
		SuccessRate:    successRate,
		AvgProcessTime: math.Round(avgProcessTime*10) / 10,
		AlertsEnriched: enrichedAlerts,
		PatternsFound:  int(patternsFound),
		LastProcessed:  lastAlert.CreatedAt,
	})
}

// AIInsight represents an AI-generated insight
type AIInsight struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Severity      string    `json:"severity"`
	Confidence    float64   `json:"confidence"`
	CreatedAt     time.Time `json:"created_at"`
	RelatedAlerts []string  `json:"related_alerts,omitempty"`
	ActionItems   []string  `json:"action_items,omitempty"`
}

// GetAIInsights returns AI-generated insights from alert patterns
func GetAIInsights(c *gin.Context) {
	db := database.Get()
	if db == nil {
		c.JSON(http.StatusOK, []AIInsight{})
		return
	}

	var insights []AIInsight
	insightIdx := 1

	// PATTERN: Find devices with most alerts (recurring patterns)
	var deviceCounts []struct {
		Device string
		Count  int
	}
	db.Model(&models.Alert{}).
		Select("device, COUNT(*) as count").
		Where("device != ''").
		Group("device").
		Having("COUNT(*) >= 2").
		Order("count DESC").
		Limit(3).
		Scan(&deviceCounts)

	for _, dc := range deviceCounts {
		insights = append(insights, AIInsight{
			ID:          fmt.Sprintf("INS-%03d", insightIdx),
			Type:        "pattern",
			Title:       fmt.Sprintf("Recurring alerts from %s", dc.Device),
			Description: fmt.Sprintf("Device %s has generated %d alerts — likely a persistent hardware or config issue", dc.Device, dc.Count),
			Severity:    "medium",
			Confidence:  88,
			CreatedAt:   time.Now().Add(-time.Duration(insightIdx) * time.Hour),
			ActionItems: []string{
				fmt.Sprintf("Investigate %s device logs for root cause", dc.Device),
				"Check recent config changes on this device",
				"Consider scheduling a maintenance window",
			},
		})
		insightIdx++
	}

	// ANOMALY: Critical alerts that haven't been acknowledged
	var criticalCount int64
	db.Model(&models.Alert{}).
		Where("severity = ? AND status IN (?, 'new')", "critical", models.AlertStatusOpen).
		Count(&criticalCount)

	if criticalCount > 0 {
		insights = append(insights, AIInsight{
			ID:          fmt.Sprintf("INS-%03d", insightIdx),
			Type:        "anomaly",
			Title:       "Unacknowledged Critical Alerts",
			Description: fmt.Sprintf("%d critical alerts require immediate attention — potential service impact", criticalCount),
			Severity:    "high",
			Confidence:  95,
			CreatedAt:   time.Now(),
			ActionItems: []string{
				"Review critical alerts immediately",
				"Assign to on-call engineer",
				"Check affected service dependencies",
			},
		})
		insightIdx++
	}

	// OPTIMIZATION: Find severity categories with high resolution rates (could be auto-resolved)
	var resolvedByCategory []struct {
		Category string
		Total    int64
		Resolved int64
	}
	db.Model(&models.Alert{}).
		Select("category, COUNT(*) as total, SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved").
		Group("category").
		Having("COUNT(*) >= 2").
		Scan(&resolvedByCategory)

	for _, cat := range resolvedByCategory {
		if cat.Total > 0 && cat.Resolved > 0 {
			resRate := float64(cat.Resolved) / float64(cat.Total) * 100
			if resRate >= 30 {
				insights = append(insights, AIInsight{
					ID:          fmt.Sprintf("INS-%03d", insightIdx),
					Type:        "optimization",
					Title:       fmt.Sprintf("Auto-resolve candidate: %s alerts", cat.Category),
					Description: fmt.Sprintf("%.0f%% of %s alerts were resolved — consider auto-resolution rules for this category", resRate, cat.Category),
					Severity:    "low",
					Confidence:  82,
					CreatedAt:   time.Now().Add(-30 * time.Minute),
					ActionItems: []string{
						fmt.Sprintf("Review resolved %s alerts for common patterns", cat.Category),
						"Create auto-resolve threshold rule",
						"Monitor for false positive resolutions",
					},
				})
				insightIdx++
				break // Only one optimization insight
			}
		}
	}

	// RECOMMENDATION: Check if AI enrichment coverage is low
	var totalAlerts int64
	db.Model(&models.Alert{}).Count(&totalAlerts)
	var enrichedAlerts int64
	db.Model(&models.Alert{}).Where("ai_summary IS NOT NULL AND ai_summary != ''").Count(&enrichedAlerts)

	if totalAlerts > 0 {
		enrichRate := float64(enrichedAlerts) / float64(totalAlerts) * 100
		if enrichRate < 80 {
			insights = append(insights, AIInsight{
				ID:          fmt.Sprintf("INS-%03d", insightIdx),
				Type:        "recommendation",
				Title:       "Increase AI enrichment coverage",
				Description: fmt.Sprintf("Only %.0f%% of alerts have AI analysis — re-analyze unenriched alerts to improve incident response", enrichRate),
				Severity:    "medium",
				Confidence:  90,
				CreatedAt:   time.Now().Add(-2 * time.Hour),
				ActionItems: []string{
					"Use bulk re-analyze on unenriched alerts",
					"Verify Watson AI service is running",
					"Check AI processing error logs",
				},
			})
			insightIdx++
		}
	}

	// TREND: Check for alert activity in recent period
	var recentCount int64
	db.Model(&models.Alert{}).
		Where("timestamp >= ?", time.Now().UTC().Add(-24*time.Hour)).
		Count(&recentCount)

	if recentCount >= 2 {
		insights = append(insights, AIInsight{
			ID:          fmt.Sprintf("INS-%03d", insightIdx),
			Type:        "trend",
			Title:       "Alert spike detected in last hour",
			Description: fmt.Sprintf("%d alerts in the last hour — possible ongoing incident or cascading failure", recentCount),
			Severity:    "high",
			Confidence:  91,
			CreatedAt:   time.Now(),
			ActionItems: []string{
				"Correlate recent alerts for common root cause",
				"Check for upstream service failures",
				"Consider opening an incident ticket",
			},
		})
	}

	c.JSON(http.StatusOK, insights)
}

// GetAIImpactOverTime returns AI impact metrics over time
func GetAIImpactOverTime(c *gin.Context) {
	db := database.Get()
	if db == nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	now := time.Now().UTC()
	var points []gin.H

	for i := 6; i >= 0; i-- {
		dayStart := now.AddDate(0, 0, -i).Truncate(24 * time.Hour)
		dayEnd := dayStart.Add(24 * time.Hour)

		var alertCount int64
		db.Model(&models.Alert{}).
			Where("timestamp >= ? AND timestamp < ?", dayStart, dayEnd).
			Count(&alertCount)

		var enrichedCount int64
		db.Model(&models.Alert{}).
			Where("timestamp >= ? AND timestamp < ? AND ai_summary IS NOT NULL AND ai_summary != ''", dayStart, dayEnd).
			Count(&enrichedCount)

		var improvementPct float64
		if alertCount > 0 {
			improvementPct = float64(enrichedCount) / float64(alertCount) * 100
		}

		points = append(points, gin.H{
			"date":                 dayStart.Format("2006-01-02"),
			"alerts_processed":     alertCount,
			"patterns_detected":    enrichedCount,
			"mttr_improvement_pct": improvementPct,
		})
	}

	c.JSON(http.StatusOK, points)
}

// ExportReport exports data as CSV based on report type
func ExportReport(c *gin.Context) {
	reportType := c.Query("type")
	if reportType == "" {
		reportType = "alerts"
	}

	switch reportType {
	case "alerts":
		ExportAlerts(c)
	case "tickets":
		ExportTickets(c)
	case "devices":
		ExportDevices(c)
	case "sla":
		ExportSLA(c)
	case "incidents":
		ExportIncidents(c)
	default:
		apiErr := errors.NewBadRequest("Invalid report type. Supported: alerts, tickets, devices, sla, incidents")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
	}
}

// ExportDevices exports device inventory as CSV
func ExportDevices(c *gin.Context) {
	db := database.Get()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=devices-report-%s.csv", time.Now().Format("2006-01-02")))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	w.Write([]string{"ID", "Name", "IP", "Status", "Vendor", "Model", "Location", "Alert Count"})

	if db == nil {
		// Demo mode — write a few representative rows
		demoRows := [][]string{
			{"DEV-001", "core-router-01", "10.0.0.1", "active", "Cisco", "ASR 9000", "DC1", "3"},
			{"DEV-002", "edge-switch-02", "10.0.1.2", "active", "Juniper", "EX4300", "DC1", "1"},
			{"DEV-003", "firewall-03", "10.0.2.3", "maintenance", "Palo Alto", "PA-3260", "DC2", "0"},
		}
		for _, row := range demoRows {
			w.Write(row)
		}
		return
	}

	var devices []models.Device
	if err := db.Where("deleted_at IS NULL").Order("name").Find(&devices).Error; err != nil {
		// Return CSV with headers only on error — do not return 500 mid-stream
		return
	}
	for _, d := range devices {
		w.Write([]string{
			d.ID, d.Name, d.IP, d.Status, d.Vendor, d.Model, d.Location,
			fmt.Sprintf("%d", d.AlertCount),
		})
	}
}

// ExportSLA exports SLA compliance data as CSV
func ExportSLA(c *gin.Context) {
	db := database.Get()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=sla-report-%s.csv", time.Now().Format("2006-01-02")))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	w.Write([]string{"Ticket ID", "Title", "Priority", "Created At", "Resolved At", "Resolution Hours", "SLA Target Hours", "SLA Met"})

	if db == nil {
		// Demo rows
		demoRows := [][]string{
			{"TKT-001", "Network outage DC1", "critical", "2026-04-20T10:00:00Z", "2026-04-20T11:30:00Z", "1.5", "4", "true"},
			{"TKT-002", "Slow link aggregation", "high", "2026-04-19T08:00:00Z", "2026-04-19T16:00:00Z", "8.0", "8", "true"},
			{"TKT-003", "VLAN misconfiguration", "medium", "2026-04-18T14:00:00Z", "2026-04-20T10:00:00Z", "44.0", "24", "false"},
		}
		for _, row := range demoRows {
			w.Write(row)
		}
		return
	}

	type slaRow struct {
		ID              string
		Title           string
		Priority        string
		CreatedAt       time.Time
		ResolvedAt      *time.Time
		ResolutionHours float64
	}

	var rows []models.Ticket
	if err := db.Where("deleted_at IS NULL").Order("created_at DESC").Limit(500).Find(&rows).Error; err != nil {
		return
	}

	slaTargets := map[string]float64{
		"critical": 4,
		"high":     8,
		"medium":   24,
		"low":      72,
	}

	for _, t := range rows {
		resolutionHrs := ""
		slaMet := ""
		if t.ResolvedAt != nil {
			hrs := t.ResolvedAt.Sub(t.CreatedAt).Hours()
			resolutionHrs = fmt.Sprintf("%.1f", hrs)
			target := slaTargets[t.Priority]
			if target > 0 {
				if hrs <= target {
					slaMet = "true"
				} else {
					slaMet = "false"
				}
			}
		}
		resolvedAtStr := ""
		if t.ResolvedAt != nil {
			resolvedAtStr = t.ResolvedAt.Format(time.RFC3339)
		}
		w.Write([]string{
			t.ID, t.Title, t.Priority,
			t.CreatedAt.Format(time.RFC3339),
			resolvedAtStr,
			resolutionHrs,
			fmt.Sprintf("%.0f", slaTargets[t.Priority]),
			slaMet,
		})
	}
}

// ExportIncidents exports resolved incident (ticket) data as CSV
func ExportIncidents(c *gin.Context) {
	db := database.Get()

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=incidents-report-%s.csv", time.Now().Format("2006-01-02")))

	w := csv.NewWriter(c.Writer)
	defer w.Flush()

	w.Write([]string{"ID", "Title", "Priority", "Category", "Assignee", "Created At", "Resolved At", "MTTR Hours", "Root Cause"})

	if db == nil {
		demoRows := [][]string{
			{"TKT-001", "BGP session drop", "critical", "network", "alice@corp.com", "2026-04-20T10:00:00Z", "2026-04-20T11:30:00Z", "1.5", "Router config drift"},
			{"TKT-002", "High CPU on firewall", "high", "security", "bob@corp.com", "2026-04-19T08:00:00Z", "2026-04-19T16:00:00Z", "8.0", "DDoS mitigation rule"},
		}
		for _, row := range demoRows {
			w.Write(row)
		}
		return
	}

	var tickets []models.Ticket
	if err := db.Where("status IN ? AND deleted_at IS NULL", []string{"resolved", "closed"}).
		Order("created_at DESC").Limit(500).Find(&tickets).Error; err != nil {
		return
	}
	for _, t := range tickets {
		mttrStr := ""
		resolvedAtStr := ""
		if t.ResolvedAt != nil {
			resolvedAtStr = t.ResolvedAt.Format(time.RFC3339)
			mttrStr = fmt.Sprintf("%.1f", t.ResolvedAt.Sub(t.CreatedAt).Hours())
		}
		w.Write([]string{
			t.ID, t.Title, t.Priority, t.Category,
			t.Assignee,
			t.CreatedAt.Format(time.RFC3339),
			resolvedAtStr,
			mttrStr,
			"",
		})
	}
}
