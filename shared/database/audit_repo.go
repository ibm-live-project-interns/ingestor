package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// AuditRepository handles audit log database operations
type AuditRepository struct {
	db *gorm.DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Create inserts a new audit log entry
func (r *AuditRepository) Create(log *models.AuditLog) error {
	if err := r.db.Create(log).Error; err != nil {
		logger.Error("Failed to create audit log: %v", err)
		return fmt.Errorf("failed to create audit log: %w", err)
	}
	return nil
}

// AuditFilter represents filter options for querying audit logs
type AuditFilter struct {
	// Search by username or resource ID
	Search string

	// Filter by specific fields
	Action   string
	Resource string
	UserID   uint
	Username string
	Result   string // "success" or "failure"

	// Date range filtering
	StartDate *time.Time
	EndDate   *time.Time

	// Pagination
	Limit  int
	Offset int
}

// List retrieves audit logs with optional filtering and pagination
func (r *AuditRepository) List(filter AuditFilter) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	query := r.db.Model(&models.AuditLog{})

	// Apply search filter (username or resource ID)
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		query = query.Where(
			"username ILIKE ? OR resource_id ILIKE ? OR action ILIKE ? OR resource ILIKE ?",
			search, search, search, search,
		)
	}

	// Apply specific field filters
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.Resource != "" {
		query = query.Where("resource = ?", filter.Resource)
	}
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Username != "" {
		query = query.Where("username = ?", filter.Username)
	}
	if filter.Result != "" {
		query = query.Where("result = ?", filter.Result)
	}

	// Apply date range filtering
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}

	// Count total before pagination
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count audit logs: %v", err)
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	} else {
		query = query.Limit(50) // Default limit
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by created_at descending (most recent first)
	if err := query.Order("created_at DESC").Find(&logs).Error; err != nil {
		logger.Error("Failed to list audit logs: %v", err)
		return nil, 0, fmt.Errorf("failed to list audit logs: %w", err)
	}

	return logs, total, nil
}

// GetDistinctActions returns all distinct action values in the audit log
func (r *AuditRepository) GetDistinctActions() ([]string, error) {
	var actions []string
	if err := r.db.Model(&models.AuditLog{}).Distinct("action").Pluck("action", &actions).Error; err != nil {
		logger.Error("Failed to get distinct actions: %v", err)
		return nil, fmt.Errorf("failed to get distinct actions: %w", err)
	}
	return actions, nil
}

// GetDistinctResources returns all distinct resource values in the audit log
func (r *AuditRepository) GetDistinctResources() ([]string, error) {
	var resources []string
	if err := r.db.Model(&models.AuditLog{}).Distinct("resource").Pluck("resource", &resources).Error; err != nil {
		logger.Error("Failed to get distinct resources: %v", err)
		return nil, fmt.Errorf("failed to get distinct resources: %w", err)
	}
	return resources, nil
}

// GetStats returns summary statistics for the audit log
// Returns counts for last 24 hours: total actions, failed actions, active users, most active user
func (r *AuditRepository) GetStats() (map[string]interface{}, error) {
	since := time.Now().Add(-24 * time.Hour)
	stats := make(map[string]interface{})

	// Total actions in last 24h
	var totalActions int64
	if err := r.db.Model(&models.AuditLog{}).Where("created_at >= ?", since).Count(&totalActions).Error; err != nil {
		return nil, fmt.Errorf("failed to count total actions: %w", err)
	}
	stats["total_actions_24h"] = totalActions

	// Failed actions in last 24h
	var failedActions int64
	if err := r.db.Model(&models.AuditLog{}).Where("created_at >= ? AND result = ?", since, "failure").Count(&failedActions).Error; err != nil {
		return nil, fmt.Errorf("failed to count failed actions: %w", err)
	}
	stats["failed_actions_24h"] = failedActions

	// Active users in last 24h (distinct user count)
	var activeUsers int64
	if err := r.db.Model(&models.AuditLog{}).Where("created_at >= ?", since).Distinct("user_id").Count(&activeUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}
	stats["active_users_24h"] = activeUsers

	// Most active user in last 24h
	type UserActivity struct {
		Username string
		Count    int64
	}
	var mostActive UserActivity
	err := r.db.Model(&models.AuditLog{}).
		Select("username, COUNT(*) as count").
		Where("created_at >= ?", since).
		Group("username").
		Order("count DESC").
		Limit(1).
		Scan(&mostActive).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get most active user: %w", err)
	}
	if mostActive.Username != "" {
		stats["most_active_user"] = mostActive.Username
		stats["most_active_user_actions"] = mostActive.Count
	} else {
		stats["most_active_user"] = "N/A"
		stats["most_active_user_actions"] = 0
	}

	return stats, nil
}
