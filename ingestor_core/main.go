package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Metadata is what the datasource / upstream sends to Ingestor Core.
// You can extend this later as needed.
type Metadata struct {
	Router string `json:"router"`         // e.g. router id / name
	Note   string `json:"note"`           // description / message
	Type   string `json:"type,omitempty"` // optional, falls back to "info"
}

// RoutedEvent is what we actually send to the Event Router.
// It matches the Event struct expected by event_router:
//
//	type + message
type RoutedEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// forwardToRouter takes incoming metadata, converts it to a RoutedEvent,
// and forwards it to the Event Router on :8081.
func forwardToRouter(meta Metadata) (string, error) {
	// Map metadata to the event format expected by Event Router.
	eventType := meta.Type
	if eventType == "" {
		// Default to "info" if not provided
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

	url := "http://localhost:8081/route"

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
	router := gin.Default()

	// Ingestor Core metadata endpoint
	router.POST("/ingest/metadata", func(c *gin.Context) {
		var meta Metadata

		// Bind incoming JSON to Metadata struct
		if err := c.ShouldBindJSON(&meta); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("invalid payload: %v", err),
			})
			return
		}

		// Forward to Event Router
		routerResp, err := forwardToRouter(meta)
		if err != nil {
			log.Println("Error forwarding to Event Router:", err)
			c.JSON(http.StatusBadGateway, gin.H{
				"status": "router_unreachable",
				"error":  err.Error(),
			})
			return
		}

		// Successful flow
		c.JSON(http.StatusOK, gin.H{
			"status":          "received",
			"forwarded_to":    "event_router",
			"router_response": routerResp,
		})
	})

	log.Println("🌟 Ingestor Core running on :8001")
	if err := router.Run(":8001"); err != nil {
		log.Fatal("failed to start Ingestor Core:", err)
	}
}
