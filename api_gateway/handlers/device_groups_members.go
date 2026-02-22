package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.Lock()
	var updated *DeviceGroup
	for i := range demoDeviceGroups {
		if demoDeviceGroups[i].ID == groupID {
			// Build a set of existing device IDs for deduplication
			existing := make(map[string]bool, len(demoDeviceGroups[i].DeviceIDs))
			for _, id := range demoDeviceGroups[i].DeviceIDs {
				existing[id] = true
			}

			// Add only new device IDs
			for _, id := range req.DeviceIDs {
				if !existing[id] {
					demoDeviceGroups[i].DeviceIDs = append(demoDeviceGroups[i].DeviceIDs, id)
					existing[id] = true
				}
			}
			demoDeviceGroups[i].DeviceCount = len(demoDeviceGroups[i].DeviceIDs)
			demoDeviceGroups[i].UpdatedAt = time.Now()

			g := demoDeviceGroups[i]
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

	ensureDemoDeviceGroupsInitialized()

	deviceGroupMu.Lock()
	var updated *DeviceGroup
	for i := range demoDeviceGroups {
		if demoDeviceGroups[i].ID == groupID {
			// Find and remove the device
			deviceFound := false
			newDeviceIDs := make([]string, 0, len(demoDeviceGroups[i].DeviceIDs))
			for _, id := range demoDeviceGroups[i].DeviceIDs {
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

			demoDeviceGroups[i].DeviceIDs = newDeviceIDs
			demoDeviceGroups[i].DeviceCount = len(newDeviceIDs)
			demoDeviceGroups[i].UpdatedAt = time.Now()

			g := demoDeviceGroups[i]
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
