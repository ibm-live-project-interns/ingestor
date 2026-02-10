package validator

import (
	"fmt"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

var allowedSeverities = map[string]bool{
	constants.SeverityCritical: true,
	constants.SeverityHigh:     true,
	constants.SeverityMedium:   true,
	constants.SeverityLow:      true,
	constants.SeverityInfo:     true,
}

func ValidateEvent(event models.Event) error {
	if event.EventType == "" {
		return fmt.Errorf("validation_error: event_type is required")
	}
	if event.SourceHost == "" {
		return fmt.Errorf("validation_error: source_host is required")
	}
	if event.Message == "" {
		return fmt.Errorf("validation_error: message is required")
	}
	if event.Severity == "" {
		return fmt.Errorf("validation_error: severity is required")
	}
	if !allowedSeverities[event.Severity] {
		return fmt.Errorf("validation_error: invalid severity '%s'", event.Severity)
	}

	// ❗ future timestamp validation
	if event.EventTimestamp.After(time.Now().Add(1 * time.Minute)) {
		return fmt.Errorf("validation_error: event_timestamp is in the future")
	}

	return nil
}
