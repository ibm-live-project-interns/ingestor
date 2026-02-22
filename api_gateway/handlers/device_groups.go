package handlers

import (
	"fmt"
	"net/http"
	"strings"
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
// Handlers
// ==========================================

// GetDeviceGroups returns all device groups with device counts.
// GET /api/v1/device-groups
func GetDeviceGroups(c *gin.Context) {
	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.RLock()
	snapshot := make([]DeviceGroup, len(demoDeviceGroups))
	copy(snapshot, demoDeviceGroups)

	// Collect all grouped device IDs (while we hold the read lock)
	allGroupedDevices := map[string]bool{}
	for _, g := range demoDeviceGroups {
		for _, id := range g.DeviceIDs {
			allGroupedDevices[id] = true
		}
	}
	deviceGroupMu.RUnlock()

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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.RLock()
	var found *DeviceGroup
	for i := range demoDeviceGroups {
		if demoDeviceGroups[i].ID == groupID {
			g := demoDeviceGroups[i]
			g.DeviceCount = len(g.DeviceIDs)
			found = &g
			break
		}
	}
	deviceGroupMu.RUnlock()

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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.Lock()

	// Check for duplicate name
	for _, g := range demoDeviceGroups {
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
	demoDeviceGroups = append(demoDeviceGroups, newGroup)
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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.Lock()

	// Check for duplicate name (excluding current group)
	for _, g := range demoDeviceGroups {
		if g.ID != groupID && strings.EqualFold(g.Name, strings.TrimSpace(req.Name)) {
			deviceGroupMu.Unlock()
			apiErr := errors.NewValidation("A device group with this name already exists")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	var updated *DeviceGroup
	for i := range demoDeviceGroups {
		if demoDeviceGroups[i].ID == groupID {
			demoDeviceGroups[i].Name = strings.TrimSpace(req.Name)
			demoDeviceGroups[i].Description = strings.TrimSpace(req.Description)
			if strings.TrimSpace(req.Color) != "" {
				demoDeviceGroups[i].Color = strings.TrimSpace(req.Color)
			}
			if req.DeviceIDs != nil {
				demoDeviceGroups[i].DeviceIDs = req.DeviceIDs
				demoDeviceGroups[i].DeviceCount = len(req.DeviceIDs)
			}
			demoDeviceGroups[i].UpdatedAt = time.Now()

			g := demoDeviceGroups[i]
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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.Lock()
	found := false
	for i := range demoDeviceGroups {
		if demoDeviceGroups[i].ID == groupID {
			demoDeviceGroups = append(demoDeviceGroups[:i], demoDeviceGroups[i+1:]...)
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

