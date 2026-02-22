package handlers

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Device represents a network device in the system (used for demo mode).
// GORM tags map to the PostgreSQL devices table columns from init.sql.
// JSON tags use snake_case to match what the frontend expects.
type Device struct {
	ID          string    `json:"id" gorm:"column:id;primaryKey"`
	Name        string    `json:"name" gorm:"column:name"`
	Type        string    `json:"type" gorm:"column:icon"`
	IP          string    `json:"ip" gorm:"column:ip"`
	Location    string    `json:"location" gorm:"column:location"`
	Status      string    `json:"status" gorm:"column:status"`
	Vendor      string    `json:"vendor" gorm:"column:vendor"`
	Model       string    `json:"model" gorm:"column:model"`
	LastSeen    time.Time `json:"last_seen" gorm:"-"`
	AlertCount  int       `json:"alert_count" gorm:"column:alert_count"`
	Uptime      string    `json:"uptime" gorm:"-"`
	Description string    `json:"description,omitempty" gorm:"-"`
}

// ---------------------------------------------------------------------------
// Device enrichment: the PostgreSQL devices table only stores basic fields
// (id, name, ip, icon, model, vendor, location, status, alert_count).
// Fields like type, health_score, cpu_usage, memory_usage, network_in/out,
// uptime, and last_seen are computed in Go after the DB query.
// ---------------------------------------------------------------------------

// rawDevice represents a device row as it actually exists in the PostgreSQL
// devices table. Only includes columns present in the schema.
type rawDevice struct {
	ID         string     `gorm:"column:id"`
	Name       string     `gorm:"column:name"`
	IP         string     `gorm:"column:ip"`
	Icon       string     `gorm:"column:icon"`
	Model      string     `gorm:"column:model"`
	Vendor     string     `gorm:"column:vendor"`
	Location   string     `gorm:"column:location"`
	Status     string     `gorm:"column:status"`
	AlertCount int        `gorm:"column:alert_count"`
	CreatedAt  *time.Time `gorm:"column:created_at"`
	UpdatedAt  *time.Time `gorm:"column:updated_at"`
}

