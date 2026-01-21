package constants

// Severity level constants
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMedium   = "medium"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// AllSeverities returns all valid severity levels
var AllSeverities = []string{
	SeverityCritical,
	SeverityHigh,
	SeverityMedium,
	SeverityLow,
	SeverityInfo,
}

// IsValidSeverity checks if the given severity is valid
func IsValidSeverity(severity string) bool {
	for _, validSeverity := range AllSeverities {
		if severity == validSeverity {
			return true
		}
	}
	return false
}

// GetSeverityPriority returns a numeric priority for routing (lower = higher priority)
func GetSeverityPriority(severity string) int {
	switch severity {
	case SeverityCritical:
		return 1
	case SeverityHigh:
		return 2
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 4
	case SeverityInfo:
		return 5
	default:
		return 99 // Unknown severity gets lowest priority
	}
}
