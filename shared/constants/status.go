package constants

// Alert status constants (matches SQL CHECK constraint in init.sql)
const (
	StatusNew          = "new"
	StatusOpen         = "open"
	StatusAcknowledged = "acknowledged"
	StatusInProgress   = "in-progress"
	StatusResolved     = "resolved"
	StatusDismissed    = "dismissed"
)

// AllAlertStatuses returns all valid alert status values
var AllAlertStatuses = []string{
	StatusNew,
	StatusOpen,
	StatusAcknowledged,
	StatusInProgress,
	StatusResolved,
	StatusDismissed,
}

// IsValidAlertStatus checks if the given status string is a valid alert status
func IsValidAlertStatus(status string) bool {
	for _, validStatus := range AllAlertStatuses {
		if status == validStatus {
			return true
		}
	}
	return false
}
