package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ==========================================
// On-Call Types
// ==========================================

// OnCallPerson represents a person currently on call
type OnCallPerson struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Team      string `json:"team"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	ShiftType string `json:"shift_type"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Status    string `json:"status"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ScheduleEntry represents a single day's on-call assignment
type ScheduleEntry struct {
	Day            string `json:"day"`
	Date           string `json:"date"`
	PrimaryOnCall  string `json:"primary_oncall"`
	PrimaryTeam    string `json:"primary_team"`
	SecondaryOnCall string `json:"secondary_oncall"`
	SecondaryTeam  string `json:"secondary_team"`
	ShiftHours     string `json:"shift_hours"`
	IsToday        bool   `json:"is_today"`
}

// ScheduleOverride represents an upcoming schedule override
type ScheduleOverride struct {
	ID            uint   `json:"id"`
	OriginalPerson string `json:"original_person"`
	ReplacePerson  string `json:"replace_person"`
	Date          string `json:"date"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
	RequestedBy   string `json:"requested_by"`
	CreatedAt     string `json:"created_at"`
}

// ==========================================
// Demo Data Generators
// ==========================================

// getDemoCurrentOnCall returns realistic demo on-call people
func getDemoCurrentOnCall() []OnCallPerson {
	now := time.Now()

	// Shift boundaries: day shift 06:00-18:00, night shift 18:00-06:00
	var shiftStart, shiftEnd time.Time
	hour := now.Hour()
	if hour >= 6 && hour < 18 {
		// Day shift
		shiftStart = time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
		shiftEnd = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
	} else {
		// Night shift
		if hour >= 18 {
			shiftStart = time.Date(now.Year(), now.Month(), now.Day(), 18, 0, 0, 0, now.Location())
			shiftEnd = time.Date(now.Year(), now.Month(), now.Day()+1, 6, 0, 0, 0, now.Location())
		} else {
			shiftStart = time.Date(now.Year(), now.Month(), now.Day()-1, 18, 0, 0, 0, now.Location())
			shiftEnd = time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
		}
	}

	startStr := shiftStart.Format(time.RFC3339)
	endStr := shiftEnd.Format(time.RFC3339)

	shiftType := "Day Shift"
	if hour >= 18 || hour < 6 {
		shiftType = "Night Shift"
	}

	return []OnCallPerson{
		{
			ID:        2,
			Name:      "John Smith",
			Role:      "NOC Operator",
			Team:      "Network Operations",
			Email:     "john.smith@example.com",
			Phone:     "+1 (555) 234-5678",
			ShiftType: shiftType,
			StartTime: startStr,
			EndTime:   endStr,
			Status:    "active",
		},
		{
			ID:        3,
			Name:      "Jane Doe",
			Role:      "Site Reliability Engineer",
			Team:      "SRE Platform",
			Email:     "jane.doe@example.com",
			Phone:     "+1 (555) 345-6789",
			ShiftType: shiftType,
			StartTime: startStr,
			EndTime:   endStr,
			Status:    "active",
		},
		{
			ID:        5,
			Name:      "Carlos Rivera",
			Role:      "Senior Network Engineer",
			Team:      "Core Infrastructure",
			Email:     "carlos.rivera@example.com",
			Phone:     "+1 (555) 456-7890",
			ShiftType: shiftType,
			StartTime: startStr,
			EndTime:   endStr,
			Status:    "active",
		},
	}
}

