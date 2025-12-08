package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()

	router.POST("/events", func(c *gin.Context) {
		var evt Event

		if err := c.ShouldBindJSON(&evt); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := DispatchEvent(evt)
		c.JSON(http.StatusOK, result)
	})

	router.Run(":9000")
}
