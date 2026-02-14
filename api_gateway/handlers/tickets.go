package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"api_gateway/services"
	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// sendTicketEmailNotification sends email notifications for ticket events asynchronously.
// It notifies the assignee (if set and different from actor) and any active users with email_alerts enabled.
func sendTicketEmailNotification(ticket models.Ticket, eventType, eventMessage, comment, commentAuthor string) {
	if services.Email == nil {
		return
	}

	emailData := services.TicketEmailData{
		TicketID:      ticket.ID,
		Title:         ticket.Title,
		Priority:      ticket.Priority,
		Status:        ticket.Status,
		Assignee:      ticket.Assignee,
		Category:      ticket.Category,
		EventType:     eventType,
		EventMessage:  eventMessage,
		Comment:       comment,
		CommentAuthor: commentAuthor,
	}

	// If ticket has an assignee, try to find their email and notify them
	if ticket.Assignee != "" {
		db := database.Get()
		if db != nil && db.DB != nil {
			userRepo := database.NewUserRepository(db.DB)
			// Try to find user by username or email
			users, _, _ := userRepo.GetAll(database.UserFilter{})
			for _, u := range users {
				if (u.Username == ticket.Assignee || u.Email == ticket.Assignee ||
					u.FirstName+" "+u.LastName == ticket.Assignee) && u.IsActive && u.EmailAlerts {
					username := u.FirstName
					if username == "" {
						username = u.Username
					}
					if err := services.Email.SendTicketNotification(u.Email, username, emailData); err != nil {
						logger.Error("Failed to send ticket notification to %s: %v", u.Email, err)
					} else {
						logger.Info("Sent ticket %s notification to assignee %s", eventType, u.Email)
					}
					break
				}
			}
		}
	}
}

// ticketRepo returns the ticket repository using the global database
// Returns nil if database is not available (demo mode)
func ticketRepo() *database.TicketRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewTicketRepository(db.DB)
}

// resolveDeviceNames populates DeviceName for tickets that have DeviceID but no DeviceName.
// It queries the devices table in a single batch to avoid N+1 queries.
func resolveDeviceNames(tickets []models.Ticket) {
	db := database.Get()
	if db == nil || db.DB == nil {
		return
	}

	// Collect device IDs that need resolution
	var needResolution []string
	for i := range tickets {
		if tickets[i].DeviceID != nil && *tickets[i].DeviceID != "" && tickets[i].DeviceName == "" {
			needResolution = append(needResolution, *tickets[i].DeviceID)
		}
	}
	if len(needResolution) == 0 {
		return
	}

	// Batch query device names
	type deviceRow struct {
		ID   string
		Name string
	}
	var rows []deviceRow
	if err := db.DB.Raw("SELECT id, name FROM devices WHERE id IN ? AND deleted_at IS NULL", needResolution).Scan(&rows).Error; err != nil {
		logger.Warn("Failed to resolve device names: %v", err)
		return
	}

	nameMap := make(map[string]string, len(rows))
	for _, r := range rows {
		nameMap[r.ID] = r.Name
	}

	// Apply resolved names
	for i := range tickets {
		if tickets[i].DeviceID != nil && tickets[i].DeviceName == "" {
			if name, ok := nameMap[*tickets[i].DeviceID]; ok {
				tickets[i].DeviceName = name
			}
		}
	}
}

