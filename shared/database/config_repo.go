package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ConfigRepository handles CRUD for all configuration types
type ConfigRepository struct {
	db *gorm.DB
}

// NewConfigRepository creates a new ConfigRepository
func NewConfigRepository(db *gorm.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

// ==========================================
// Threshold Rules
// ==========================================

// ConfigPagination holds optional pagination parameters for config list queries.
// When Limit is 0, no pagination is applied (all results returned).
type ConfigPagination struct {
	Limit  int
	Offset int
}

func (r *ConfigRepository) ListRules() ([]models.ThresholdRule, error) {
	rules, _, err := r.ListRulesPaginated(ConfigPagination{})
	return rules, err
}

func (r *ConfigRepository) ListRulesPaginated(pg ConfigPagination) ([]models.ThresholdRule, int64, error) {
	var rules []models.ThresholdRule
	var total int64

	query := r.db.Model(&models.ThresholdRule{})
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count threshold rules: %v", err)
		return nil, 0, fmt.Errorf("failed to count rules: %w", err)
	}

	q := r.db.Order("created_at DESC")
	if pg.Limit > 0 {
		q = q.Limit(pg.Limit).Offset(pg.Offset)
	}
	if err := q.Find(&rules).Error; err != nil {
		logger.Error("Failed to list threshold rules: %v", err)
		return nil, 0, fmt.Errorf("failed to list rules: %w", err)
	}
	return rules, total, nil
}

func (r *ConfigRepository) GetRuleByID(id string) (*models.ThresholdRule, error) {
	var rule models.ThresholdRule
	if err := r.db.First(&rule, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}
	return &rule, nil
}

func (r *ConfigRepository) CreateRule(rule *models.ThresholdRule) error {
	if err := r.db.Create(rule).Error; err != nil {
		logger.Error("Failed to create rule %s: %v", rule.ID, err)
		return fmt.Errorf("failed to create rule: %w", err)
	}
	logger.Info("Created threshold rule: %s", rule.ID)
	return nil
}

