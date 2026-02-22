package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// ==========================================
// Runbook Types
// ==========================================

// RunbookStep represents a single step in a runbook procedure.
type RunbookStep struct {
	Order       int    `json:"order"`
	Instruction string `json:"instruction"`
}

// Runbook represents an operational runbook / knowledge base article.
type Runbook struct {
	ID                int           `json:"id"`
	Title             string        `json:"title"`
	Category          string        `json:"category"`
	Description       string        `json:"description"`
	Steps             []RunbookStep `json:"steps"`
	RelatedAlertTypes []string      `json:"related_alert_types"`
	Author            string        `json:"author"`
	LastUpdated       time.Time     `json:"last_updated"`
	UsageCount        int           `json:"usage_count"`
	CreatedAt         time.Time     `json:"created_at"`
}

// CreateRunbookRequest is the expected payload for creating/updating a runbook.
type CreateRunbookRequest struct {
	Title             string   `json:"title"`
	Category          string   `json:"category"`
	Description       string   `json:"description"`
	Steps             []string `json:"steps"`
	RelatedAlertTypes []string `json:"related_alert_types"`
}

// ==========================================
// Role Checks
// ==========================================

// canManageRunbooks checks if the current user has a role that permits
// creating, updating, or deleting runbooks (sysadmin or senior-eng).
func canManageRunbooks(c *gin.Context) bool {
	role, exists := c.Get("role")
	if !exists {
		return false
	}
	roleStr, ok := role.(string)
	if !ok {
		return false
	}
	rid := rbac.RoleID(roleStr)
	return rid == rbac.RoleSysAdmin || rid == rbac.RoleSeniorEng
}

// ==========================================
// Handlers
// ==========================================

// GetRunbooks returns all runbooks with optional search and category filtering.
// GET /api/v1/runbooks
func GetRunbooks(c *gin.Context) {
	// Demo mode only (no database table exists)
	// Use write lock because initDemoRunbooksLocked may initialize the slice.
	runbookMu.Lock()
	runbooks := initDemoRunbooksLocked()
	// Take a snapshot copy so we can release the lock before doing I/O.
	snapshot := make([]Runbook, len(runbooks))
	copy(snapshot, runbooks)
	runbookMu.Unlock()

	logger.Info("Demo mode: returning runbooks (count=%d)", len(snapshot))

	// Apply filters
	filtered := filterRunbooks(snapshot, c)

	// Apply pagination
	limit := 25
	offset := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	total := len(filtered)
	if offset > len(filtered) {
		filtered = []Runbook{}
	} else if offset+limit > len(filtered) {
		filtered = filtered[offset:]
	} else {
		filtered = filtered[offset : offset+limit]
	}

	stats := getDemoRunbookStats(snapshot)

	c.JSON(http.StatusOK, gin.H{
		"runbooks": filtered,
		"total":    total,
		"stats":    stats,
	})
}

// GetRunbookByID returns a single runbook by ID.
// GET /api/v1/runbooks/:id
func GetRunbookByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid runbook ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	runbookMu.Lock()
	runbooks := initDemoRunbooksLocked()
	var found *Runbook
	for i := range runbooks {
		if runbooks[i].ID == id {
			// Increment usage count on view
			runbooks[i].UsageCount++
			// Copy so we can release the lock before writing the response.
			rb := runbooks[i]
			found = &rb
			break
		}
	}
	runbookMu.Unlock()

	if found != nil {
		c.JSON(http.StatusOK, gin.H{
			"runbook": found,
		})
		return
	}

	apiErr := errors.NewNotFound("runbook")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// CreateRunbook creates a new runbook entry.
