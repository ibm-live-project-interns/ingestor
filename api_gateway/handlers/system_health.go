package handlers

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// startTime tracks when the API gateway was started, used for uptime calculation.
var startTime = time.Now()

// GetSystemHealth returns detailed system health information.
// Only accessible by sysadmin role (enforced via route middleware).
// GET /api/v1/system/health
func GetSystemHealth(c *gin.Context) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	health := gin.H{
		"status":         "healthy",
		"uptime_seconds": time.Since(startTime).Seconds(),
		"go_version":     runtime.Version(),
		"goroutines":     runtime.NumGoroutine(),
		"num_cpu":        runtime.NumCPU(),
		"memory": gin.H{
			"alloc_mb":       memStats.Alloc / 1024 / 1024,
			"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
			"sys_mb":         memStats.Sys / 1024 / 1024,
			"num_gc":         memStats.NumGC,
			"heap_objects":   memStats.HeapObjects,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// If DB is available, add connection pool stats
	db := database.Get()
	if db != nil && db.DB != nil {
		sqlDB, err := db.DB.DB()
		if err == nil {
			stats := sqlDB.Stats()
			health["database"] = gin.H{
				"status":           "connected",
				"open_connections": stats.OpenConnections,
				"in_use":           stats.InUse,
				"idle":             stats.Idle,
				"max_open":         stats.MaxOpenConnections,
				"wait_count":       stats.WaitCount,
				"wait_duration_ms": stats.WaitDuration.Milliseconds(),
			}
		} else {
			health["database"] = gin.H{
				"status": "error",
				"error":  err.Error(),
			}
		}
	} else {
		health["database"] = gin.H{
			"status": "disconnected",
		}
		health["status"] = "degraded"
	}

	logger.Info("System health check performed: goroutines=%d, alloc_mb=%d",
		runtime.NumGoroutine(), memStats.Alloc/1024/1024)

	c.JSON(http.StatusOK, health)
}