func (r *ConfigRepository) UpdateRule(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.Model(&models.ThresholdRule{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("rule not found: %s", id)
	}
	logger.Info("Updated threshold rule: %s", id)
	return nil
}

func (r *ConfigRepository) DeleteRule(id string) error {
	result := r.db.Delete(&models.ThresholdRule{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete rule: %w", result.Error)
	}
	logger.Info("Deleted threshold rule: %s", id)
	return nil
}

func (r *ConfigRepository) GenerateRuleID() (string, error) {
	var count int64
	r.db.Unscoped().Model(&models.ThresholdRule{}).Count(&count)
	return fmt.Sprintf("RULE-%03d", count+1), nil
}

// ==========================================
// Notification Channels
// ==========================================

func (r *ConfigRepository) ListChannels() ([]models.NotificationChannel, error) {
	channels, _, err := r.ListChannelsPaginated(ConfigPagination{})
	return channels, err
}

func (r *ConfigRepository) ListChannelsPaginated(pg ConfigPagination) ([]models.NotificationChannel, int64, error) {
	var channels []models.NotificationChannel
	var total int64

	query := r.db.Model(&models.NotificationChannel{})
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count channels: %v", err)
		return nil, 0, fmt.Errorf("failed to count channels: %w", err)
	}

	q := r.db.Order("created_at DESC")
	if pg.Limit > 0 {
		q = q.Limit(pg.Limit).Offset(pg.Offset)
	}
	if err := q.Find(&channels).Error; err != nil {
		logger.Error("Failed to list channels: %v", err)
		return nil, 0, fmt.Errorf("failed to list channels: %w", err)
	}
	return channels, total, nil
}

func (r *ConfigRepository) GetChannelByID(id string) (*models.NotificationChannel, error) {
	var ch models.NotificationChannel
	if err := r.db.First(&ch, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}
	return &ch, nil
}

func (r *ConfigRepository) CreateChannel(ch *models.NotificationChannel) error {
	if err := r.db.Create(ch).Error; err != nil {
		logger.Error("Failed to create channel %s: %v", ch.ID, err)
		return fmt.Errorf("failed to create channel: %w", err)
	}
	logger.Info("Created notification channel: %s", ch.ID)
	return nil
}

func (r *ConfigRepository) UpdateChannel(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.Model(&models.NotificationChannel{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update channel: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("channel not found: %s", id)
	}
	logger.Info("Updated notification channel: %s", id)
	return nil
}

func (r *ConfigRepository) DeleteChannel(id string) error {
	result := r.db.Delete(&models.NotificationChannel{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete channel: %w", result.Error)
	}
	logger.Info("Deleted notification channel: %s", id)
	return nil
}

func (r *ConfigRepository) GenerateChannelID() (string, error) {
	var count int64
	r.db.Unscoped().Model(&models.NotificationChannel{}).Count(&count)
	return fmt.Sprintf("CH-%03d", count+1), nil
}

// ==========================================
// Escalation Policies
// ==========================================

func (r *ConfigRepository) ListPolicies() ([]models.EscalationPolicy, error) {
	policies, _, err := r.ListPoliciesPaginated(ConfigPagination{})
	return policies, err
}

func (r *ConfigRepository) ListPoliciesPaginated(pg ConfigPagination) ([]models.EscalationPolicy, int64, error) {
	var policies []models.EscalationPolicy
	var total int64

	query := r.db.Model(&models.EscalationPolicy{})
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count policies: %v", err)
		return nil, 0, fmt.Errorf("failed to count policies: %w", err)
	}

	q := r.db.Order("created_at DESC")
	if pg.Limit > 0 {
		q = q.Limit(pg.Limit).Offset(pg.Offset)
	}
	if err := q.Find(&policies).Error; err != nil {
		logger.Error("Failed to list policies: %v", err)
		return nil, 0, fmt.Errorf("failed to list policies: %w", err)
	}
	return policies, total, nil
}

func (r *ConfigRepository) GetPolicyByID(id string) (*models.EscalationPolicy, error) {
	var pol models.EscalationPolicy
	if err := r.db.First(&pol, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}
	return &pol, nil
}

func (r *ConfigRepository) CreatePolicy(pol *models.EscalationPolicy) error {
	if err := r.db.Create(pol).Error; err != nil {
		logger.Error("Failed to create policy %s: %v", pol.ID, err)
		return fmt.Errorf("failed to create policy: %w", err)
	}
	logger.Info("Created escalation policy: %s", pol.ID)
	return nil
}

func (r *ConfigRepository) UpdatePolicy(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.Model(&models.EscalationPolicy{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update policy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("policy not found: %s", id)
	}
	logger.Info("Updated escalation policy: %s", id)
	return nil
}

func (r *ConfigRepository) DeletePolicy(id string) error {
	result := r.db.Delete(&models.EscalationPolicy{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete policy: %w", result.Error)
	}
	logger.Info("Deleted escalation policy: %s", id)
	return nil
}

func (r *ConfigRepository) GeneratePolicyID() (string, error) {
	var count int64
	r.db.Unscoped().Model(&models.EscalationPolicy{}).Count(&count)
	return fmt.Sprintf("POL-%03d", count+1), nil
}

// ==========================================
// Maintenance Windows
// ==========================================

func (r *ConfigRepository) ListWindows() ([]models.MaintenanceWindow, error) {
	windows, _, err := r.ListWindowsPaginated(ConfigPagination{})
	return windows, err
}

func (r *ConfigRepository) ListWindowsPaginated(pg ConfigPagination) ([]models.MaintenanceWindow, int64, error) {
	var windows []models.MaintenanceWindow
	var total int64

	query := r.db.Model(&models.MaintenanceWindow{})
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count maintenance windows: %v", err)
		return nil, 0, fmt.Errorf("failed to count windows: %w", err)
	}

	q := r.db.Order("created_at DESC")
	if pg.Limit > 0 {
		q = q.Limit(pg.Limit).Offset(pg.Offset)
	}
	if err := q.Find(&windows).Error; err != nil {
		logger.Error("Failed to list maintenance windows: %v", err)
		return nil, 0, fmt.Errorf("failed to list windows: %w", err)
	}
	return windows, total, nil
}

func (r *ConfigRepository) GetWindowByID(id string) (*models.MaintenanceWindow, error) {
	var win models.MaintenanceWindow
	if err := r.db.First(&win, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get window: %w", err)
	}
	return &win, nil
}

func (r *ConfigRepository) CreateWindow(win *models.MaintenanceWindow) error {
	if err := r.db.Create(win).Error; err != nil {
		logger.Error("Failed to create window %s: %v", win.ID, err)
		return fmt.Errorf("failed to create window: %w", err)
	}
	logger.Info("Created maintenance window: %s", win.ID)
	return nil
}

func (r *ConfigRepository) UpdateWindow(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.Model(&models.MaintenanceWindow{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update window: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("window not found: %s", id)
	}
	logger.Info("Updated maintenance window: %s", id)
	return nil
}

func (r *ConfigRepository) DeleteWindow(id string) error {
	result := r.db.Delete(&models.MaintenanceWindow{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete window: %w", result.Error)
	}
	logger.Info("Deleted maintenance window: %s", id)
	return nil
}

func (r *ConfigRepository) GenerateWindowID() (string, error) {
	var count int64
	r.db.Unscoped().Model(&models.MaintenanceWindow{}).Count(&count)
	return fmt.Sprintf("MW-%03d", count+1), nil
}
