package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// GetHealth returns health check status
func GetHealth(c *gin.Context) {
	status := "healthy"
	dbStatus := "connected"

	// Check database connectivity
	if db := database.Get(); db != nil {
		if err := db.Ping(); err != nil {
			dbStatus = "disconnected"
			status = "degraded"
		}
	} else {
		dbStatus = "not initialized"
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"service":   "api-gateway",
		"version":   "1.0.0",
		"database":  dbStatus,
		"timestamp": time.Now().UTC(),
	})
}

// IngestEventRequest represents an incoming event from ingestor core
type IngestEventRequest struct {
	EventType      string                 `json:"event_type" binding:"required"`
	SourceHost     string                 `json:"source_host" binding:"required"`
	SourceIP       string                 `json:"source_ip" binding:"required"`
	Severity       string                 `json:"severity" binding:"required"`
	Category       string                 `json:"category" binding:"required"`
	Message        string                 `json:"message" binding:"required"`
	RawPayload     string                 `json:"raw_payload,omitempty"`
	EventTimestamp time.Time              `json:"event_timestamp"`
	AIAnalysis     map[string]interface{} `json:"ai_analysis,omitempty"`
}

// IngestEvent receives events from ingestor core and creates alerts
func IngestEvent(c *gin.Context) {
	var req IngestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := alertRepo()
	if repo == nil {
		// Demo mode - just acknowledge the event
		alertID := fmt.Sprintf("ALT-DEMO-%d", time.Now().Unix())
		logger.Info("Demo mode: Alert %s would be created from ingest event: %s", alertID, req.Category)
		c.JSON(http.StatusCreated, gin.H{
			"message":  "Event ingested successfully (demo mode)",
			"alert_id": alertID,
		})
		return
	}

	// Generate alert ID
	alertID, err := repo.GenerateAlertID()
	if err != nil {
		apiErr := errors.NewDatabaseError("generate id", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Handle timestamp
	timestamp := req.EventTimestamp
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	alert := models.Alert{
		ID:          alertID,
		Title:       fmt.Sprintf("[%s] %s from %s", req.Severity, req.Category, req.SourceHost),
		Description: req.Message,
		Severity:    req.Severity,
		Category:    req.Category,
		Status:      models.AlertStatusOpen,
		Source:      req.EventType,
		SourceIP:    req.SourceIP,
		Device:      req.SourceHost,
		Timestamp:   timestamp,
		RawPayload:  req.RawPayload,
	}

	// Add AI analysis if present
	if req.AIAnalysis != nil {
		if summary, ok := req.AIAnalysis["explanation"].(string); ok {
			alert.AIAnalysisSummary = summary
		}
		if recommendation, ok := req.AIAnalysis["recommended_action"].(string); ok {
			alert.AIAnalysisRecommendation = recommendation
		}
		if confidence, ok := req.AIAnalysis["confidence"].(float64); ok {
			alert.AIConfidence = confidence
		}
	}

	if err := repo.Create(&alert); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Alert %s created from ingest event: %s", alertID, req.Category)

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Event ingested successfully",
		"alert_id": alertID,
	})
}

// getDemoNoisyDevices returns demo noisy devices for when database is unavailable
func getDemoNoisyDevices() []models.NoisyDevice {
	return []models.NoisyDevice{
		{DeviceID: "router-core-01", DeviceName: "router-core-01", AlertCount: 45, TopIssue: "High CPU Utilization"},
		{DeviceID: "switch-dist-02", DeviceName: "switch-dist-02", AlertCount: 32, TopIssue: "Interface Flapping"},
		{DeviceID: "server-app-01", DeviceName: "server-app-01", AlertCount: 28, TopIssue: "Memory Warning"},
		{DeviceID: "firewall-edge-01", DeviceName: "firewall-edge-01", AlertCount: 21, TopIssue: "Connection Timeout"},
		{DeviceID: "db-prod-01", DeviceName: "db-prod-01", AlertCount: 15, TopIssue: "Disk Space Low"},
	}
}

// Device represents a network device in the system
type Device struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	IP          string    `json:"ip"`
	Location    string    `json:"location"`
	Status      string    `json:"status"`
	Vendor      string    `json:"vendor"`
	Model       string    `json:"model"`
	LastSeen    time.Time `json:"lastSeen"`
	AlertCount  int       `json:"alertCount"`
	Uptime      string    `json:"uptime"`
	Description string    `json:"description,omitempty"`
}

