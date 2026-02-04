package models

import (
	"time"

	"gorm.io/gorm"
)

// Ticket represents a support ticket stored in the database
type Ticket struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Core ticket fields
	Title       string `gorm:"not null;size:255" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	Priority    string `gorm:"not null;size:20;index" json:"priority"` // critical, high, medium, low
	Status      string `gorm:"not null;size:20;index;default:'open'" json:"status"`
	Category    string `gorm:"size:50;index" json:"category"`

	// Assignment
	Assignee string `gorm:"size:100;index" json:"assignee,omitempty"`
	Reporter string `gorm:"size:100" json:"reporter"`

	// Related entities
	AlertID  *string `gorm:"size:50;index" json:"alert_id,omitempty"`
	DeviceID string  `gorm:"size:100;index" json:"device_id,omitempty"`

	// Timing
	DueDate    *time.Time `json:"due_date,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// Tags stored as comma-separated string
	Tags string `gorm:"size:500" json:"-"`
}

// TableName returns the table name for Ticket
func (Ticket) TableName() string {
	return "tickets"
}

// Ticket status constants
const (
	TicketStatusOpen       = "open"
	TicketStatusInProgress = "in-progress"
	TicketStatusPending    = "pending"
	TicketStatusResolved   = "resolved"
	TicketStatusClosed     = "closed"
)

// Ticket priority constants
const (
	TicketPriorityCritical = "critical"
	TicketPriorityHigh     = "high"
	TicketPriorityMedium   = "medium"
	TicketPriorityLow      = "low"
)

// IsValidPriority checks if the ticket priority is valid
func (t *Ticket) IsValidPriority() bool {
	validPriorities := map[string]bool{
		TicketPriorityCritical: true,
		TicketPriorityHigh:     true,
		TicketPriorityMedium:   true,
		TicketPriorityLow:      true,
	}
	return validPriorities[t.Priority]
}

// IsValidStatus checks if the ticket status is valid
func (t *Ticket) IsValidStatus() bool {
	validStatuses := map[string]bool{
		TicketStatusOpen:       true,
		TicketStatusInProgress: true,
		TicketStatusPending:    true,
		TicketStatusResolved:   true,
		TicketStatusClosed:     true,
	}
	return validStatuses[t.Status]
}

// Comment represents a ticket comment
type Comment struct {
	ID        string         `gorm:"primaryKey;size:50" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TicketID string `gorm:"not null;size:50;index" json:"ticket_id"`
	Author   string `gorm:"not null;size:100" json:"author"`
	Content  string `gorm:"type:text;not null" json:"content"`
}

// TableName returns the table name for Comment
func (Comment) TableName() string {
	return "ticket_comments"
}

// CreateTicketRequest represents the request to create a ticket
type CreateTicketRequest struct {
	Title       string   `json:"title" binding:"required,max=255"`
	Description string   `json:"description" binding:"required"`
	Priority    string   `json:"priority" binding:"required,oneof=critical high medium low"`
	Category    string   `json:"category" binding:"required"`
	AlertID     *string  `json:"alert_id,omitempty"`
	DeviceID    string   `json:"device_id,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// UpdateTicketRequest represents the request to update a ticket
type UpdateTicketRequest struct {
	Title       string   `json:"title,omitempty" binding:"omitempty,max=255"`
	Description string   `json:"description,omitempty"`
	Priority    string   `json:"priority,omitempty" binding:"omitempty,oneof=critical high medium low"`
	Status      string   `json:"status,omitempty" binding:"omitempty,oneof=open in-progress pending resolved closed"`
	Category    string   `json:"category,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}
