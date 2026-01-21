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
	"github.com/ibm-live-project-interns/ingestor/shared/config"
)

type Event struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	SourceHost string `json:"source_host,omitempty"`
	SourceIP   string `json:"source_ip,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	Category   string `json:"category,omitempty"`
}

func loadConfig() map[string]string {
	configPath := config.GetEnv("EVENT_ROUTER_CONFIG_PATH", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", configPath, err)
	}

	config := make(map[string]string)
	json.Unmarshal(data, &config)
	return config
}

func forwardEvent(url string, event Event) (string, error) {
	body, _ := json.Marshal(event)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	return string(respBody), nil
}

func main() {
	port := config.GetEnv("EVENT_ROUTER_PORT", "8082")

	router := gin.Default()
	config := loadConfig()
		initKafka()
	defer kafkaProducer.Close()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "event-router"})
	})

	router.POST("/route", func(c *gin.Context) {
		var evt Event

		if err := c.ShouldBindJSON(&evt); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
			if err := publishToKafka(evt); err != nil {
		c.JSON(500, gin.H{"error": "failed to publish event to kafka"})
		return
	}

		destURL, ok := config[evt.Type]
		if !ok {
			c.JSON(400, gin.H{
				"error": fmt.Sprintf("No route configured for event type: %s", evt.Type),
			})
			return
		}

		response, err := forwardEvent(destURL, evt)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, gin.H{
			"status":           "forwarded",
			"forwarded_to":     destURL,
			"downstream_reply": response,
		})
	})

	log.Printf("🌐 Event Router running on :%s\n", port)
	router.Run(":" + port)
}
