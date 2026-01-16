package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// forwardToRouter takes an Event, converts it to a RoutedEvent, and forwards to Event Router
func forwardToRouter(event models.Event, eventRouterURL string) (string, error) {
	// Use the shared model's ToRoutedEvent method
	routedEvent := event.ToRoutedEvent()
	// Override message to include more context
	routedEvent.Message = fmt.Sprintf("[%s] %s: %s", event.EventType, event.SourceHost, event.Message)

	payload, err := json.Marshal(routedEvent)
	if err != nil {
		return "", fmt.Errorf("failed to marshal routed event: %w", err)
	}

	url := eventRouterURL + "/route"
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("failed to post to event router: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read router response: %w", err)
	}

	return string(bodyBytes), nil
}

func main() {
	port := config.GetEnv("INGESTOR_CORE_PORT", "8001")
	eventRouterURL := config.GetEnv("EVENT_ROUTER_URL", "http://localhost:8082")

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "ingestor-core"})
	})

	// Main event ingestion endpoint
	router.POST("/ingest/event", func(c *gin.Context) {
		var event models.Event

		if err := c.ShouldBindJSON(&event); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid payload: %v", err),
			})
			return
		}

		if err := event.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("validation failed: %v", err),
			})
			return
		}

		routerResp, err := forwardToRouter(event, eventRouterURL)
		if err != nil {
			log.Println("Error forwarding to Event Router:", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"status": "router_unreachable",
				"error":  err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":          "received",
			"event_type":      event.EventType,
			"severity":        event.Severity,
			"forwarded_to":    "event_router",
			"router_response": routerResp,
		})
	})

	// LEGACY: Keep /ingest/metadata for backwards compatibility (deprecated)
	router.POST("/ingest/metadata", func(c *gin.Context) {
		log.Println("Warning: /ingest/metadata is deprecated, use /ingest/event instead")

		type Metadata struct {
			Router string `json:"router"`
			Note   string `json:"note"`
			Type   string `json:"type,omitempty"`
		}

		var meta Metadata
		if err := c.ShouldBindJSON(&meta); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid payload: %v", err),
			})
			return
		}

		eventType := meta.Type
		if eventType == "" {
			eventType = "info"
		}

		routedEvent := models.RoutedEvent{
			Type:    eventType,
			Message: meta.Note,
		}

		payload, _ := json.Marshal(routedEvent)
		url := eventRouterURL + "/route"
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
		if err != nil {
			log.Println("Error forwarding to Event Router:", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"status": "router_unreachable",
				"error":  err.Error(),
			})
			return
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusOK, gin.H{
			"status":          "received",
			"forwarded_to":    "event_router",
			"router_response": string(bodyBytes),
		})
	})

	log.Printf("Ingestor Core starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
