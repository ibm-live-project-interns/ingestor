package enricher

import (
	"os"
	"time"

	"ingestor/internal/model"
)

// Enrich adds metadata to a normalized event
func Enrich(event model.Event) model.Event {
	event.IngestedAt = time.Now()
	event.Service = "ingestor"

	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "dev"
	}
	event.Environment = env

	if event.Tags == nil {
		event.Tags = make(map[string]string)
	}
	event.Tags["enriched"] = "true"

	return event
}
