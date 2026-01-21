package validator

import (
	"testing"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

func TestValidateEvent_Success(t *testing.T) {
	event := models.Event{
		EventType:      "syslog",
		SourceHost:     "router-1",
		SourceIP:       "192.168.1.1",
		Severity:       "critical",
		Category:       "network",
		Message:        "Interface down",
		EventTimestamp: time.Now(),
	}

	err := ValidateEvent(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateEvent_MissingFields(t *testing.T) {
	event := models.Event{
		EventType: "syslog",
	}

	err := ValidateEvent(event)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
}

func TestValidateEvent_InvalidSeverity(t *testing.T) {
	event := models.Event{
		EventType:      "syslog",
		SourceHost:     "router-1",
		SourceIP:       "192.168.1.1",
		Severity:       "invalid",
		Category:       "network",
		Message:        "Test",
		EventTimestamp: time.Now(),
	}

	err := ValidateEvent(event)
	if err == nil {
		t.Fatalf("expected invalid severity error, got nil")
	}
}
