package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// Event represents an incoming event to be processed by AI
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// EventResult represents the AI processing result
type EventResult struct {
	EventID     string   `json:"eventId"`
	Status      string   `json:"status"`
	AITitle     string   `json:"aiTitle"`
	AISummary   string   `json:"aiSummary"`
	Confidence  int      `json:"confidence"`
	RootCauses  []string `json:"rootCauses"`
	Actions     []string `json:"recommendedActions"`
	ProcessedAt string   `json:"processedAt"`
}

// DispatchEvent processes an event through AI analysis
func DispatchEvent(evt Event) EventResult {
	// In production, this would call IBM watsonx AI
	// For now, return mock AI analysis
	return EventResult{
		EventID:    evt.ID,
		Status:     "processed",
		AITitle:    "AI-analyzed: " + evt.Type,
		AISummary:  "Event processed by AI agents. Source: " + evt.Source,
		Confidence: 85,
		RootCauses: []string{
			"Potential network connectivity issue",
			"Configuration change detected",
		},
		Actions: []string{
			"Monitor for recurring events",
			"Review recent configuration changes",
		},
		ProcessedAt: time.Now().Format(time.RFC3339),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	port := getEnv("AGENTS_API_PORT", "9000")

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "agents-api"})
	})

	router.POST("/events", func(c *gin.Context) {
		var evt Event

		if err := c.ShouldBindJSON(&evt); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := DispatchEvent(evt)
		c.JSON(http.StatusOK, result)
	})

	log.Printf("🤖 Agents API running on :%s\n", port)
	router.Run(":" + port)
}
