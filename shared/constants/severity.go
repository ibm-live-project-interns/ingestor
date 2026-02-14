package constants

// Severity level constants
const (
	SeverityCritical = "critical"
	SeverityHigh     = "high"
	SeverityMajor    = "major"
	SeverityMedium   = "medium"
	SeverityMinor    = "minor"
	SeverityLow      = "low"
	SeverityInfo     = "info"
)

// AllSeverities returns all valid severity levels (ordered by priority descending)
var AllSeverities = []string{
	SeverityCritical,
	SeverityHigh,
	SeverityMajor,
	SeverityMedium,
	SeverityMinor,
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
	case SeverityMajor:
		return 3
	case SeverityMedium:
		return 4
	case SeverityMinor:
		return 5
	case SeverityLow:
		return 6
	case SeverityInfo:
		return 7
	default:
		return 99 // Unknown severity gets lowest priority
	}
}
