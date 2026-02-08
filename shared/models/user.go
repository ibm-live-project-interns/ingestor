package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a system user stored in the database
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Basic Info
	Email    string `gorm:"uniqueIndex;not null;size:255" json:"email"`
	Username string `gorm:"uniqueIndex;not null;size:50" json:"username"`
	Password string `gorm:"not null" json:"-"` // Hashed password, never return in JSON

	// Profile
	FirstName string `gorm:"size:100" json:"first_name"`
	LastName  string `gorm:"size:100" json:"last_name"`
	Avatar    string `gorm:"size:500" json:"avatar,omitempty"`

	// Role & Permissions - use rbac.RoleID constants
	Role string `gorm:"not null;size:50;default:'network-ops'" json:"role"`

	// OAuth
	GoogleID     string `gorm:"size:100;index" json:"-"`
	OAuthToken   string `gorm:"type:text" json:"-"`
	OAuthRefresh string `gorm:"type:text" json:"-"`

	// Account Status
	IsActive       bool       `gorm:"default:false" json:"is_active"`
	EmailVerified  bool       `gorm:"default:false" json:"email_verified"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	FailedAttempts int        `gorm:"default:0" json:"-"`
	LockedUntil    *time.Time `json:"-"`

	// Verification
	VerificationToken string     `gorm:"size:100;index" json:"-"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`

	// Password Reset
	ResetToken    string     `gorm:"size:100;index" json:"-"`
	ResetTokenExp *time.Time `json:"-"`
}

// TableName returns the table name for User
func (User) TableName() string {
	return "users"
}

// UserResponse is the safe user representation for API responses
type UserResponse struct {
	ID            uint       `json:"id"`
	Email         string     `json:"email"`
	Username      string     `json:"username"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	Avatar        string     `json:"avatar,omitempty"`
	Role          string     `json:"role"`
	IsActive      bool       `json:"is_active"`
	EmailVerified bool       `json:"email_verified"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// ToResponse converts User to UserResponse (safe for API)
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		Username:      u.Username,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Avatar:        u.Avatar,
		Role:          u.Role,
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		LastLogin:     u.LastLogin,
		CreatedAt:     u.CreatedAt,
	}
}

// Session represents an active user session
type Session struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null;type:text" json:"-"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	IPAddress string    `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent string    `gorm:"size:500" json:"user_agent,omitempty"`
	IsActive  bool      `gorm:"default:true;index" json:"is_active"`
}

// TableName returns the table name for Session
func (Session) TableName() string {
	return "sessions"
}

// APIKey represents an API key for programmatic access
type APIKey struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID      uint       `gorm:"not null;index" json:"user_id"`
	Name        string     `gorm:"not null;size:100" json:"name"`
	Key         string     `gorm:"uniqueIndex;not null;size:100" json:"-"`
	Prefix      string     `gorm:"index;not null;size:10" json:"prefix"`
	Permissions string     `gorm:"type:text" json:"permissions"` // JSON array
	LastUsed    *time.Time `json:"last_used,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	IsActive    bool       `gorm:"default:true;index" json:"is_active"`
}

// TableName returns the table name for APIKey
func (APIKey) TableName() string {
	return "api_keys"
}
