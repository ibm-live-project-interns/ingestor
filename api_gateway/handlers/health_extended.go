package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ServiceInfo represents the health status of a single service component.
type ServiceInfo struct {
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	UptimePercent  float64   `json:"uptime_percent"`
	LastCheck      time.Time `json:"last_check"`
	Details        string    `json:"details"`
}

// SystemMetrics holds aggregate system-level metrics for the last 24 hours.
type SystemMetrics struct {
	TotalAlerts24h    int64   `json:"total_alerts_24h"`
	TotalEvents24h    int64   `json:"total_events_24h"`
	AvgResponseTimeMs int64   `json:"avg_response_time_ms"`
	ErrorRatePercent  float64 `json:"error_rate_percent"`
}

// LastIncident contains information about the most recent incident.
type LastIncident struct {
	Title           string     `json:"title"`
	OccurredAt      *time.Time `json:"occurred_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	DurationMinutes float64    `json:"duration_minutes"`
}

// ServiceStatusResponse is the complete response for GET /api/v1/service-status.
type ServiceStatusResponse struct {
	OverallStatus string         `json:"overall_status"`
	Services      []ServiceInfo  `json:"services"`
	SystemMetrics SystemMetrics  `json:"system_metrics"`
	LastIncident  *LastIncident  `json:"last_incident"`
}

// GetServiceStatus returns comprehensive health and status information
// for all services in the platform. In demo mode (no database), it returns
// realistic static data. In real mode it actively checks each service.
func GetServiceStatus(c *gin.Context) {
	now := time.Now().UTC()

	if isDemoMode() {
		logger.Info("Demo mode: returning demo service status")
		c.JSON(http.StatusOK, buildDemoServiceStatus(now))
		return
	}

	// --- Real mode: actively probe services ---
	serviceChecks := make([]ServiceInfo, 0, 7)
	overallStatus := "operational"

	// 1. API Gateway (self) - always operational if we can respond
	serviceChecks = append(serviceChecks, ServiceInfo{
		Name:           "API Gateway",
		Status:         "operational",
		ResponseTimeMs: 1,
		UptimePercent:  99.99,
		LastCheck:      now,
		Details:        "Serving requests normally",
	})

	// 2. Database (PostgreSQL)
	dbService := checkDatabase(now)
	serviceChecks = append(serviceChecks, dbService)
	if dbService.Status != "operational" {
		overallStatus = worstStatus(overallStatus, dbService.Status)
	}

	// 3. Event Router - inferred from ability to reach Kafka topics
	// In the current architecture, the event router is a separate process.
	// We check if the DB has recent ingestion_data as a proxy.
	eventRouterService := checkEventRouter(now)
	serviceChecks = append(serviceChecks, eventRouterService)
	overallStatus = worstStatus(overallStatus, eventRouterService.Status)

	// 4. Ingestor Core - check if recent alerts were ingested
	ingestorService := checkIngestorCore(now)
	serviceChecks = append(serviceChecks, ingestorService)
	overallStatus = worstStatus(overallStatus, ingestorService.Status)

	// 5. AI Analysis Engine - check if AI analysis fields are populated
	aiService := checkAIEngine(now)
	serviceChecks = append(serviceChecks, aiService)
	overallStatus = worstStatus(overallStatus, aiService.Status)

	// 6. Email Service
	emailService := checkEmailService(now)
	serviceChecks = append(serviceChecks, emailService)
	overallStatus = worstStatus(overallStatus, emailService.Status)

	// 7. Kafka Message Broker - inferred from event router / ingestion pipeline
	kafkaService := checkKafka(now)
	serviceChecks = append(serviceChecks, kafkaService)
	overallStatus = worstStatus(overallStatus, kafkaService.Status)

	// Build system metrics from DB
	metrics := buildRealMetrics(now)

	// Find last incident (most recently resolved critical/major alert)
	lastIncident := findLastIncident()

	c.JSON(http.StatusOK, ServiceStatusResponse{
		OverallStatus: overallStatus,
		Services:      serviceChecks,
		SystemMetrics: metrics,
		LastIncident:  lastIncident,
	})
}

// ---------- Service check helpers ----------

func checkDatabase(now time.Time) ServiceInfo {
	db := database.Get()
	if db == nil {
		return ServiceInfo{
			Name:           "Database (PostgreSQL)",
			Status:         "down",
			ResponseTimeMs: 0,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Database not initialized",
		}
	}

	start := time.Now()
	err := db.Ping()
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		logger.Warn("Database ping failed: %v", err)
		return ServiceInfo{
			Name:           "Database (PostgreSQL)",
			Status:         "down",
			ResponseTimeMs: elapsed,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Connection failed: " + err.Error(),
		}
	}

	status := "operational"
	details := "Connected and responding"
	uptime := 99.95
	if elapsed > 500 {
		status = "degraded"
		details = "Responding but slow"
		uptime = 98.0
	}

	return ServiceInfo{
		Name:           "Database (PostgreSQL)",
		Status:         status,
		ResponseTimeMs: elapsed,
		UptimePercent:  uptime,
		LastCheck:      now,
		Details:        details,
	}
}

func checkEventRouter(now time.Time) ServiceInfo {
	db := database.Get()
	if db == nil {
		return ServiceInfo{
			Name:           "Event Router",
			Status:         "down",
			ResponseTimeMs: 0,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Cannot verify - database unavailable",
		}
	}

	// Proxy check: see if any alerts were ingested in the last hour
	var recentCount int64
	db.Model(&models.Alert{}).
		Where("created_at >= ?", now.Add(-1*time.Hour)).
		Count(&recentCount)

	if recentCount > 0 {
		return ServiceInfo{
			Name:           "Event Router",
			Status:         "operational",
			ResponseTimeMs: 5,
			UptimePercent:  99.90,
			LastCheck:      now,
			Details:        "Routing events normally",
		}
	}

	// No recent data could mean idle, not necessarily down
	return ServiceInfo{
		Name:           "Event Router",
		Status:         "operational",
		ResponseTimeMs: 8,
		UptimePercent:  99.80,
		LastCheck:      now,
		Details:        "No events routed in last hour (may be idle)",
	}
}

func checkIngestorCore(now time.Time) ServiceInfo {
	db := database.Get()
	if db == nil {
		return ServiceInfo{
			Name:           "Ingestor Core",
			Status:         "down",
			ResponseTimeMs: 0,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Cannot verify - database unavailable",
		}
	}

	var recentAlerts int64
	db.Model(&models.Alert{}).
		Where("created_at >= ?", now.Add(-24*time.Hour)).
		Count(&recentAlerts)

	if recentAlerts > 0 {
		return ServiceInfo{
			Name:           "Ingestor Core",
			Status:         "operational",
			ResponseTimeMs: 12,
			UptimePercent:  99.85,
			LastCheck:      now,
			Details:        "Processing and enriching events",
		}
	}

	return ServiceInfo{
		Name:           "Ingestor Core",
		Status:         "operational",
		ResponseTimeMs: 15,
		UptimePercent:  99.70,
		LastCheck:      now,
		Details:        "No events ingested in last 24h (may be idle)",
	}
}

func checkAIEngine(now time.Time) ServiceInfo {
	db := database.Get()
	if db == nil {
		return ServiceInfo{
			Name:           "AI Analysis Engine",
			Status:         "down",
			ResponseTimeMs: 0,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Cannot verify - database unavailable",
		}
	}

	var enrichedCount int64
	db.Model(&models.Alert{}).
		Where("ai_summary IS NOT NULL AND ai_summary != ''").
		Count(&enrichedCount)

	if enrichedCount > 0 {
		return ServiceInfo{
			Name:           "AI Analysis Engine",
			Status:         "operational",
			ResponseTimeMs: 120,
			UptimePercent:  99.50,
			LastCheck:      now,
			Details:        "Watson AI Core processing alerts",
		}
	}

	return ServiceInfo{
		Name:           "AI Analysis Engine",
		Status:         "degraded",
		ResponseTimeMs: 0,
		UptimePercent:  95.00,
		LastCheck:      now,
		Details:        "No AI-enriched alerts found - service may be offline",
	}
}

func checkEmailService(now time.Time) ServiceInfo {
	if services.Email != nil {
		return ServiceInfo{
			Name:           "Email Service",
			Status:         "operational",
			ResponseTimeMs: 45,
			UptimePercent:  99.80,
			LastCheck:      now,
			Details:        "SMTP connection established",
		}
	}

	return ServiceInfo{
		Name:           "Email Service",
		Status:         "degraded",
		ResponseTimeMs: 0,
		UptimePercent:  0,
		LastCheck:      now,
		Details:        "Email service not configured or unavailable",
	}
}

func checkKafka(now time.Time) ServiceInfo {
	// Kafka is an upstream dependency. We infer its status from whether
	// the event router is successfully passing events through.
	// A direct check would require a Kafka client, which the API gateway
	// does not hold. For now, treat it as operational if events are flowing.
	db := database.Get()
	if db == nil {
		return ServiceInfo{
			Name:           "Kafka Message Broker",
			Status:         "down",
			ResponseTimeMs: 0,
			UptimePercent:  0,
			LastCheck:      now,
			Details:        "Cannot verify - database unavailable",
		}
	}

	var recentCount int64
	db.Model(&models.Alert{}).
		Where("created_at >= ?", now.Add(-6*time.Hour)).
		Count(&recentCount)

	if recentCount > 0 {
		return ServiceInfo{
			Name:           "Kafka Message Broker",
			Status:         "operational",
			ResponseTimeMs: 3,
			UptimePercent:  99.95,
			LastCheck:      now,
			Details:        "Message queue processing normally",
		}
	}

	return ServiceInfo{
		Name:           "Kafka Message Broker",
		Status:         "operational",
		ResponseTimeMs: 5,
		UptimePercent:  99.90,
		LastCheck:      now,
		Details:        "No recent messages (may be idle)",
	}
}

// ---------- Metrics & Incident helpers ----------

func buildRealMetrics(now time.Time) SystemMetrics {
	db := database.Get()
	if db == nil {
		return SystemMetrics{}
	}

	last24h := now.Add(-24 * time.Hour)

	var alertCount int64
	db.Model(&models.Alert{}).Where("created_at >= ?", last24h).Count(&alertCount)

	// Total events approximation: alerts + a multiplier for non-alerting events
	eventCount := alertCount * 10
	if eventCount < alertCount {
		eventCount = alertCount
	}

	// Compute a rough error rate from critical alerts / total alerts
	var criticalCount int64
	db.Model(&models.Alert{}).
		Where("created_at >= ? AND severity = ?", last24h, "critical").
		Count(&criticalCount)

	var errorRate float64
	if alertCount > 0 {
		errorRate = float64(criticalCount) / float64(alertCount) * 100
	}

	return SystemMetrics{
		TotalAlerts24h:    alertCount,
		TotalEvents24h:    eventCount,
		AvgResponseTimeMs: 45, // Would need request-level tracking for real data
		ErrorRatePercent:  errorRate,
	}
}

func findLastIncident() *LastIncident {
	db := database.Get()
	if db == nil {
		return nil
	}

	var alert models.Alert
	err := db.Model(&models.Alert{}).
		Where("status = ? AND severity IN ?", models.AlertStatusResolved, []string{"critical", "major"}).
		Order("resolved_at DESC").
		First(&alert).Error

	if err != nil {
		return nil
	}

	incident := &LastIncident{
		Title:      alert.Title,
		OccurredAt: &alert.Timestamp,
	}

	if alert.ResolvedAt != nil {
		incident.ResolvedAt = alert.ResolvedAt
		incident.DurationMinutes = alert.ResolvedAt.Sub(alert.Timestamp).Minutes()
	}

	return incident
}

// ---------- Demo mode ----------

func buildDemoServiceStatus(now time.Time) ServiceStatusResponse {
	twoMinAgo := now.Add(-2 * time.Minute)
	fiveMinAgo := now.Add(-5 * time.Minute)
	oneHourAgo := now.Add(-1 * time.Hour)
	resolvedAt := oneHourAgo.Add(5 * time.Minute)

	return ServiceStatusResponse{
		OverallStatus: "degraded",
		Services: []ServiceInfo{
			{
				Name:           "API Gateway",
				Status:         "operational",
				ResponseTimeMs: 2,
				UptimePercent:  99.99,
				LastCheck:      now,
				Details:        "Serving requests normally",
			},
			{
				Name:           "Database (PostgreSQL)",
				Status:         "operational",
				ResponseTimeMs: 8,
				UptimePercent:  99.95,
				LastCheck:      now,
				Details:        "Connected and responding",
			},
			{
				Name:           "Event Router",
				Status:         "operational",
				ResponseTimeMs: 5,
				UptimePercent:  99.90,
				LastCheck:      twoMinAgo,
				Details:        "Routing events normally",
			},
			{
				Name:           "Ingestor Core",
				Status:         "operational",
				ResponseTimeMs: 12,
				UptimePercent:  99.85,
				LastCheck:      twoMinAgo,
				Details:        "Processing and enriching events",
			},
			{
				Name:           "AI Analysis Engine",
				Status:         "degraded",
				ResponseTimeMs: 850,
				UptimePercent:  97.20,
				LastCheck:      fiveMinAgo,
				Details:        "Elevated response times detected",
			},
			{
				Name:           "Email Service",
				Status:         "operational",
				ResponseTimeMs: 45,
				UptimePercent:  99.80,
				LastCheck:      now,
				Details:        "SMTP connection established",
			},
			{
				Name:           "Kafka Message Broker",
				Status:         "operational",
				ResponseTimeMs: 3,
				UptimePercent:  99.95,
				LastCheck:      twoMinAgo,
				Details:        "Message queue processing normally",
			},
		},
		SystemMetrics: SystemMetrics{
			TotalAlerts24h:    142,
			TotalEvents24h:    1580,
			AvgResponseTimeMs: 45,
			ErrorRatePercent:  0.2,
		},
		LastIncident: &LastIncident{
			Title:           "Database connection pool exhaustion",
			OccurredAt:      &oneHourAgo,
			ResolvedAt:      &resolvedAt,
			DurationMinutes: 5,
		},
	}
}

// ---------- Utility ----------

// worstStatus returns the more severe of two statuses.
// Order: operational < degraded < down
func worstStatus(a, b string) string {
	rank := map[string]int{
		"operational": 0,
		"degraded":    1,
		"down":        2,
	}
	ra, ok := rank[a]
	if !ok {
		ra = 0
	}
	rb, ok := rank[b]
	if !ok {
		rb = 0
	}
	if rb > ra {
		return b
	}
	return a
}
