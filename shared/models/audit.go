package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONB represents a JSON field stored as JSONB in PostgreSQL
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed for JSONB")
	}
	result := make(JSONB)
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	*j = result
	return nil
}

// AuditLog represents an entry in the audit log table
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`

	// Who performed the action
	UserID   uint   `gorm:"not null;index" json:"user_id"`
	Username string `gorm:"not null;size:100;index" json:"username"`

	// What was done
	Action   string `gorm:"not null;size:100;index" json:"action"`   // e.g., "user.create", "alert.acknowledge", "ticket.delete"
	Resource string `gorm:"not null;size:100;index" json:"resource"` // e.g., "user", "alert", "ticket", "config"

	// Target of the action
	ResourceID string `gorm:"size:100;index" json:"resource_id"` // ID of the affected resource

	// Additional context
	Details   JSONB  `gorm:"type:jsonb" json:"details,omitempty"`    // Arbitrary JSON details
	IPAddress string `gorm:"size:45" json:"ip_address,omitempty"`    // Client IP address (IPv4 or IPv6)
	Result    string `gorm:"not null;size:20;default:'success'" json:"result"` // "success" or "failure"
}

// TableName returns the table name for AuditLog
func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditLogResponse is the safe representation for API responses
type AuditLogResponse struct {
	ID         uint                   `json:"id"`
	CreatedAt  time.Time              `json:"created_at"`
	UserID     uint                   `json:"user_id"`
	Username   string                 `json:"username"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	ResourceID string                 `json:"resource_id"`
	Details    map[string]interface{} `json:"details,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	Result     string                 `json:"result"`
}

// ToResponse converts AuditLog to AuditLogResponse
func (a *AuditLog) ToResponse() AuditLogResponse {
	details := map[string]interface{}(a.Details)
	if details == nil {
		details = make(map[string]interface{})
	}
	return AuditLogResponse{
		ID:         a.ID,
		CreatedAt:  a.CreatedAt,
		UserID:     a.UserID,
		Username:   a.Username,
		Action:     a.Action,
		Resource:   a.Resource,
		ResourceID: a.ResourceID,
		Details:    details,
		IPAddress:  a.IPAddress,
		Result:     a.Result,
	}
}
