package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ibm-live-project-interns/ingestor/shared/database"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// ==========================================
// Topology Types
// ==========================================

// TopologyNode represents a network device node in the topology graph
type TopologyNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	IP       string `json:"ip"`
	Location string `json:"location"`
}

// TopologyEdge represents a connection between two nodes
type TopologyEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Bandwidth   string  `json:"bandwidth"`
	Utilization float64 `json:"utilization"`
	Status      string  `json:"status"`
}

// TopologyResponse is the complete topology payload returned by the API
type TopologyResponse struct {
	Nodes     []TopologyNode `json:"nodes"`
	Edges     []TopologyEdge `json:"edges"`
	Locations []string       `json:"locations"`
}

// ==========================================
// Demo Data
// ==========================================

// getDemoTopology returns a realistic network topology with ~15 nodes and ~20 edges
func getDemoTopology() TopologyResponse {
	nodes := []TopologyNode{
		// Data Center A (Core infrastructure)
		{ID: "core-rtr-01", Label: "Core Router 01", Type: "router", Status: "online", IP: "10.0.0.1", Location: "Data Center A"},
		{ID: "core-rtr-02", Label: "Core Router 02", Type: "router", Status: "online", IP: "10.0.0.2", Location: "Data Center A"},
		{ID: "dist-sw-01", Label: "Distribution Switch 01", Type: "switch", Status: "online", IP: "10.0.1.1", Location: "Data Center A"},
		{ID: "dist-sw-02", Label: "Distribution Switch 02", Type: "switch", Status: "warning", IP: "10.0.1.2", Location: "Data Center A"},
		{ID: "fw-edge-01", Label: "Edge Firewall 01", Type: "firewall", Status: "online", IP: "10.0.0.254", Location: "Data Center A"},
		{ID: "lb-prod-01", Label: "Prod Load Balancer", Type: "server", Status: "online", IP: "10.0.2.10", Location: "Data Center A"},

		// Data Center B (Application / DB tier)
		{ID: "app-srv-01", Label: "App Server 01", Type: "server", Status: "online", IP: "10.1.1.10", Location: "Data Center B"},
		{ID: "app-srv-02", Label: "App Server 02", Type: "server", Status: "online", IP: "10.1.1.11", Location: "Data Center B"},
		{ID: "db-srv-01", Label: "Database Server 01", Type: "server", Status: "online", IP: "10.1.2.20", Location: "Data Center B"},
		{ID: "db-srv-02", Label: "Database Server 02", Type: "server", Status: "warning", IP: "10.1.2.21", Location: "Data Center B"},
		{ID: "dist-sw-03", Label: "Distribution Switch 03", Type: "switch", Status: "online", IP: "10.1.0.1", Location: "Data Center B"},

		// Branch Office (Edge / access layer)
		{ID: "branch-rtr-01", Label: "Branch Router 01", Type: "router", Status: "online", IP: "10.2.0.1", Location: "Branch Office"},
		{ID: "access-sw-01", Label: "Access Switch 01", Type: "switch", Status: "online", IP: "10.2.1.1", Location: "Branch Office"},
		{ID: "access-sw-02", Label: "Access Switch 02", Type: "switch", Status: "offline", IP: "10.2.1.2", Location: "Branch Office"},
		{ID: "ap-floor1-01", Label: "AP Floor 1", Type: "access-point", Status: "online", IP: "10.2.3.10", Location: "Branch Office"},
		{ID: "ap-floor2-01", Label: "AP Floor 2", Type: "access-point", Status: "online", IP: "10.2.3.11", Location: "Branch Office"},
	}

	edges := []TopologyEdge{
		// Core backbone - redundant pair
		{Source: "core-rtr-01", Target: "core-rtr-02", Bandwidth: "100 Gbps", Utilization: 32.5, Status: "active"},

		// Core to firewall
		{Source: "fw-edge-01", Target: "core-rtr-01", Bandwidth: "40 Gbps", Utilization: 48.2, Status: "active"},

		// Core routers to DC-A distribution switches
		{Source: "core-rtr-01", Target: "dist-sw-01", Bandwidth: "40 Gbps", Utilization: 55.1, Status: "active"},
		{Source: "core-rtr-02", Target: "dist-sw-02", Bandwidth: "40 Gbps", Utilization: 72.8, Status: "degraded"},

		// Distribution to load balancer
		{Source: "dist-sw-01", Target: "lb-prod-01", Bandwidth: "10 Gbps", Utilization: 41.3, Status: "active"},
		{Source: "dist-sw-02", Target: "lb-prod-01", Bandwidth: "10 Gbps", Utilization: 38.7, Status: "active"},

		// DC-A core to DC-B distribution (inter-DC links)
		{Source: "core-rtr-01", Target: "dist-sw-03", Bandwidth: "40 Gbps", Utilization: 28.9, Status: "active"},
		{Source: "core-rtr-02", Target: "dist-sw-03", Bandwidth: "40 Gbps", Utilization: 15.4, Status: "active"},

		// DC-B distribution to app servers
		{Source: "dist-sw-03", Target: "app-srv-01", Bandwidth: "10 Gbps", Utilization: 62.0, Status: "active"},
		{Source: "dist-sw-03", Target: "app-srv-02", Bandwidth: "10 Gbps", Utilization: 45.6, Status: "active"},

		// DC-B distribution to database servers
		{Source: "dist-sw-03", Target: "db-srv-01", Bandwidth: "10 Gbps", Utilization: 35.2, Status: "active"},
		{Source: "dist-sw-03", Target: "db-srv-02", Bandwidth: "10 Gbps", Utilization: 88.5, Status: "degraded"},

		// App to DB connections
		{Source: "app-srv-01", Target: "db-srv-01", Bandwidth: "10 Gbps", Utilization: 52.3, Status: "active"},
		{Source: "app-srv-02", Target: "db-srv-02", Bandwidth: "10 Gbps", Utilization: 78.1, Status: "degraded"},

		// WAN link: Core to branch router
		{Source: "core-rtr-01", Target: "branch-rtr-01", Bandwidth: "1 Gbps", Utilization: 67.4, Status: "active"},

		// Branch router to access switches
		{Source: "branch-rtr-01", Target: "access-sw-01", Bandwidth: "1 Gbps", Utilization: 23.8, Status: "active"},
		{Source: "branch-rtr-01", Target: "access-sw-02", Bandwidth: "1 Gbps", Utilization: 0.0, Status: "down"},

		// Access switches to wireless APs
		{Source: "access-sw-01", Target: "ap-floor1-01", Bandwidth: "1 Gbps", Utilization: 18.2, Status: "active"},
		{Source: "access-sw-01", Target: "ap-floor2-01", Bandwidth: "1 Gbps", Utilization: 12.7, Status: "active"},
	}

	locations := []string{"Data Center A", "Data Center B", "Branch Office"}

	return TopologyResponse{
		Nodes:     nodes,
		Edges:     edges,
		Locations: locations,
	}
}

