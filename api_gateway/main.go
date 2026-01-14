package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ==========================================
// Configuration from Environment
// ==========================================

type Config struct {
	Port               string
	GinMode            string
	CORSAllowedOrigins []string
	JWTSecret          string
	JWTExpiryHours     int
	RateLimitEnabled   bool
	RateLimitRPM       int
}

func loadConfig() Config {
	port := os.Getenv("API_GATEWAY_PORT")
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = "8080"
	}

	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}

	corsOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173,http://localhost:5174,http://localhost:3000"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// Generate a random secret for development (NOT for production!)
		log.Println("⚠️  WARNING: JWT_SECRET not set, using random secret. Set JWT_SECRET in production!")
		bytes := make([]byte, 32)
		rand.Read(bytes)
		jwtSecret = hex.EncodeToString(bytes)
	}

	return Config{
		Port:               port,
		GinMode:            ginMode,
		CORSAllowedOrigins: strings.Split(corsOrigins, ","),
		JWTSecret:          jwtSecret,
		JWTExpiryHours:     24,
		RateLimitEnabled:   os.Getenv("RATE_LIMIT_ENABLED") == "true",
		RateLimitRPM:       100,
	}
}

var config Config

// ==========================================
// Data Models
// ==========================================

type DeviceInfo struct {
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Icon   string `json:"icon"`
	Model  string `json:"model,omitempty"`
	Vendor string `json:"vendor,omitempty"`
}

type TimestampInfo struct {
	Absolute string `json:"absolute"`
	Relative string `json:"relative,omitempty"`
}

type Alert struct {
	ID         string        `json:"id"`
	Severity   string        `json:"severity"`
	Status     string        `json:"status"`
	Timestamp  TimestampInfo `json:"timestamp"`
	Device     DeviceInfo    `json:"device"`
	AITitle    string        `json:"aiTitle"`
	AISummary  string        `json:"aiSummary"`
	Confidence int           `json:"confidence"`
}

type ExtendedDeviceInfo struct {
	Name           string `json:"name"`
	IP             string `json:"ip"`
	Location       string `json:"location"`
	Vendor         string `json:"vendor"`
	Model          string `json:"model"`
	Interface      string `json:"interface"`
	InterfaceAlias string `json:"interfaceAlias"`
}

type AlertDetail struct {
	Alert
	SimilarEvents  int                `json:"similarEvents"`
	AIAnalysis     AIAnalysis         `json:"aiAnalysis"`
	RawData        string             `json:"rawData"`
	History        []HistoryItem      `json:"history"`
	ExtendedDevice ExtendedDeviceInfo `json:"extendedDevice"`
}

type AIAnalysis struct {
	Summary            string   `json:"summary"`
	RootCauses         []string `json:"rootCauses"`
	BusinessImpact     string   `json:"businessImpact"`
	RecommendedActions []string `json:"recommendedActions"`
}

type HistoryItem struct {
	ID         string `json:"id"`
	Timestamp  string `json:"timestamp"`
	Title      string `json:"title"`
	Resolution string `json:"resolution"`
	Severity   string `json:"severity"`
}

type SeverityCount struct {
	Group string `json:"group"`
	Value int    `json:"value"`
}

type NoisyDevice struct {
	Device     DeviceInfo `json:"device"`
	Model      string     `json:"model"`
	AlertCount int        `json:"alertCount"`
	Severity   string     `json:"severity"`
}

type AIMetric struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Change string  `json:"change"`
	Trend  string  `json:"trend"`
}

type AlertSummary struct {
	ActiveCount   int `json:"activeCount"`
	CriticalCount int `json:"criticalCount"`
	MajorCount    int `json:"majorCount"`
	MinorCount    int `json:"minorCount"`
	InfoCount     int `json:"infoCount"`
}

type TrendKPI struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
	Trend string `json:"trend"`
	Tag   *Tag   `json:"tag,omitempty"`
}

type Tag struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type RecurringAlert struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Count         int    `json:"count"`
	Severity      string `json:"severity"`
	AvgResolution string `json:"avgResolution"`
	Percentage    int    `json:"percentage"`
}

type AlertDistribution struct {
	Group string `json:"group"`
	Value int    `json:"value"`
}

type AIInsight struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

// Auth models
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
}

type Role struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     Role   `json:"role"`
}

type RegisterRequest struct {
	FirstName string `json:"firstName" binding:"required"`
	LastName  string `json:"lastName" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=6"`
	Role      Role   `json:"role"`
}

type JWTClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	jwt.RegisteredClaims
}

