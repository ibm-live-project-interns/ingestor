package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ticketRepo returns the ticket repository using the global database
// Returns nil if database is not available (demo mode)
func ticketRepo() *database.TicketRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewTicketRepository(db.DB)
}

// getDemoTickets returns demo tickets for when database is unavailable
func getDemoTickets() []models.Ticket {
	now := time.Now()
	alertID1 := "ALT-001"
	alertID3 := "ALT-003"
	alertID5 := "ALT-005"
	return []models.Ticket{
		{
			ID:          "TKT-001",
			Title:       "Network Latency Issue - Core Router",
			Description: "High latency detected on core router affecting multiple segments",
			Priority:    "high",
			Status:      models.TicketStatusOpen,
			Category:    "Network",
			Assignee:    "John Smith",
			Reporter:    "System",
			AlertID:     &alertID1,
			DeviceID:    "router-core-01",
			CreatedAt:   now.Add(-2 * time.Hour),
			UpdatedAt:   now.Add(-30 * time.Minute),
		},
		{
			ID:          "TKT-002",
			Title:       "Server Memory Utilization Alert",
			Description: "Server memory usage exceeded 85% threshold",
			Priority:    "medium",
			Status:      models.TicketStatusInProgress,
			Category:    "Server",
			Assignee:    "Jane Doe",
			Reporter:    "Admin",
			AlertID:     &alertID3,
			DeviceID:    "server-app-01",
			CreatedAt:   now.Add(-5 * time.Hour),
			UpdatedAt:   now.Add(-1 * time.Hour),
		},
		{
			ID:          "TKT-003",
			Title:       "Firewall Configuration Review",
			Description: "Security audit required for firewall rule changes",
			Priority:    "low",
			Status:      models.TicketStatusOpen,
			Category:    "Security",
			Assignee:    "",
			Reporter:    "Security Team",
			CreatedAt:   now.Add(-24 * time.Hour),
			UpdatedAt:   now.Add(-24 * time.Hour),
		},
		{
			ID:          "TKT-004",
			Title:       "Critical - Database Connection Pool Exhausted",
			Description: "Production database experiencing connection pool exhaustion",
			Priority:    "critical",
			Status:      models.TicketStatusInProgress,
			Category:    "Database",
			Assignee:    "DBA Team",
			Reporter:    "Monitoring System",
			AlertID:     &alertID5,
			DeviceID:    "db-prod-01",
			CreatedAt:   now.Add(-1 * time.Hour),
			UpdatedAt:   now.Add(-15 * time.Minute),
		},
		{
			ID:          "TKT-005",
			Title:       "Scheduled Maintenance - Switch Upgrade",
			Description: "Planned upgrade of distribution switches in Building A",
			Priority:    "low",
			Status:      models.TicketStatusResolved,
			Category:    "Network",
			Assignee:    "Network Team",
			Reporter:    "Change Management",
			CreatedAt:   now.Add(-72 * time.Hour),
			UpdatedAt:   now.Add(-48 * time.Hour),
		},
	}
}

// getDemoTicketStats returns demo stats for when database is unavailable
func getDemoTicketStats() map[string]interface{} {
	return map[string]interface{}{
		"total":       15,
		"open":        6,
		"in_progress": 4,
		"resolved":    3,
		"closed":      2,
		"by_priority": map[string]int64{
			"critical": 2,
			"high":     4,
			"medium":   5,
			"low":      4,
		},
		"by_category": map[string]int64{
			"Network":  5,
			"Server":   4,
			"Security": 3,
			"Database": 3,
		},
	}
}

// GetTickets returns all tickets with optional filtering
func GetTickets(c *gin.Context) {
	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return demo tickets
		tickets := getDemoTickets()
		c.JSON(http.StatusOK, gin.H{
			"tickets": tickets,
			"total":   len(tickets),
		})
		return
	}

	filter := database.TicketFilter{
		Priority: c.Query("priority"),
		Status:   c.Query("status"),
		Category: c.Query("category"),
		Assignee: c.Query("assignee"),
	}

	// Parse pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	tickets, total, err := repo.List(filter)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"total":   total,
	})
}

// GetTicketByID returns a single ticket by ID
func GetTicketByID(c *gin.Context) {
	id := c.Param("id")

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - find from demo tickets
		for _, ticket := range getDemoTickets() {
			if ticket.ID == id {
				c.JSON(http.StatusOK, ticket)
				return
			}
		}
		apiErr := errors.NewTicketNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	ticket, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if ticket == nil {
		apiErr := errors.NewTicketNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, ticket)
}

