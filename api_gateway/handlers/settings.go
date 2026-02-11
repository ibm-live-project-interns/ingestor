package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// NotificationPreferences represents user notification settings
type NotificationPreferences struct {
	EmailAlerts       bool `json:"emailAlerts"`
	PushNotifications bool `json:"pushNotifications"`
	SoundEnabled      bool `json:"soundEnabled"`
	CriticalOnly      bool `json:"criticalOnly"`
}

// GetNotificationPreferences returns the current user's notification settings
func GetNotificationPreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	db := database.Get()
	if db == nil {
		// No DB — return defaults
		c.JSON(http.StatusOK, NotificationPreferences{
			EmailAlerts:       true,
			PushNotifications: true,
			SoundEnabled:      false,
			CriticalOnly:      false,
		})
		return
	}

	var user models.User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, NotificationPreferences{
		EmailAlerts:       user.EmailAlerts,
		PushNotifications: user.PushNotifications,
		SoundEnabled:      user.SoundEnabled,
		CriticalOnly:      user.CriticalOnly,
	})
}

// UpdateNotificationPreferences updates the current user's notification settings
func UpdateNotificationPreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req NotificationPreferences
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := database.Get()
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Database not available"})
		return
	}

	result := db.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"email_alerts":       req.EmailAlerts,
		"push_notifications": req.PushNotifications,
		"sound_enabled":      req.SoundEnabled,
		"critical_only":      req.CriticalOnly,
	})
	if result.Error != nil {
		logger.Error("Failed to update notification preferences for user %v: %v", userID, result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update preferences"})
		return
	}

	logger.Info("Updated notification preferences for user %v: email=%v push=%v sound=%v critical=%v",
		userID, req.EmailAlerts, req.PushNotifications, req.SoundEnabled, req.CriticalOnly)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Notification preferences updated",
		"preferences": req,
	})
}
