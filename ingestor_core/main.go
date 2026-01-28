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

	// 1. Parse raw JSON
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid JSON payload",
		})
		return
	}

	// 2. Normalize
	event := normalizer.Normalize(raw)

	// 3. Validate
	if err := validator.ValidateEvent(event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 4. Forward to Event Router
	resp, err := forwarder.Forward(event, eventRouterURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 5. Success
	c.JSON(http.StatusOK, gin.H{
		"status":       "ingested",
		"event_type":   event.EventType,
		"severity":     event.Severity,
		"router_reply": resp,
	})
})

	log.Printf("🚀 Ingestor Core running on :%s", port)
	log.Fatal(router.Run(":" + port))
}
