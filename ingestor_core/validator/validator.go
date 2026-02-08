package validator

import (
	"fmt"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

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
	if !constants.IsValidSeverity(event.Severity) {
		return fmt.Errorf("validation_error: invalid severity '%s'", event.Severity)
	}
	return nil
}
