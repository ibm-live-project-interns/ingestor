package enricher

import (
	"testing"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

func TestEnrich_AddsDefaults(t *testing.T) {
	event := models.Event{
		EventType: "metadata",
		Message: "test event",
	}

	enriched := Enrich(event)

	if enriched.Category != "general" {
		t.Errorf("expected default category 'general', got %s", enriched.Category)
	}
}
