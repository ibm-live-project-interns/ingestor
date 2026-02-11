package enricher

import (
	"log"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Enrich adds derived metadata to an event without mutating original data
func Enrich(event models.Event) models.Event {
	// Add default category if missing
	if event.Category == "" {
		event.Category = "general"
		log.Printf("[enricher] Defaulted category to 'general' for event from %s", event.SourceHost)
	}

	// Ensure timestamp exists
	if event.EventTimestamp.IsZero() {
		event.EventTimestamp = time.Now()
		log.Printf("[enricher] Defaulted timestamp to now for event from %s", event.SourceHost)
	}

	log.Printf("[enricher] Enriched event: type=%s severity=%s source=%s", event.EventType, event.Severity, event.SourceHost)
	return event
}
