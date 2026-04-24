package handlers

import (
	"sync"
	"time"
)

// ==========================================
// Demo Data Types
// ==========================================

// demoRunbookEntry is the internal demo-mode runbook type.
// It mirrors RunbookResponse but is kept separate so the demo file
// doesn't conflict with the models.Runbook DB type.
type demoRunbookEntry struct {
	ID                int
	Title             string
	Category          string
	Description       string
	Steps             []RunbookStep
	RelatedAlertTypes []string
	Author            string
	LastUpdated       time.Time
	UsageCount        int
	CreatedAt         time.Time
}

// demoToRunbookResponse converts a demoRunbookEntry to the API RunbookResponse type.
func demoToRunbookResponse(d demoRunbookEntry) RunbookResponse {
	return RunbookResponse{
		ID:                d.ID,
		Title:             d.Title,
		Category:          d.Category,
		Description:       d.Description,
		Steps:             d.Steps,
		RelatedAlertTypes: d.RelatedAlertTypes,
		Author:            d.Author,
		LastUpdated:       d.LastUpdated,
		UsageCount:        d.UsageCount,
		CreatedAt:         d.CreatedAt,
	}
}

// ==========================================
// Demo Data State
// ==========================================

// runbookMu protects demoRunbooks and nextDemoRunbookID from concurrent access.
var runbookMu sync.RWMutex

// nextDemoRunbookID tracks the next ID to assign in demo mode.
var nextDemoRunbookID = 11

// demoRunbooks holds the in-memory runbook list for demo mode mutations.
// All access must be protected by runbookMu.
var demoRunbooks []demoRunbookEntry

// initDemoRunbooksLocked ensures the demo data is initialized.
// Caller MUST hold runbookMu (write lock) before calling.
func initDemoRunbooksLocked() []demoRunbookEntry {
	if demoRunbooks == nil {
		demoRunbooks = getDemoRunbooks()
	}
	return demoRunbooks
}

// getDemoRunbookStats computes summary stats from the current demo data.
func getDemoRunbookStats(runbooks []demoRunbookEntry) map[string]interface{} {
	if runbooks == nil {
		runbookMu.Lock()
		runbooks = initDemoRunbooksLocked()
		runbookMu.Unlock()
	}
	categories := map[string]bool{}
	var mostUsed demoRunbookEntry
	var recentlyUpdated demoRunbookEntry

	for _, rb := range runbooks {
		categories[rb.Category] = true
		if rb.UsageCount > mostUsed.UsageCount {
			mostUsed = rb
		}
		if recentlyUpdated.ID == 0 || rb.LastUpdated.After(recentlyUpdated.LastUpdated) {
			recentlyUpdated = rb
		}
	}

	return map[string]interface{}{
		"total_runbooks":      len(runbooks),
		"total_categories":    len(categories),
		"most_used_title":     mostUsed.Title,
		"most_used_count":     mostUsed.UsageCount,
		"recently_updated":    recentlyUpdated.Title,
		"recently_updated_at": recentlyUpdated.LastUpdated,
	}
}

// ==========================================
// Demo Runbook Definitions
// ==========================================

