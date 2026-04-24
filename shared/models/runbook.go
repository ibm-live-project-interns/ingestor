package models

import "time"

// RunbookStep is a single step in a runbook procedure.
type RunbookStep struct {
	Order       int    `json:"order"`
	Instruction string `json:"instruction"`
}

// Runbook represents an operational runbook / knowledge base article.
// Steps and RelatedAlertTypes are stored as JSONB in the database.
type Runbook struct {
	ID                int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Title             string    `gorm:"not null;size:255" json:"title"`
	Category          string    `gorm:"size:50" json:"category"`
	Description       string    `gorm:"type:text" json:"description"`
	Steps             string    `gorm:"type:jsonb;default:'[]'" json:"-"`                              // JSON-encoded []RunbookStep
	RelatedAlertTypes string    `gorm:"column:related_alert_types;type:jsonb;default:'[]'" json:"-"` // JSON-encoded []string
	Author            string    `gorm:"size:100" json:"author"`
	UsageCount        int       `gorm:"default:0" json:"usage_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (Runbook) TableName() string { return "runbooks" }