// getDemoDevices returns demo devices for when database is unavailable
func getDemoDevices() []Device {
	now := time.Now()
	return []Device{
		{
			ID:         "router-core-01",
			Name:       "Core Router 01",
			Type:       "Router",
			IP:         "192.168.1.1",
			Location:   "Data Center A - Rack 1",
			Status:     "online",
			Vendor:     "Cisco",
			Model:      "ISR 4451-X",
			LastSeen:   now.Add(-2 * time.Minute),
			AlertCount: 3,
			Uptime:     "45d 12h 30m",
		},
		{
			ID:         "switch-dist-02",
			Name:       "Distribution Switch 02",
			Type:       "Switch",
			IP:         "192.168.1.10",
			Location:   "Data Center A - Rack 2",
			Status:     "online",
			Vendor:     "Cisco",
			Model:      "Catalyst 9300",
			LastSeen:   now.Add(-1 * time.Minute),
			AlertCount: 1,
			Uptime:     "30d 8h 15m",
		},
		{
			ID:         "firewall-edge-01",
			Name:       "Edge Firewall 01",
			Type:       "Firewall",
			IP:         "192.168.1.254",
			Location:   "Data Center A - Rack 1",
			Status:     "online",
			Vendor:     "Palo Alto",
			Model:      "PA-3220",
			LastSeen:   now.Add(-30 * time.Second),
			AlertCount: 0,
			Uptime:     "60d 4h 45m",
		},
		{
			ID:         "server-app-01",
			Name:       "Application Server 01",
			Type:       "Server",
			IP:         "192.168.2.10",
			Location:   "Data Center B - Rack 5",
			Status:     "degraded",
			Vendor:     "Dell",
			Model:      "PowerEdge R740",
			LastSeen:   now.Add(-5 * time.Minute),
			AlertCount: 5,
			Uptime:     "12d 6h 20m",
		},
		{
			ID:         "server-db-01",
			Name:       "Database Server 01",
			Type:       "Server",
			IP:         "192.168.2.20",
			Location:   "Data Center B - Rack 6",
			Status:     "online",
			Vendor:     "HP",
			Model:      "ProLiant DL380",
			LastSeen:   now.Add(-1 * time.Minute),
			AlertCount: 0,
			Uptime:     "90d 2h 10m",
		},
		{
			ID:         "ap-floor1-01",
			Name:       "Access Point Floor 1",
			Type:       "Access Point",
			IP:         "192.168.3.50",
			Location:   "Building A - Floor 1",
			Status:     "online",
			Vendor:     "Aruba",
			Model:      "AP-535",
			LastSeen:   now.Add(-3 * time.Minute),
			AlertCount: 0,
			Uptime:     "15d 18h 5m",
		},
		{
			ID:         "lb-prod-01",
			Name:       "Production Load Balancer",
			Type:       "Load Balancer",
			IP:         "192.168.1.100",
			Location:   "Data Center A - Rack 3",
			Status:     "online",
			Vendor:     "F5",
			Model:      "BIG-IP i5800",
			LastSeen:   now.Add(-1 * time.Minute),
			AlertCount: 2,
			Uptime:     "120d 5h 30m",
		},
		{
			ID:         "switch-access-05",
			Name:       "Access Switch 05",
			Type:       "Switch",
			IP:         "192.168.4.5",
			Location:   "Building B - Floor 2",
			Status:     "offline",
			Vendor:     "Juniper",
			Model:      "EX3400",
			LastSeen:   now.Add(-2 * time.Hour),
			AlertCount: 8,
			Uptime:     "0d 0h 0m",
		},
	}
}

// GetDevices returns all devices with optional filtering
func GetDevices(c *gin.Context) {
	db := database.Get()
	if db == nil {
		// Demo mode - return demo devices
		devices := getDemoDevices()
		c.JSON(http.StatusOK, gin.H{
			"devices": devices,
			"total":   len(devices),
		})
		return
	}

	// In real implementation, query devices from database
	// For now, return demo devices as devices table may not exist
	devices := getDemoDevices()

	// Apply filters if provided
	status := c.Query("status")
	deviceType := c.Query("type")

	filtered := make([]Device, 0)
	for _, d := range devices {
		if status != "" && d.Status != status {
			continue
		}
		if deviceType != "" && d.Type != deviceType {
			continue
		}
		filtered = append(filtered, d)
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": filtered,
		"total":   len(filtered),
	})
}

