package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// Metadata is what the datasource / upstream sends to Ingestor Core.
type Metadata struct {
	Router string `json:"router"`         // e.g. router id / name
	Note   string `json:"note"`           // description / message
	Type   string `json:"type,omitempty"` // optional, falls back to "info"
}

// RoutedEvent is what we actually send to the Event Router.
type RoutedEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// forwardToRouter takes incoming metadata, converts it to a RoutedEvent,
// and forwards it to the Event Router.
func forwardToRouter(meta Metadata, eventRouterURL string) (string, error) {
	eventType := meta.Type
	if eventType == "" {
		eventType = "info"
	}

	event := RoutedEvent{
		Type:    eventType,
		Message: meta.Note,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	url := eventRouterURL + "/route"

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", fmt.Errorf("error calling event router: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read router response: %w", err)
	}

	return string(bodyBytes), nil
}

func main() {
	port := getEnv("INGESTOR_CORE_PORT", "8001")
	eventRouterURL := getEnv("EVENT_ROUTER_URL", "http://localhost:8082")

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "ingestor-core"})
	})

	// Ingestor Core metadata endpoint
	router.POST("/ingest/metadata", func(c *gin.Context) {
		var meta Metadata

		if err := c.ShouldBindJSON(&meta); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid payload: %v", err),
			})
			return
		}

		routerResp, err := forwardToRouter(meta, eventRouterURL)
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
			"forwarded_to":    "event_router",
			"router_response": routerResp,
		})
	})

	log.Printf("🌟 Ingestor Core running on :%s (Event Router: %s)\n", port, eventRouterURL)
	if err := router.Run(":" + port); err != nil {
		log.Fatal("failed to start Ingestor Core:", err)
	}
}
