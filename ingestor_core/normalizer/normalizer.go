package normalizer

import (
	"strings"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Normalize converts raw incoming JSON into a normalized Event
func Normalize(raw map[string]interface{}) models.Event {
	event := models.Event{
		Severity: constants.SeverityInfo,    // default = "info"
		Category: constants.DefaultCategory, // "general"
	}

	// event_type
	if v, ok := raw["event_type"].(string); ok && v != "" {
		event.EventType = v
	}

	// source_host
	if v, ok := raw["source_host"].(string); ok && v != "" {
		event.SourceHost = v
	}

	// source_ip
	if v, ok := raw["source_ip"].(string); ok && v != "" {
		event.SourceIP = v
	}

	// message
	if v, ok := raw["message"].(string); ok && v != "" {
		event.Message = v
	}

	// category
	if v, ok := raw["category"].(string); ok && v != "" {
		event.Category = v
	}

	// severity (normalize to canonical lowercase)
	if v, ok := raw["severity"].(string); ok && v != "" {
		event.Severity = normalizeSeverity(v)
	}

	// event_timestamp
	if v, ok := raw["event_timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			event.EventTimestamp = ts
		}
	}

	// Ensure timestamp always exists
	if event.EventTimestamp.IsZero() {
		event.EventTimestamp = time.Now().UTC()
	}

	return event
}

func normalizeSeverity(raw string) string {
	switch strings.ToUpper(raw) {
	case "CRITICAL", "ERROR", "ALERT", "EMERGENCY":
		return constants.SeverityCritical
	case "WARN", "WARNING":
		return constants.SeverityHigh
	case "NOTICE":
		return constants.SeverityMedium
	case "DEBUG":
		return constants.SeverityLow
	default:
		return constants.SeverityInfo
	}
}
