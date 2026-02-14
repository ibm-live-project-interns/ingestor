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
		EventType:      "unknown",
		Severity:       "info",
		Category:       "general",
		SourceIP:       "0.0.0.0",
		Message:        "",
		EventTimestamp: time.Now(),
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

	// category
	if v, ok := raw["category"].(string); ok && v != "" {
		event.Category = v
	}

	// message
	if v, ok := raw["message"].(string); ok && v != "" {
		event.Message = v
	}

	// severity — normalize common aliases to canonical values
	if v, ok := raw["severity"].(string); ok && v != "" {
		event.Severity = normalizeSeverity(v)
	}

	// raw_payload
	if v, ok := raw["raw_payload"].(string); ok {
		event.RawPayload = v
	}

	// timestamp (RFC3339 expected) — accept both "timestamp" and "event_timestamp"
	if v, ok := raw["event_timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			event.EventTimestamp = ts
		}
	} else if v, ok := raw["timestamp"].(string); ok {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			event.EventTimestamp = ts
		}
	}

	return event
}

// normalizeSeverity maps common severity aliases (e.g. syslog levels) to
// canonical values defined in constants.AllSeverities.
func normalizeSeverity(raw string) string {
	lower := strings.ToLower(raw)

	// If already a canonical value, return as-is.
	if constants.IsValidSeverity(lower) {
		return lower
	}

	// Map common syslog / external severity names.
	switch lower {
	case "error", "err", "emergency", "emerg", "alert", "crit":
		return constants.SeverityCritical
	case "warning", "warn":
		return constants.SeverityMedium
	case "notice":
		return constants.SeverityLow
	case "debug", "trace":
		return constants.SeverityInfo
	default:
		return constants.SeverityInfo
	}
}
