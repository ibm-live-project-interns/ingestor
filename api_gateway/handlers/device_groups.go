package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// ==========================================
// Device Group Types
// ==========================================

// DeviceGroup represents a logical grouping of network devices.
type DeviceGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	DeviceIDs   []string  `json:"device_ids"`
	DeviceCount int       `json:"device_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateDeviceGroupRequest is the expected payload for creating a device group.
type CreateDeviceGroupRequest struct {
	Name        string   `json:"name" binding:"required,max=100"`
	Description string   `json:"description"`
	Color       string   `json:"color"`
	DeviceIDs   []string `json:"device_ids"`
}

// AddDevicesToGroupRequest is the expected payload for adding devices to a group.
type AddDevicesToGroupRequest struct {
	DeviceIDs []string `json:"device_ids" binding:"required"`
}

// ==========================================
// Demo Data
// ==========================================

// deviceGroupMu protects demoDeviceGroups and nextDemoGroupID from concurrent access.
var deviceGroupMu sync.RWMutex

// nextDemoGroupID tracks the next ID suffix to assign in demo mode.
var nextDemoGroupID = 6

// getDefaultDeviceGroups returns realistic demo device groups.
func getDefaultDeviceGroups() []DeviceGroup {
	now := time.Now()
	return []DeviceGroup{
		{
			ID:          "grp-001",
			Name:        "Core Network",
			Description: "Core switches and routers forming the network backbone",
			Color:       "#4589ff",
			DeviceIDs:   []string{"dev-001", "dev-004", "dev-007"},
			DeviceCount: 3,
			CreatedAt:   now.Add(-90 * 24 * time.Hour),
			UpdatedAt:   now.Add(-2 * 24 * time.Hour),
		},
		{
			ID:          "grp-002",
			Name:        "DMZ / Security",
			Description: "Firewalls and security appliances in the demilitarized zone",
			Color:       "#da1e28",
			DeviceIDs:   []string{"dev-002"},
			DeviceCount: 1,
			CreatedAt:   now.Add(-85 * 24 * time.Hour),
			UpdatedAt:   now.Add(-5 * 24 * time.Hour),
		},
		{
			ID:          "grp-003",
			Name:        "Edge Routing",
			Description: "Edge and border routers connecting to upstream ISPs",
			Color:       "#198038",
			DeviceIDs:   []string{"dev-003"},
			DeviceCount: 1,
			CreatedAt:   now.Add(-80 * 24 * time.Hour),
			UpdatedAt:   now.Add(-10 * 24 * time.Hour),
		},
		{
			ID:          "grp-004",
			Name:        "Wireless Infrastructure",
			Description: "Access points and wireless controllers across all floors",
			Color:       "#8a3ffc",
			DeviceIDs:   []string{"dev-005", "dev-006", "dev-010"},
			DeviceCount: 3,
			CreatedAt:   now.Add(-60 * 24 * time.Hour),
			UpdatedAt:   now.Add(-1 * 24 * time.Hour),
		},
		{
			ID:          "grp-005",
			Name:        "Data Center",
			Description: "Load balancers, UPS systems, and data center equipment",
			Color:       "#ee5396",
			DeviceIDs:   []string{"dev-008", "dev-009"},
			DeviceCount: 2,
			CreatedAt:   now.Add(-45 * 24 * time.Hour),
			UpdatedAt:   now.Add(-3 * 24 * time.Hour),
		},
	}
}

// demoDeviceGroups holds the in-memory device group list for demo mode mutations.
// All access must be protected by deviceGroupMu.
var demoDeviceGroups []DeviceGroup

// initDemoDeviceGroupsLocked ensures the demo data is initialized.
// Caller MUST hold deviceGroupMu write lock before calling.
func initDemoDeviceGroupsLocked() []DeviceGroup {
	if demoDeviceGroups == nil {
		demoDeviceGroups = getDefaultDeviceGroups()
	}
	return demoDeviceGroups
}

// ==========================================
// Handlers
// ==========================================

// GetDeviceGroups returns all device groups with device counts.
// GET /api/v1/device-groups
func GetDeviceGroups(c *gin.Context) {
	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()
	snapshot := make([]DeviceGroup, len(groups))
	copy(snapshot, groups)
	deviceGroupMu.Unlock()

	logger.Info("Demo mode: returning device groups (count=%d)", len(snapshot))

	// Apply optional search filter
	if search := c.Query("search"); search != "" {
		searchLower := strings.ToLower(search)
		filtered := make([]DeviceGroup, 0, len(snapshot))
		for _, g := range snapshot {
			if strings.Contains(strings.ToLower(g.Name), searchLower) ||
				strings.Contains(strings.ToLower(g.Description), searchLower) {
				filtered = append(filtered, g)
			}
		}
		snapshot = filtered
	}

	// Ensure device_count is accurate for each group
	for i := range snapshot {
		snapshot[i].DeviceCount = len(snapshot[i].DeviceIDs)
	}

	// Compute summary stats
	totalDevices := 0
	largestGroupName := ""
	largestGroupCount := 0
	for _, g := range snapshot {
		totalDevices += len(g.DeviceIDs)
		if len(g.DeviceIDs) > largestGroupCount {
			largestGroupCount = len(g.DeviceIDs)
			largestGroupName = g.Name
		}
	}

	// Collect all grouped device IDs to compute ungrouped count
	allGroupedDevices := map[string]bool{}
	deviceGroupMu.RLock()
	allGroups := initDemoDeviceGroupsLocked()
	for _, g := range allGroups {
		for _, id := range g.DeviceIDs {
			allGroupedDevices[id] = true
		}
	}
	deviceGroupMu.RUnlock()

	// Assume 10 total demo devices (dev-001 through dev-010)
	totalKnownDevices := 10
	ungroupedDevices := totalKnownDevices - len(allGroupedDevices)
	if ungroupedDevices < 0 {
		ungroupedDevices = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"device_groups": snapshot,
		"total":         len(snapshot),
		"stats": gin.H{
			"total_groups":      len(snapshot),
			"total_devices":     totalDevices,
			"ungrouped_devices": ungroupedDevices,
			"largest_group":     largestGroupName,
			"largest_count":     largestGroupCount,
		},
	})
}

// GetDeviceGroupByID returns a single device group with its devices.
// GET /api/v1/device-groups/:id
func GetDeviceGroupByID(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		apiErr := errors.NewBadRequest("Device group ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()
	var found *DeviceGroup
	for i := range groups {
		if groups[i].ID == groupID {
			g := groups[i]
			g.DeviceCount = len(g.DeviceIDs)
			found = &g
			break
		}
	}
	deviceGroupMu.Unlock()

	if found != nil {
		c.JSON(http.StatusOK, gin.H{
			"device_group": found,
		})
		return
	}

	apiErr := errors.NewNotFound("device group")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// CreateDeviceGroup creates a new device group.
// POST /api/v1/device-groups
func CreateDeviceGroup(c *gin.Context) {
	var req CreateDeviceGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Name) == "" {
		apiErr := errors.NewValidation("Name is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Default color if not provided
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#4589ff"
	}

	// Default device IDs to empty slice
	deviceIDs := req.DeviceIDs
	if deviceIDs == nil {
		deviceIDs = []string{}
	}

	now := time.Now()

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()

	// Check for duplicate name
	for _, g := range groups {
		if strings.EqualFold(g.Name, strings.TrimSpace(req.Name)) {
			deviceGroupMu.Unlock()
			apiErr := errors.NewValidation("A device group with this name already exists")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	newGroup := DeviceGroup{
		ID:          fmt.Sprintf("grp-%03d", nextDemoGroupID),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Color:       color,
		DeviceIDs:   deviceIDs,
		DeviceCount: len(deviceIDs),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	nextDemoGroupID++
	demoDeviceGroups = append(groups, newGroup)
	deviceGroupMu.Unlock()

	logger.Info("Demo mode: created device group id=%s name=%q", newGroup.ID, newGroup.Name)

	c.JSON(http.StatusCreated, gin.H{
		"device_group": newGroup,
		"message":      "Device group created successfully",
	})
}

// UpdateDeviceGroup updates an existing device group.
// PUT /api/v1/device-groups/:id
func UpdateDeviceGroup(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		apiErr := errors.NewBadRequest("Device group ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req CreateDeviceGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		apiErr := errors.NewValidation("Name is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()

	// Check for duplicate name (excluding current group)
	for _, g := range groups {
		if g.ID != groupID && strings.EqualFold(g.Name, strings.TrimSpace(req.Name)) {
			deviceGroupMu.Unlock()
			apiErr := errors.NewValidation("A device group with this name already exists")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	var updated *DeviceGroup
	for i := range groups {
		if groups[i].ID == groupID {
			groups[i].Name = strings.TrimSpace(req.Name)
			groups[i].Description = strings.TrimSpace(req.Description)
			if strings.TrimSpace(req.Color) != "" {
				groups[i].Color = strings.TrimSpace(req.Color)
			}
			if req.DeviceIDs != nil {
				groups[i].DeviceIDs = req.DeviceIDs
				groups[i].DeviceCount = len(req.DeviceIDs)
			}
			groups[i].UpdatedAt = time.Now()

			g := groups[i]
			updated = &g
			break
		}
	}
	deviceGroupMu.Unlock()

	if updated != nil {
		logger.Info("Demo mode: updated device group id=%s name=%q", groupID, updated.Name)
		c.JSON(http.StatusOK, gin.H{
			"device_group": updated,
			"message":      "Device group updated successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("device group")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// DeleteDeviceGroup removes a device group by ID.
// DELETE /api/v1/device-groups/:id
func DeleteDeviceGroup(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		apiErr := errors.NewBadRequest("Device group ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()
	found := false
	for i := range groups {
		if groups[i].ID == groupID {
			demoDeviceGroups = append(groups[:i], groups[i+1:]...)
			found = true
			break
		}
	}
	deviceGroupMu.Unlock()

	if found {
		logger.Info("Demo mode: deleted device group id=%s", groupID)
		c.JSON(http.StatusOK, gin.H{
			"message": "Device group deleted successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("device group")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// AddDevicesToGroup adds one or more devices to a group.
// POST /api/v1/device-groups/:id/devices
func AddDevicesToGroup(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		apiErr := errors.NewBadRequest("Device group ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req AddDevicesToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if len(req.DeviceIDs) == 0 {
		apiErr := errors.NewValidation("At least one device ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()
	var updated *DeviceGroup
	for i := range groups {
		if groups[i].ID == groupID {
			// Build a set of existing device IDs for deduplication
			existing := make(map[string]bool, len(groups[i].DeviceIDs))
			for _, id := range groups[i].DeviceIDs {
				existing[id] = true
			}

			// Add only new device IDs
			for _, id := range req.DeviceIDs {
				if !existing[id] {
					groups[i].DeviceIDs = append(groups[i].DeviceIDs, id)
					existing[id] = true
				}
			}
			groups[i].DeviceCount = len(groups[i].DeviceIDs)
			groups[i].UpdatedAt = time.Now()

			g := groups[i]
			updated = &g
			break
		}
	}
	deviceGroupMu.Unlock()

	if updated != nil {
		logger.Info("Demo mode: added devices to group id=%s, new count=%d", groupID, updated.DeviceCount)
		c.JSON(http.StatusOK, gin.H{
			"device_group": updated,
			"message":      "Devices added to group successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("device group")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// RemoveDeviceFromGroup removes a single device from a group.
// DELETE /api/v1/device-groups/:id/devices/:deviceId
func RemoveDeviceFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	deviceID := c.Param("deviceId")

	if groupID == "" || deviceID == "" {
		apiErr := errors.NewBadRequest("Device group ID and device ID are required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	deviceGroupMu.Lock()
	groups := initDemoDeviceGroupsLocked()
	var updated *DeviceGroup
	for i := range groups {
		if groups[i].ID == groupID {
			// Find and remove the device
			deviceFound := false
			newDeviceIDs := make([]string, 0, len(groups[i].DeviceIDs))
			for _, id := range groups[i].DeviceIDs {
				if id == deviceID {
					deviceFound = true
					continue
				}
				newDeviceIDs = append(newDeviceIDs, id)
			}

			if !deviceFound {
				deviceGroupMu.Unlock()
				apiErr := errors.NewNotFound("device in group")
				c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
				return
			}

			groups[i].DeviceIDs = newDeviceIDs
			groups[i].DeviceCount = len(newDeviceIDs)
			groups[i].UpdatedAt = time.Now()

			g := groups[i]
			updated = &g
			break
		}
	}
	deviceGroupMu.Unlock()

	if updated != nil {
		logger.Info("Demo mode: removed device %s from group %s, new count=%d", deviceID, groupID, updated.DeviceCount)
		c.JSON(http.StatusOK, gin.H{
			"device_group": updated,
			"message":      "Device removed from group successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("device group")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}
