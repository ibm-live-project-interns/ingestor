package models

import "time"

// DeviceGroup represents a logical grouping of network devices.
// DeviceIDs is stored as JSONB (JSON-encoded []string) in the database.
type DeviceGroup struct {
	ID          string    `gorm:"primaryKey;size:50" json:"id"`
	Name        string    `gorm:"not null;size:255" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Color       string    `gorm:"size:20" json:"color"`
	DeviceIDs   string    `gorm:"column:device_ids;type:jsonb;default:'[]'" json:"-"` // JSON-encoded []string
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (DeviceGroup) TableName() string { return "device_groups" }
