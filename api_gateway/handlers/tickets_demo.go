package handlers

import (
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

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
			ResolvedAt:  timePtr(now.Add(-48 * time.Hour)),
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