// ==========================================
// In-Memory Storage (Replace with DB later)
// ==========================================

var alertsStore = []Alert{
	{
		ID:       "alert-001",
		Severity: "critical",
		Status:   "new",
		Timestamp: TimestampInfo{
			Absolute: time.Now().Format("2006-01-02 15:04:05"),
			Relative: "2m ago",
		},
		Device: DeviceInfo{
			Name:   "Core-SW-01",
			IP:     "192.168.1.10",
			Icon:   "switch",
			Model:  "Cisco Catalyst 9300",
			Vendor: "Cisco Systems",
		},
		AITitle:    "Interface GigabitEthernet0/1 Down",
		AISummary:  "Network interface has transitioned to down state. Link failure detected.",
		Confidence: 94,
	},
	{
		ID:       "alert-002",
		Severity: "major",
		Status:   "acknowledged",
		Timestamp: TimestampInfo{
			Absolute: time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05"),
			Relative: "5m ago",
		},
		Device: DeviceInfo{
			Name:   "FW-DMZ-03",
			IP:     "172.16.3.1",
			Icon:   "firewall",
			Model:  "Palo Alto PA-5220",
			Vendor: "Palo Alto Networks",
		},
		AITitle:    "High CPU Utilization Detected (85%)",
		AISummary:  "Firewall processing load exceeding normal thresholds.",
		Confidence: 88,
	},
}

// Simple user store (replace with DB)
var usersStore = map[string]User{
	"admin": {ID: "1", Username: "admin", Email: "admin@example.com", Role: Role{ID: "admin", Text: "Administrator"}},
}

// Ticket model
type Ticket struct {
	ID          string `json:"id"`
	TicketNumber string `json:"ticketNumber"`
	AlertID     string `json:"alertId,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	DeviceName  string `json:"deviceName"`
	AssignedTo  string `json:"assignedTo"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedBy   string `json:"createdBy"`
}

// Tickets store
var ticketsStore = []Ticket{
	{
		ID:           "ticket-001",
		TicketNumber: "TKT-20260114-001",
		AlertID:      "alert-001",
		Title:        "Interface Down on Core-SW-01",
		Description:  "GigabitEthernet0/1 interface is down. Needs immediate investigation.",
		Priority:     "critical",
		Status:       "open",
		DeviceName:   "Core-SW-01",
		AssignedTo:   "Network Team",
		CreatedAt:    "2026-01-14 10:30:00",
		UpdatedAt:    "2026-01-14 10:30:00",
		CreatedBy:    "admin",
	},
	{
		ID:           "ticket-002",
		TicketNumber: "TKT-20260114-002",
		AlertID:      "alert-002",
		Title:        "High CPU on Firewall FW-DMZ-03",
		Description:  "CPU utilization at 85%. Performance degradation possible.",
		Priority:     "high",
		Status:       "in-progress",
		DeviceName:   "FW-DMZ-03",
		AssignedTo:   "Security Team",
		CreatedAt:    "2026-01-14 09:15:00",
		UpdatedAt:    "2026-01-14 11:00:00",
		CreatedBy:    "admin",
	},
}

var ticketCounter = 3

// ==========================================
// JWT Helpers
// ==========================================

func generateToken(user User) (string, error) {
	claims := JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(config.JWTExpiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "noc-dashboard",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.JWTSecret))
}

func validateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// ==========================================
// Middleware
// ==========================================

// AuthMiddleware validates JWT tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := validateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("userId", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// RequestLogger logs incoming requests
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Printf("[%s] %s %s %d %v",
			c.Request.Method,
			path,
			c.ClientIP(),
			status,
			latency,
		)
	}
}

// ==========================================
// Auth Handlers
// ==========================================

func login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In production, validate against database with hashed passwords
	// For demo, accept any non-empty credentials
	if req.Username == "" || req.Password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Create or get user
	user := User{
		ID:       fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Username: req.Username,
		Email:    req.Username + "@example.com",
		Role:     req.Role,
	}

	token, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In production, hash password and store in database
	user := User{
		ID:       fmt.Sprintf("user-%d", time.Now().UnixNano()),
		Username: req.Email,
		Email:    req.Email,
		Role:     req.Role,
	}

	token, err := generateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"token":   token,
	})
}

// ==========================================
// Alert Handlers
// ==========================================

func getAlerts(c *gin.Context) {
	c.JSON(http.StatusOK, alertsStore)
}

