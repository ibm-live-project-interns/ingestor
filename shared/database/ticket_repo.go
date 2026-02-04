package database

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// TicketRepository handles ticket database operations
type TicketRepository struct {
	db *gorm.DB
}

// NewTicketRepository creates a new ticket repository
func NewTicketRepository(db *gorm.DB) *TicketRepository {
	return &TicketRepository{db: db}
}

// TicketFilter represents filter options for querying tickets
type TicketFilter struct {
	Priority string
	Status   string
	Category string
	Assignee string
	Reporter string
	AlertID  string
	DeviceID string
	Limit    int
	Offset   int
}

// Create creates a new ticket
func (r *TicketRepository) Create(ticket *models.Ticket) error {
	if err := r.db.Create(ticket).Error; err != nil {
		logger.Error("Failed to create ticket: %v", err)
		return fmt.Errorf("failed to create ticket: %w", err)
	}
	logger.Info("Created ticket: %s", ticket.ID)
	return nil
}

// GetByID retrieves a ticket by ID
func (r *TicketRepository) GetByID(id string) (*models.Ticket, error) {
	var ticket models.Ticket
	if err := r.db.First(&ticket, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("Failed to get ticket %s: %v", id, err)
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}
	return &ticket, nil
}

// List retrieves tickets with optional filtering
func (r *TicketRepository) List(filter TicketFilter) ([]models.Ticket, int64, error) {
	var tickets []models.Ticket
	var total int64

	query := r.db.Model(&models.Ticket{})

	// Apply filters
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Assignee != "" {
		query = query.Where("assignee = ?", filter.Assignee)
	}
	if filter.Reporter != "" {
		query = query.Where("reporter = ?", filter.Reporter)
	}
	if filter.AlertID != "" {
		query = query.Where("alert_id = ?", filter.AlertID)
	}
	if filter.DeviceID != "" {
		query = query.Where("device_id = ?", filter.DeviceID)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count tickets: %v", err)
		return nil, 0, fmt.Errorf("failed to count tickets: %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by created_at descending
	if err := query.Order("created_at DESC").Find(&tickets).Error; err != nil {
		logger.Error("Failed to list tickets: %v", err)
		return nil, 0, fmt.Errorf("failed to list tickets: %w", err)
	}

	return tickets, total, nil
}

// Update updates a ticket
func (r *TicketRepository) Update(ticket *models.Ticket) error {
	if err := r.db.Save(ticket).Error; err != nil {
		logger.Error("Failed to update ticket %s: %v", ticket.ID, err)
		return fmt.Errorf("failed to update ticket: %w", err)
	}
	logger.Debug("Updated ticket: %s", ticket.ID)
	return nil
}

// UpdateFields updates specific fields of a ticket
func (r *TicketRepository) UpdateFields(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()

	// Handle status change to resolved/closed
	if status, ok := updates["status"].(string); ok {
		if status == models.TicketStatusResolved || status == models.TicketStatusClosed {
			now := time.Now().UTC()
			updates["resolved_at"] = &now
		}
	}

	result := r.db.Model(&models.Ticket{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update ticket %s: %v", id, result.Error)
		return fmt.Errorf("failed to update ticket: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("ticket not found: %s", id)
	}

	logger.Info("Ticket %s updated", id)
	return nil
}

// Delete soft deletes a ticket
func (r *TicketRepository) Delete(id string) error {
	result := r.db.Delete(&models.Ticket{}, "id = ?", id)
	if result.Error != nil {
		logger.Error("Failed to delete ticket %s: %v", id, result.Error)
		return fmt.Errorf("failed to delete ticket: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("ticket not found: %s", id)
	}
	logger.Info("Deleted ticket: %s", id)
	return nil
}

// GenerateTicketID generates a unique ticket ID
func (r *TicketRepository) GenerateTicketID() (string, error) {
	var count int64
	if err := r.db.Model(&models.Ticket{}).Count(&count).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("TKT-%06d", count+1), nil
}

// AddComment adds a comment to a ticket
func (r *TicketRepository) AddComment(comment *models.Comment) error {
	if err := r.db.Create(comment).Error; err != nil {
		logger.Error("Failed to add comment: %v", err)
		return fmt.Errorf("failed to add comment: %w", err)
	}
	logger.Debug("Added comment %s to ticket %s", comment.ID, comment.TicketID)
	return nil
}

// GetComments retrieves all comments for a ticket
func (r *TicketRepository) GetComments(ticketID string) ([]models.Comment, error) {
	var comments []models.Comment
	if err := r.db.Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&comments).Error; err != nil {
		return nil, fmt.Errorf("failed to get comments: %w", err)
	}
	return comments, nil
}

// GetTicketStats returns ticket statistics
func (r *TicketRepository) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total count
	var total int64
	if err := r.db.Model(&models.Ticket{}).Count(&total).Error; err != nil {
		return nil, err
	}
	stats["total"] = total

	// By status
	var statusCounts []struct {
		Status string
		Count  int
	}
	if err := r.db.Model(&models.Ticket{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, err
	}
	byStatus := make(map[string]int)
	for _, sc := range statusCounts {
		byStatus[sc.Status] = sc.Count
	}
	stats["by_status"] = byStatus

	// By priority
	var priorityCounts []struct {
		Priority string
		Count    int
	}
	if err := r.db.Model(&models.Ticket{}).
		Select("priority, COUNT(*) as count").
		Group("priority").
		Scan(&priorityCounts).Error; err != nil {
		return nil, err
	}
	byPriority := make(map[string]int)
	for _, pc := range priorityCounts {
		byPriority[pc.Priority] = pc.Count
	}
	stats["by_priority"] = byPriority

	return stats, nil
}

// SetTags sets tags for a ticket (stored as comma-separated)
func (r *TicketRepository) SetTags(id string, tags []string) error {
	tagStr := strings.Join(tags, ",")
	return r.UpdateFields(id, map[string]interface{}{"tags": tagStr})
}

// GetTicketTags retrieves tags from a ticket's Tags field
func GetTicketTags(tagsField string) []string {
	if tagsField == "" {
		return []string{}
	}
	return strings.Split(tagsField, ",")
}
