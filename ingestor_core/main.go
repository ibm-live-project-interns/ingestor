package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"

	"ingestor/ingestor_core/normalizer"
	"ingestor/ingestor_core/validator"
	"ingestor/ingestor_core/enricher"
	"ingestor/ingestor_core/forwarder"
)

func main() {
	port := config.GetEnv("INGESTOR_CORE_PORT", "8001")
	eventRouterURL := config.GetEnv("EVENT_ROUTER_URL", "http://event-router:8082")

	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
	status := "healthy"

	// Check Event Router connectivity
	_, err := http.Get(eventRouterURL + "/health")
	if err != nil {
		status = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"service": "ingestor-core",
		"status":  status,
	})
})
	router.GET("/ready", func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "ingestor-core",
		"ready":   true,
	})
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

		// 2. Normalize (raw → models.Event)
		event := normalizer.Normalize(raw)

		// 3. Validate normalized event
		if err := validator.ValidateEvent(event); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 4. Enrich event (system metadata)
		event = enricher.Enrich(event)

		// 5. Forward to Event Router (as RoutedEvent)
		resp, err := forwarder.Forward(event.ToRoutedEvent(), eventRouterURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 6. Success response
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
