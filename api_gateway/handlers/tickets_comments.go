package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// AddCommentRequest represents the request to add a comment to a ticket
type AddCommentRequest struct {
	Content string `json:"content" binding:"required"`
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