func getAlertByID(c *gin.Context) {
	id := c.Param("id")
	for _, alert := range alertsStore {
		if alert.ID == id {
			detail := AlertDetail{
				Alert:         alert,
				SimilarEvents: 7,
				AIAnalysis: AIAnalysis{
					Summary:      "The network interface has transitioned to a down state while the administrative status remains up.",
					RootCauses:   []string{"Physical layer failure detected", "Possible cable fault or SFP failure", "Remote device may be powered off"},
					BusinessImpact: "High - Loss of redundancy to distribution layer.",
					RecommendedActions: []string{
						"Verify physical cable connection",
						"Check remote device status",
						"Review interface error counters",
					},
				},
				RawData: `SNMP-v2-MIB::sysUpTime.0 = Timeticks: (123456789)
IF-MIB::ifOperStatus.24 = INTEGER: down(2)
IF-MIB::ifAdminStatus.24 = INTEGER: up(1)`,
				History: []HistoryItem{
					{ID: "hist-001", Timestamp: "2024-03-13 09:13:33", Title: "Interface Down", Resolution: "Cable reseated", Severity: "critical"},
				},
				ExtendedDevice: ExtendedDeviceInfo{
					Name:           alert.Device.Name,
					IP:             alert.Device.IP,
					Location:       "Data Center 1, Rack A12",
					Vendor:         alert.Device.Vendor,
					Model:          alert.Device.Model,
					Interface:      "GigabitEthernet0/1",
					InterfaceAlias: "Uplink to Distribution",
				},
			}
			c.JSON(http.StatusOK, detail)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
}

func getAlertsSummary(c *gin.Context) {
	summary := AlertSummary{
		ActiveCount:   len(alertsStore),
		CriticalCount: 1,
		MajorCount:    1,
		MinorCount:    0,
		InfoCount:     0,
	}
	c.JSON(http.StatusOK, summary)
}

func getSeverityDistribution(c *gin.Context) {
	counts := make(map[string]int)
	for _, alert := range alertsStore {
		sev := strings.Title(alert.Severity)
		counts[sev]++
	}

	distribution := []SeverityCount{}
	for _, sev := range []string{"Critical", "Major", "Minor", "Info"} {
		distribution = append(distribution, SeverityCount{Group: sev, Value: counts[sev]})
	}
	c.JSON(http.StatusOK, distribution)
}

func getAlertsOverTime(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{"group": "Critical", "date": "2024-01-01T00:00:00Z", "value": 5},
		{"group": "Critical", "date": "2024-01-01T04:00:00Z", "value": 8},
		{"group": "Major", "date": "2024-01-01T00:00:00Z", "value": 10},
		{"group": "Major", "date": "2024-01-01T04:00:00Z", "value": 15},
	})
}

func getNoisyDevices(c *gin.Context) {
	deviceCounts := make(map[string]int)
	deviceInfo := make(map[string]DeviceInfo)
	deviceSeverity := make(map[string]string)

	for _, alert := range alertsStore {
		deviceCounts[alert.Device.Name]++
		deviceInfo[alert.Device.Name] = alert.Device
		if deviceSeverity[alert.Device.Name] == "" || alert.Severity == "critical" {
			deviceSeverity[alert.Device.Name] = alert.Severity
		}
	}

	var devices []NoisyDevice
	for name, count := range deviceCounts {
		info := deviceInfo[name]
		devices = append(devices, NoisyDevice{
			Device:     info,
			Model:      info.Model,
			AlertCount: count,
			Severity:   deviceSeverity[name],
		})
	}
	c.JSON(http.StatusOK, devices)
}

func getAIMetrics(c *gin.Context) {
	metrics := []AIMetric{
		{Name: "Resolution Time", Value: 50, Change: "-50%", Trend: "positive"},
		{Name: "Escalations", Value: 47, Change: "-47%", Trend: "positive"},
		{Name: "Accuracy", Value: 94.8, Change: "94.8%", Trend: "positive"},
	}
	c.JSON(http.StatusOK, metrics)
}

func getTrendsKPI(c *gin.Context) {
	kpis := []TrendKPI{
		{ID: "alert-volume", Label: "Alert Volume", Value: fmt.Sprintf("%d", len(alertsStore)), Trend: "stable"},
		{ID: "mttr", Label: "MTTR", Value: "5m", Trend: "up"},
		{ID: "recurring-alerts", Label: "Recurring Alerts", Value: "15%", Trend: "stable"},
		{ID: "escalation-rate", Label: "Escalation Rate", Value: "0%", Trend: "stable", Tag: &Tag{Text: "Low", Type: "green"}},
	}
	c.JSON(http.StatusOK, kpis)
}

