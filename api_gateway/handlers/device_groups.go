package handlers

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// Device Group Types
// ==========================================

// DeviceGroup is the API response type for a device group.
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

// CreateDeviceGroupRequest is the expected payload for creating/updating a device group.
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
// Repository Helper
// ==========================================

func deviceGroupRepo() *database.DeviceGroupRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewDeviceGroupRepository(db.DB)
}

// toHandlerGroup converts a models.DeviceGroup (DB type) to the API response DeviceGroup.
func toHandlerGroup(m models.DeviceGroup) DeviceGroup {
	ids := database.DecodeDeviceIDs(m.DeviceIDs)
	return DeviceGroup{
		ID:          m.ID,
		Name:        m.Name,
		Description: m.Description,
		Color:       m.Color,
		DeviceIDs:   ids,
		DeviceCount: len(ids),
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

// generateGroupID creates a short unique string ID for a new device group.
func generateGroupID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("grp-%x", b)
}

// groupStats computes summary stats from a list of DeviceGroup responses.
func groupStats(groups []DeviceGroup, totalKnownDevices int) gin.H {
	allGroupedDevices := map[string]bool{}
	totalDevices := 0
	largestGroupName := ""
	largestGroupCount := 0
	for _, g := range groups {
		for _, id := range g.DeviceIDs {
			allGroupedDevices[id] = true
		}
		totalDevices += len(g.DeviceIDs)
		if len(g.DeviceIDs) > largestGroupCount {
			largestGroupCount = len(g.DeviceIDs)
			largestGroupName = g.Name
		}
	}
	ungrouped := totalKnownDevices - len(allGroupedDevices)
	if ungrouped < 0 {
		ungrouped = 0
	}
	return gin.H{
		"total_groups":      len(groups),
		"total_devices":     totalDevices,
		"ungrouped_devices": ungrouped,
		"largest_group":     largestGroupName,
		"largest_count":     largestGroupCount,
	}
}

// ==========================================
// Handlers
// ==========================================

// GetDeviceGroups returns all device groups with device counts.
// GET /api/v1/device-groups
func GetDeviceGroups(c *gin.Context) {
	repo := deviceGroupRepo()
	if repo == nil {
		// Demo mode fallback
		ensureDemoDeviceGroupsInitialized()
		deviceGroupMu.RLock()
		snapshot := make([]DeviceGroup, len(demoDeviceGroups))
		copy(snapshot, demoDeviceGroups)
		deviceGroupMu.RUnlock()

		if search := c.Query("search"); search != "" {
			searchLower := strings.ToLower(search)
			filtered := snapshot[:0]
			for _, g := range snapshot {
				if strings.Contains(strings.ToLower(g.Name), searchLower) ||
					strings.Contains(strings.ToLower(g.Description), searchLower) {
					filtered = append(filtered, g)
				}
			}
			snapshot = filtered
		}
		for i := range snapshot {
			snapshot[i].DeviceCount = len(snapshot[i].DeviceIDs)
		}
		c.JSON(http.StatusOK, gin.H{
			"device_groups": snapshot,
			"total":         len(snapshot),
			"stats":         groupStats(snapshot, 10),
		})
		return
	}

	search := c.Query("search")
	dbGroups, err := repo.List(search)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	groups := make([]DeviceGroup, len(dbGroups))
	for i, g := range dbGroups {
		groups[i] = toHandlerGroup(g)
	}

	logger.Info("Returning %d device groups from database", len(groups))
	c.JSON(http.StatusOK, gin.H{
		"device_groups": groups,
		"total":         len(groups),
		"stats":         groupStats(groups, 10),
	})
}

// GetDeviceGroupByID returns a single device group.
// GET /api/v1/device-groups/:id
func GetDeviceGroupByID(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		apiErr := errors.NewBadRequest("Device group ID is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := deviceGroupRepo()
	if repo == nil {
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
			c.JSON(http.StatusOK, gin.H{"device_group": found})
			return
		}
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	dbGroup, err := repo.GetByID(groupID)
	if err != nil {
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	g := toHandlerGroup(*dbGroup)
	c.JSON(http.StatusOK, gin.H{"device_group": g})
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
	if strings.TrimSpace(req.Name) == "" {
		apiErr := errors.NewValidation("Name is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#4589ff"
	}
	deviceIDs := req.DeviceIDs
	if deviceIDs == nil {
		deviceIDs = []string{}
	}

	repo := deviceGroupRepo()
	if repo == nil {
		// Demo mode fallback
		now := time.Now()
		ensureDemoDeviceGroupsInitialized()
		deviceGroupMu.Lock()
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
		c.JSON(http.StatusCreated, gin.H{"device_group": newGroup, "message": "Device group created successfully"})
		return
	}

	now := time.Now()
	dbGroup := models.DeviceGroup{
		ID:          generateGroupID(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Color:       color,
		DeviceIDs:   database.EncodeDeviceIDs(deviceIDs),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.Create(&dbGroup); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Device group created: id=%s name=%q", dbGroup.ID, dbGroup.Name)
	c.JSON(http.StatusCreated, gin.H{
		"device_group": toHandlerGroup(dbGroup),
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

	repo := deviceGroupRepo()
	if repo == nil {
		ensureDemoDeviceGroupsInitialized()
		deviceGroupMu.Lock()
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
			c.JSON(http.StatusOK, gin.H{"device_group": updated, "message": "Device group updated successfully"})
			return
		}
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	dbGroup, err := repo.GetByID(groupID)
	if err != nil {
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	dbGroup.Name = strings.TrimSpace(req.Name)
	dbGroup.Description = strings.TrimSpace(req.Description)
	if strings.TrimSpace(req.Color) != "" {
		dbGroup.Color = strings.TrimSpace(req.Color)
	}
	if req.DeviceIDs != nil {
		dbGroup.DeviceIDs = database.EncodeDeviceIDs(req.DeviceIDs)
	}

	if err := repo.Update(dbGroup); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Device group updated: id=%s name=%q", groupID, dbGroup.Name)
	c.JSON(http.StatusOK, gin.H{
		"device_group": toHandlerGroup(*dbGroup),
		"message":      "Device group updated successfully",
	})
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

	repo := deviceGroupRepo()
	if repo == nil {
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
			c.JSON(http.StatusOK, gin.H{"message": "Device group deleted successfully"})
			return
		}
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if _, err := repo.GetByID(groupID); err != nil {
		apiErr := errors.NewNotFound("device group")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := repo.Delete(groupID); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Device group deleted: id=%s", groupID)
	c.JSON(http.StatusOK, gin.H{"message": "Device group deleted successfully"})
}
