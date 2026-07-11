package handlers

import (
	"fmt"
	"net/http"
	"time"

	"api_gateway/services"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// SendAllTestEmails sends all 34 email templates to a given address for testing
func SendAllTestEmails(c *gin.Context) {
	toEmail := c.Query("email")
	if toEmail == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required 'email' query parameter"})
		return
	}

	if services.Email == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Email service not initialized"})
		return
	}

	tests := buildEmailTestCases(toEmail)

	results := make([]gin.H, 0, len(tests))
	successCount := 0
	failCount := 0

	// Send each email with a small delay to avoid rate limiting
	for i, test := range tests {
		logger.Info("Sending test email %d/%d: %s to %s", i+1, len(tests), test.Name, toEmail)
		err := test.Send()
		result := gin.H{
			"template": test.Name,
			"subject":  test.Subject,
		}
		if err != nil {
			result["status"] = "failed"
			result["error"] = err.Error()
			failCount++
			logger.Error("Failed to send %s: %v", test.Name, err)
		} else {
			result["status"] = "sent"
			successCount++
		}
		results = append(results, result)

		// Small delay between emails to avoid SMTP rate limits
		if i < len(tests)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Sent %d/%d emails to %s", successCount, len(tests), toEmail),
		"success": successCount,
		"failed":  failCount,
		"total":   len(tests),
		"to":      toEmail,
		"results": results,
	})
}
