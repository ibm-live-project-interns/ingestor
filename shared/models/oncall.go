package models

import "time"

// OnCallSchedule represents an on-call schedule entry stored in the database
type OnCallSchedule struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id"`
	Username     string    `json:"username" gorm:"size:255;not null"`
	StartTime    time.Time `json:"start_time" gorm:"not null"`
	EndTime      time.Time `json:"end_time" gorm:"not null"`
	RotationType string    `json:"rotation_type" gorm:"size:50;default:weekly"`
	IsPrimary    bool      `json:"is_primary" gorm:"default:true"`
	CreatedBy    string    `json:"created_by" gorm:"size:255"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName returns the table name for OnCallSchedule
func (OnCallSchedule) TableName() string {
	return "on_call_schedules"
}

// OnCallOverride represents a schedule override where one person covers for another
type OnCallOverride struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	ScheduleID     uint      `json:"schedule_id"`
	OriginalUserID uint      `json:"original_user_id"`
	OverrideUserID uint      `json:"override_user_id"`
	StartTime      time.Time `json:"start_time" gorm:"not null"`
	EndTime        time.Time `json:"end_time" gorm:"not null"`
	Reason         string    `json:"reason" gorm:"type:text"`
	CreatedBy      string    `json:"created_by" gorm:"size:255"`
	CreatedAt      time.Time `json:"created_at"`
}

// TableName returns the table name for OnCallOverride
func (OnCallOverride) TableName() string {
	return "on_call_overrides"
}
