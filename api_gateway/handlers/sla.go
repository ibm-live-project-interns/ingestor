package handlers

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// SLA thresholds per severity (in minutes)
var slaThresholds = map[string]float64{
	constants.SeverityCritical: 15,   // 15 minutes
	constants.SeverityHigh:     30,   // 30 minutes
	constants.SeverityMedium:   120,  // 2 hours
	constants.SeverityLow:      480,  // 8 hours
	constants.SeverityInfo:     1440, // 24 hours (informational, generous threshold)
}

// slaThresholdLabel returns a human-readable SLA threshold string
func slaThresholdLabel(severity string) string {
	switch severity {
	case constants.SeverityCritical:
		return "15m"
	case constants.SeverityHigh:
		return "30m"
	case constants.SeverityMedium:
		return "2h"
	case constants.SeverityLow:
		return "8h"
	case constants.SeverityInfo:
		return "24h"
	default:
		return "N/A"
	}
}

// parsePeriod converts a period string to a time.Duration and returns the start time
func parsePeriod(period string) time.Time {
	now := time.Now().UTC()
	switch period {
	case "7d":
		return now.AddDate(0, 0, -7)
	case "30d":
		return now.AddDate(0, 0, -30)
	case "90d":
		return now.AddDate(0, 0, -90)
	default:
		// Default to 7 days
		return now.AddDate(0, 0, -7)
	}
}

// resolutionMinutes computes the resolution time of an alert in minutes.
// Returns -1 if the alert is not resolved.
func resolutionMinutes(alert models.Alert) float64 {
	if alert.ResolvedAt == nil {
		return -1
	}
	return alert.ResolvedAt.Sub(alert.Timestamp).Minutes()
}

// acknowledgmentMinutes computes the time to acknowledge in minutes.
// Returns -1 if the alert is not acknowledged.
func acknowledgmentMinutes(alert models.Alert) float64 {
	if alert.AckedAt == nil {
		return -1
	}
	return alert.AckedAt.Sub(alert.Timestamp).Minutes()
}

// isViolation checks whether a resolved alert violated its SLA threshold
func isViolation(alert models.Alert) bool {
	resolveTime := resolutionMinutes(alert)
	if resolveTime < 0 {
		return false
	}
	threshold, ok := slaThresholds[alert.Severity]
	if !ok {
		return false
	}
	return resolveTime > threshold
}

// formatDuration formats minutes into a human-readable duration string
func formatDuration(minutes float64) string {
	if minutes < 0 {
		return "N/A"
	}
	if minutes < 1 {
		seconds := int(minutes * 60)
		return formatPlural(seconds, "second")
	}
	if minutes < 60 {
		return formatPlural(int(minutes), "minute")
	}
	hours := minutes / 60
	if hours < 24 {
		h := int(hours)
		m := int(minutes) % 60
		if m == 0 {
			return formatPlural(h, "hour")
		}
		return formatPlural(h, "hour") + " " + formatPlural(m, "minute")
	}
	days := int(hours / 24)
	remainingHours := int(hours) % 24
	if remainingHours == 0 {
		return formatPlural(days, "day")
	}
	return formatPlural(days, "day") + " " + formatPlural(remainingHours, "hour")
}

func formatPlural(count int, unit string) string {
	if count == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(count) + " " + unit + "s"
}

// getDemoSLAOverview returns demo/empty SLA overview data
func getDemoSLAOverview() gin.H {
	return gin.H{
		"compliance_percent": 0.0,
		"mttr_minutes":       0.0,
		"mttr_display":       "N/A",
		"mtta_minutes":       0.0,
		"mtta_display":       "N/A",
		"total_violations":   0,
		"total_alerts":       0,
		"resolved_count":     0,
		"period":             "7d",
		"by_severity": []gin.H{},
	}
}

// getDemoSLAViolations returns demo/empty SLA violations data
func getDemoSLAViolations() gin.H {
	return gin.H{
		"violations": []gin.H{},
		"total":      0,
		"period":     "7d",
	}
}

// getDemoSLATrend returns demo/empty SLA trend data
func getDemoSLATrend() gin.H {
	return gin.H{
		"trend":  []gin.H{},
		"period": "7d",
	}
}

