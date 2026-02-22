package handlers

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// MetricDataPoint represents a single time-series metric sample
type MetricDataPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	CPUUsage     float64   `json:"cpu_usage"`
	MemoryUsage  float64   `json:"memory_usage"`
	BandwidthIn  float64   `json:"bandwidth_in"`
	BandwidthOut float64   `json:"bandwidth_out"`
	ErrorRate    float64   `json:"error_rate"`
}

// GetDeviceMetrics returns time-series performance metrics for a device.
// Accepts query param "period" (1h, 6h, 24h, 7d), defaulting to 24h.
// In demo mode (no DB), generates realistic synthetic data seeded from
// the device's baseline CPU/memory values.
func GetDeviceMetrics(c *gin.Context) {
	deviceID := c.Param("id")

	// Parse and validate period
	period := c.DefaultQuery("period", "24h")
	var duration time.Duration
	var intervalMinutes int
	switch period {
	case "1h":
		duration = 1 * time.Hour
		intervalMinutes = 1 // 60 data points
	case "6h":
		duration = 6 * time.Hour
		intervalMinutes = 5 // 72 data points
	case "24h":
		duration = 24 * time.Hour
		intervalMinutes = 15 // 96 data points
	case "7d":
		duration = 7 * 24 * time.Hour
		intervalMinutes = 60 // 168 data points
	default:
		apiErr := errors.NewBadRequest("Invalid period. Use 1h, 6h, 24h, or 7d")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Try to get device baseline from DB (use enriched CPU/memory if available)
	var baseCPU, baseMem float64
	var deviceFound bool

	db := database.Get()
	if db != nil {
		// Check if device exists by querying actual DB columns
		var device rawDevice
		if err := db.Table("devices").
			Select("id, name, ip, icon, model, vendor, location, status, alert_count, created_at, updated_at").
			Where("id = ? AND deleted_at IS NULL", deviceID).
			First(&device).Error; err == nil {
			deviceFound = true
			// Use the enrichment logic to derive baseline CPU/memory
			alertCounts := getRecentAlertCounts(db, []string{device.Name})
			enriched := enrichOneDevice(device, alertCounts[device.Name])
			baseCPU = enriched.CPUUsage
			baseMem = enriched.MemoryUsage
		}
	}

	// If no DB baseline, check demo devices for baseline
	if !deviceFound {
		for _, d := range getDemoDevices() {
			if d.ID == deviceID {
				deviceFound = true
				// Demo devices don't carry CPU/memory; assign a realistic baseline
				// based on device status
				switch d.Status {
				case "online":
					baseCPU = 35.0
					baseMem = 55.0
				case "degraded":
					baseCPU = 72.0
					baseMem = 78.0
				case "offline":
					baseCPU = 0.0
					baseMem = 0.0
				default:
					baseCPU = 45.0
					baseMem = 60.0
				}
				break
			}
		}
	}

	if !deviceFound {
		apiErr := errors.NewNotFound(fmt.Sprintf("device %s", deviceID))
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Generate realistic metric data using random walk with occasional spikes
	metrics := generateRealisticMetrics(baseCPU, baseMem, duration, intervalMinutes, deviceID)

	logger.Info("Returning %d metric data points for device %s (period=%s)", len(metrics), deviceID, period)
	c.JSON(http.StatusOK, gin.H{
		"metrics":   metrics,
		"device_id": deviceID,
		"period":    period,
	})
}

// generateRealisticMetrics produces time-series data that mimics real device
// behavior: gradual drift, minor jitter, diurnal patterns, and occasional
// spikes. The deviceID is used as a seed so the same device always produces
// consistent (but not identical) data within the same second.
func generateRealisticMetrics(baseCPU, baseMem float64, totalDuration time.Duration, intervalMins int, deviceID string) []MetricDataPoint {
	now := time.Now().UTC()
	start := now.Add(-totalDuration)
	interval := time.Duration(intervalMins) * time.Minute
	numPoints := int(totalDuration / interval)
	if numPoints < 1 {
		numPoints = 1
	}

	// Derive a deterministic seed from deviceID so the same device gives
	// visually consistent results across rapid refreshes in the same second,
	// but differs meaningfully between devices.
	seed := int64(0)
	for _, ch := range deviceID {
		seed = seed*31 + int64(ch)
	}
	seed += now.Unix() / 60 // changes every minute to simulate live data
	rng := rand.New(rand.NewSource(seed))

	points := make([]MetricDataPoint, 0, numPoints)

	// State variables for random walk
	cpu := baseCPU
	mem := baseMem
	bwIn := 200.0 + rng.Float64()*300.0  // baseline 200-500 Mbps
	bwOut := 100.0 + rng.Float64()*200.0  // baseline 100-300 Mbps
	errRate := 0.01 + rng.Float64()*0.04  // baseline 0.01-0.05%

	for i := 0; i < numPoints; i++ {
		ts := start.Add(time.Duration(i) * interval)
		hourOfDay := float64(ts.Hour()) + float64(ts.Minute())/60.0

		// Diurnal multiplier: higher load during business hours (8-18),
		// lower overnight. Smooth sinusoidal shape centered at 13:00.
		diurnal := 1.0 + 0.15*math.Sin((hourOfDay-7.0)*math.Pi/12.0)
		if hourOfDay < 6 || hourOfDay > 22 {
			diurnal = 0.75 + rng.Float64()*0.1
		}

		// Random walk with mean reversion toward baseline
		cpu += (baseCPU*diurnal-cpu)*0.08 + rng.NormFloat64()*1.5
		mem += (baseMem*diurnal-mem)*0.05 + rng.NormFloat64()*1.0
		bwIn += rng.NormFloat64()*15.0 - (bwIn-350.0)*0.03
		bwOut += rng.NormFloat64()*10.0 - (bwOut-180.0)*0.03
		errRate += rng.NormFloat64()*0.005 - (errRate-0.03)*0.05

		// Occasional spikes (~3% chance per point)
		if rng.Float64() < 0.03 {
			spike := 10.0 + rng.Float64()*25.0
			cpu += spike
		}
		if rng.Float64() < 0.02 {
			spike := 8.0 + rng.Float64()*15.0
			mem += spike
		}
		if rng.Float64() < 0.015 {
			bwIn += 200.0 + rng.Float64()*300.0
		}
		if rng.Float64() < 0.01 {
			errRate += 0.5 + rng.Float64()*2.0
		}

		// Clamp values to realistic bounds
		cpu = clampf(cpu, 0.0, 100.0)
		mem = clampf(mem, 0.0, 100.0)
		bwIn = clampf(bwIn, 0.0, 10000.0)
		bwOut = clampf(bwOut, 0.0, 10000.0)
		errRate = clampf(errRate, 0.0, 100.0)

		points = append(points, MetricDataPoint{
			Timestamp:    ts,
			CPUUsage:     math.Round(cpu*100) / 100,
			MemoryUsage:  math.Round(mem*100) / 100,
			BandwidthIn:  math.Round(bwIn*100) / 100,
			BandwidthOut: math.Round(bwOut*100) / 100,
			ErrorRate:    math.Round(errRate*1000) / 1000,
		})
	}

	return points
}

// clampf constrains v to the range [lo, hi].
func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
