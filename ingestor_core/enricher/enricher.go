package enricher

import (
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Enrich adds derived metadata to an event
func Enrich(event models.Event) models.Event {
	// Add default category if missing
	if event.Category == "" {
		event.Category = "general"
	}

	// Add enrichment marker
	event.RawPayload = "[enriched] " + event.RawPayload

	// Ensure timestamp exists
	if event.EventTimestamp.IsZero() {
		event.EventTimestamp = time.Now()
	}

	return event
}
