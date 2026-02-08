package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// AlertRepository handles alert database operations
type AlertRepository struct {
	db *gorm.DB
}

// NewAlertRepository creates a new alert repository
func NewAlertRepository(db *gorm.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// AlertFilter represents filter options for querying alerts
type AlertFilter struct {
	Severity string
	Status   string
	Category string
	Device   string
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

// Create creates a new alert
func (r *AlertRepository) Create(alert *models.Alert) error {
	if err := r.db.Create(alert).Error; err != nil {
		logger.Error("Failed to create alert: %v", err)
		return fmt.Errorf("failed to create alert: %w", err)
	}
	logger.Debug("Created alert: %s", alert.ID)
	return nil
}

// GetByID retrieves an alert by ID
func (r *AlertRepository) GetByID(id string) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.First(&alert, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("Failed to get alert %s: %v", id, err)
		return nil, fmt.Errorf("failed to get alert: %w", err)
	}
	return &alert, nil
}

// List retrieves alerts with optional filtering
func (r *AlertRepository) List(filter AlertFilter) ([]models.Alert, int64, error) {
	var alerts []models.Alert
	var total int64

	query := r.db.Model(&models.Alert{})

	// Apply filters
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Device != "" {
		query = query.Where("device = ?", filter.Device)
	}
	if filter.From != nil {
		query = query.Where("timestamp >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("timestamp <= ?", *filter.To)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		logger.Error("Failed to count alerts: %v", err)
		return nil, 0, fmt.Errorf("failed to count alerts: %w", err)
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by timestamp descending
	if err := query.Order("timestamp DESC").Find(&alerts).Error; err != nil {
		logger.Error("Failed to list alerts: %v", err)
		return nil, 0, fmt.Errorf("failed to list alerts: %w", err)
	}

	return alerts, total, nil
}

// Update updates an alert
func (r *AlertRepository) Update(alert *models.Alert) error {
	if err := r.db.Save(alert).Error; err != nil {
		logger.Error("Failed to update alert %s: %v", alert.ID, err)
		return fmt.Errorf("failed to update alert: %w", err)
	}
	logger.Debug("Updated alert: %s", alert.ID)
	return nil
}

// UpdateFields updates specific fields on an alert by ID
func (r *AlertRepository) UpdateFields(id string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now().UTC()
	result := r.db.Model(&models.Alert{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update alert fields %s: %v", id, result.Error)
		return fmt.Errorf("failed to update alert fields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("alert not found: %s", id)
	}
	logger.Info("Alert %s fields updated", id)
	return nil
}

// UpdateStatus updates only the status of an alert
func (r *AlertRepository) UpdateStatus(id, status, byUser string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}

	switch status {
	case models.AlertStatusAcknowledged:
		now := time.Now().UTC()
		updates["acknowledged_at"] = &now
		updates["acknowledged_by"] = byUser
	case models.AlertStatusResolved:
		now := time.Now().UTC()
		updates["resolved_at"] = &now
		updates["resolved_by"] = byUser
	case models.AlertStatusDismissed:
		updates["dismissed_by"] = byUser
	}

	result := r.db.Model(&models.Alert{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		logger.Error("Failed to update alert status %s: %v", id, result.Error)
		return fmt.Errorf("failed to update alert status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("alert not found: %s", id)
	}

	logger.Info("Alert %s status updated to %s by %s", id, status, byUser)
	return nil
}

// GetSummary returns aggregated alert statistics
func (r *AlertRepository) GetSummary() (*models.AlertsSummary, error) {
	summary := &models.AlertsSummary{
		BySeverity:  make(map[string]int),
		ByStatus:    make(map[string]int),
		ByCategory:  make(map[string]int),
		LastUpdated: time.Now().UTC(),
	}

	// Get total count
	var total int64
	if err := r.db.Model(&models.Alert{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count alerts: %w", err)
	}
	summary.Total = int(total)

	// Get severity distribution
	var severityCounts []struct {
		Severity string
		Count    int
	}
	if err := r.db.Model(&models.Alert{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Scan(&severityCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get severity distribution: %w", err)
	}
	for _, sc := range severityCounts {
		summary.BySeverity[sc.Severity] = sc.Count
	}

	// Get status distribution
	var statusCounts []struct {
		Status string
		Count  int
	}
	if err := r.db.Model(&models.Alert{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get status distribution: %w", err)
	}
	for _, sc := range statusCounts {
		summary.ByStatus[sc.Status] = sc.Count
	}

	// Get category distribution
	var categoryCounts []struct {
		Category string
		Count    int
	}
	if err := r.db.Model(&models.Alert{}).
		Select("category, COUNT(*) as count").
		Group("category").
		Scan(&categoryCounts).Error; err != nil {
		return nil, fmt.Errorf("failed to get category distribution: %w", err)
	}
	for _, cc := range categoryCounts {
		summary.ByCategory[cc.Category] = cc.Count
	}

	return summary, nil
}

// GetSeverityDistribution returns severity distribution with percentages
func (r *AlertRepository) GetSeverityDistribution() ([]models.SeverityDistribution, error) {
	var total int64
	if err := r.db.Model(&models.Alert{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count alerts: %w", err)
	}

	var counts []struct {
		Severity string
		Count    int
	}
	if err := r.db.Model(&models.Alert{}).
		Select("severity, COUNT(*) as count").
		Group("severity").
		Scan(&counts).Error; err != nil {
		return nil, fmt.Errorf("failed to get severity distribution: %w", err)
	}

	countMap := make(map[string]int)
	for _, c := range counts {
		countMap[c.Severity] = c.Count
	}

	// Build distribution with all severities
	severities := []string{"critical", "high", "medium", "low", "info"}
	var distribution []models.SeverityDistribution
	for _, sev := range severities {
		count := countMap[sev]
		percent := 0.0
		if total > 0 {
			percent = float64(count) / float64(total) * 100
		}
		distribution = append(distribution, models.SeverityDistribution{
			Severity: sev,
			Count:    count,
			Percent:  percent,
		})
	}

	return distribution, nil
}

// GetAlertsOverTime returns alert counts grouped by hour
func (r *AlertRepository) GetAlertsOverTime(hours int) ([]models.TimeSeriesPoint, error) {
	if hours <= 0 {
		hours = 24
	}

	since := time.Now().UTC().Add(-time.Duration(hours) * time.Hour)

	var results []struct {
		Hour  time.Time
		Count int
	}

	// PostgreSQL specific date_trunc
	if err := r.db.Model(&models.Alert{}).
		Select("date_trunc('hour', timestamp) as hour, COUNT(*) as count").
		Where("timestamp >= ?", since).
		Group("date_trunc('hour', timestamp)").
		Order("hour").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get alerts over time: %w", err)
	}

	// Convert to TimeSeriesPoint
	var points []models.TimeSeriesPoint
	for _, r := range results {
		points = append(points, models.TimeSeriesPoint{
			Timestamp: r.Hour,
			Value:     r.Count,
			Label:     r.Hour.Format("15:04"),
		})
	}

	return points, nil
}

// GetNoisyDevices returns devices with high alert counts
func (r *AlertRepository) GetNoisyDevices(limit int) ([]models.NoisyDevice, error) {
	if limit <= 0 {
		limit = 10
	}

	var results []struct {
		Device     string
		AlertCount int
		TopTitle   string
	}

	// Get devices with most alerts
	subQuery := r.db.Model(&models.Alert{}).
		Select("device, COUNT(*) as alert_count").
		Group("device").
		Order("alert_count DESC").
		Limit(limit)

	if err := r.db.Table("(?) as device_counts", subQuery).
		Select("device_counts.device, device_counts.alert_count").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get noisy devices: %w", err)
	}

	var noisyDevices []models.NoisyDevice
	for _, res := range results {
		// Get top issue for this device
		var topAlert models.Alert
		r.db.Model(&models.Alert{}).
			Where("device = ?", res.Device).
			Order("timestamp DESC").
			First(&topAlert)

		noisyDevices = append(noisyDevices, models.NoisyDevice{
			DeviceID:   res.Device,
			DeviceName: res.Device,
			AlertCount: res.AlertCount,
			TopIssue:   topAlert.Title,
		})
	}

	return noisyDevices, nil
}

// GenerateAlertID generates a unique alert ID
func (r *AlertRepository) GenerateAlertID() (string, error) {
	var count int64
	if err := r.db.Model(&models.Alert{}).Count(&count).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("ALT-%06d", count+1), nil
}
