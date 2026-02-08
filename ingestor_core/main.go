package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"

	"github.com/ibm-live-project-interns/ingestor/ingestor_core/normalizer"
	"github.com/ibm-live-project-interns/ingestor/ingestor_core/validator"
	"github.com/ibm-live-project-interns/ingestor/ingestor_core/enricher"
	"github.com/ibm-live-project-interns/ingestor/ingestor_core/forwarder"
	"github.com/ibm-live-project-interns/ingestor/ingestor_core/health"
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
			log.Printf("[ingestor-core] ERROR: invalid JSON payload: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid JSON payload",
				"detail": err.Error(),
			})
			return
		}

		log.Printf("[ingestor-core] Received event: event_type=%v source_host=%v severity=%v", raw["event_type"], raw["source_host"], raw["severity"])

		// 2. Normalize
		event := normalizer.Normalize(raw)
		log.Printf("[ingestor-core] Normalized: type=%s severity=%s source=%s ip=%s", event.EventType, event.Severity, event.SourceHost, event.SourceIP)

		// 3. Validate
		if err := validator.ValidateEvent(event); err != nil {
			log.Printf("[ingestor-core] VALIDATION FAILED: %v (event_type=%s severity=%s source=%s)", err, event.EventType, event.Severity, event.SourceHost)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		log.Printf("[ingestor-core] Validation passed for event from %s", event.SourceHost)

		// 4. Enrich
		event = enricher.Enrich(event)

		// 5. Forward to Event Router
		routedEvent := event.ToRoutedEvent()
		log.Printf("[ingestor-core] Forwarding to Event Router: type(severity)=%s event_type=%s source=%s", routedEvent.Type, routedEvent.EventType, routedEvent.SourceHost)

		resp, err := forwarder.Forward(routedEvent, eventRouterURL)
		if err != nil {
			log.Printf("[ingestor-core] ERROR: forwarding to Event Router failed: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "failed to forward event to router",
				"detail": err.Error(),
			})
			return
		}

		// 6. Success
		log.Printf("[ingestor-core] Event ingested and forwarded successfully: type=%s severity=%s", event.EventType, event.Severity)
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
