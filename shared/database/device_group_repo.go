package database

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// DeviceGroupRepository handles CRUD for device groups.
type DeviceGroupRepository struct {
	db *gorm.DB
}

// NewDeviceGroupRepository creates a new DeviceGroupRepository.
func NewDeviceGroupRepository(db *gorm.DB) *DeviceGroupRepository {
	return &DeviceGroupRepository{db: db}
}

// List returns all device groups, optionally filtered by search term.
func (r *DeviceGroupRepository) List(search string) ([]models.DeviceGroup, error) {
	var groups []models.DeviceGroup
	q := r.db.Order("created_at ASC")
	if search != "" {
		pattern := "%" + EscapeLike(search) + "%"
		q = q.Where("name ILIKE ? OR description ILIKE ?", pattern, pattern)
	}
	if err := q.Find(&groups).Error; err != nil {
		logger.Error("Failed to list device groups: %v", err)
		return nil, fmt.Errorf("failed to list device groups: %w", err)
	}
	return groups, nil
}

// GetByID returns a single device group by ID.
func (r *DeviceGroupRepository) GetByID(id string) (*models.DeviceGroup, error) {
	var group models.DeviceGroup
	if err := r.db.First(&group, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("device group not found: %w", err)
	}
	return &group, nil
}

// Create inserts a new device group.
func (r *DeviceGroupRepository) Create(group *models.DeviceGroup) error {
	if err := r.db.Create(group).Error; err != nil {
		logger.Error("Failed to create device group: %v", err)
		return fmt.Errorf("failed to create device group: %w", err)
	}
	return nil
}

// Update saves an existing device group.
func (r *DeviceGroupRepository) Update(group *models.DeviceGroup) error {
	group.UpdatedAt = time.Now().UTC()
	if err := r.db.Save(group).Error; err != nil {
		logger.Error("Failed to update device group %s: %v", group.ID, err)
		return fmt.Errorf("failed to update device group: %w", err)
	}
	return nil
}

// Delete removes a device group by ID.
func (r *DeviceGroupRepository) Delete(id string) error {
	if err := r.db.Delete(&models.DeviceGroup{}, "id = ?", id).Error; err != nil {
		logger.Error("Failed to delete device group %s: %v", id, err)
		return fmt.Errorf("failed to delete device group: %w", err)
	}
	return nil
}

// EncodeDeviceIDs marshals []string into a JSON string for DB storage.
func EncodeDeviceIDs(ids []string) string {
	if ids == nil {
		ids = []string{}
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// DecodeDeviceIDs unmarshals a JSON string into []string.
func DecodeDeviceIDs(raw string) []string {
	if raw == "" || raw == "null" {
		return []string{}
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return []string{}
	}
	return ids
}
