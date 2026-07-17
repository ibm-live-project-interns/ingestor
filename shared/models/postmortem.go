package models

import (
	"encoding/json"
	"time"
)

// PostMortem represents a post-incident review / root cause analysis
type PostMortem struct {
	ID                 uint            `json:"id" gorm:"primaryKey"`
	AlertID            uint            `json:"alert_id"`
	AlertIDStr         string          `json:"alert_id_str,omitempty" gorm:"column:alert_id_str;size:50"`
	Title              string          `json:"title" gorm:"size:500;not null"`
	RootCause          string          `json:"root_cause" gorm:"type:text"`
	RootCauseCategory  string          `json:"root_cause_category" gorm:"size:100"`
	ImpactDescription  string          `json:"impact_description" gorm:"type:text"`
	Timeline           json.RawMessage `json:"timeline" gorm:"type:jsonb"`
	ActionItems        json.RawMessage `json:"action_items" gorm:"type:jsonb"`
	PreventionMeasures string          `json:"prevention_measures" gorm:"type:text"`
	Status             string          `json:"status" gorm:"size:50;default:draft"`
	CreatedBy          string          `json:"created_by" gorm:"size:255"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// TableName returns the table name for PostMortem
func (PostMortem) TableName() string {
	return "post_mortems"
}

// PostMortem status constants
const (
	PostMortemStatusDraft     = "draft"
	PostMortemStatusReview    = "in-review"
	PostMortemStatusPublished = "published"
)

// IsValidStatus checks if the post-mortem status is valid
func (p *PostMortem) IsValidStatus() bool {
	validStatuses := map[string]bool{
		PostMortemStatusDraft:     true,
		PostMortemStatusReview:    true,
		PostMortemStatusPublished: true,
	}
	return validStatuses[p.Status]
}