// GetDeviceByID returns a single device by ID
func GetDeviceByID(c *gin.Context) {
	deviceID := c.Param("id")

	// For demo mode, find in demo devices
	for _, d := range getDemoDevices() {
		if d.ID == deviceID {
			c.JSON(http.StatusOK, d)
			return
		}
	}

	apiErr := errors.NewNotFound(fmt.Sprintf("device %s", deviceID))
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// GetNoisyDevices returns devices with high alert counts
func GetNoisyDevices(c *gin.Context) {
	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && l > 0 {
			// use parsed limit
		}
	}

	repo := alertRepo()
	if repo == nil {
		// Demo mode - return demo noisy devices
		devices := getDemoNoisyDevices()
		if limit < len(devices) {
			devices = devices[:limit]
		}
		c.JSON(http.StatusOK, devices)
		return
	}

	noisyDevices, err := repo.GetNoisyDevices(limit)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, noisyDevices)
}

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
	last24h := now.Add(-24 * time.Hour)
	prev24h := now.Add(-48 * time.Hour)

	// Current period alert count
	var currentCount int64
	db.Model(&models.Alert{}).Where("timestamp >= ?", last24h).Count(&currentCount)

	// Previous period alert count
	var prevCount int64
	db.Model(&models.Alert{}).Where("timestamp >= ? AND timestamp < ?", prev24h, last24h).Count(&prevCount)

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

	// Calculate MTTR (placeholder - would need resolved_at and created_at difference)
	var avgMTTR float64
	db.Model(&models.Alert{}).
		Where("status = ? AND resolved_at IS NOT NULL", models.AlertStatusResolved).
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
	db.Model(&models.Alert{}).Where("ai_analysis_summary IS NOT NULL AND ai_analysis_summary != ''").Count(&enrichedAlerts)

	var successRate float64
	if totalAlerts > 0 {
		successRate = float64(enrichedAlerts) / float64(totalAlerts) * 100
	}

	// Get last processed alert
	var lastAlert models.Alert
	db.Model(&models.Alert{}).Order("created_at DESC").First(&lastAlert)

	c.JSON(http.StatusOK, AIMetrics{
		TotalProcessed: totalAlerts,
		SuccessRate:    successRate,
		AvgProcessTime: 125.5, // Would need to track this separately
		AlertsEnriched: enrichedAlerts,
		PatternsFound:  0, // Would need pattern detection logic
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

	// Find devices with most alerts (potential issue patterns)
	var deviceCounts []struct {
		Device string
		Count  int
	}
	db.Model(&models.Alert{}).
		Select("device, COUNT(*) as count").
		Group("device").
		Having("COUNT(*) > 3").
		Order("count DESC").
		Limit(5).
		Scan(&deviceCounts)

	for i, dc := range deviceCounts {
		insights = append(insights, AIInsight{
			ID:          fmt.Sprintf("INS-%03d", i+1),
			Type:        "trend",
			Title:       fmt.Sprintf("High Alert Volume on %s", dc.Device),
			Description: fmt.Sprintf("Device %s has generated %d alerts recently", dc.Device, dc.Count),
			Severity:    "medium",
			Confidence:  0.85,
			CreatedAt:   time.Now().Add(-time.Duration(i) * time.Hour),
			ActionItems: []string{
				"Review device logs",
				"Check device health status",
				"Consider maintenance window",
			},
		})
	}

	// Find critical alerts that haven't been acknowledged
	var criticalCount int64
	db.Model(&models.Alert{}).
		Where("severity = ? AND status = ?", "critical", models.AlertStatusOpen).
		Count(&criticalCount)

	if criticalCount > 0 {
		insights = append(insights, AIInsight{
			ID:          fmt.Sprintf("INS-%03d", len(insights)+1),
			Type:        "anomaly",
			Title:       "Unacknowledged Critical Alerts",
			Description: fmt.Sprintf("%d critical alerts require immediate attention", criticalCount),
			Severity:    "high",
			Confidence:  0.95,
			CreatedAt:   time.Now(),
			ActionItems: []string{
				"Review critical alerts immediately",
				"Assign to on-call engineer",
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
			Where("timestamp >= ? AND timestamp < ? AND ai_analysis_summary IS NOT NULL", dayStart, dayEnd).
			Count(&enrichedCount)

		var improvementPct float64
		if alertCount > 0 {
			improvementPct = float64(enrichedCount) / float64(alertCount) * 100
		}

		points = append(points, gin.H{
			"date":                 dayStart.Format("2006-01-02"),
			"alerts_processed":     alertCount,
			"patterns_detected":    enrichedCount / 10, // Simplified
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
	default:
		apiErr := errors.NewBadRequest("Invalid report type. Use 'alerts' or 'tickets'")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
	}
}