// CreateTicket creates a new ticket
func CreateTicket(c *gin.Context) {
	var req models.CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return a demo response
		now := time.Now()
		ticket := models.Ticket{
			ID:          fmt.Sprintf("TKT-DEMO-%d", now.Unix()),
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			Status:      models.TicketStatusOpen,
			Category:    req.Category,
			Assignee:    req.Assignee,
			Reporter:    "demo-user",
			AlertID:     req.AlertID,
			DeviceID:    req.DeviceID,
			Tags:        strings.Join(req.Tags, ","),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		logger.Info("Demo mode: Ticket %s created", ticket.ID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "Ticket created successfully (demo mode)",
			"ticket":  ticket,
		})
		return
	}

	// Get reporter from auth context
	reporter, _ := c.Get("username")
	reporterStr := "system"
	if r, ok := reporter.(string); ok && r != "" {
		reporterStr = r
	}

	// Generate ticket ID
	ticketID, err := repo.GenerateTicketID()
	if err != nil {
		apiErr := errors.NewDatabaseError("generate id", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	ticket := models.Ticket{
		ID:          ticketID,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Status:      models.TicketStatusOpen,
		Category:    req.Category,
		Assignee:    req.Assignee,
		Reporter:    reporterStr,
		AlertID:     req.AlertID,
		DeviceID:    req.DeviceID,
		Tags:        strings.Join(req.Tags, ","),
	}

	if err := repo.Create(&ticket); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Ticket %s created by %s", ticketID, reporterStr)

	c.JSON(http.StatusCreated, gin.H{
		"message": "Ticket created successfully",
		"ticket":  ticket,
	})
}

// UpdateTicket updates an existing ticket
func UpdateTicket(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return success with mock updated ticket
		now := time.Now()
		ticket := models.Ticket{
			ID:          id,
			Title:       req.Title,
			Description: req.Description,
			Priority:    req.Priority,
			Status:      req.Status,
			Category:    req.Category,
			Assignee:    req.Assignee,
			Reporter:    "demo-user",
			Tags:        strings.Join(req.Tags, ","),
			CreatedAt:   now.Add(-1 * time.Hour),
			UpdatedAt:   now,
		}
		logger.Info("Demo mode: Ticket %s updated", id)
		c.JSON(http.StatusOK, gin.H{
			"message": "Ticket updated successfully (demo mode)",
			"ticket":  ticket,
		})
		return
	}

	// Check if ticket exists
	ticket, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if ticket == nil {
		apiErr := errors.NewTicketNotFound(id)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Category != "" {
		updates["category"] = req.Category
	}
	if req.Assignee != "" {
		updates["assignee"] = req.Assignee
	}
	if req.Tags != nil {
		updates["tags"] = strings.Join(req.Tags, ",")
	}

	if len(updates) > 0 {
		if err := repo.UpdateFields(id, updates); err != nil {
			apiErr := errors.NewDatabaseError("update", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	// Get username for logging
	username, _ := c.Get("username")
	logger.Info("Ticket %s updated by %v", id, username)

	// Fetch updated ticket
	updatedTicket, _ := repo.GetByID(id)

	c.JSON(http.StatusOK, gin.H{
		"message": "Ticket updated successfully",
		"ticket":  updatedTicket,
	})
}

// GetTicketStats returns ticket statistics
func GetTicketStats(c *gin.Context) {
	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return demo stats
		c.JSON(http.StatusOK, getDemoTicketStats())
		return
	}

	stats, err := repo.GetStats()
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ExportTickets exports tickets as CSV
func ExportTickets(c *gin.Context) {
	repo := ticketRepo()
	var tickets []models.Ticket

	if repo == nil {
		// Demo mode - export demo tickets
		tickets = getDemoTickets()
	} else {
		filter := database.TicketFilter{
			Priority: c.Query("priority"),
			Status:   c.Query("status"),
			Assignee: c.Query("assignee"),
		}

		var err error
		tickets, _, err = repo.List(filter)
		if err != nil {
			apiErr := errors.NewDatabaseError("query", err)
			c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
			return
		}
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=tickets-report-%s.csv", time.Now().Format("2006-01-02")))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// Header
	writer.Write([]string{"ID", "Title", "Priority", "Status", "Category", "Assignee", "Reporter", "Created"})

	for _, ticket := range tickets {
		writer.Write([]string{
			ticket.ID,
			ticket.Title,
			ticket.Priority,
			ticket.Status,
			ticket.Category,
			ticket.Assignee,
			ticket.Reporter,
			ticket.CreatedAt.Format(time.RFC3339),
		})
	}
}
