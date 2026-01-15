package constants

// Event type constants
const (
	EventTypeSyslog   = "syslog"
	EventTypeSNMP     = "snmp"
	EventTypeMetadata = "metadata"
)

// AllEventTypes returns all valid event types
var AllEventTypes = []string{
	EventTypeSyslog,
	EventTypeSNMP,
	EventTypeMetadata,
}

// IsValidEventType checks if the given event type is valid
func IsValidEventType(eventType string) bool {
	for _, validType := range AllEventTypes {
		if eventType == validType {
			return true
		}
	}
	return false
}
