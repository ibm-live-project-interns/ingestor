package database

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// RunbookRepository handles CRUD for runbooks.
type RunbookRepository struct {
	db *gorm.DB
}

// NewRunbookRepository creates a new RunbookRepository.
func NewRunbookRepository(db *gorm.DB) *RunbookRepository {
	return &RunbookRepository{db: db}
}

// List returns paginated runbooks with optional search and category filters.
func (r *RunbookRepository) List(search, category string, limit, offset int) ([]models.Runbook, int64, error) {
	var runbooks []models.Runbook
	var total int64

	q := r.db.Model(&models.Runbook{})
	if search != "" {
		pattern := "%" + EscapeLike(search) + "%"
		q = q.Where("title ILIKE ? OR description ILIKE ? OR author ILIKE ?", pattern, pattern, pattern)
	}
	if category != "" {
		q = q.Where("LOWER(category) = LOWER(?)", category)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count runbooks: %w", err)
	}

	if limit <= 0 {
		limit = 25
	}
	if err := q.Order("usage_count DESC, created_at DESC").Limit(limit).Offset(offset).Find(&runbooks).Error; err != nil {
		logger.Error("Failed to list runbooks: %v", err)
		return nil, 0, fmt.Errorf("failed to list runbooks: %w", err)
	}
	return runbooks, total, nil
}

// GetByID returns a single runbook by numeric ID.
func (r *RunbookRepository) GetByID(id int) (*models.Runbook, error) {
	var runbook models.Runbook
	if err := r.db.First(&runbook, id).Error; err != nil {
		return nil, fmt.Errorf("runbook not found: %w", err)
	}
	return &runbook, nil
}

// Create inserts a new runbook.
func (r *RunbookRepository) Create(runbook *models.Runbook) error {
	if err := r.db.Create(runbook).Error; err != nil {
		logger.Error("Failed to create runbook: %v", err)
		return fmt.Errorf("failed to create runbook: %w", err)
	}
	return nil
}

// Update saves an existing runbook.
func (r *RunbookRepository) Update(runbook *models.Runbook) error {
	runbook.UpdatedAt = time.Now().UTC()
	if err := r.db.Save(runbook).Error; err != nil {
		logger.Error("Failed to update runbook %d: %v", runbook.ID, err)
		return fmt.Errorf("failed to update runbook: %w", err)
	}
	return nil
}

// Delete removes a runbook by ID.
func (r *RunbookRepository) Delete(id int) error {
	if err := r.db.Delete(&models.Runbook{}, id).Error; err != nil {
		logger.Error("Failed to delete runbook %d: %v", id, err)
		return fmt.Errorf("failed to delete runbook: %w", err)
	}
	return nil
}

// IncrementUsage atomically increments the usage_count for a runbook.
func (r *RunbookRepository) IncrementUsage(id int) error {
	return r.db.Model(&models.Runbook{}).Where("id = ?", id).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}

// EncodeRunbookSteps marshals []RunbookStep to a JSON string.
func EncodeRunbookSteps(steps []models.RunbookStep) string {
	if steps == nil {
		steps = []models.RunbookStep{}
	}
	b, _ := json.Marshal(steps)
	return string(b)
}

// DecodeRunbookSteps unmarshals a JSON string to []RunbookStep.
func DecodeRunbookSteps(raw string) []models.RunbookStep {
	if raw == "" || raw == "null" {
		return []models.RunbookStep{}
	}
	var steps []models.RunbookStep
	json.Unmarshal([]byte(raw), &steps)
	return steps
}

// EncodeStringSlice marshals []string to a JSON string.
func EncodeStringSlice(ss []string) string {
	if ss == nil {
		ss = []string{}
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

// DecodeStringSlice unmarshals a JSON string to []string.
func DecodeStringSlice(raw string) []string {
	if raw == "" || raw == "null" {
		return []string{}
	}
	var ss []string
	json.Unmarshal([]byte(raw), &ss)
	return ss
}