// getDemoTickets returns demo tickets for when database is unavailable
func getDemoTickets() []models.Ticket {
	now := time.Now()
	alertID1 := "ALT-001"
	alertID3 := "ALT-003"
	alertID5 := "ALT-005"
	deviceID1 := "router-core-01"
	deviceID2 := "server-app-01"
	deviceID4 := "db-prod-01"
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
			DeviceID:    &deviceID1,
			DeviceName:  "Core Router 01",
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
			DeviceID:    &deviceID2,
			DeviceName:  "App Server 01",
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
			DeviceName:  "FW-DMZ-03",
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
			DeviceID:    &deviceID4,
			DeviceName:  "Production DB 01",
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
			DeviceName:  "Distribution Switch A",
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
		"avg_resolution_hours": 4.2,
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

	// Resolve device names for tickets that have device_id but no device_name
	resolveDeviceNames(tickets)

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

	// Resolve device name if missing
	if ticket.DeviceID != nil && *ticket.DeviceID != "" && ticket.DeviceName == "" {
		single := []models.Ticket{*ticket}
		resolveDeviceNames(single)
		ticket.DeviceName = single[0].DeviceName
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
		var demoDeviceID *string
		if req.DeviceID != "" {
			demoDeviceID = &req.DeviceID
		}
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
			DeviceID:    demoDeviceID,
			DeviceName:  req.DeviceName,
			Tags:        models.StringSlice(req.Tags),
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

	// Convert empty device_id to nil to avoid FK constraint violation
	var deviceID *string
	if req.DeviceID != "" {
		deviceID = &req.DeviceID
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
		DeviceID:    deviceID,
		DeviceName:  req.DeviceName,
		Tags:        models.StringSlice(req.Tags),
	}

	if err := repo.Create(&ticket); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Ticket %s created by %s", ticketID, reporterStr)

	// Send email notification asynchronously
	go sendTicketEmailNotification(ticket, "Created",
		fmt.Sprintf("A new %s priority ticket has been created and assigned to you.", ticket.Priority),
		"", "")

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
			Tags:        models.StringSlice(req.Tags),
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
	if req.AlertID != nil {
		updates["alert_id"] = *req.AlertID
	}
	if req.Tags != nil {
		updates["tags"] = models.StringSlice(req.Tags)
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
	usernameStr := "system"
	if u, ok := username.(string); ok && u != "" {
		usernameStr = u
	}
	logger.Info("Ticket %s updated by %s", id, usernameStr)

	// Fetch updated ticket
	updatedTicket, _ := repo.GetByID(id)

	// Send email notification for significant changes (assignment or status)
	if updatedTicket != nil {
		if _, changed := updates["assignee"]; changed {
			go sendTicketEmailNotification(*updatedTicket, "Assigned",
				fmt.Sprintf("You have been assigned to ticket %s by %s.", id, usernameStr),
				"", "")
		} else if _, changed := updates["status"]; changed {
			go sendTicketEmailNotification(*updatedTicket, "Updated",
				fmt.Sprintf("Ticket %s status changed to %s by %s.", id, updatedTicket.Status, usernameStr),
				"", "")
		}
	}

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

// DeleteTicket soft deletes a ticket
func DeleteTicket(c *gin.Context) {
	id := c.Param("id")

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return success
		logger.Info("Demo mode: Ticket %s deleted", id)
		c.JSON(http.StatusOK, gin.H{
			"message":   "Ticket deleted successfully (demo mode)",
			"ticket_id": id,
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

	if err := repo.Delete(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	username, _ := c.Get("username")
	logger.Info("Ticket %s deleted by %v", id, username)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Ticket deleted successfully",
		"ticket_id": id,
	})
}

// getDemoComments returns demo comments for when database is unavailable
func getDemoComments(ticketID string) []models.Comment {
	now := time.Now()
	return []models.Comment{
		{
			ID:        fmt.Sprintf("CMT-%s-001", ticketID),
			TicketID:  ticketID,
			Author:    "John Smith",
			Content:   "Initial investigation started. Checking network logs for anomalies.",
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		},
		{
			ID:        fmt.Sprintf("CMT-%s-002", ticketID),
			TicketID:  ticketID,
			Author:    "Jane Doe",
			Content:   "Found potential root cause. Interface flapping detected on port Gi0/1.",
			CreatedAt: now.Add(-30 * time.Minute),
			UpdatedAt: now.Add(-30 * time.Minute),
		},
	}
}

// GetTicketComments returns all comments for a ticket
func GetTicketComments(c *gin.Context) {
	ticketID := c.Param("id")

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return demo comments
		comments := getDemoComments(ticketID)
		c.JSON(http.StatusOK, gin.H{
			"comments": comments,
			"total":    len(comments),
		})
		return
	}

	// Verify ticket exists
	ticket, err := repo.GetByID(ticketID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if ticket == nil {
		apiErr := errors.NewTicketNotFound(ticketID)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	comments, err := repo.GetComments(ticketID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"total":    len(comments),
	})
}

// AddCommentRequest represents the request to add a comment to a ticket
type AddCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// AddTicketComment adds a comment to a ticket
func AddTicketComment(c *gin.Context) {
	ticketID := c.Param("id")

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewValidation(err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get author from auth context
	author, _ := c.Get("username")
	authorStr := "system"
	if a, ok := author.(string); ok && a != "" {
		authorStr = a
	}

	repo := ticketRepo()
	if repo == nil {
		// Demo mode - return a demo comment
		now := time.Now()
		comment := models.Comment{
			ID:        fmt.Sprintf("CMT-DEMO-%d", now.Unix()),
			TicketID:  ticketID,
			Author:    authorStr,
			Content:   req.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		logger.Info("Demo mode: Comment %s added to ticket %s", comment.ID, ticketID)
		c.JSON(http.StatusCreated, gin.H{
			"message": "Comment added successfully (demo mode)",
			"comment": comment,
		})
		return
	}

	// Verify ticket exists
	ticket, err := repo.GetByID(ticketID)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if ticket == nil {
		apiErr := errors.NewTicketNotFound(ticketID)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Generate comment ID
	commentID := fmt.Sprintf("CMT-%s-%d", ticketID, time.Now().UnixNano())

	comment := models.Comment{
		ID:       commentID,
		TicketID: ticketID,
		Author:   authorStr,
		Content:  req.Content,
	}

	if err := repo.AddComment(&comment); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Comment %s added to ticket %s by %s", commentID, ticketID, authorStr)

	// Send email notification to ticket assignee about the new comment
	if ticket != nil {
		go sendTicketEmailNotification(*ticket, "Commented",
			fmt.Sprintf("A new comment was added to ticket %s by %s.", ticketID, authorStr),
			req.Content, authorStr)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Comment added successfully",
		"comment": comment,
	})
}