// GetSLAOverview returns SLA overview metrics computed from alert data.
// GET /api/v1/reports/sla?period=7d|30d|90d
func GetSLAOverview(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo SLA overview")
			c.JSON(http.StatusOK, getDemoSLAOverview())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	period := c.DefaultQuery("period", "7d")
	since := parsePeriod(period)

	// Fetch all alerts within the period
	alerts, err := fetchAlertsInPeriod(repo, since)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Compute metrics
	var (
		totalResolved   int
		totalViolations int
		totalMTTR       float64
		mttrCount       int
		totalMTTA       float64
		mttaCount       int
	)

	// Per-severity stats
	type severityStats struct {
		Total     int     `json:"total"`
		Met       int     `json:"met"`
		Violated  int     `json:"violated"`
		SLATarget string  `json:"sla_target"`
		Compliance float64 `json:"compliance_percent"`
	}
	bySeverity := make(map[string]*severityStats)
	for _, sev := range constants.AllSeverities {
		bySeverity[sev] = &severityStats{
			SLATarget: slaThresholdLabel(sev),
		}
	}

	for i := range alerts {
		alert := alerts[i]

		// Track per-severity totals
		if stats, ok := bySeverity[alert.Severity]; ok {
			stats.Total++
		}

		// Only resolved alerts contribute to MTTR and SLA compliance
		if alert.ResolvedAt != nil {
			totalResolved++
			resolveTime := resolutionMinutes(alert)
			if resolveTime >= 0 {
				totalMTTR += resolveTime
				mttrCount++

				// Check SLA compliance
				threshold, thresholdExists := slaThresholds[alert.Severity]
				if thresholdExists {
					if stats, ok := bySeverity[alert.Severity]; ok {
						if resolveTime > threshold {
							stats.Violated++
							totalViolations++
						} else {
							stats.Met++
						}
					}
				}
			}
		}

		// MTTA: any acknowledged alert contributes
		if alert.AckedAt != nil {
			ackTime := acknowledgmentMinutes(alert)
			if ackTime >= 0 {
				totalMTTA += ackTime
				mttaCount++
			}
		}
	}

	// Compute averages
	var avgMTTR, avgMTTA float64
	if mttrCount > 0 {
		avgMTTR = totalMTTR / float64(mttrCount)
	}
	if mttaCount > 0 {
		avgMTTA = totalMTTA / float64(mttaCount)
	}

	// Compute overall compliance
	var compliancePercent float64
	totalEvaluated := totalResolved
	if totalEvaluated > 0 {
		compliant := totalEvaluated - totalViolations
		compliancePercent = math.Round(float64(compliant)/float64(totalEvaluated)*10000) / 100
	}

	// Build per-severity response
	severityResults := make([]gin.H, 0, len(constants.AllSeverities))
	for _, sev := range constants.AllSeverities {
		stats := bySeverity[sev]
		evaluated := stats.Met + stats.Violated
		pct := 0.0
		if evaluated > 0 {
			pct = math.Round(float64(stats.Met)/float64(evaluated)*10000) / 100
		}
		stats.Compliance = pct

		severityResults = append(severityResults, gin.H{
			"severity":           sev,
			"sla_target":         stats.SLATarget,
			"total":              stats.Total,
			"met":                stats.Met,
			"violated":           stats.Violated,
			"compliance_percent": stats.Compliance,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"compliance_percent": compliancePercent,
		"mttr_minutes":       math.Round(avgMTTR*100) / 100,
		"mttr_display":       formatDuration(avgMTTR),
		"mtta_minutes":       math.Round(avgMTTA*100) / 100,
		"mtta_display":       formatDuration(avgMTTA),
		"total_violations":   totalViolations,
		"total_alerts":       len(alerts),
		"resolved_count":     totalResolved,
		"period":             period,
		"by_severity":        severityResults,
	})
}

// GetSLAViolations returns a list of alerts that violated their SLA threshold.
// GET /api/v1/reports/sla/violations?period=7d|30d|90d
func GetSLAViolations(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo SLA violations")
			c.JSON(http.StatusOK, getDemoSLAViolations())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	period := c.DefaultQuery("period", "7d")
	since := parsePeriod(period)

	alerts, err := fetchAlertsInPeriod(repo, since)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	violations := make([]gin.H, 0)
	for i := range alerts {
		alert := alerts[i]
		if alert.ResolvedAt == nil {
			continue
		}

		resolveTime := resolutionMinutes(alert)
		if resolveTime < 0 {
			continue
		}

		threshold, thresholdExists := slaThresholds[alert.Severity]
		if !thresholdExists {
			continue
		}

		if resolveTime > threshold {
			excessMinutes := resolveTime - threshold

			violations = append(violations, gin.H{
				"alert_id":        alert.ID,
				"title":           alert.Title,
				"severity":        alert.Severity,
				"device":          alert.Device,
				"source_ip":       alert.SourceIP,
				"category":        alert.Category,
				"timestamp":       alert.Timestamp,
				"resolved_at":     alert.ResolvedAt,
				"expected_minutes": threshold,
				"expected_display": formatDuration(threshold),
				"actual_minutes":   math.Round(resolveTime*100) / 100,
				"actual_display":   formatDuration(resolveTime),
				"excess_minutes":   math.Round(excessMinutes*100) / 100,
				"excess_display":   formatDuration(excessMinutes),
				"description":     alert.Description,
				"ai_summary":      alert.AIAnalysisSummary,
				"resolved_by":     alert.ResolvedBy,
			})

			// Send SLA violation email (non-blocking)
			if services.Email != nil {
				aID := alert.ID
				aSev := alert.Severity
				aDevice := alert.Device
				exceededBy := formatDuration(excessMinutes)
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = ctx
					custom := map[string]interface{}{
						"AlertID":  aID,
						"SLAType":  aSev,
						"Device":   aDevice,
						"Exceeded": exceededBy,
					}
					if err := services.Email.SendNotification("oncall@sentrix.local", "On-Call", "SLA Violation", "sla-violation", custom); err != nil {
						logger.Error("Failed to send sla-violation email for alert %s: %v", aID, err)
					}
				}()
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"violations": violations,
		"total":      len(violations),
		"period":     period,
	})
}

// GetSLATrend returns daily SLA compliance percentages for the given period.
// GET /api/v1/reports/sla/trend?period=7d|30d|90d
func GetSLATrend(c *gin.Context) {
	repo := alertRepo()
	if repo == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo SLA trend")
			c.JSON(http.StatusOK, getDemoSLATrend())
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	period := c.DefaultQuery("period", "7d")
	since := parsePeriod(period)

	alerts, err := fetchAlertsInPeriod(repo, since)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Group resolved alerts by date (YYYY-MM-DD)
	type dayBucket struct {
		Date     string
		Met      int
		Violated int
	}

	// Build day buckets from the period start to today
	now := time.Now().UTC()
	dayMap := make(map[string]*dayBucket)
	var orderedDays []string

	for d := since; !d.After(now); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		dayMap[key] = &dayBucket{Date: key}
		orderedDays = append(orderedDays, key)
	}

	// Also ensure today is included
	todayKey := now.Format("2006-01-02")
	if _, exists := dayMap[todayKey]; !exists {
		dayMap[todayKey] = &dayBucket{Date: todayKey}
		orderedDays = append(orderedDays, todayKey)
	}

	// Distribute resolved alerts into day buckets
	for i := range alerts {
		alert := alerts[i]
		if alert.ResolvedAt == nil {
			continue
		}

		resolveTime := resolutionMinutes(alert)
		if resolveTime < 0 {
			continue
		}

		// Use the resolution date as the bucket key
		dayKey := alert.ResolvedAt.Format("2006-01-02")
		bucket, ok := dayMap[dayKey]
		if !ok {
			// Alert resolved outside our range; skip
			continue
		}

		threshold, thresholdExists := slaThresholds[alert.Severity]
		if !thresholdExists {
			continue
		}

		if resolveTime > threshold {
			bucket.Violated++
		} else {
			bucket.Met++
		}
	}

	// Build trend response
	trend := make([]gin.H, 0, len(orderedDays))
	for _, dayKey := range orderedDays {
		bucket := dayMap[dayKey]
		total := bucket.Met + bucket.Violated
		compliance := 0.0
		if total > 0 {
			compliance = math.Round(float64(bucket.Met)/float64(total)*10000) / 100
		} else {
			// No data for this day; use 100% (no violations if no alerts)
			compliance = 100.0
		}
		trend = append(trend, gin.H{
			"date":               dayKey,
			"compliance_percent": compliance,
			"met":                bucket.Met,
			"violated":           bucket.Violated,
			"total":              total,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"trend":  trend,
		"period": period,
	})
}

// maxSLAAlerts is the hard cap on alerts fetched for SLA computation
// to prevent OOM on large datasets.
const maxSLAAlerts = 10000

// fetchAlertsInPeriod retrieves alerts from the given start time to now.
// It uses the repository's List method with time filtering and a hard cap
// of maxSLAAlerts to prevent unbounded memory consumption.
func fetchAlertsInPeriod(repo *database.AlertRepository, since time.Time) ([]models.Alert, error) {
	now := time.Now().UTC()
	filter := database.AlertFilter{
		From:  &since,
		To:    &now,
		Limit: maxSLAAlerts,
	}

	alerts, _, err := repo.List(filter)
	if err != nil {
		logger.Error("Failed to fetch alerts for SLA computation: %v", err)
		return nil, err
	}

	return alerts, nil
}