func getRecurringAlerts(c *gin.Context) {
	alertCounts := make(map[string]int)
	alertSeverity := make(map[string]string)

	for _, alert := range alertsStore {
		alertCounts[alert.AITitle]++
		if alertSeverity[alert.AITitle] == "" {
			alertSeverity[alert.AITitle] = alert.Severity
		}
	}

	total := len(alertsStore)
	var alerts []RecurringAlert
	i := 1
	for title, count := range alertCounts {
		pct := 0
		if total > 0 {
			pct = count * 100 / total
		}
		alerts = append(alerts, RecurringAlert{
			ID:            fmt.Sprintf("rec-%d", i),
			Name:          title,
			Count:         count,
			Severity:      alertSeverity[title],
			AvgResolution: "5m",
			Percentage:    pct,
		})
		i++
	}
	c.JSON(http.StatusOK, alerts)
}

func getAlertDistributionTime(c *gin.Context) {
	distribution := []AlertDistribution{
		{Group: "Morning", Value: 1},
		{Group: "Afternoon", Value: 1},
		{Group: "Evening", Value: 0},
		{Group: "Night", Value: 0},
	}
	c.JSON(http.StatusOK, distribution)
}

func getAIInsights(c *gin.Context) {
	insights := []AIInsight{
		{ID: "ins-1", Type: "pattern", Description: "Recurring interface flapping detected on Core-SW-01.", Action: "Investigate Scheduled Tasks"},
		{ID: "ins-2", Type: "optimization", Description: "Firewall rules processing efficiency dropping.", Action: "Optimize Rule Base"},
		{ID: "ins-3", Type: "recommendation", Description: "BGP session resets correlated with ISP-B link.", Action: "Review QoS Policy"},
	}
	c.JSON(http.StatusOK, insights)
}

func getAIImpactOverTime(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{
		{"group": "Accuracy", "date": "2024-01-01T00:00:00Z", "value": 92},
		{"group": "Accuracy", "date": "2024-01-01T04:00:00Z", "value": 94},
		{"group": "Automated Resolution", "date": "2024-01-01T00:00:00Z", "value": 45},
		{"group": "Automated Resolution", "date": "2024-01-01T04:00:00Z", "value": 52},
	})
}

func getHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   os.Getenv("APP_VERSION"),
	})
}

// ==========================================
// Action Handlers
// ==========================================

func acknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	for i, alert := range alertsStore {
		if alert.ID == id {
			alertsStore[i].Status = "acknowledged"
			log.Printf("✓ Alert %s acknowledged by %s", id, c.GetString("username"))
			c.JSON(http.StatusOK, gin.H{"message": "Alert acknowledged", "status": "acknowledged"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
}

func dismissAlert(c *gin.Context) {
	id := c.Param("id")
	for i, alert := range alertsStore {
		if alert.ID == id {
			alertsStore[i].Status = "dismissed"
			log.Printf("✓ Alert %s dismissed by %s", id, c.GetString("username"))
			c.JSON(http.StatusOK, gin.H{"message": "Alert dismissed", "status": "dismissed"})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Alert not found"})
}

func createTicket(c *gin.Context) {
	var req struct {
		AlertID     string `json:"alertId"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		DeviceName  string `json:"deviceName"`
		Assignee    string `json:"assignee"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := c.GetString("username")
	now := time.Now()
	ticketNumber := fmt.Sprintf("TKT-%s-%03d", now.Format("20060102"), ticketCounter)
	ticketCounter++

	newTicket := Ticket{
		ID:           fmt.Sprintf("ticket-%d", now.UnixNano()),
		TicketNumber: ticketNumber,
		AlertID:      req.AlertID,
		Title:        req.Title,
		Description:  req.Description,
		Priority:     req.Priority,
		Status:       "open",
		DeviceName:   req.DeviceName,
		AssignedTo:   req.Assignee,
		CreatedAt:    now.Format("2006-01-02 15:04:05"),
		UpdatedAt:    now.Format("2006-01-02 15:04:05"),
		CreatedBy:    username,
	}

	ticketsStore = append(ticketsStore, newTicket)
	log.Printf("🎟️ Ticket %s created by %s", ticketNumber, username)
	c.JSON(http.StatusCreated, newTicket)
}

func getTickets(c *gin.Context) {
	c.JSON(http.StatusOK, ticketsStore)
}

func getTicketByID(c *gin.Context) {
	id := c.Param("id")
	for _, ticket := range ticketsStore {
		if ticket.ID == id {
			c.JSON(http.StatusOK, ticket)
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
}

func updateTicket(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    string `json:"priority"`
		Status      string `json:"status"`
		AssignedTo  string `json:"assignedTo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for i, ticket := range ticketsStore {
		if ticket.ID == id {
			if req.Title != "" {
				ticketsStore[i].Title = req.Title
			}
			if req.Description != "" {
				ticketsStore[i].Description = req.Description
			}
			if req.Priority != "" {
				ticketsStore[i].Priority = req.Priority
			}
			if req.Status != "" {
				ticketsStore[i].Status = req.Status
			}
			if req.AssignedTo != "" {
				ticketsStore[i].AssignedTo = req.AssignedTo
			}
			ticketsStore[i].UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			
			log.Printf("🎟️ Ticket %s updated by %s", ticket.TicketNumber, c.GetString("username"))
			c.JSON(http.StatusOK, ticketsStore[i])
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
}

func exportReport(c *gin.Context) {
	format := c.DefaultQuery("format", "csv")
	log.Printf("📊 Report exported in %s format by %s", format, c.GetString("username"))
	c.JSON(http.StatusOK, gin.H{"url": "/reports/download/" + format, "message": "Report generated"})
}

// Ingest endpoint
func ingestEvent(c *gin.Context) {
	var event struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newAlert := Alert{
		ID:       "alert-" + time.Now().Format("20060102150405"),
		Severity: mapEventTypeToSeverity(event.Type),
		Status:   "new",
		Timestamp: TimestampInfo{
			Absolute: time.Now().Format("2006-01-02 15:04:05"),
			Relative: "just now",
		},
		Device: DeviceInfo{
			Name: "Unknown Device",
			IP:   "0.0.0.0",
			Icon: "server",
		},
		AITitle:    event.Message,
		AISummary:  "Event received: " + event.Message,
		Confidence: 85,
	}

	alertsStore = append([]Alert{newAlert}, alertsStore...)
	log.Printf("📨 Ingested event: type=%s", event.Type)
	c.JSON(http.StatusOK, gin.H{"status": "ingested", "alert_id": newAlert.ID})
}

func mapEventTypeToSeverity(eventType string) string {
	switch eventType {
	case "critical":
		return "critical"
	case "warning":
		return "major"
	default:
		return "info"
	}
}

// ==========================================
// Main
// ==========================================

func main() {
	config = loadConfig()

	gin.SetMode(config.GinMode)
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(RequestLogger())
	router.Use(SecurityHeaders())

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     config.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public routes (no auth required)
		v1.POST("/login", login)
		v1.POST("/register", register)
		v1.GET("/health", getHealth)

		// Protected routes (auth required)
		protected := v1.Group("")
		protected.Use(AuthMiddleware())
		{
			// Alerts
			protected.GET("/alerts", getAlerts)
			protected.GET("/alerts/:id", getAlertByID)
			protected.GET("/alerts/summary", getAlertsSummary)
			protected.GET("/alerts/severity-distribution", getSeverityDistribution)
			protected.GET("/alerts/over-time", getAlertsOverTime)
			protected.GET("/alerts/recurring", getRecurringAlerts)
			protected.GET("/alerts/distribution/time", getAlertDistributionTime)

			// Actions
			protected.POST("/alerts/:id/acknowledge", acknowledgeAlert)
			protected.POST("/alerts/:id/dismiss", dismissAlert)
			protected.GET("/reports/export", exportReport)

			// Tickets
			protected.GET("/tickets", getTickets)
			protected.GET("/tickets/:id", getTicketByID)
			protected.POST("/tickets", createTicket)
			protected.PUT("/tickets/:id", updateTicket)

			// Trends
			protected.GET("/trends/kpi", getTrendsKPI)

			// Devices
			protected.GET("/devices/noisy", getNoisyDevices)

			// AI
			protected.GET("/ai/metrics", getAIMetrics)
			protected.GET("/ai/insights", getAIInsights)
			protected.GET("/ai/impact-over-time", getAIImpactOverTime)

			// Ingest (internal service)
			protected.POST("/events", ingestEvent)
		}
	}

	log.Printf("🚀 API Gateway starting on :%s", config.Port)
	log.Printf("📋 CORS allowed origins: %v", config.CORSAllowedOrigins)
	log.Printf("🔐 JWT expiry: %d hours", config.JWTExpiryHours)

	if err := router.Run(":" + config.Port); err != nil {
		log.Fatal("Failed to start API Gateway:", err)
	}
}
