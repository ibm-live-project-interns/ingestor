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

// timePtr returns a pointer to the given time.Time value.
func timePtr(t time.Time) *time.Time { return &t }

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

	// If ticket has an assignee, look up their email with a targeted query and notify them
	if ticket.Assignee != "" {
		db := database.Get()
		if db != nil && db.DB != nil {
			userRepo := database.NewUserRepository(db.DB)
			u, err := userRepo.GetByUsernameOrEmail(ticket.Assignee)
			if err != nil {
				logger.Error("Failed to look up assignee %s for ticket notification: %v", ticket.Assignee, err)
			} else if u != nil {
				username := u.FirstName
				if username == "" {
					username = u.Username
				}
				if err := services.Email.SendTicketNotification(u.Email, username, emailData); err != nil {
					logger.Error("Failed to send ticket notification to %s: %v", u.Email, err)
				} else {
					logger.Info("Sent ticket %s notification to assignee %s", eventType, u.Email)
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

	// Validate query parameter lengths (max 255 chars)
	for _, v := range []string{filter.Priority, filter.Status, filter.Category, filter.Assignee} {
		if len(v) > 255 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query parameter too long"})
			return
		}
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

	// Apply pagination defaults/caps
	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 200 {
		filter.Limit = 200
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
	if !requireJSONContentType(c) {
		return
	}
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
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
	if !requireJSONContentType(c) {
		return
	}
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
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
	if isDemoMode() {
		c.JSON(http.StatusOK, gin.H{"message": "Action recorded (demo mode)", "demo": true})
		return
	}
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