// ==========================================
// Handler
// ==========================================

// GetTopology returns the network topology (nodes and edges).
// Queries real devices from the database when available and synthesizes
// connections, falling back to a comprehensive demo topology otherwise.
// GET /api/v1/topology
func GetTopology(c *gin.Context) {
	db := database.Get()
	if db == nil {
		logger.Info("Demo mode: returning demo network topology")
		topo := getDemoTopology()
		c.JSON(http.StatusOK, topo)
		return
	}

	// Query real devices from database
	var dbDevices []struct {
		ID       string     `json:"id"`
		Name     string     `json:"name"`
		IP       string     `json:"ip"`
		Type     string     `json:"type"`
		Location string     `json:"location"`
		Status   string     `json:"status"`
		LastSeen *time.Time `json:"last_seen"`
	}

	if err := db.Table("devices").Order("name ASC").Find(&dbDevices).Error; err != nil {
		logger.Error("Failed to query devices for topology: %v, falling back to demo data", err)
		topo := getDemoTopology()
		c.JSON(http.StatusOK, topo)
		return
	}

	if len(dbDevices) == 0 {
		logger.Warn("No devices in database, returning demo topology")
		topo := getDemoTopology()
		c.JSON(http.StatusOK, topo)
		return
	}

	// Build nodes from real devices
	nodes := make([]TopologyNode, 0, len(dbDevices))
	locationSet := make(map[string]bool)

	for _, d := range dbDevices {
		nodeType := mapDeviceTypeToTopologyType(d.Type)
		nodes = append(nodes, TopologyNode{
			ID:       d.ID,
			Label:    d.Name,
			Type:     nodeType,
			Status:   normalizeStatus(d.Status),
			IP:       d.IP,
			Location: d.Location,
		})
		if d.Location != "" {
			locationSet[d.Location] = true
		}
	}

	locations := make([]string, 0, len(locationSet))
	for loc := range locationSet {
		locations = append(locations, loc)
	}

	// Synthesize edges by connecting devices within the same location
	// via the first router or switch found in that location (star topology heuristic).
	edges := synthesizeEdges(nodes)

	logger.Info("Returning topology with %d nodes and %d edges from database", len(nodes), len(edges))

	c.JSON(http.StatusOK, TopologyResponse{
		Nodes:     nodes,
		Edges:     edges,
		Locations: locations,
	})
}

