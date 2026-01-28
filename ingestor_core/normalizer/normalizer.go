package normalizer

import (
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Normalize converts raw incoming JSON into a normalized Event
func Normalize(raw map[string]interface{}) models.Event {
	event := models.Event{
		EventType: "unknown",
		Severity:  "INFO",
		Timestamp: time.Now(),
	}

	// event_type
	if v, ok := raw["event_type"].(string); ok && v != "" {
		event.EventType = v
	}

	// source_host
	if v, ok := raw["source_host"].(string); ok && v != "" {
		event.SourceHost = v
	}

	// message
	if v, ok := raw["message"].(string); ok && v != "" {
		event.Message = v
	}

	// severity
	if v, ok := raw["severity"].(string); ok && v != "" {
		event.Severity = v
	}

	// timestamp (RFC3339 expected)
	if v, ok := raw["timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			event.Timestamp = ts
		}
	}

	return event
}
