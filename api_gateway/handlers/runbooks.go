package handlers

import (
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
	"github.com/ibm-live-project-interns/ingestor/shared/rbac"
)

// ==========================================
// Runbook API Types (response types)
// ==========================================

// RunbookStep is re-exported from models for handler convenience.
type RunbookStep = models.RunbookStep

// RunbookResponse is the API response representation of a runbook.
type RunbookResponse struct {
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
// Repository Helper & Converters
// ==========================================

func runbookRepo() *database.RunbookRepository {
	db := database.Get()
	if db == nil || db.DB == nil {
		return nil
	}
	return database.NewRunbookRepository(db.DB)
}

// toRunbookResponse converts a models.Runbook (DB type) to RunbookResponse.
func toRunbookResponse(m models.Runbook) RunbookResponse {
	steps := database.DecodeRunbookSteps(m.Steps)
	related := database.DecodeStringSlice(m.RelatedAlertTypes)
	return RunbookResponse{
		ID:                m.ID,
		Title:             m.Title,
		Category:          m.Category,
		Description:       m.Description,
		Steps:             steps,
		RelatedAlertTypes: related,
		Author:            m.Author,
		LastUpdated:       m.UpdatedAt,
		UsageCount:        m.UsageCount,
		CreatedAt:         m.CreatedAt,
	}
}

// buildSteps converts []string (instructions) to []RunbookStep.
func buildSteps(instructions []string) []RunbookStep {
	steps := make([]RunbookStep, len(instructions))
	for i, s := range instructions {
		steps[i] = RunbookStep{Order: i + 1, Instruction: strings.TrimSpace(s)}
	}
	return steps
}

// ==========================================
// Role Checks
// ==========================================

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

// validRunbookCategories returns the allowed runbook category values.
var validRunbookCategories = map[string]bool{
	"Hardware": true,
	"Network":  true,
	"Software": true,
	"Security": true,
}

// ==========================================
// Handlers
// ==========================================

// GetRunbooks returns all runbooks with optional search and category filtering.
// GET /api/v1/runbooks
func GetRunbooks(c *gin.Context) {
	repo := runbookRepo()
	if repo == nil {
		// Demo mode fallback
		runbookMu.Lock()
		runbooks := initDemoRunbooksLocked()
		snapshot := make([]RunbookResponse, len(runbooks))
		for i, rb := range runbooks {
			snapshot[i] = demoToRunbookResponse(rb)
		}
		runbookMu.Unlock()

		filtered := filterRunbookResponses(snapshot, c)

		limit, offset := parsePagination(c, 25)
		total := len(filtered)
		if offset > len(filtered) {
			filtered = []RunbookResponse{}
		} else if offset+limit > len(filtered) {
			filtered = filtered[offset:]
		} else {
			filtered = filtered[offset : offset+limit]
		}

		c.JSON(http.StatusOK, gin.H{
			"runbooks": filtered,
			"total":    total,
			"stats":    getDemoRunbookStats(nil),
		})
		return
	}

	search := c.Query("search")
	category := c.Query("category")
	limit, offset := parsePagination(c, 25)

	dbRunbooks, total, err := repo.List(search, category, limit, offset)
	if err != nil {
		apiErr := errors.NewDatabaseError("query", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	responses := make([]RunbookResponse, len(dbRunbooks))
	for i, rb := range dbRunbooks {
		responses[i] = toRunbookResponse(rb)
	}

	// Fetch all runbooks for stats computation (not limited by search/category filter)
	allRunbooks, allTotal, _ := repo.List("", "", 500, 0)

	// Compute stats from all runbooks
	categories := make(map[string]bool)
	mostUsedTitle := "N/A"
	mostUsedCount := 0
	recentTitle := "N/A"
	recentAt := time.Time{}

	for _, rb := range allRunbooks {
		categories[rb.Category] = true
		if rb.UsageCount > mostUsedCount {
			mostUsedCount = rb.UsageCount
			mostUsedTitle = rb.Title
		}
		if rb.UpdatedAt.After(recentAt) {
			recentAt = rb.UpdatedAt
			recentTitle = rb.Title
		}
	}

	stats := gin.H{
		"total_runbooks":      allTotal,
		"total_categories":    len(categories),
		"most_used_title":     mostUsedTitle,
		"most_used_count":     mostUsedCount,
		"recently_updated":    recentTitle,
		"recently_updated_at": recentAt,
	}

	logger.Info("Returning %d runbooks from database", len(responses))
	c.JSON(http.StatusOK, gin.H{
		"runbooks": responses,
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

	repo := runbookRepo()
	if repo == nil {
		runbookMu.Lock()
		runbooks := initDemoRunbooksLocked()
		var found *RunbookResponse
		for i := range runbooks {
			if runbooks[i].ID == id {
				runbooks[i].UsageCount++
				rb := demoToRunbookResponse(runbooks[i])
				found = &rb
				break
			}
		}
		runbookMu.Unlock()
		if found != nil {
			c.JSON(http.StatusOK, gin.H{"runbook": found})
			return
		}
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	dbRunbook, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	// Increment usage count (non-blocking, ignore error)
	go repo.IncrementUsage(id)

	rb := toRunbookResponse(*dbRunbook)
	rb.UsageCount++ // reflect the increment in the response
	c.JSON(http.StatusOK, gin.H{"runbook": rb})
}

// CreateRunbook creates a new runbook entry.
// POST /api/v1/runbooks
func CreateRunbook(c *gin.Context) {
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
	if !validRunbookCategories[req.Category] {
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

	username, _ := c.Get("username")
	authorName, _ := username.(string)
	if authorName == "" {
		authorName = "Unknown"
	}

	steps := buildSteps(req.Steps)
	related := req.RelatedAlertTypes
	if related == nil {
		related = []string{}
	}

	repo := runbookRepo()
	if repo == nil {
		// Demo mode fallback
		now := time.Now()
		runbookMu.Lock()
		newRb := demoRunbookEntry{
			ID:                nextDemoRunbookID,
			Title:             strings.TrimSpace(req.Title),
			Category:          req.Category,
			Description:       strings.TrimSpace(req.Description),
			Steps:             steps,
			RelatedAlertTypes: related,
			Author:            authorName,
			LastUpdated:       now,
			UsageCount:        0,
			CreatedAt:         now,
		}
		nextDemoRunbookID++
		runbooks := initDemoRunbooksLocked()
		demoRunbooks = append(runbooks, newRb)
		runbookMu.Unlock()
		logger.Info("Demo mode: created runbook id=%d title=%q", newRb.ID, newRb.Title)
		rb := demoToRunbookResponse(newRb)
		c.JSON(http.StatusCreated, gin.H{"runbook": rb, "message": "Runbook created successfully"})
		return
	}

	now := time.Now()
	dbRunbook := models.Runbook{
		Title:             strings.TrimSpace(req.Title),
		Category:          req.Category,
		Description:       strings.TrimSpace(req.Description),
		Steps:             database.EncodeRunbookSteps(steps),
		RelatedAlertTypes: database.EncodeStringSlice(related),
		Author:            authorName,
		UsageCount:        0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := repo.Create(&dbRunbook); err != nil {
		apiErr := errors.NewDatabaseError("create", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Runbook created: id=%d title=%q", dbRunbook.ID, dbRunbook.Title)
	c.JSON(http.StatusCreated, gin.H{
		"runbook": toRunbookResponse(dbRunbook),
		"message": "Runbook created successfully",
	})
}

// UpdateRunbook updates an existing runbook.
// PUT /api/v1/runbooks/:id
func UpdateRunbook(c *gin.Context) {
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
	if !validRunbookCategories[req.Category] {
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

	repo := runbookRepo()
	if repo == nil {
		runbookMu.Lock()
		runbooks := initDemoRunbooksLocked()
		var updated *RunbookResponse
		for i := range runbooks {
			if runbooks[i].ID == id {
				runbooks[i].Title = strings.TrimSpace(req.Title)
				runbooks[i].Category = req.Category
				runbooks[i].Description = strings.TrimSpace(req.Description)
				runbooks[i].Steps = buildSteps(req.Steps)
				runbooks[i].RelatedAlertTypes = req.RelatedAlertTypes
				runbooks[i].LastUpdated = time.Now()
				rb := demoToRunbookResponse(runbooks[i])
				updated = &rb
				break
			}
		}
		runbookMu.Unlock()
		if updated != nil {
			c.JSON(http.StatusOK, gin.H{"runbook": updated, "message": "Runbook updated successfully"})
			return
		}
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	dbRunbook, err := repo.GetByID(id)
	if err != nil {
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	steps := buildSteps(req.Steps)
	related := req.RelatedAlertTypes
	if related == nil {
		related = []string{}
	}

	dbRunbook.Title = strings.TrimSpace(req.Title)
	dbRunbook.Category = req.Category
	dbRunbook.Description = strings.TrimSpace(req.Description)
	dbRunbook.Steps = database.EncodeRunbookSteps(steps)
	dbRunbook.RelatedAlertTypes = database.EncodeStringSlice(related)

	if err := repo.Update(dbRunbook); err != nil {
		apiErr := errors.NewDatabaseError("update", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Runbook updated: id=%d title=%q", id, dbRunbook.Title)
	c.JSON(http.StatusOK, gin.H{
		"runbook": toRunbookResponse(*dbRunbook),
		"message": "Runbook updated successfully",
	})
}

// DeleteRunbook removes a runbook by ID.
// DELETE /api/v1/runbooks/:id
func DeleteRunbook(c *gin.Context) {
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

	repo := runbookRepo()
	if repo == nil {
		runbookMu.Lock()
		runbooks := initDemoRunbooksLocked()
		found := false
		for i := range runbooks {
			if runbooks[i].ID == id {
				demoRunbooks = append(runbooks[:i], runbooks[i+1:]...)
				found = true
				break
			}
		}
		runbookMu.Unlock()
		if found {
			c.JSON(http.StatusOK, gin.H{"message": "Runbook deleted successfully"})
			return
		}
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if _, err := repo.GetByID(id); err != nil {
		apiErr := errors.NewNotFound("runbook")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	if err := repo.Delete(id); err != nil {
		apiErr := errors.NewDatabaseError("delete", err)
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	logger.Info("Runbook deleted: id=%d", id)
	c.JSON(http.StatusOK, gin.H{"message": "Runbook deleted successfully"})
}

// ==========================================
// Runbook Auto-Suggestion
// ==========================================

// SuggestRunbooks returns the top 3 matching runbooks based on category/severity/query.
// GET /api/v1/runbooks/suggest?category=Network&severity=critical
func SuggestRunbooks(c *gin.Context) {
	category := c.Query("category")
	severity := c.Query("severity")
	query := c.Query("query")

	if category == "" && severity == "" && query == "" {
		apiErr := errors.NewBadRequest("At least one of 'category', 'severity', or 'query' is required")
		c.JSON(apiErr.HTTPStatus, apiErr.ToResponse())
		return
	}

	repo := runbookRepo()
	var snapshot []RunbookResponse

	if repo != nil {
		// Pull top 50 from DB ordered by usage
		dbRunbooks, _, err := repo.List("", "", 50, 0)
		if err == nil {
			for _, rb := range dbRunbooks {
				snapshot = append(snapshot, toRunbookResponse(rb))
			}
		}
	}

	// Fall back to demo if no DB data
	if len(snapshot) == 0 {
		runbookMu.Lock()
		demos := initDemoRunbooksLocked()
		for _, d := range demos {
			snapshot = append(snapshot, demoToRunbookResponse(d))
		}
		runbookMu.Unlock()
	}

	type scoredRunbook struct {
		Runbook RunbookResponse
		Score   int
	}
	scored := make([]scoredRunbook, 0, len(snapshot))

	for _, rb := range snapshot {
		score := 0
		if category != "" && strings.EqualFold(rb.Category, category) {
			score += 10
		}
		if severity != "" {
			sevLower := strings.ToLower(severity)
			for _, at := range rb.RelatedAlertTypes {
				if strings.Contains(strings.ToLower(at), sevLower) {
					score += 5
					break
				}
			}
			if sevLower == "critical" || sevLower == "high" {
				if rb.UsageCount > 30 {
					score += 3
				} else if rb.UsageCount > 15 {
					score += 1
				}
			}
		}
		if query != "" {
			queryLower := strings.ToLower(query)
			for _, word := range strings.Fields(queryLower) {
				if len(word) < 3 {
					continue
				}
				if strings.Contains(strings.ToLower(rb.Title), word) {
					score += 4
				}
				if strings.Contains(strings.ToLower(rb.Description), word) {
					score += 2
				}
			}
		}
		base := rb.UsageCount / 20
		if base > 3 {
			base = 3
		}
		score += base
		scored = append(scored, scoredRunbook{Runbook: rb, Score: score})
	}

	// Simple descending sort
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].Score > scored[i].Score ||
				(scored[j].Score == scored[i].Score && scored[j].Runbook.UsageCount > scored[i].Runbook.UsageCount) {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

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
		suggestions = append(suggestions, SuggestionResult{
			ID:            rb.ID,
			Title:         rb.Title,
			Category:      rb.Category,
			EstimatedTime: fmt.Sprintf("%d min", len(rb.Steps)*7),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": suggestions,
		"total":       len(suggestions),
	})
}

// ==========================================
// Filtering & Pagination Helpers
// ==========================================

func parsePagination(c *gin.Context, defaultLimit int) (limit, offset int) {
	limit = defaultLimit
	offset = 0
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return
}

func filterRunbookResponses(runbooks []RunbookResponse, c *gin.Context) []RunbookResponse {
	search := c.Query("search")
	category := c.Query("category")
	if search == "" && category == "" {
		return runbooks
	}
	filtered := make([]RunbookResponse, 0, len(runbooks))
	for _, rb := range runbooks {
		if category != "" && !strings.EqualFold(rb.Category, category) {
			continue
		}
		if search != "" {
			sl := strings.ToLower(search)
			matched := strings.Contains(strings.ToLower(rb.Title), sl) ||
				strings.Contains(strings.ToLower(rb.Description), sl) ||
				strings.Contains(strings.ToLower(rb.Author), sl)
			if !matched {
				for _, at := range rb.RelatedAlertTypes {
					if strings.Contains(strings.ToLower(at), sl) {
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
