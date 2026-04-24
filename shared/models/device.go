package models

import (
	"time"

	"gorm.io/gorm"
)

// Device represents a network device stored in the `devices` table.
// The schema mirrors infra/prod/postgres-init/init.sql exactly so that
// GORM auto-migrations do not attempt to alter existing production columns.
type Device struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name     string `gorm:"size:100;not null;index" json:"name"`
	IP       string `gorm:"column:ip;size:45" json:"ip,omitempty"`
	Icon     string `gorm:"size:50" json:"icon,omitempty"`
	Model    string `gorm:"size:100" json:"model,omitempty"`
	Vendor   string `gorm:"size:100" json:"vendor,omitempty"`
	Location string `gorm:"size:200;index" json:"location,omitempty"`
	// Allowed values: active, inactive, maintenance, decommissioned
	Status     string `gorm:"size:20;default:'active';index" json:"status"`
	AlertCount int    `gorm:"default:0" json:"alert_count"`
}

// TableName returns the table name for Device.
func (Device) TableName() string { return "devices" }
