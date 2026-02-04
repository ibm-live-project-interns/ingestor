package models

import (
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
)

// Alert represents a network alert stored in the database
type Alert struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Core alert fields
	Title       string `gorm:"not null;size:255" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	Severity    string `gorm:"not null;size:20;index" json:"severity"` // Use constants.Severity*
	Category    string `gorm:"not null;size:50;index" json:"category"`
	Status      string `gorm:"not null;size:20;index;default:'open'" json:"status"` // open, acknowledged, resolved, dismissed

	// Source information
	Source   string `gorm:"size:50" json:"source"`          // syslog, snmp, metadata
	SourceIP string `gorm:"size:45;index" json:"source_ip"` // IPv4 or IPv6
	Device   string `gorm:"size:100;index" json:"device"`

	// Timing
	Timestamp   time.Time  `gorm:"not null;index" json:"timestamp"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	AckedAt     *time.Time `json:"acknowledged_at,omitempty"`
	AckedBy     string     `gorm:"size:100" json:"acknowledged_by,omitempty"`
	ResolvedBy  string     `gorm:"size:100" json:"resolved_by,omitempty"`
	DismissedBy string     `gorm:"size:100" json:"dismissed_by,omitempty"`

	// AI Analysis (stored as JSON)
	AIAnalysisSummary        string  `gorm:"type:text" json:"ai_summary,omitempty"`
	AIAnalysisRootCause      string  `gorm:"type:text" json:"ai_root_cause,omitempty"`
	AIAnalysisImpact         string  `gorm:"type:text" json:"ai_impact,omitempty"`
	AIAnalysisRecommendation string  `gorm:"type:text" json:"ai_recommendation,omitempty"`
	AIConfidence             float64 `json:"ai_confidence,omitempty"`

	// Raw data
	RawPayload string `gorm:"type:text" json:"raw_payload,omitempty"`

	// Relation to tickets
	TicketID *string `gorm:"size:50;index" json:"ticket_id,omitempty"`
}

// TableName returns the table name for Alert
func (Alert) TableName() string {
	return "alerts"
}

// IsValidSeverity checks if the alert severity is valid
func (a *Alert) IsValidSeverity() bool {
	return constants.IsValidSeverity(a.Severity)
}

// IsValidStatus checks if the alert status is valid
func (a *Alert) IsValidStatus() bool {
	validStatuses := map[string]bool{
		AlertStatusOpen:         true,
		AlertStatusAcknowledged: true,
		AlertStatusResolved:     true,
		AlertStatusDismissed:    true,
	}
	return validStatuses[a.Status]
}

// Alert status constants
const (
	AlertStatusOpen         = "open"
	AlertStatusAcknowledged = "acknowledged"
	AlertStatusResolved     = "resolved"
	AlertStatusDismissed    = "dismissed"
)

// AlertsSummary provides aggregated alert statistics
type AlertsSummary struct {
	Total       int            `json:"total"`
	BySeverity  map[string]int `json:"by_severity"`
	ByStatus    map[string]int `json:"by_status"`
	ByCategory  map[string]int `json:"by_category"`
	LastUpdated time.Time      `json:"last_updated"`
}

// SeverityDistribution represents severity distribution data
type SeverityDistribution struct {
	Severity string  `json:"severity"`
	Count    int     `json:"count"`
	Percent  float64 `json:"percent"`
}

// TimeSeriesPoint represents a data point over time
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     int       `json:"value"`
	Label     string    `json:"label,omitempty"`
}

// RecurringAlert represents a pattern of recurring alerts
type RecurringAlert struct {
	Pattern   string    `json:"pattern"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Devices   []string  `json:"devices"`
	Severity  string    `json:"severity"`
}

// NoisyDevice represents a device generating many alerts
type NoisyDevice struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	AlertCount int    `json:"alert_count"`
	TopIssue   string `json:"top_issue"`
}
