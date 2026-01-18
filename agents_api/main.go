package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ Failed to load .env file")
	}
	log.Println("✅ .env loaded successfully")

	// Initialize Gin router
	router := gin.Default()

	// Core Agents API endpoint
	router.POST("/events", func(c *gin.Context) {
		var evt Event

		// Validate incoming event
		if err := c.ShouldBindJSON(&evt); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		// Dispatch event to AI processing pipeline
		result := DispatchEvent(evt)

		// Return unified response
		c.JSON(http.StatusOK, result)
	})

	log.Println("🚀 Agents API running on :9000")
	if err := router.Run(":9000"); err != nil {
		log.Fatal("❌ Failed to start Agents API:", err)
	}
}
