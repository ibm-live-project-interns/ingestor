package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// Notification Channels
// ==========================================

func GetChannels(c *gin.Context) {
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusOK, []models.NotificationChannel{})
		return
	}

	pg, paginated := parseConfigPagination(c)
	if !paginated {
		channels, err := repo.ListChannels()
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
		c.JSON(http.StatusOK, channels)
		return
	}

	channels, total, err := repo.ListChannelsPaginated(pg)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  channels,
		"total": total,
	})
}

func GetChannelByID(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		apiErr := errors.NewNotFound("channel " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	ch, err := repo.GetChannelByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if ch == nil {
		apiErr := errors.NewNotFound("channel " + id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, ch)
}

func CreateChannel(c *gin.Context) {
	var req models.CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate name is non-empty (defense-in-depth, binding:"required" should catch this)
	if strings.TrimSpace(req.Name) == "" {
		apiErr := errors.NewValidation("Channel name is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate channel type is one of the allowed values
	if !validChannelTypes[req.Type] {
		apiErr := errors.NewValidation("Channel type must be one of: email, slack, webhook, pagerduty, Twilio")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	id, _ := repo.GenerateChannelID()
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	ch := models.NotificationChannel{
		ID:     id,
		Name:   req.Name,
		Type:   req.Type,
		Meta:   req.Meta,
		Active: active,
	}
	if err := repo.CreateChannel(&ch); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	logger.Info("Notification channel %s created", id)
	c.JSON(http.StatusCreated, gin.H{"message": "Channel created", "channel": ch})
}

func UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var req models.UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Meta != "" {
		updates["meta"] = req.Meta
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fields to update"})
		return
	}
	if err := repo.UpdateChannel(id, updates); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	updated, _ := repo.GetChannelByID(id)
	c.JSON(http.StatusOK, gin.H{"message": "Channel updated", "channel": updated})
}

func DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	repo := configRepo()
	if repo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}
	if err := repo.DeleteChannel(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Channel deleted", "id": id})
}
