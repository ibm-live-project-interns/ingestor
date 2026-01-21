package models

import (
	"errors"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/constants"
)

// Event represents a normalized network event from datasource or external systems
type Event struct {
	// Core normalized fields (from datasource / normalizer)
	EventType      string    `json:"event_type" binding:"required,oneof=syslog snmp metadata"`
	SourceHost     string    `json:"source_host" binding:"required"`
	SourceIP       string    `json:"source_ip" binding:"required,ip"`
	Severity       string    `json:"severity" binding:"required,oneof=critical high medium low info"`
	Category       string    `json:"category" binding:"required"`
	Message        string    `json:"message" binding:"required"`
	RawPayload     string    `json:"raw_payload"`
	EventTimestamp time.Time `json:"event_timestamp" binding:"required"`

	// Enrichment metadata (added by Ingestor Core)
	ReceivedAt time.Time `json:"received_at,omitempty"`
	Ingestor   string    `json:"ingestor,omitempty"`
}

// Validate performs business logic validation on the Event
func (e *Event) Validate() error {
	// Validate event type
	if !constants.IsValidEventType(e.EventType) {
		return errors.New("invalid event_type: must be syslog, snmp, or metadata")
	}

	// Validate severity
	if !constants.IsValidSeverity(e.Severity) {
		return errors.New("invalid severity: must be critical, high, medium, low, or info")
	}

	// Validate timestamp is not too far in the future
	if e.EventTimestamp.After(time.Now().Add(5 * time.Minute)) {
		return errors.New("event_timestamp cannot be more than 5 minutes in the future")
	}

	// Validate timestamp is not too old
	if e.EventTimestamp.Before(time.Now().Add(-7 * 24 * time.Hour)) {
		return errors.New("event_timestamp cannot be older than 7 days")
	}

	// Validate required string fields
	if e.SourceHost == "" || e.SourceIP == "" || e.Message == "" {
		return errors.New("source_host, source_ip, and message cannot be empty")
	}

	return nil
}

// RoutedEvent is the format expected by Event Router
type RoutedEvent struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	SourceHost string `json:"source_host,omitempty"`
	SourceIP   string `json:"source_ip,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	Category   string `json:"category,omitempty"`
}

// ToRoutedEvent converts a normalized Event to a RoutedEvent
func (e *Event) ToRoutedEvent() RoutedEvent {
	return RoutedEvent{
		Type:       e.Severity, // routing is done by severity
		Message:    e.Message,
		SourceHost: e.SourceHost,
		SourceIP:   e.SourceIP,
		EventType:  e.EventType,
		Category:   e.Category,
	}
}