// enrichedDevice is the JSON response shape sent to the frontend.
// Fields not present in the DB are computed/inferred in Go.
type enrichedDevice struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	IP           string    `json:"ip"`
	Location     string    `json:"location"`
	Status       string    `json:"status"`
	Vendor       string    `json:"vendor"`
	Model        string    `json:"model"`
	HealthScore  int       `json:"health_score"`
	RecentAlerts int       `json:"recent_alerts"`
	Uptime       string    `json:"uptime"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	NetworkIn    int64     `json:"network_in"`
	NetworkOut   int64     `json:"network_out"`
	LastSeen     time.Time `json:"last_seen"`
	Description  string    `json:"description,omitempty"`
	Firmware     string    `json:"firmware,omitempty"`
	SerialNumber string    `json:"serial_number,omitempty"`
	MACAddress   string    `json:"mac_address,omitempty"`
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

// inferDeviceType determines a device type from its icon, model, and vendor fields.
func inferDeviceType(icon, model, vendor string) string {
	// First check the icon field which is set in the DB seed data
	switch strings.ToLower(strings.TrimSpace(icon)) {
	case "switch":
		return "Switch"
	case "router":
		return "Router"
	case "firewall":
		return "Firewall"
	case "server":
		return "Server"
	case "access_point", "ap", "wireless":
		return "Access Point"
	case "load_balancer", "lb":
		return "Load Balancer"
	}

	// Fall back to checking model and vendor strings
	lowerModel := strings.ToLower(model)
	lowerVendor := strings.ToLower(vendor)
	combined := lowerModel + " " + lowerVendor

	switch {
	case strings.Contains(combined, "catalyst") || strings.Contains(combined, "ex3400") ||
		strings.Contains(combined, "ex4300") || strings.Contains(lowerModel, "switch"):
		return "Switch"
	case strings.Contains(combined, "isr") || strings.Contains(combined, "mx960") ||
		strings.Contains(combined, "mx480") || strings.Contains(lowerModel, "router"):
		return "Router"
	case strings.Contains(combined, "palo alto") || strings.Contains(combined, "pa-") ||
		strings.Contains(combined, "fortigate") || strings.Contains(lowerModel, "firewall"):
		return "Firewall"
	case strings.Contains(combined, "big-ip") || strings.Contains(combined, "f5") ||
		strings.Contains(lowerModel, "load balancer"):
		return "Load Balancer"
	case strings.Contains(combined, "ap-") || strings.Contains(combined, "aruba") ||
		strings.Contains(lowerModel, "access point"):
		return "Access Point"
	case strings.Contains(combined, "poweredge") || strings.Contains(combined, "proliant") ||
		strings.Contains(combined, "dell") || strings.Contains(combined, "hp"):
		return "Server"
	default:
		return "Server"
	}
}

// generateUptime produces a realistic uptime string like "45d 12h" based on
// a deterministic hash of the device ID. The value drifts slowly over time
// so it looks alive on refresh.
func generateUptime(deviceID string) string {
	h := deterministicHash(deviceID)
	// Base days in range 5-180, hours 0-23
	baseDays := int(h%176) + 5
	baseHours := int((h / 7) % 24)
	// Add a slow drift: current day-of-year modulo a small range
	drift := time.Now().YearDay() % 10
	days := baseDays + drift
	return fmt.Sprintf("%dd %dh", days, baseHours)
}

// generateDeviceMetricValue produces a deterministic float in [lo, hi) for a
// given device ID and metric name. It changes slowly over time (every ~5 min).
func generateDeviceMetricValue(deviceID, metric string, lo, hi float64) float64 {
	h := deterministicHash(deviceID + ":" + metric)
	// Add slow time component so values shift on refresh
	timeSlot := time.Now().Unix() / 300 // changes every 5 minutes
	combined := h + timeSlot
	if combined < 0 {
		combined = -combined
	}
	// Map to [0, 1)
	normalized := float64(combined%10000) / 10000.0
	return math.Round((lo+normalized*(hi-lo))*100) / 100
}

// getRecentAlertCounts queries the alerts table and returns a map of
// device_name -> alert count for alerts in the last 24 hours. It tries
// both the GORM-created "device" column and the SQL-schema "device_name"
// column, falling back gracefully if one doesn't exist.
func getRecentAlertCounts(db *database.Database, deviceNames []string) map[string]int {
	counts := make(map[string]int, len(deviceNames))
	if len(deviceNames) == 0 {
		return counts
	}

	cutoff := time.Now().UTC().Add(-24 * time.Hour)

	// Try GORM model column "device" first (used by IngestEvent handler)
	type deviceAlertCount struct {
		Device string
		Count  int
	}
	var results []deviceAlertCount
	err := db.Model(&models.Alert{}).
		Select("device, COUNT(*) as count").
		Where("device IN ? AND timestamp >= ?", deviceNames, cutoff).
		Group("device").
		Scan(&results).Error

	if err == nil && len(results) > 0 {
		for _, r := range results {
			counts[r.Device] = r.Count
		}
		return counts
	}

	// Fallback: try the SQL-schema column "device_name" (used by init.sql seed data)
	type deviceNameAlertCount struct {
		DeviceName string `gorm:"column:device_name"`
		Count      int
	}
	var results2 []deviceNameAlertCount
	err2 := db.Table("alerts").
		Select("device_name, COUNT(*) as count").
		Where("device_name IN ? AND timestamp >= ?", deviceNames, cutoff).
		Group("device_name").
		Scan(&results2).Error

	if err2 == nil {
		for _, r := range results2 {
			counts[r.DeviceName] = r.Count
		}
	}

	return counts
}

// enrichOneDevice converts a raw DB device row into a fully populated enrichedDevice.
func enrichOneDevice(d rawDevice, recentAlerts int) enrichedDevice {
	now := time.Now().UTC()

	// Determine last_seen: use updated_at if available, else now
	lastSeen := now
	if d.UpdatedAt != nil && !d.UpdatedAt.IsZero() {
		lastSeen = *d.UpdatedAt
	}

	// Infer type from icon/model/vendor
	deviceType := inferDeviceType(d.Icon, d.Model, d.Vendor)

	// Compute health score: 100 - (recentAlerts * 5), clamped to [20, 100]
	healthScore := 100 - (recentAlerts * 5)
	if healthScore < 20 {
		healthScore = 20
	}
	if healthScore > 100 {
		healthScore = 100
	}

	// If the device is offline, reduce health significantly
	if strings.EqualFold(d.Status, "offline") {
		healthScore = 20
	}

	// Generate uptime (offline devices get "0d 0h")
	uptime := generateUptime(d.ID)
	if strings.EqualFold(d.Status, "offline") {
		uptime = "0d 0h"
	}

	// CPU usage: range depends on health. Healthier devices have lower CPU.
	cpuLo := 15.0
	cpuHi := 45.0
	if healthScore < 60 {
		cpuLo = 50.0
		cpuHi = 85.0
	} else if healthScore < 80 {
		cpuLo = 35.0
		cpuHi = 65.0
	}
	cpuUsage := generateDeviceMetricValue(d.ID, "cpu", cpuLo, cpuHi)

	// Memory usage: range depends on health
	memLo := 30.0
	memHi := 55.0
	if healthScore < 60 {
		memLo = 60.0
		memHi = 90.0
	} else if healthScore < 80 {
		memLo = 45.0
		memHi = 70.0
	}
	memUsage := generateDeviceMetricValue(d.ID, "mem", memLo, memHi)

	// Network in/out in bytes/sec - vary by device type
	var netInLo, netInHi, netOutLo, netOutHi float64
	switch deviceType {
	case "Router", "Switch":
		netInLo, netInHi = 5000000, 50000000   // 5-50 MB/s
		netOutLo, netOutHi = 4000000, 45000000  // 4-45 MB/s
	case "Firewall":
		netInLo, netInHi = 2000000, 30000000    // 2-30 MB/s
		netOutLo, netOutHi = 1500000, 25000000  // 1.5-25 MB/s
	case "Load Balancer":
		netInLo, netInHi = 10000000, 80000000   // 10-80 MB/s
		netOutLo, netOutHi = 8000000, 70000000  // 8-70 MB/s
	default: // Server, AP, etc.
		netInLo, netInHi = 1000000, 20000000    // 1-20 MB/s
		netOutLo, netOutHi = 500000, 15000000   // 0.5-15 MB/s
	}
	networkIn := int64(generateDeviceMetricValue(d.ID, "netin", netInLo, netInHi))
	networkOut := int64(generateDeviceMetricValue(d.ID, "netout", netOutLo, netOutHi))

	// Offline devices have zero throughput and resource usage
	if strings.EqualFold(d.Status, "offline") {
		cpuUsage = 0
		memUsage = 0
		networkIn = 0
		networkOut = 0
	}

	return enrichedDevice{
		ID:           d.ID,
		Name:         d.Name,
		Type:         deviceType,
		IP:           d.IP,
		Location:     d.Location,
		Status:       d.Status,
		Vendor:       d.Vendor,
		Model:        d.Model,
		HealthScore:  healthScore,
		RecentAlerts: recentAlerts,
		Uptime:       uptime,
		CPUUsage:     cpuUsage,
		MemoryUsage:  memUsage,
		NetworkIn:    networkIn,
		NetworkOut:   networkOut,
		LastSeen:     lastSeen,
	}
}
