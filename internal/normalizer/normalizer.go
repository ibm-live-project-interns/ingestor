package normalizer

import (
	"time"

	"ingestor/internal/model"
)

// Normalize converts raw incoming data into a standard Event format
func Normalize(source string, raw map[string]interface{}) model.Event {
	event := model.Event{
		Source: source,
		Type:   "unknown",
	}

	if msg, ok := raw["message"].(string); ok {
		event.Message = msg
	}

	if sev, ok := raw["severity"].(string); ok {
		event.Severity = sev
	} else {
		event.Severity = "INFO"
	}

	if ts, ok := raw["timestamp"].(string); ok {
		parsed, err := time.Parse(time.RFC3339, ts)
		if err == nil {
			event.Timestamp = parsed
		}
	}

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	return event
}
