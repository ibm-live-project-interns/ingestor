package models

import (
	"time"

	"gorm.io/gorm"
)

// ThresholdRule represents an alert threshold rule
type ThresholdRule struct {
	ID          string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"not null;size:100" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Condition   string         `gorm:"not null;size:255" json:"condition"`
	Duration    string         `gorm:"size:50" json:"duration"`
	Severity    string         `gorm:"not null;size:20;default:'warning'" json:"severity"`
	Enabled     bool           `gorm:"default:true" json:"enabled"`
}

func (ThresholdRule) TableName() string { return "threshold_rules" }

// NotificationChannel represents a notification channel (Slack, Email, SMS)
type NotificationChannel struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"not null;size:100" json:"name"`
	Type      string         `gorm:"not null;size:20" json:"type"` // Slack, Email, Twilio
	Meta      string         `gorm:"type:text" json:"meta"`
	Active    bool           `gorm:"default:true" json:"active"`
}

func (NotificationChannel) TableName() string { return "notification_channels" }

// EscalationPolicy represents an alert escalation policy
type EscalationPolicy struct {
	ID          string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"not null;size:100" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	Steps       int            `gorm:"default:1" json:"steps"`
	Active      bool           `gorm:"default:true" json:"active"`
}

func (EscalationPolicy) TableName() string { return "escalation_policies" }

// MaintenanceWindow represents a scheduled maintenance window
type MaintenanceWindow struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"not null;size:100" json:"name"`
	Schedule  string         `gorm:"size:200" json:"schedule"`
	Duration  string         `gorm:"size:100" json:"duration"`
	Status    string         `gorm:"size:20;default:'scheduled'" json:"status"` // scheduled, active, completed
}

func (MaintenanceWindow) TableName() string { return "maintenance_windows" }

// Request structs for create/update

type CreateRuleRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	Condition   string `json:"condition" binding:"required"`
	Duration    string `json:"duration"`
	Severity    string `json:"severity" binding:"required,oneof=critical major warning info"`
	Enabled     *bool  `json:"enabled"`
}

type UpdateRuleRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Condition   string `json:"condition,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Severity    string `json:"severity,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type CreateChannelRequest struct {
	Name   string `json:"name" binding:"required,max=100"`
	Type   string `json:"type" binding:"required,oneof=Slack Email Twilio"`
	Meta   string `json:"meta"`
	Active *bool  `json:"active"`
}

type UpdateChannelRequest struct {
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
	Meta   string `json:"meta,omitempty"`
	Active *bool  `json:"active,omitempty"`
}

type CreatePolicyRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	Steps       int    `json:"steps" binding:"required,min=1,max=10"`
	Active      *bool  `json:"active"`
}

type UpdatePolicyRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Steps       int    `json:"steps,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type CreateWindowRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Schedule string `json:"schedule"`
	Duration string `json:"duration"`
	Status   string `json:"status"`
}

type UpdateWindowRequest struct {
	Name     string `json:"name,omitempty"`
	Schedule string `json:"schedule,omitempty"`
	Duration string `json:"duration,omitempty"`
	Status   string `json:"status,omitempty"`
}
