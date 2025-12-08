package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

type Event struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func loadConfig() map[string]string {
	data, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Error reading config.json: %v", err)
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

	respBody, _ := ioutil.ReadAll(resp.Body)

	return string(respBody), nil
}

func main() {
	router := gin.Default()
	config := loadConfig()

	router.POST("/route", func(c *gin.Context) {
		var evt Event

		if err := c.ShouldBindJSON(&evt); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
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
			"status":            "forwarded",
			"forwarded_to":      destURL,
			"downstream_reply":  response,
		})
	})

	fmt.Println("🌐 Event Router running on :8081")
	router.Run(":8081")
}
