package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// ---------------------------------------------------------------------------
// Device handlers
// ---------------------------------------------------------------------------

// GetDevices returns all devices with optional filtering - queries real DB with demo fallback
func GetDevices(c *gin.Context) {
	db := database.Get()
	if db == nil {
		logger.Warn("No database connection, returning demo devices")
		devices := getDemoDevices()
		c.JSON(http.StatusOK, gin.H{
			"devices": devices,
			"total":   len(devices),
		})
		return
	}

	// Query only columns that actually exist in the devices table
	var rawDevices []rawDevice
	query := db.Table("devices").
		Select("id, name, ip, icon, model, vendor, location, status, alert_count, created_at, updated_at").
		Where("deleted_at IS NULL").
		Order("name ASC")

	// Apply filters
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if deviceType := c.Query("type"); deviceType != "" {
		allowedTypes := map[string]bool{
			"router": true, "switch": true, "firewall": true, "server": true,
			"access_point": true, "load_balancer": true, "gateway": true, "other": true,
		}
		if !allowedTypes[strings.ToLower(deviceType)] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device type"})
			return
		}
		// The DB uses "icon" for device category; match on that
		query = query.Where("LOWER(icon) = LOWER(?)", deviceType)
	}

	if err := query.Find(&rawDevices).Error; err != nil {
		logger.Error("Failed to query devices from DB: %v, falling back to demo data", err)
		devices := getDemoDevices()
		c.JSON(http.StatusOK, gin.H{
			"devices": devices,
			"total":   len(devices),
		})
		return
	}

	// Collect device names for a single batch alert-count query
	names := make([]string, len(rawDevices))
	for i, d := range rawDevices {
		names[i] = d.Name
	}
	alertCounts := getRecentAlertCounts(db, names)

	// Enrich each device with computed fields
	enriched := make([]enrichedDevice, 0, len(rawDevices))
	for _, d := range rawDevices {
		recentAlerts := alertCounts[d.Name]
		enriched = append(enriched, enrichOneDevice(d, recentAlerts))
	}

	logger.Info("Returning %d enriched devices from database", len(enriched))
	c.JSON(http.StatusOK, gin.H{
		"devices": enriched,
		"total":   len(enriched),
	})
}

// GetDeviceByID returns a single device by ID - queries real DB with demo fallback
func GetDeviceByID(c *gin.Context) {
	deviceID := c.Param("id")

	db := database.Get()
	if db != nil {
		var device rawDevice
		err := db.Table("devices").
			Select("id, name, ip, icon, model, vendor, location, status, alert_count, created_at, updated_at").
			Where("id = ? AND deleted_at IS NULL", deviceID).
			First(&device).Error

		if err == nil {
			// Count recent alerts for this specific device
			alertCounts := getRecentAlertCounts(db, []string{device.Name})
			recentAlerts := alertCounts[device.Name]

			enriched := enrichOneDevice(device, recentAlerts)

			// Generate deterministic firmware, serial number, and MAC for detail view
			h := deterministicHash(device.ID)
			enriched.Firmware = fmt.Sprintf("v%d.%d.%d", 1+(h%5), h%10, (h/3)%20)
			idPrefix := device.ID
			if len(idPrefix) > 3 {
				idPrefix = idPrefix[:3]
			}
			enriched.SerialNumber = fmt.Sprintf("SN-%s-%06d", strings.ToUpper(idPrefix), h%1000000)
			enriched.MACAddress = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
				(h/1)%256, (h/7)%256, (h/13)%256, (h/19)%256, (h/29)%256, (h/37)%256)

			logger.Info("Returning enriched device %s from database", deviceID)
			c.JSON(http.StatusOK, enriched)
			return
		}
		logger.Warn("Device %s not found in database, checking demo data", deviceID)
	}

	// Fallback to demo devices
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

	// Ensure device names are populated (the repo sets DeviceName = Device from
	// alerts, but if the device column was empty we fall back to DeviceID)
	for i := range noisyDevices {
		if noisyDevices[i].DeviceName == "" {
			noisyDevices[i].DeviceName = noisyDevices[i].DeviceID
		}
	}

	c.JSON(http.StatusOK, noisyDevices)
}
