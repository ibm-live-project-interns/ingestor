package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	 "github.com/ibm-live-project-interns/ingestor/shared/models"
	"github.com/gin-gonic/gin"
	"github.com/ibm-live-project-interns/ingestor/shared/config"
)



func loadConfig() map[string]string {
	configPath := config.GetEnv("EVENT_ROUTER_CONFIG_PATH", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", configPath, err)
	}

	cfg := make(map[string]string)
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Error parsing config file %s: %v", configPath, err)
	}
	return cfg
}

func forwardEvent(url string, event models.RoutedEvent) (string, error) {
	body, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("downstream returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

func main() {
	port := config.GetEnv("EVENT_ROUTER_PORT", "8082")

	router := gin.Default()
	routeConfig := loadConfig()
	log.Printf("[event-router] Loaded %d routing rules", len(routeConfig))
	for severity, url := range routeConfig {
		log.Printf("[event-router] Route: severity=%s → %s", severity, url)
	}

	initKafka()
	defer kafkaProducer.Close()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "event-router"})
	})

	router.POST("/route", func(c *gin.Context) {
		var evt models.RoutedEvent

		if err := c.ShouldBindJSON(&evt); err != nil {
			log.Printf("[event-router] ERROR: invalid event payload: %v", err)
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[event-router] Received event: type=%s severity=%s source=%s", evt.EventType, evt.Type, evt.SourceHost)

		if err := publishToKafka(evt); err != nil {
			log.Printf("[event-router] ERROR: failed to publish to Kafka: %v", err)
			c.JSON(500, gin.H{"error": "failed to publish event to kafka"})
			return
		}

		if evt.Type == "" {
			log.Printf("[event-router] WARN: event has empty severity (Type field), cannot route")
			c.JSON(400, gin.H{"error": "event severity (type) is required for routing"})
			return
		}

		destURL, ok := routeConfig[evt.Type]
		if !ok {
			log.Printf("[event-router] WARN: no route for severity=%s, available routes: %v", evt.Type, routeConfig)
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("No route configured for severity: %s", evt.Type),
			})
			return
		}

		log.Printf("[event-router] Forwarding event to %s (severity=%s)", destURL, evt.Type)
		response, err := forwardEvent(destURL, evt)
		if err != nil {
			log.Printf("[event-router] ERROR: forwarding failed: %v", err)
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		log.Printf("[event-router] Successfully forwarded event (severity=%s) to %s", evt.Type, destURL)
		c.JSON(200, gin.H{
			"status":           "forwarded",
			"forwarded_to":     destURL,
			"downstream_reply": response,
		})
	})

	log.Printf("🌐 Event Router running on :%s\n", port)
	router.Run(":" + port)
}