// ==========================================
// Helpers
// ==========================================

// mapDeviceTypeToTopologyType normalizes device type strings from the DB
// to the canonical topology types: router, switch, firewall, server, access-point.
func mapDeviceTypeToTopologyType(deviceType string) string {
	lower := toLower(deviceType)
	switch {
	case containsStr(lower, "router"):
		return "router"
	case containsStr(lower, "switch"):
		return "switch"
	case containsStr(lower, "firewall"):
		return "firewall"
	case containsStr(lower, "access") && containsStr(lower, "point"):
		return "access-point"
	case containsStr(lower, "ap") || containsStr(lower, "wireless"):
		return "access-point"
	case containsStr(lower, "server") || containsStr(lower, "load"):
		return "server"
	default:
		return "server"
	}
}

// normalizeStatus ensures a device status maps to one of: online, offline, warning.
func normalizeStatus(status string) string {
	lower := toLower(status)
	switch {
	case containsStr(lower, "online") || containsStr(lower, "up") || containsStr(lower, "active"):
		return "online"
	case containsStr(lower, "offline") || containsStr(lower, "down"):
		return "offline"
	case containsStr(lower, "degrad") || containsStr(lower, "warn"):
		return "warning"
	default:
		return "online"
	}
}

// synthesizeEdges builds a simple star topology per location.
// Within each location, the first router (or switch if no router) acts as the
// hub and all other devices in that location connect to it.
// Additionally, hub devices across different locations are interconnected.
func synthesizeEdges(nodes []TopologyNode) []TopologyEdge {
	// Group nodes by location
	locationNodes := make(map[string][]TopologyNode)
	for _, n := range nodes {
		locationNodes[n.Location] = append(locationNodes[n.Location], n)
	}

	edges := make([]TopologyEdge, 0)
	hubs := make([]TopologyNode, 0) // one hub per location for inter-location links

	for _, locNodes := range locationNodes {
		// Find hub: prefer router, then switch, then first device
		var hub *TopologyNode
		for i := range locNodes {
			if locNodes[i].Type == "router" {
				hub = &locNodes[i]
				break
			}
		}
		if hub == nil {
			for i := range locNodes {
				if locNodes[i].Type == "switch" {
					hub = &locNodes[i]
					break
				}
			}
		}
		if hub == nil && len(locNodes) > 0 {
			hub = &locNodes[0]
		}
		if hub == nil {
			continue
		}

		hubs = append(hubs, *hub)

		// Connect all non-hub devices in this location to the hub
		for _, n := range locNodes {
			if n.ID == hub.ID {
				continue
			}
			edgeStatus := "active"
			if n.Status == "offline" || hub.Status == "offline" {
				edgeStatus = "down"
			} else if n.Status == "warning" || hub.Status == "warning" {
				edgeStatus = "degraded"
			}
			edges = append(edges, TopologyEdge{
				Source:      hub.ID,
				Target:      n.ID,
				Bandwidth:   inferBandwidth(hub.Type, n.Type),
				Utilization: 0, // Real utilization would require SNMP polling data
				Status:      edgeStatus,
			})
		}
	}

	// Connect hubs across locations
	for i := 0; i < len(hubs); i++ {
		for j := i + 1; j < len(hubs); j++ {
			edgeStatus := "active"
			if hubs[i].Status == "offline" || hubs[j].Status == "offline" {
				edgeStatus = "down"
			}
			edges = append(edges, TopologyEdge{
				Source:      hubs[i].ID,
				Target:      hubs[j].ID,
				Bandwidth:   "10 Gbps",
				Utilization: 0,
				Status:      edgeStatus,
			})
		}
	}

	return edges
}

// inferBandwidth returns a reasonable default bandwidth string based on device types.
func inferBandwidth(typeA, typeB string) string {
	if typeA == "router" && typeB == "router" {
		return "100 Gbps"
	}
	if typeA == "router" || typeB == "router" {
		return "40 Gbps"
	}
	if typeA == "switch" || typeB == "switch" {
		return "10 Gbps"
	}
	if typeA == "access-point" || typeB == "access-point" {
		return "1 Gbps"
	}
	return "10 Gbps"
}
