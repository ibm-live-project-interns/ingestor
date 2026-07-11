package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(); err != nil {
		log.Fatal("Failed to load .env file")
	}
	log.Println(".env loaded successfully")

	// Initialize CVE/RAG pipeline (non-fatal)
	if err := EnsureRecentNetworkCVEs(); err != nil {
		log.Printf("CVE initialization failed: %v. RAG context will be unavailable.", err)
	} else {
		log.Printf("CVE/RAG pipeline initialized with %d CVEs", len(GetRecentCVEs()))
	}

	// Background CVE cache refresh every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := EnsureRecentNetworkCVEs(); err != nil {
				log.Printf("CVE background refresh failed: %v", err)
			}
		}
	}()

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

	// Health check
	router.GET("/health", func(c *gin.Context) {
		cveCount := len(GetRecentCVEs())
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"service":   "agents-api",
			"cve_count": cveCount,
			"rag":       cveCount > 0,
		})
	})

	log.Println("Agents API running on :9000")
	if err := router.Run(":9000"); err != nil {
		log.Fatal("Failed to start Agents API:", err)
	}
}
