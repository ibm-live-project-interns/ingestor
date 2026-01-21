package enricher

import (
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

// Enrich adds system-level metadata to a normalized event.
// This runs AFTER validation and BEFORE forwarding.
func Enrich(event models.Event) models.Event {

	// Always stamp when ingestor received the event
	event.ReceivedAt = time.Now().UTC()

	// Default category if missing
	if event.Category == "" {
		event.Category = "network"
	}

	// Fallback source IP if missing
	if event.SourceIP == "" {
		event.SourceIP = "unknown"
	}

	// Track which service processed the event
	event.Ingestor = "ingestor-core"

	return event
}
