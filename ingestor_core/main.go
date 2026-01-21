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
	"ingestor/ingestor_core/health"
)

func main() {
	port := config.GetEnv("INGESTOR_CORE_PORT", "8001")
	eventRouterURL := config.GetEnv("EVENT_ROUTER_URL", "http://event-router:8082")

	router := gin.Default()

	// ✅ Health check (Ticket #5)
	router.GET("/health", func(c *gin.Context) {
		routerHealth := health.CheckHTTPHealth(eventRouterURL)

		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "ingestor-core",
			"dependencies": gin.H{
				"event_router": routerHealth,
			},
		})
	})

	// ✅ Main ingestion endpoint
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

		// 4. Enrich
		event = enricher.Enrich(event)

		// 5. Forward
		resp, err := forwarder.Forward(event.ToRoutedEvent(), eventRouterURL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": err.Error(),
			})
			return
		}

		// 6. Success
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
