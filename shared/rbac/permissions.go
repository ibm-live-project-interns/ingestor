package rbac

// Permission represents a permission that can be granted to a role
type Permission string

// Permissions - MUST be in sync with UI's role.types.ts
const (
	PermViewAlerts        Permission = "view-alerts"
	PermAcknowledgeAlerts Permission = "acknowledge-alerts"
	PermCreateTickets     Permission = "create-tickets"
	PermViewTickets       Permission = "view-tickets"
	PermViewDevices       Permission = "view-devices"
	PermManageDevices     Permission = "manage-devices"
	PermViewConfig        Permission = "view-config"
	PermViewAnalytics     Permission = "view-analytics"
	PermExportReports     Permission = "export-reports"
	PermViewServices      Permission = "view-services"
	PermViewSLA           Permission = "view-sla"
	PermViewAll           Permission = "view-all"
	PermViewTeamMetrics   Permission = "view-team-metrics"
)

// RoleID represents a user role
type RoleID string

// Roles - MUST be in sync with UI's roleConfig.ts
const (
	RoleNetworkOps   RoleID = "network-ops"
	RoleSRE          RoleID = "sre"
	RoleNetworkAdmin RoleID = "network-admin"
	RoleSeniorEng    RoleID = "senior-eng"
	RoleSysAdmin     RoleID = "sysadmin"
)

// Role represents a user role with its permissions
type Role struct {
	ID          RoleID       `json:"id"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
}

// RolePermissions maps roles to their permissions
// This is the SINGLE SOURCE OF TRUTH for RBAC
var RolePermissions = map[RoleID][]Permission{
	RoleNetworkOps: {
		PermViewAlerts,
		PermAcknowledgeAlerts,
		PermCreateTickets,
		PermViewTickets,
		PermExportReports,
	},
	RoleSRE: {
		PermViewAlerts,
		PermViewServices,
		PermViewSLA,
		PermExportReports,
	},
	RoleNetworkAdmin: {
		PermViewDevices,
		PermManageDevices,
		PermViewConfig,
		PermViewAlerts,
	},
	RoleSeniorEng: {
		PermViewAll,
		PermViewAnalytics,
		PermExportReports,
		PermViewTeamMetrics,
	},
	RoleSysAdmin: {
		PermViewAlerts,
		PermAcknowledgeAlerts,
		PermCreateTickets,
		PermViewTickets,
		PermViewDevices,
		PermManageDevices,
		PermViewConfig,
		PermViewAnalytics,
		PermExportReports,
		PermViewServices,
		PermViewSLA,
		PermViewTeamMetrics,
		PermViewAll,
	},
}

// HasPermission checks if a role has a specific permission
func HasPermission(roleID RoleID, permission Permission) bool {
	perms, ok := RolePermissions[roleID]
	if !ok {
		return false
	}

	// view-all grants access to all view permissions
	for _, p := range perms {
		if p == permission {
			return true
		}
		if p == PermViewAll && isViewPermission(permission) {
			return true
		}
	}
	return false
}

// isViewPermission checks if a permission is a view-only permission
func isViewPermission(p Permission) bool {
	switch p {
	case PermViewAlerts, PermViewTickets, PermViewDevices, PermViewConfig,
		PermViewAnalytics, PermViewServices, PermViewSLA, PermViewTeamMetrics:
		return true
	}
	return false
}

// GetRolePermissions returns all permissions for a role
func GetRolePermissions(roleID RoleID) []Permission {
	perms, ok := RolePermissions[roleID]
	if !ok {
		return []Permission{}
	}
	return perms
}

// ValidRoles returns all valid role IDs
func ValidRoles() []RoleID {
	return []RoleID{
		RoleNetworkOps,
		RoleSRE,
		RoleNetworkAdmin,
		RoleSeniorEng,
		RoleSysAdmin,
	}
}

// IsValidRole checks if a role ID is valid
func IsValidRole(roleID string) bool {
	for _, r := range ValidRoles() {
		if string(r) == roleID {
			return true
		}
	}
	return false
}
