package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// On-Call Schedule CRUD Handlers
// ==========================================

// GetOnCallSchedules returns all on-call schedules (paginated)
// GET /api/v1/on-call/schedules
func GetOnCallSchedules(c *gin.Context) {
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: returning demo on-call schedules")
			c.JSON(http.StatusOK, gin.H{
				"schedules": getDemoOnCallSchedules(),
				"total":     5,
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	// Parse pagination
	limit := 25
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var schedules []models.OnCallSchedule
	var total int64

	query := db.Model(&models.OnCallSchedule{})

	// Apply optional filters
	if rotationType := c.Query("rotation_type"); rotationType != "" {
		query = query.Where("rotation_type = ?", rotationType)
	}
	if username := c.Query("username"); username != "" {
		query = query.Where("username ILIKE ?", "%"+database.EscapeLike(username)+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		apiErr := errors.NewDatabaseError("count", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := query.Order("start_time DESC").Limit(limit).Offset(offset).Find(&schedules).Error; err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"schedules": schedules,
		"total":     total,
	})
}

// CreateOnCallScheduleRequest is the expected payload for creating a schedule
type CreateOnCallScheduleRequest struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username" binding:"required"`
	StartTime    string `json:"start_time" binding:"required"`
	EndTime      string `json:"end_time" binding:"required"`
	RotationType string `json:"rotation_type"`
	IsPrimary    bool   `json:"is_primary"`
}

// CreateOnCallSchedule creates a new on-call schedule
// POST /api/v1/on-call/schedules
func CreateOnCallSchedule(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: on-call schedule creation simulated")
			c.JSON(http.StatusCreated, gin.H{
				"message": "On-call schedule created (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	var req CreateOnCallScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate username is not empty
	if strings.TrimSpace(req.Username) == "" {
		apiErr := errors.NewValidation("Username is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid start_time format: use RFC3339 (e.g., 2026-01-15T08:00:00Z)")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid end_time format: use RFC3339 (e.g., 2026-01-15T20:00:00Z)")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate time range
	if !endTime.After(startTime) {
		apiErr := errors.NewValidation("end_time must be after start_time")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Default rotation type
	rotationType := req.RotationType
	if rotationType == "" {
		rotationType = "weekly"
	}
	validRotations := map[string]bool{"weekly": true, "daily": true, "biweekly": true, "monthly": true}
	if !validRotations[rotationType] {
		apiErr := errors.NewValidation("rotation_type must be one of: weekly, daily, biweekly, monthly")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get creator from context
	username, _ := c.Get("username")
	createdBy, _ := username.(string)
	if createdBy == "" {
		createdBy = "system"
	}

	schedule := models.OnCallSchedule{
		UserID:       req.UserID,
		Username:     strings.TrimSpace(req.Username),
		StartTime:    startTime,
		EndTime:      endTime,
		RotationType: rotationType,
		IsPrimary:    req.IsPrimary,
		CreatedBy:    createdBy,
	}

	if err := db.Create(&schedule).Error; err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("On-call schedule %d created by %s for %s", schedule.ID, createdBy, schedule.Username)

	// Send on-call rotation reminder email to assignee (non-blocking)
	if services.Email != nil && req.UserID > 0 {
		sched := schedule
		assigneeID := req.UserID
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = ctx
			var assignee models.User
			if err := db.First(&assignee, assigneeID).Error; err != nil {
				logger.Error("Failed to look up on-call assignee %d: %v", assigneeID, err)
				return
			}
			custom := map[string]interface{}{
				"UserName":  sched.Username,
				"StartTime": sched.StartTime.Format(time.RFC3339),
				"EndTime":   sched.EndTime.Format(time.RFC3339),
				"Rotation":  sched.RotationType,
			}
			if err := services.Email.SendNotification(assignee.Email, assignee.Username, "On-Call Schedule Updated", "oncall-rotation-reminder", custom); err != nil {
				logger.Error("Failed to send oncall-rotation-reminder email: %v", err)
			}
		}()
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "On-call schedule created successfully",
		"schedule": schedule,
	})
}

// UpdateOnCallSchedule updates an existing on-call schedule
// PUT /api/v1/on-call/schedules/:id
func UpdateOnCallSchedule(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: on-call schedule update simulated")
			c.JSON(http.StatusOK, gin.H{
				"message": "On-call schedule updated (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid schedule ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if schedule exists
	var schedule models.OnCallSchedule
	if err := db.First(&schedule, id).Error; err != nil {
		apiErr := errors.NewNotFound("on-call schedule")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req CreateOnCallScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Build updates map
	updates := make(map[string]interface{})

	if strings.TrimSpace(req.Username) != "" {
		updates["username"] = strings.TrimSpace(req.Username)
	}
	if req.UserID > 0 {
		updates["user_id"] = req.UserID
	}
	if req.StartTime != "" {
		startTime, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			apiErr := errors.NewBadRequest("Invalid start_time format: use RFC3339")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		updates["start_time"] = startTime
	}
	if req.EndTime != "" {
		endTime, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			apiErr := errors.NewBadRequest("Invalid end_time format: use RFC3339")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		updates["end_time"] = endTime
	}
	if req.RotationType != "" {
		validRotations := map[string]bool{"weekly": true, "daily": true, "biweekly": true, "monthly": true}
		if !validRotations[req.RotationType] {
			apiErr := errors.NewValidation("rotation_type must be one of: weekly, daily, biweekly, monthly")
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		updates["rotation_type"] = req.RotationType
	}
	updates["is_primary"] = req.IsPrimary
	updates["updated_at"] = time.Now().UTC()

	if err := db.Model(&schedule).Updates(updates).Error; err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Fetch updated schedule
	db.First(&schedule, id)

	logger.Info("On-call schedule %d updated", id)

	c.JSON(http.StatusOK, gin.H{
		"message":  "On-call schedule updated successfully",
		"schedule": schedule,
	})
}

// DeleteOnCallSchedule deletes an on-call schedule
// DELETE /api/v1/on-call/schedules/:id
func DeleteOnCallSchedule(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: on-call schedule deletion simulated")
			c.JSON(http.StatusOK, gin.H{
				"message": "On-call schedule deleted (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid schedule ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if schedule exists
	var schedule models.OnCallSchedule
	if err := db.First(&schedule, id).Error; err != nil {
		apiErr := errors.NewNotFound("on-call schedule")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Delete associated overrides first (cascade), then the schedule
	if err := db.Where("schedule_id = ?", id).Delete(&models.OnCallOverride{}).Error; err != nil {
		logger.Warn("Failed to delete overrides for schedule %d: %v", id, err)
	}
	if err := db.Delete(&schedule).Error; err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("On-call schedule %d deleted", id)

	c.JSON(http.StatusOK, gin.H{
		"message": "On-call schedule deleted successfully",
	})
}

// CreateOnCallOverrideRequest is the expected payload for creating an override
type CreateOnCallOverrideRequest struct {
	ScheduleID     uint   `json:"schedule_id" binding:"required"`
	OriginalUserID uint   `json:"original_user_id"`
	OverrideUserID uint   `json:"override_user_id"`
	StartTime      string `json:"start_time" binding:"required"`
	EndTime        string `json:"end_time" binding:"required"`
	Reason         string `json:"reason"`
}

// CreateOnCallOverride creates a new on-call override
// POST /api/v1/on-call/overrides
func CreateOnCallOverride(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: on-call override creation simulated")
			c.JSON(http.StatusCreated, gin.H{
				"message": "On-call override created (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	var req CreateOnCallOverrideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid start_time format: use RFC3339")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid end_time format: use RFC3339")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if !endTime.After(startTime) {
		apiErr := errors.NewValidation("end_time must be after start_time")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Verify the schedule exists
	var schedule models.OnCallSchedule
	if err := db.First(&schedule, req.ScheduleID).Error; err != nil {
		apiErr := errors.NewNotFound("on-call schedule")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get creator from context
	username, _ := c.Get("username")
	createdBy, _ := username.(string)
	if createdBy == "" {
		createdBy = "system"
	}

	override := models.OnCallOverride{
		ScheduleID:     req.ScheduleID,
		OriginalUserID: req.OriginalUserID,
		OverrideUserID: req.OverrideUserID,
		StartTime:      startTime,
		EndTime:        endTime,
		Reason:         strings.TrimSpace(req.Reason),
		CreatedBy:      createdBy,
	}

	if err := db.Create(&override).Error; err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("On-call override %d created by %s for schedule %d", override.ID, createdBy, req.ScheduleID)

	c.JSON(http.StatusCreated, gin.H{
		"message":  "On-call override created successfully",
		"override": override,
	})
}

// DeleteOnCallOverride deletes an on-call override
// DELETE /api/v1/on-call/overrides/:id
func DeleteOnCallOverride(c *gin.Context) {
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
	db := database.Get()
	if db == nil || db.DB == nil {
		if isDemoMode() {
			logger.Info("Demo mode: on-call override deletion simulated")
			c.JSON(http.StatusOK, gin.H{
				"message": "On-call override deleted (demo mode)",
			})
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service temporarily unavailable"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid override ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Check if override exists
	var override models.OnCallOverride
	if err := db.First(&override, id).Error; err != nil {
		apiErr := errors.NewNotFound("on-call override")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := db.Delete(&override).Error; err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("On-call override %d deleted", id)

	c.JSON(http.StatusOK, gin.H{
		"message": "On-call override deleted successfully",
	})
}

// ==========================================
// Demo Data for On-Call CRUD
// ==========================================

// getDemoOnCallSchedules returns demo schedule entries for the current week
func getDemoOnCallSchedules() []models.OnCallSchedule {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))

	return []models.OnCallSchedule{
		{
			ID: 1, UserID: 2, Username: "John Smith",
			StartTime: weekStart, EndTime: weekStart.Add(7 * 24 * time.Hour),
			RotationType: "weekly", IsPrimary: true, CreatedBy: "admin",
			CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour),
		},
		{
			ID: 2, UserID: 3, Username: "Jane Doe",
			StartTime: weekStart.Add(7 * 24 * time.Hour), EndTime: weekStart.Add(14 * 24 * time.Hour),
			RotationType: "weekly", IsPrimary: true, CreatedBy: "admin",
			CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour),
		},
		{
			ID: 3, UserID: 5, Username: "Carlos Rivera",
			StartTime: weekStart.Add(14 * 24 * time.Hour), EndTime: weekStart.Add(21 * 24 * time.Hour),
			RotationType: "weekly", IsPrimary: true, CreatedBy: "admin",
			CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour),
		},
		{
			ID: 4, UserID: 4, Username: "Sarah Chen",
			StartTime: weekStart, EndTime: weekStart.Add(7 * 24 * time.Hour),
			RotationType: "weekly", IsPrimary: false, CreatedBy: "admin",
			CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour),
		},
		{
			ID: 5, UserID: 6, Username: fmt.Sprintf("Marcus Johnson"),
			StartTime: weekStart.Add(7 * 24 * time.Hour), EndTime: weekStart.Add(14 * 24 * time.Hour),
			RotationType: "weekly", IsPrimary: false, CreatedBy: "admin",
			CreatedAt: now.Add(-30 * 24 * time.Hour), UpdatedAt: now.Add(-30 * 24 * time.Hour),
		},
	}
}
