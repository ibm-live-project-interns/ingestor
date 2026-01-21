package model

import "time"

// Event represents the normalized internal event format
type Event struct {
	ID          string            `json:"id"`
	Source      string            `json:"source"`
	Type        string            `json:"type"`
	Severity    string            `json:"severity"`
	Message     string            `json:"message"`
	Timestamp   time.Time         `json:"timestamp"`

	// Metadata enrichment fields
	IngestedAt  time.Time         `json:"ingested_at"`
	Environment string            `json:"environment"`
	Service     string            `json:"service"`
	Tags        map[string]string `json:"tags,omitempty"`
}
