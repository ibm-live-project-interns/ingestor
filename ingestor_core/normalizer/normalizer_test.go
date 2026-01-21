package normalizer

import (
	"testing"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/models"
)

func TestNormalize_SyslogEvent(t *testing.T) {
	raw := map[string]interface{}{
		"event_type": "syslog",
		"source_host": "router-1",
		"source_ip": "192.168.1.1",
		"severity": "ERROR",
		"category": "network",
		"message": "Interface down",
		"event_timestamp": time.Now().Format(time.RFC3339),
	}

	event := Normalize(raw)

	if event.EventType != "syslog" {
		t.Errorf("expected event_type=syslog, got %s", event.EventType)
	}

	if event.Severity != "critical" {
		t.Errorf("expected severity=critical, got %s", event.Severity)
	}

	if event.SourceHost != "router-1" {
		t.Errorf("unexpected source_host: %s", event.SourceHost)
	}
}

func TestNormalize_MetadataDefaults(t *testing.T) {
	raw := map[string]interface{}{
		"event_type": "metadata",
		"source_host": "service-auth",
		"source_ip": "10.0.0.5",
		"message": "Service version updated",
	}

	event := Normalize(raw)

	if event.Severity != "info" {
		t.Errorf("expected default severity=info, got %s", event.Severity)
	}

	if event.EventType != "metadata" {
		t.Errorf("expected metadata event, got %s", event.EventType)
	}
}
