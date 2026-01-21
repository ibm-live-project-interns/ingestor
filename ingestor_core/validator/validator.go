package validator

import (
	"fmt"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// ValidateEvent ensures required fields are present
func ValidateEvent(event models.Event) error {
	if event.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if event.SourceHost == "" {
		return fmt.Errorf("source_host is required")
	}
	if event.Message == "" {
		return fmt.Errorf("message is required")
	}
	if event.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	return nil
}
