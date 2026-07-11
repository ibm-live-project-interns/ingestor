package handlers

import (
	"sync"
	"time"
)

// ==========================================
// Device Groups - Demo Data & Initialization
// ==========================================

// deviceGroupMu protects demoDeviceGroups and nextDemoGroupID from concurrent access.
var deviceGroupMu sync.RWMutex

// initDeviceGroupsOnce ensures demo data initialization happens exactly once, thread-safely.
var initDeviceGroupsOnce sync.Once

// nextDemoGroupID tracks the next ID suffix to assign in demo mode.
var nextDemoGroupID = 6

// demoDeviceGroups holds the in-memory device group list for demo mode mutations.
// All access must be protected by deviceGroupMu.
var demoDeviceGroups []DeviceGroup

// getDefaultDeviceGroups returns realistic demo device groups.
func getDefaultDeviceGroups() []DeviceGroup {
	now := time.Now()
	return []DeviceGroup{
		{
			ID:          "grp-001",
			Name:        "Core Network",
			Description: "Core switches and routers forming the network backbone",
			Color:       "#4589ff",
			DeviceIDs:   []string{"dev-001", "dev-004", "dev-007"},
			DeviceCount: 3,
			CreatedAt:   now.Add(-90 * 24 * time.Hour),
			UpdatedAt:   now.Add(-2 * 24 * time.Hour),
		},
		{
			ID:          "grp-002",
			Name:        "DMZ / Security",
			Description: "Firewalls and security appliances in the demilitarized zone",
			Color:       "#da1e28",
			DeviceIDs:   []string{"dev-002"},
			DeviceCount: 1,
			CreatedAt:   now.Add(-85 * 24 * time.Hour),
			UpdatedAt:   now.Add(-5 * 24 * time.Hour),
		},
		{
			ID:          "grp-003",
			Name:        "Edge Routing",
			Description: "Edge and border routers connecting to upstream ISPs",
			Color:       "#198038",
			DeviceIDs:   []string{"dev-003"},
			DeviceCount: 1,
			CreatedAt:   now.Add(-80 * 24 * time.Hour),
			UpdatedAt:   now.Add(-10 * 24 * time.Hour),
		},
		{
			ID:          "grp-004",
			Name:        "Wireless Infrastructure",
			Description: "Access points and wireless controllers across all floors",
			Color:       "#8a3ffc",
			DeviceIDs:   []string{"dev-005", "dev-006", "dev-010"},
			DeviceCount: 3,
			CreatedAt:   now.Add(-60 * 24 * time.Hour),
			UpdatedAt:   now.Add(-1 * 24 * time.Hour),
		},
		{
			ID:          "grp-005",
			Name:        "Data Center",
			Description: "Load balancers, UPS systems, and data center equipment",
			Color:       "#ee5396",
			DeviceIDs:   []string{"dev-008", "dev-009"},
			DeviceCount: 2,
			CreatedAt:   now.Add(-45 * 24 * time.Hour),
			UpdatedAt:   now.Add(-3 * 24 * time.Hour),
		},
	}
}

// ensureDemoDeviceGroupsInitialized uses sync.Once to guarantee thread-safe,
// one-time initialization of the demo device groups slice.
func ensureDemoDeviceGroupsInitialized() {
	initDeviceGroupsOnce.Do(func() {
		deviceGroupMu.Lock()
		defer deviceGroupMu.Unlock()
		demoDeviceGroups = getDefaultDeviceGroups()
	})
}