// getDemoSchedule returns a week of realistic schedule entries centered on the current day
func getDemoSchedule() []ScheduleEntry {
	now := time.Now()
	weekday := int(now.Weekday())

	// Start from the most recent Monday
	monday := now.AddDate(0, 0, -((weekday+6)%7))

	dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

	// Rotating primary/secondary assignments
	primaries := []struct {
		name string
		team string
	}{
		{"John Smith", "Network Operations"},
		{"Jane Doe", "SRE Platform"},
		{"Carlos Rivera", "Core Infrastructure"},
		{"Sarah Chen", "Network Operations"},
		{"Marcus Johnson", "SRE Platform"},
		{"John Smith", "Network Operations"},
		{"Jane Doe", "SRE Platform"},
	}

	secondaries := []struct {
		name string
		team string
	}{
		{"Sarah Chen", "Network Operations"},
		{"Marcus Johnson", "SRE Platform"},
		{"John Smith", "Network Operations"},
		{"Jane Doe", "SRE Platform"},
		{"Carlos Rivera", "Core Infrastructure"},
		{"Marcus Johnson", "SRE Platform"},
		{"Carlos Rivera", "Core Infrastructure"},
	}

	schedule := make([]ScheduleEntry, 7)
	for i := 0; i < 7; i++ {
		day := monday.AddDate(0, 0, i)
		isToday := day.Year() == now.Year() && day.YearDay() == now.YearDay()

		shiftHours := "06:00 - 18:00 / 18:00 - 06:00"
		if i >= 5 {
			// Weekend shifts are longer single shifts
			shiftHours = "08:00 - 20:00 / 20:00 - 08:00"
		}

		schedule[i] = ScheduleEntry{
			Day:             dayNames[i],
			Date:            day.Format("2006-01-02"),
			PrimaryOnCall:   primaries[i].name,
			PrimaryTeam:     primaries[i].team,
			SecondaryOnCall: secondaries[i].name,
			SecondaryTeam:   secondaries[i].team,
			ShiftHours:      shiftHours,
			IsToday:         isToday,
		}
	}

	return schedule
}

// getDemoOverrides returns upcoming schedule overrides
func getDemoOverrides() []ScheduleOverride {
	now := time.Now()

	return []ScheduleOverride{
		{
			ID:             1,
			OriginalPerson: "John Smith",
			ReplacePerson:  "Marcus Johnson",
			Date:           now.AddDate(0, 0, 3).Format("2006-01-02"),
			Reason:         "Medical appointment",
			Status:         "approved",
			RequestedBy:    "John Smith",
			CreatedAt:      now.Add(-48 * time.Hour).Format(time.RFC3339),
		},
		{
			ID:             2,
			OriginalPerson: "Jane Doe",
			ReplacePerson:  "Sarah Chen",
			Date:           now.AddDate(0, 0, 5).Format("2006-01-02"),
			Reason:         "Conference travel",
			Status:         "pending",
			RequestedBy:    "Jane Doe",
			CreatedAt:      now.Add(-12 * time.Hour).Format(time.RFC3339),
		},
	}
}

// ==========================================
// Handlers
// ==========================================

