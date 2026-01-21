package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/models"

	"ingestor/ingestor_core/normalizer"
	"ingestor/ingestor_core/validator"
	"ingestor/ingestor_core/forwarder"
)

func main() {
	port := config.GetEnv("INGESTOR_CORE_PORT", "8001")
	eventRouterURL := config.GetEnv("EVENT_ROUTER_URL", "http://event-router:8082")

	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Main ingestion endpoint
	router.POST("/ingest/event", func(c *gin.Context) {
		var raw map[string]interface{}

		if err := c.ShouldBindJSON(&raw); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
			return
		}

		// Normalize
		event := normalizer.Normalize(raw)

		// Validate
		if err := validator.Validate(event); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Forward
		if err := forwarder.Send(event, eventRouterURL); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":   "accepted",
			"event_id": event.ID,
			"type":     event.EventType,
			"severity": event.Severity,
		})
	})

	log.Printf("🚀 Ingestor Core running on :%s", port)
	log.Fatal(router.Run(":" + port))
}