// getDemoRunbooks returns realistic demo runbooks for when database is unavailable.
func getDemoRunbooks() []demoRunbookEntry {
	now := time.Now()
	return []demoRunbookEntry{
		{
			ID:       1,
			Title:    "Hardware Failure: Switch Module Replacement",
			Category: "Hardware",
			Description: "Step-by-step procedure for identifying and replacing a failed switch module in the data center. " +
				"Covers pre-checks, failover verification, physical replacement, and post-swap validation.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Identify the failed module using the alert details and device dashboard."},
				{Order: 2, Instruction: "Verify redundant path is active by checking the topology view."},
				{Order: 3, Instruction: "Notify the NOC team and create a maintenance window in the system."},
				{Order: 4, Instruction: "Power down the affected module following ESD safety procedures."},
				{Order: 5, Instruction: "Replace the module with a tested spare from inventory."},
				{Order: 6, Instruction: "Power on and verify the module initializes correctly (check LED status)."},
				{Order: 7, Instruction: "Run interface diagnostics and validate traffic is flowing."},
				{Order: 8, Instruction: "Close the maintenance window and update the ticket with resolution details."},
			},
			RelatedAlertTypes: []string{"hardware_failure", "interface_down", "module_error"},
			Author:            "Jane Doe",
			LastUpdated:       now.Add(-2 * 24 * time.Hour),
			UsageCount:        47,
			CreatedAt:         now.Add(-120 * 24 * time.Hour),
		},
		{
			ID:       2,
			Title:    "Network Outage: Core Router Failover",
			Category: "Network",
			Description: "Emergency response procedure for a core router outage. Covers failover activation, " +
				"traffic rerouting, root cause analysis, and service restoration.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Confirm the outage scope using the network topology and affected devices list."},
				{Order: 2, Instruction: "Activate the standby router using the failover console command."},
				{Order: 3, Instruction: "Verify BGP sessions re-establish with upstream peers (check BGP summary)."},
				{Order: 4, Instruction: "Monitor traffic flow for 5 minutes to confirm stability."},
				{Order: 5, Instruction: "Investigate the failed router: collect crash logs, check power supply, inspect hardware."},
				{Order: 6, Instruction: "Escalate to vendor support if hardware fault is confirmed (attach logs)."},
				{Order: 7, Instruction: "Document the timeline in the incident ticket and notify stakeholders."},
			},
			RelatedAlertTypes: []string{"network_outage", "bgp_down", "router_unreachable"},
			Author:            "John Smith",
			LastUpdated:       now.Add(-5 * 24 * time.Hour),
			UsageCount:        32,
			CreatedAt:         now.Add(-90 * 24 * time.Hour),
		},
		{
			ID:       3,
			Title:    "Security Incident: Unauthorized Access Response",
			Category: "Security",
			Description: "Incident response procedure for detected unauthorized access attempts. " +
				"Covers containment, evidence collection, credential rotation, and post-incident review.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Isolate the affected device or segment by applying an ACL block."},
				{Order: 2, Instruction: "Capture current session logs and authentication records for forensics."},
				{Order: 3, Instruction: "Rotate credentials for any compromised accounts immediately."},
				{Order: 4, Instruction: "Scan for lateral movement indicators on adjacent network segments."},
				{Order: 5, Instruction: "Notify the security team and escalate per the incident response policy."},
				{Order: 6, Instruction: "Restore access only after confirming the threat is neutralized."},
				{Order: 7, Instruction: "Conduct a post-incident review within 48 hours and update firewall rules."},
			},
			RelatedAlertTypes: []string{"security_breach", "unauthorized_access", "brute_force"},
			Author:            "Bob Wilson",
			LastUpdated:       now.Add(-1 * 24 * time.Hour),
			UsageCount:        18,
			CreatedAt:         now.Add(-60 * 24 * time.Hour),
		},
		{
			ID:       4,
			Title:    "High CPU Utilization: Diagnosis and Mitigation",
			Category: "Software",
			Description: "Diagnostic procedure for sustained high CPU utilization on network devices. " +
				"Covers process identification, traffic analysis, and remediation steps.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Log into the device and run 'show processes cpu sorted' to identify top consumers."},
				{Order: 2, Instruction: "Check if a routing protocol reconvergence is in progress (show ip route summary)."},
				{Order: 3, Instruction: "Verify there is no broadcast storm or traffic loop (check interface counters)."},
				{Order: 4, Instruction: "If a specific process is consuming excessive CPU, check for known vendor bugs."},
				{Order: 5, Instruction: "Apply CPU rate-limiting for control plane protection if needed."},
				{Order: 6, Instruction: "If caused by a software bug, schedule a maintenance window for patching."},
				{Order: 7, Instruction: "Monitor for 30 minutes after remediation to confirm CPU returns to baseline."},
			},
			RelatedAlertTypes: []string{"high_cpu", "cpu_threshold", "process_runaway"},
			Author:            "Jane Doe",
			LastUpdated:       now.Add(-3 * 24 * time.Hour),
			UsageCount:        63,
			CreatedAt:         now.Add(-150 * 24 * time.Hour),
		},
		{
			ID:       5,
			Title:    "Memory Leak: Detection and Recovery",
			Category: "Software",
			Description: "Procedure for diagnosing memory leaks on network equipment. " +
				"Covers memory trend analysis, process identification, and controlled recovery.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Check current memory utilization: 'show memory statistics' or equivalent."},
				{Order: 2, Instruction: "Review memory trend over the past 24h from the device metrics dashboard."},
				{Order: 3, Instruction: "Identify the process with growing memory allocation using process memory commands."},
				{Order: 4, Instruction: "Cross-reference the process with known memory leak advisories from the vendor."},
				{Order: 5, Instruction: "If a patch is available, schedule an upgrade during the next maintenance window."},
				{Order: 6, Instruction: "If memory is critically low (>90%), perform a controlled reload during low-traffic hours."},
				{Order: 7, Instruction: "Set up proactive monitoring with a lower threshold to catch recurrence early."},
			},
			RelatedAlertTypes: []string{"memory_leak", "high_memory", "memory_threshold"},
			Author:            "John Smith",
			LastUpdated:       now.Add(-7 * 24 * time.Hour),
			UsageCount:        28,
			CreatedAt:         now.Add(-100 * 24 * time.Hour),
		},
		{
			ID:       6,
			Title:    "DNS Resolution Failure: Troubleshooting Guide",
			Category: "Network",
			Description: "Comprehensive troubleshooting guide for DNS resolution failures affecting " +
				"network devices and services. Covers local and upstream DNS diagnosis.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Verify the DNS server is reachable with ping and traceroute from the affected device."},
				{Order: 2, Instruction: "Test DNS resolution locally: 'nslookup <hostname> <dns-server>'."},
				{Order: 3, Instruction: "Check the DNS server logs for errors or query failures."},
				{Order: 4, Instruction: "Verify DNS forwarder configuration and upstream resolver connectivity."},
				{Order: 5, Instruction: "If the primary DNS is down, switch devices to the secondary DNS server."},
				{Order: 6, Instruction: "Flush DNS cache on affected devices and clients."},
				{Order: 7, Instruction: "Confirm resolution is working end-to-end before closing the ticket."},
			},
			RelatedAlertTypes: []string{"dns_failure", "name_resolution", "dns_timeout"},
			Author:            "Jane Doe",
			LastUpdated:       now.Add(-10 * 24 * time.Hour),
			UsageCount:        22,
			CreatedAt:         now.Add(-80 * 24 * time.Hour),
		},
		{
			ID:       7,
			Title:    "Certificate Expiry: Renewal and Deployment",
			Category: "Security",
			Description: "Procedure for renewing and deploying TLS/SSL certificates before or after expiry. " +
				"Covers certificate generation, validation, deployment, and verification.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Identify all devices and services using the expiring certificate."},
				{Order: 2, Instruction: "Generate a new CSR (Certificate Signing Request) with the correct SANs."},
				{Order: 3, Instruction: "Submit the CSR to the CA and retrieve the signed certificate."},
				{Order: 4, Instruction: "Validate the certificate chain: root CA -> intermediate -> leaf."},
				{Order: 5, Instruction: "Deploy the new certificate to all affected devices during a maintenance window."},
				{Order: 6, Instruction: "Restart relevant services (web server, VPN, API gateway) to pick up the new cert."},
				{Order: 7, Instruction: "Verify certificate validity using openssl s_client or a browser check."},
				{Order: 8, Instruction: "Update the certificate monitoring system with the new expiry date."},
			},
			RelatedAlertTypes: []string{"cert_expiry", "ssl_error", "tls_handshake_failure"},
			Author:            "Bob Wilson",
			LastUpdated:       now.Add(-4 * 24 * time.Hour),
			UsageCount:        15,
			CreatedAt:         now.Add(-45 * 24 * time.Hour),
		},
		{
			ID:       8,
			Title:    "Firewall Rule Change: Safe Deployment Process",
			Category: "Security",
			Description: "Change management procedure for deploying firewall rule modifications. " +
				"Covers impact analysis, peer review, staged deployment, and rollback planning.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Document the proposed rule change with source, destination, port, and action."},
				{Order: 2, Instruction: "Perform impact analysis: identify all traffic flows affected by the change."},
				{Order: 3, Instruction: "Get peer review approval from a senior engineer before proceeding."},
				{Order: 4, Instruction: "Create a rollback plan with the exact commands to reverse the change."},
				{Order: 5, Instruction: "Deploy the rule change during the approved maintenance window."},
				{Order: 6, Instruction: "Monitor firewall logs for 15 minutes to detect unintended blocks or allows."},
				{Order: 7, Instruction: "Verify that legitimate traffic is not impacted (test critical services)."},
				{Order: 8, Instruction: "If issues are detected, execute the rollback plan immediately."},
				{Order: 9, Instruction: "Document the change in the audit log and close the change ticket."},
			},
			RelatedAlertTypes: []string{"firewall_change", "acl_violation", "policy_update"},
			Author:            "John Smith",
			LastUpdated:       now.Add(-6 * 24 * time.Hour),
			UsageCount:        35,
			CreatedAt:         now.Add(-110 * 24 * time.Hour),
		},
		{
			ID:       9,
			Title:    "BGP Session Flapping: Stabilization Procedure",
			Category: "Network",
			Description: "Procedure for diagnosing and stabilizing flapping BGP sessions. " +
				"Covers log analysis, dampening configuration, and peer coordination.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Review BGP neighbor state and check for repeated up/down transitions."},
				{Order: 2, Instruction: "Examine syslog for BGP notification messages and hold timer expirations."},
				{Order: 3, Instruction: "Check the physical link for CRC errors, input errors, or flapping carrier."},
				{Order: 4, Instruction: "Verify MTU matches on both ends of the BGP peering."},
				{Order: 5, Instruction: "Apply BGP route dampening to reduce churn while investigating."},
				{Order: 6, Instruction: "Contact the upstream peer NOC if the issue originates from their side."},
				{Order: 7, Instruction: "Once stable, remove dampening and monitor for 1 hour."},
			},
			RelatedAlertTypes: []string{"bgp_flap", "bgp_down", "routing_instability"},
			Author:            "Jane Doe",
			LastUpdated:       now.Add(-8 * 24 * time.Hour),
			UsageCount:        19,
			CreatedAt:         now.Add(-70 * 24 * time.Hour),
		},
		{
			ID:       10,
			Title:    "SNMP Trap Storm: Containment and Investigation",
			Category: "Hardware",
			Description: "Procedure for handling excessive SNMP trap generation from network devices. " +
				"Covers rate limiting, root cause identification, and trap filter tuning.",
			Steps: []RunbookStep{
				{Order: 1, Instruction: "Identify the source device(s) generating excessive traps from the event dashboard."},
				{Order: 2, Instruction: "Apply SNMP trap rate-limiting on the source device to reduce noise."},
				{Order: 3, Instruction: "Analyze the trap OIDs to determine the underlying condition (link flap, fan failure, etc)."},
				{Order: 4, Instruction: "Address the root cause on the device (replace hardware, fix config, etc)."},
				{Order: 5, Instruction: "Update SNMP trap filters to suppress non-actionable traps going forward."},
				{Order: 6, Instruction: "Verify normal trap rates have resumed before closing the incident."},
			},
			RelatedAlertTypes: []string{"snmp_storm", "trap_flood", "device_noise"},
			Author:            "Bob Wilson",
			LastUpdated:       now.Add(-12 * 24 * time.Hour),
			UsageCount:        11,
			CreatedAt:         now.Add(-40 * 24 * time.Hour),
		},
	}
}