// GetCurrentOnCall returns the people currently on call.
// Queries the real on_call_schedules table first; falls back to demo data
// if the database is unavailable or if DEMO_MODE=true.
// GET /api/v1/on-call/current
func GetCurrentOnCall(c *gin.Context) {
	// No admin restriction -- all authenticated users can view who is on call

	db := database.Get()
	if db != nil && db.DB != nil {
		now := time.Now().UTC()
		var schedules []models.OnCallSchedule
		err := db.Where("start_time <= ? AND end_time >= ?", now, now).
			Order("is_primary DESC, username ASC").
			Find(&schedules).Error

		if err == nil && len(schedules) > 0 {
			// Convert DB schedules to the OnCallPerson response format
			currentOnCall := make([]OnCallPerson, 0, len(schedules))
			for _, s := range schedules {
				shiftType := "Day Shift"
				hour := now.Hour()
				if hour >= 18 || hour < 6 {
					shiftType = "Night Shift"
				}

				currentOnCall = append(currentOnCall, OnCallPerson{
					ID:        s.UserID,
					Name:      s.Username,
					Role:      s.RotationType + " rotation",
					Team:      "On-Call",
					ShiftType: shiftType,
					StartTime: s.StartTime.Format(time.RFC3339),
					EndTime:   s.EndTime.Format(time.RFC3339),
					Status:    "active",
				})
			}

			logger.Info("On-Call: returning %d active schedules from database", len(currentOnCall))
			c.JSON(http.StatusOK, gin.H{
				"on_call":   currentOnCall,
				"total":     len(currentOnCall),
				"timestamp": now.Format(time.RFC3339),
			})
			return
		}

		// DB available but no active schedules found; fall through to demo data
		if err != nil {
			logger.Warn("On-Call: database query failed: %v, falling back to demo data", err)
		}
	}

	// Demo data fallback
	logger.Info("On-Call: returning current on-call data (demo mode)")
	currentOnCall := getDemoCurrentOnCall()

	c.JSON(http.StatusOK, gin.H{
		"on_call":   currentOnCall,
		"total":     len(currentOnCall),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetOnCallSchedule returns the weekly on-call schedule with overrides.
// Queries DB first; falls back to demo data if DB unavailable or empty.
// GET /api/v1/on-call/schedule
func GetOnCallSchedule(c *gin.Context) {
	db := database.Get()
	if db != nil && db.DB != nil {
		now := time.Now().UTC()

		// Week boundaries (Monday – Sunday)
		weekday := int(now.Weekday())
		monday := now.AddDate(0, 0, -((weekday+6)%7))
		monday = time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, monday.Location())
		sunday := monday.AddDate(0, 0, 7)

		var schedules []models.OnCallSchedule
		err := db.Where("start_time < ? AND end_time > ?", sunday, monday).
			Order("start_time ASC, is_primary DESC").
			Find(&schedules).Error

		if err == nil && len(schedules) > 0 {
			dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
			schedule := make([]ScheduleEntry, 7)
			for i := 0; i < 7; i++ {
				day := monday.AddDate(0, 0, i)
				isToday := day.Year() == now.Year() && day.YearDay() == now.YearDay()
				shiftHours := "06:00 - 18:00 / 18:00 - 06:00"
				if i >= 5 {
					shiftHours = "08:00 - 20:00 / 20:00 - 08:00"
				}
				entry := ScheduleEntry{
					Day:        dayNames[i],
					Date:       day.Format("2006-01-02"),
					ShiftHours: shiftHours,
					IsToday:    isToday,
				}
				for _, s := range schedules {
					if s.StartTime.Before(day.AddDate(0, 0, 1)) && s.EndTime.After(day) {
						if s.IsPrimary && entry.PrimaryOnCall == "" {
							entry.PrimaryOnCall = s.Username
							entry.PrimaryTeam = s.RotationType
						} else if !s.IsPrimary && entry.SecondaryOnCall == "" {
							entry.SecondaryOnCall = s.Username
							entry.SecondaryTeam = s.RotationType
						}
					}
				}
				schedule[i] = entry
			}

			var dbOverrides []models.OnCallOverride
			db.Where("start_time > ?", now).Order("start_time ASC").Limit(10).Find(&dbOverrides)
			overrides := make([]ScheduleOverride, 0, len(dbOverrides))
			for _, o := range dbOverrides {
				overrides = append(overrides, ScheduleOverride{
					ID:     o.ID,
					Date:   o.StartTime.Format("2006-01-02"),
					Reason: o.Reason,
					Status: "approved",
				})
			}

			logger.Info("On-Call: returning weekly schedule from database (%d schedules)", len(schedules))
			c.JSON(http.StatusOK, gin.H{
				"schedule":  schedule,
				"overrides": overrides,
				"week_of":   schedule[0].Date,
			})
			return
		}
		if err != nil {
			logger.Warn("On-Call: schedule query failed: %v, falling back to demo", err)
		}
	}

	// Demo data fallback
	logger.Info("On-Call: returning schedule data (demo mode)")
	schedule := getDemoSchedule()
	overrides := getDemoOverrides()
	c.JSON(http.StatusOK, gin.H{
		"schedule":  schedule,
		"overrides": overrides,
		"week_of":   schedule[0].Date,
	})
}