// POST /api/v1/runbooks
func CreateRunbook(c *gin.Context) {
	// Only sysadmin and senior-eng can create runbooks
	if !canManageRunbooks(c) {
		apiErr := errors.NewInsufficientRole("sysadmin or senior-eng")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req CreateRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Title) == "" {
		apiErr := errors.NewValidation("Title is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if strings.TrimSpace(req.Category) == "" {
		apiErr := errors.NewValidation("Category is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		apiErr := errors.NewValidation("Description is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if len(req.Steps) == 0 {
		apiErr := errors.NewValidation("At least one step is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate category is one of the allowed values
	validCategories := map[string]bool{
		"Hardware": true,
		"Network":  true,
		"Software": true,
		"Security": true,
	}
	if !validCategories[req.Category] {
		apiErr := errors.NewValidation("Category must be one of: Hardware, Network, Software, Security")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get username from context
	username, _ := c.Get("username")
	authorName, _ := username.(string)
	if authorName == "" {
		authorName = "Unknown"
	}

	// Build steps
	steps := make([]RunbookStep, len(req.Steps))
	for i, s := range req.Steps {
		steps[i] = RunbookStep{
			Order:       i + 1,
			Instruction: strings.TrimSpace(s),
		}
	}

	now := time.Now()

	runbookMu.Lock()
	newRunbook := Runbook{
		ID:                nextDemoRunbookID,
		Title:             strings.TrimSpace(req.Title),
		Category:          req.Category,
		Description:       strings.TrimSpace(req.Description),
		Steps:             steps,
		RelatedAlertTypes: req.RelatedAlertTypes,
		Author:            authorName,
		LastUpdated:       now,
		UsageCount:        0,
		CreatedAt:         now,
	}
	nextDemoRunbookID++

	runbooks := initDemoRunbooksLocked()
	demoRunbooks = append(runbooks, newRunbook)
	runbookMu.Unlock()

	logger.Info("Demo mode: created runbook id=%d title=%q", newRunbook.ID, newRunbook.Title)

	c.JSON(http.StatusCreated, gin.H{
		"runbook": newRunbook,
		"message": "Runbook created successfully",
	})
}

// UpdateRunbook updates an existing runbook.
// PUT /api/v1/runbooks/:id
func UpdateRunbook(c *gin.Context) {
	// Only sysadmin and senior-eng can update runbooks
	if !canManageRunbooks(c) {
		apiErr := errors.NewInsufficientRole("sysadmin or senior-eng")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid runbook ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	var req CreateRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apiErr := errors.NewBadRequest("Invalid request body: " + err.Error())
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Validate required fields
	if strings.TrimSpace(req.Title) == "" {
		apiErr := errors.NewValidation("Title is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if strings.TrimSpace(req.Category) == "" {
		apiErr := errors.NewValidation("Category is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	validCategories := map[string]bool{
		"Hardware": true,
		"Network":  true,
		"Software": true,
		"Security": true,
	}
	if !validCategories[req.Category] {
		apiErr := errors.NewValidation("Category must be one of: Hardware, Network, Software, Security")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		apiErr := errors.NewValidation("Description is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}
	if len(req.Steps) == 0 {
		apiErr := errors.NewValidation("At least one step is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	runbookMu.Lock()
	runbooks := initDemoRunbooksLocked()
	var updated *Runbook
	for i := range runbooks {
		if runbooks[i].ID == id {
			// Build steps
			steps := make([]RunbookStep, len(req.Steps))
			for j, s := range req.Steps {
				steps[j] = RunbookStep{
					Order:       j + 1,
					Instruction: strings.TrimSpace(s),
				}
			}

			runbooks[i].Title = strings.TrimSpace(req.Title)
			runbooks[i].Category = req.Category
			runbooks[i].Description = strings.TrimSpace(req.Description)
			runbooks[i].Steps = steps
			runbooks[i].RelatedAlertTypes = req.RelatedAlertTypes
			runbooks[i].LastUpdated = time.Now()

			rb := runbooks[i]
			updated = &rb
			break
		}
	}
	runbookMu.Unlock()

	if updated != nil {
		logger.Info("Demo mode: updated runbook id=%d title=%q", id, updated.Title)
		c.JSON(http.StatusOK, gin.H{
			"runbook": updated,
			"message": "Runbook updated successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("runbook")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// DeleteRunbook removes a runbook by ID.
// DELETE /api/v1/runbooks/:id
func DeleteRunbook(c *gin.Context) {
	// Only sysadmin and senior-eng can delete runbooks
	if !canManageRunbooks(c) {
		apiErr := errors.NewInsufficientRole("sysadmin or senior-eng")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		apiErr := errors.NewBadRequest("Invalid runbook ID: must be a number")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	runbookMu.Lock()
	runbooks := initDemoRunbooksLocked()
	found := false
	for i := range runbooks {
		if runbooks[i].ID == id {
			// Remove from slice
			demoRunbooks = append(runbooks[:i], runbooks[i+1:]...)
			found = true
			break
		}
	}
	runbookMu.Unlock()

	if found {
		logger.Info("Demo mode: deleted runbook id=%d", id)
		c.JSON(http.StatusOK, gin.H{
			"message": "Runbook deleted successfully",
		})
		return
	}

	apiErr := errors.NewNotFound("runbook")
	c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
}

// ==========================================
// Runbook Auto-Suggestion
// ==========================================

// SuggestRunbooks returns the top 3 matching runbooks based on category and/or severity.
// GET /api/v1/runbooks/suggest?category=Network&severity=critical
func SuggestRunbooks(c *gin.Context) {
	category := c.Query("category")
	severity := c.Query("severity")
	query := c.Query("query") // optional: alert title or keywords for fuzzy matching

	if category == "" && severity == "" && query == "" {
		apiErr := errors.NewBadRequest("At least one of 'category', 'severity', or 'query' query parameters is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Get current runbooks (demo mode only since there is no DB table)
	runbookMu.Lock()
	runbooks := initDemoRunbooksLocked()
	snapshot := make([]Runbook, len(runbooks))
	copy(snapshot, runbooks)
	runbookMu.Unlock()

	// Score each runbook based on matching criteria
	type scoredRunbook struct {
		Runbook Runbook
		Score   int
	}
	scored := make([]scoredRunbook, 0, len(snapshot))

	for _, rb := range snapshot {
		score := 0

		// Category match (case-insensitive) — strongest signal
		if category != "" && strings.EqualFold(rb.Category, category) {
			score += 10
		}

		// Severity-based prioritization: match related alert types against severity keywords
		if severity != "" {
			severityLower := strings.ToLower(severity)
			for _, alertType := range rb.RelatedAlertTypes {
				if strings.Contains(strings.ToLower(alertType), severityLower) {
					score += 5
					break
				}
			}
			// Boost frequently used runbooks for critical/high severity alerts
			if severityLower == "critical" || severityLower == "high" {
				if rb.UsageCount > 30 {
					score += 3
				} else if rb.UsageCount > 15 {
					score += 1
				}
			}
		}

		// Keyword matching: match query words against title, description, and related alert types
		if query != "" {
			queryLower := strings.ToLower(query)
			queryWords := strings.Fields(queryLower)
			titleLower := strings.ToLower(rb.Title)
			descLower := strings.ToLower(rb.Description)

			for _, word := range queryWords {
				// Skip short/common words
				if len(word) < 3 {
					continue
				}
				if strings.Contains(titleLower, word) {
					score += 4
				}
				if strings.Contains(descLower, word) {
					score += 2
				}
				for _, alertType := range rb.RelatedAlertTypes {
					if strings.Contains(strings.ToLower(alertType), word) {
						score += 3
						break
					}
				}
			}
		}

		// Give every runbook a small base score from usage count so
		// there is always a fallback result (most popular runbooks)
		// when no strong matches exist.
		baseScore := rb.UsageCount / 20 // 0-3 range for typical usage counts
		if baseScore > 3 {
			baseScore = 3
		}
		score += baseScore

		scored = append(scored, scoredRunbook{Runbook: rb, Score: score})
	}

	// Sort by score descending, then by usage count descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score ||
				(scored[j].Score == scored[i].Score && scored[j].Runbook.UsageCount > scored[i].Runbook.UsageCount) {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Return top 3
	limit := 3
	if len(scored) < limit {
		limit = len(scored)
	}

	type SuggestionResult struct {
		ID            int    `json:"id"`
		Title         string `json:"title"`
		Category      string `json:"category"`
		EstimatedTime string `json:"estimated_time"`
	}

	suggestions := make([]SuggestionResult, 0, limit)
	for i := 0; i < limit; i++ {
		rb := scored[i].Runbook
		// Estimate time based on number of steps (roughly 5-10 min per step)
		estimatedMins := len(rb.Steps) * 7
		suggestions = append(suggestions, SuggestionResult{
			ID:            rb.ID,
			Title:         rb.Title,
			Category:      rb.Category,
			EstimatedTime: fmt.Sprintf("%d min", estimatedMins),
		})
	}

	logger.Info("Runbook suggestion: category=%q severity=%q query=%q returned=%d", category, severity, query, len(suggestions))

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// ==========================================
// Filtering Helpers
// ==========================================

// filterRunbooks applies search and category query parameters to the runbook list.
func filterRunbooks(runbooks []Runbook, c *gin.Context) []Runbook {
	search := c.Query("search")
	category := c.Query("category")

	if search == "" && category == "" {
		// No filters -- return a copy to avoid mutation issues
		result := make([]Runbook, len(runbooks))
		copy(result, runbooks)
		return result
	}

	filtered := make([]Runbook, 0, len(runbooks))
	for _, rb := range runbooks {
		// Category filter
		if category != "" && !strings.EqualFold(rb.Category, category) {
			continue
		}

		// Search filter (case-insensitive, matches title, description, author, or alert types)
		if search != "" {
			searchLower := strings.ToLower(search)
			matched := strings.Contains(strings.ToLower(rb.Title), searchLower) ||
				strings.Contains(strings.ToLower(rb.Description), searchLower) ||
				strings.Contains(strings.ToLower(rb.Author), searchLower)

			if !matched {
				// Also search related alert types
				for _, alertType := range rb.RelatedAlertTypes {
					if strings.Contains(strings.ToLower(alertType), searchLower) {
						matched = true
						break
					}
				}
			}

			if !matched {
				continue
			}
		}

		filtered = append(filtered, rb)
	}

	return filtered
}
