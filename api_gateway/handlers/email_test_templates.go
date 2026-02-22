package handlers

import (
	"time"

	"api_gateway/services"
)

// emailTest defines a single test email template with its name, subject, and send function.
type emailTest struct {
	Name    string
	Subject string
	Send    func() error
}

// buildEmailTestCases returns all 34 test email template definitions for a given
// target address. Each entry contains the template name, expected subject line,
// and a closure that invokes the appropriate email service method.
func buildEmailTestCases(toEmail string) []emailTest {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	frontendURL := services.Email.FrontendURL()

	return []emailTest{
		// ==================== EXISTING 5 ====================
		{
			Name:    "verify-email",
			Subject: "Verify Your Sentrix Account",
			Send: func() error {
				return services.Email.SendVerificationEmail(toEmail, "Test User", "test-token-abc123")
			},
		},
		{
			Name:    "reset-password",
			Subject: "Reset Your Sentrix Password",
			Send: func() error {
				return services.Email.SendPasswordResetEmail(toEmail, "Test User", "reset-token-xyz789")
			},
		},
		{
			Name:    "welcome",
			Subject: "Welcome to Sentrix!",
			Send: func() error {
				return services.Email.SendWelcomeEmail(toEmail, "Test User")
			},
		},
		{
			Name:    "alert-notification",
			Subject: "[critical] core-switch-01 – High CPU Utilization",
			Send: func() error {
				return services.Email.SendAlertNotification(toEmail, "Test User", services.AlertEmailData{
					AlertID:   "ALT-1042",
					Title:     "High CPU Utilization Detected",
					Severity:  "critical",
					Device:    "core-switch-01",
					SourceIP:  "10.0.1.1",
					Category:  "Performance",
					AISummary: "CPU utilization has exceeded 95% for more than 5 minutes. Likely caused by a broadcast storm originating from VLAN 100. Recommend checking spanning tree configuration.",
					Timestamp: now,
				})
			},
		},
		{
			Name:    "ticket-notification",
			Subject: "[Ticket Created] TKT-2087 – Replace failing power supply",
			Send: func() error {
				return services.Email.SendTicketNotification(toEmail, "Test User", services.TicketEmailData{
					TicketID:     "TKT-2087",
					Title:        "Replace failing power supply on core-switch-01",
					Priority:     "high",
					Status:       "open",
					Assignee:     "John Smith",
					Category:     "Hardware",
					EventType:    "Created",
					EventMessage: "A new high-priority ticket has been created and assigned to you.",
				})
			},
		},

		// ==================== ALERT LIFECYCLE (4) ====================
		{
			Name:    "alert-acknowledged",
			Subject: "Alert Acknowledged – ALT-1042",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Alert Acknowledged – ALT-1042", "alert-acknowledged", map[string]interface{}{
					"AlertID":        "ALT-1042",
					"Title":          "High CPU Utilization Detected",
					"Severity":       "critical",
					"SeverityColor":  "#da1e28",
					"Device":         "core-switch-01",
					"AcknowledgedBy": "Jane Doe (SRE)",
					"Timestamp":      now,
					"ActionURL":      frontendURL + "/alerts/ALT-1042",
				})
			},
		},
		{
			Name:    "alert-resolved",
			Subject: "Alert Resolved – ALT-1042",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Alert Resolved – ALT-1042", "alert-resolved", map[string]interface{}{
					"AlertID":    "ALT-1042",
					"Title":      "High CPU Utilization Detected",
					"Severity":   "critical",
					"Device":     "core-switch-01",
					"ResolvedBy": "Jane Doe (SRE)",
					"Duration":   "42 minutes",
					"RootCause":  "Broadcast storm caused by misconfigured spanning tree on VLAN 100.",
					"Resolution": "Disabled redundant port on access switch, reconfigured STP priority. CPU utilization returned to normal (12%).",
					"Timestamp":  now,
					"ActionURL":  frontendURL + "/alerts/ALT-1042",
				})
			},
		},
		{
			Name:    "alert-dismissed",
			Subject: "Alert Dismissed – ALT-1043",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Alert Dismissed – ALT-1043", "alert-dismissed", map[string]interface{}{
					"AlertID":     "ALT-1043",
					"Title":       "Interface Flapping on GigabitEthernet0/1",
					"Severity":    "medium",
					"Device":      "dist-switch-03",
					"DismissedBy": "Bob Wilson (NOC Operator)",
					"Reason":      "Known issue – scheduled cable replacement tomorrow during maintenance window MW-005.",
					"Timestamp":   now,
					"ActionURL":   frontendURL + "/alerts/ALT-1043",
				})
			},
		},
		{
			Name:    "alert-escalated",
			Subject: "[ESCALATED] ALT-1044 – Firewall Cluster Failover",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[ESCALATED] ALT-1044 – Firewall Cluster Failover", "alert-escalated", map[string]interface{}{
					"AlertID":          "ALT-1044",
					"Title":            "Primary Firewall Cluster Failover",
					"Severity":         "critical",
					"Device":           "fw-cluster-01",
					"TimeElapsed":      "47 minutes (SLA: 15 min)",
					"EscalationLevel":  "Level 2",
					"PreviousAssignee": "Mike Chen (NOC Operator)",
					"PolicyName":       "Critical Infrastructure",
					"ActionURL":        frontendURL + "/alerts/ALT-1044",
				})
			},
		},

		// ==================== USER MANAGEMENT (4) ====================
		{
			Name:    "user-created-by-admin",
			Subject: "Your Sentrix Account Has Been Created",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Sarah Johnson", "Your Sentrix Account Has Been Created", "user-created-by-admin", map[string]interface{}{
					"Email":        toEmail,
					"TempPassword": "[REDACTED-TEST-ONLY]",
					"Role":         "Network Operations (NOC Operator)",
					"CreatedBy":    "Admin User (sysadmin)",
					"ActionURL":    frontendURL + "/login",
				})
			},
		},
		{
			Name:    "account-role-changed",
			Subject: "Your Role Has Been Updated",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Your Sentrix Role Has Been Updated", "account-role-changed", map[string]interface{}{
					"OldRole":           "NOC Operator (network-ops)",
					"NewRole":           "Site Reliability Engineer (sre)",
					"ChangedBy":         "Admin User (sysadmin)",
					"PermissionChanges": "You now have access to SLA reports, incident post-mortems, and reliability dashboards. Your monitoring permissions remain unchanged.",
					"ActionURL":         frontendURL,
				})
			},
		},
		{
			Name:    "account-deactivated",
			Subject: "Your Sentrix Account Has Been Deactivated",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Your Sentrix Account Has Been Deactivated", "account-deactivated", map[string]interface{}{
					"Email":         toEmail,
					"DeactivatedBy": "Admin User (sysadmin)",
					"Reason":        "Employee offboarding – last day was February 14, 2026.",
				})
			},
		},
		{
			Name:    "password-reset-by-admin",
			Subject: "Your Password Has Been Reset by Admin",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Your Sentrix Password Has Been Reset", "password-reset-by-admin", map[string]interface{}{
					"TempPassword": "[REDACTED-TEST-ONLY]",
					"ResetBy":      "Admin User (sysadmin)",
					"ActionURL":    frontendURL + "/login",
				})
			},
		},

		// ==================== SLA (3) ====================
		{
			Name:    "sla-violation",
			Subject: "[SLA VIOLATION] ALT-1045 – Response Time Exceeded",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[SLA VIOLATION] ALT-1045 – Response Time Exceeded", "sla-violation", map[string]interface{}{
					"AlertID":    "ALT-1045",
					"Title":      "BGP Session Down on Edge Router",
					"Severity":   "critical",
					"Device":     "edge-router-02",
					"SLATarget":  "15 minutes",
					"ActualTime": "1 hour 12 minutes",
					"ExceededBy": "57 minutes",
					"Assignee":   "Mike Chen (NOC Operator)",
					"ActionURL":  frontendURL + "/alerts/ALT-1045",
				})
			},
		},
		{
			Name:    "sla-weekly-report",
			Subject: "Weekly SLA Report – Feb 3–9, 2026",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Weekly SLA Compliance Report – Feb 3–9, 2026", "sla-weekly-report", map[string]interface{}{
					"WeekStart":       "Feb 3, 2026",
					"WeekEnd":         "Feb 9, 2026",
					"ComplianceRate":  "94.2%",
					"ComplianceColor": "#f1c21b",
					"TotalAlerts":     "187",
					"Violations":      "11",
					"CriticalMTTR":    "12 min",
					"HighMTTR":        "28 min",
					"MediumMTTR":      "1h 45m",
					"TrendVsLastWeek": "Compliance improved by 2.1% vs last week (92.1%). Critical MTTR improved by 3 minutes. 2 fewer violations.",
					"ActionURL":       frontendURL + "/reports/sla",
				})
			},
		},
		{
			Name:    "sla-monthly-report",
			Subject: "Monthly SLA Report – January 2026",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Monthly SLA Report – January 2026", "sla-monthly-report", map[string]interface{}{
					"MonthName":       "January",
					"Year":            "2026",
					"ComplianceRate":  "96.8%",
					"ComplianceColor": "#24a148",
					"TotalAlerts":     "743",
					"Violations":      "24",
					"AvgMTTR":         "23 min",
					"TopDevice1":      "edge-router-02 – 8 violations (BGP flapping)",
					"TopDevice2":      "core-switch-01 – 5 violations (CPU spikes)",
					"TopDevice3":      "dist-switch-07 – 4 violations (interface errors)",
					"Recommendations": "Consider upgrading edge-router-02 firmware to address recurring BGP instability. Schedule preventive maintenance for core-switch-01 power supply.",
					"ActionURL":       frontendURL + "/reports/sla",
				})
			},
		},

		// ==================== MAINTENANCE (3) ====================
		{
			Name:    "maintenance-upcoming",
			Subject: "Upcoming Maintenance – Core Network Upgrade",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Upcoming Maintenance – Core Network Upgrade", "maintenance-upcoming", map[string]interface{}{
					"WindowName":      "Core Network Firmware Upgrade",
					"StartTime":       "Feb 15, 2026 02:00",
					"EndTime":         "Feb 15, 2026 06:00",
					"Duration":        "4 hours",
					"AffectedDevices": "core-switch-01, core-switch-02, edge-router-01, edge-router-02",
					"Description":     "Scheduled firmware upgrade for all core network devices. Expected brief connectivity interruptions (< 30 seconds per device) during failover.",
					"ContactPerson":   "Jane Doe (Senior Engineer) – jane.doe@company.com",
					"ReminderType":    "24-hour advance",
					"ActionURL":       frontendURL + "/configuration",
				})
			},
		},
		{
			Name:    "maintenance-started",
			Subject: "Maintenance Active – Core Network Upgrade",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Maintenance Active – Core Network Upgrade", "maintenance-started", map[string]interface{}{
					"WindowName":      "Core Network Firmware Upgrade",
					"StartTime":       "Feb 15, 2026 02:00",
					"EndTime":         "Feb 15, 2026 06:00",
					"AffectedDevices": "core-switch-01, core-switch-02, edge-router-01, edge-router-02",
					"ContactPerson":   "Jane Doe (Senior Engineer) – jane.doe@company.com",
				})
			},
		},
		{
			Name:    "maintenance-completed",
			Subject: "Maintenance Completed – Core Network Upgrade",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Maintenance Completed – Core Network Upgrade", "maintenance-completed", map[string]interface{}{
					"WindowName":       "Core Network Firmware Upgrade",
					"ActualDuration":   "3 hours 42 minutes",
					"CompletionStatus": "Completed successfully",
					"AffectedDevices":  "core-switch-01, core-switch-02, edge-router-01, edge-router-02",
					"Summary":          "All 4 devices upgraded to firmware v12.4.3. No unexpected issues. Total downtime per device: 18–24 seconds during failover.",
					"SuppressedAlerts": "14",
					"ActionURL":        frontendURL,
				})
			},
		},

		// ==================== ESCALATION (2) ====================
		{
			Name:    "escalation-triggered",
			Subject: "[ESCALATION L2] ALT-1046 – WAN Link Saturation",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[ESCALATION Level 2] ALT-1046 – WAN Link Saturation", "escalation-triggered", map[string]interface{}{
					"AlertID":          "ALT-1046",
					"Title":            "WAN Link Saturation – 98% Utilization",
					"Severity":         "critical",
					"Device":           "wan-router-01",
					"TimeElapsed":      "32 minutes",
					"EscalationLevel":  "2",
					"PreviousAssignee": "Bob Wilson (NOC Operator)",
					"PolicyName":       "Critical Infrastructure",
					"NextEscalationIn": "15 minutes",
					"ActionURL":        frontendURL + "/alerts/ALT-1046",
				})
			},
		},
		{
			Name:    "escalation-final",
			Subject: "[FINAL ESCALATION] ALT-1046 – WAN Link Saturation",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[FINAL ESCALATION] ALT-1046 – WAN Link Saturation", "escalation-final", map[string]interface{}{
					"AlertID":           "ALT-1046",
					"Title":             "WAN Link Saturation – 98% Utilization",
					"Severity":          "critical",
					"Device":            "wan-router-01",
					"TimeElapsed":       "1 hour 47 minutes",
					"LevelsPassed":      "3 of 3",
					"PolicyName":        "Critical Infrastructure",
					"EscalationHistory": "L1: Bob Wilson (missed) → L2: Mike Chen (no resolution) → L3: You (final)",
					"ActionURL":         frontendURL + "/alerts/ALT-1046",
				})
			},
		},

		// ==================== ON-CALL (3) ====================
		{
			Name:    "oncall-rotation-reminder",
			Subject: "On-Call Rotation – Your Shift Starts Tomorrow",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "On-Call Rotation – Your Shift Starts Tomorrow", "oncall-rotation-reminder", map[string]interface{}{
					"ScheduleName": "Primary NOC On-Call",
					"Direction":    "starts",
					"ShiftStart":   "Feb 15, 2026 08:00",
					"ShiftEnd":     "Feb 22, 2026 08:00",
					"HandoffFrom":  "Mike Chen",
					"ActiveAlerts":  "3 active alerts: 1 critical (BGP flapping on edge-router-02), 2 medium (interface errors on dist-switch-03, dist-switch-07)",
					"HandoffNotes": "BGP issue on edge-router-02 is being monitored – vendor TAC case #SR-2026-1234 open. Expected firmware fix in next maintenance window.",
					"ActionURL":    frontendURL + "/on-call",
				})
			},
		},
		{
			Name:    "oncall-override-applied",
			Subject: "On-Call Override – Schedule Change",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "On-Call Override Applied – Schedule Change", "oncall-override-applied", map[string]interface{}{
					"StartTime":           "Feb 16, 2026 18:00",
					"EndTime":             "Feb 17, 2026 08:00",
					"OriginalEngineer":    "Test User",
					"ReplacementEngineer": "Sarah Johnson",
					"Reason":              "Personal time off – pre-approved by manager.",
					"ApprovedBy":          "Admin User (sysadmin)",
					"ActionURL":           frontendURL + "/on-call",
				})
			},
		},
		{
			Name:    "oncall-missed-alert",
			Subject: "[MISSED] On-Call Alert Not Acknowledged – ALT-1047",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[MISSED] On-Call Alert – ALT-1047 Not Acknowledged", "oncall-missed-alert", map[string]interface{}{
					"AlertID":        "ALT-1047",
					"Title":          "OSPF Neighbor Down on Distribution Layer",
					"Severity":       "high",
					"Device":         "dist-switch-05",
					"TimeSinceAlert": "22 minutes",
					"BackupEngineer": "Jane Doe (SRE)",
					"ActionURL":      frontendURL + "/alerts/ALT-1047",
				})
			},
		},

		// ==================== SECURITY (4) ====================
		{
			Name:    "security-failed-logins",
			Subject: "Suspicious Login Activity on Your Account",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Suspicious Login Activity on Your Sentrix Account", "security-failed-logins", map[string]interface{}{
					"AttemptCount":     "3",
					"IPAddress":        "203.0.113.42",
					"Timestamp":        now,
					"Location":         "Unknown (external IP)",
					"LockoutThreshold": "5",
					"ActionURL":        frontendURL + "/settings",
				})
			},
		},
		{
			Name:    "security-account-locked",
			Subject: "Your Account Has Been Locked",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Your Sentrix Account Has Been Locked", "security-account-locked", map[string]interface{}{
					"AttemptCount":    "5",
					"IPAddress":       "203.0.113.42",
					"Timestamp":       now,
					"UnlockTime":      time.Now().Add(30 * time.Minute).UTC().Format("2006-01-02 15:04:05"),
					"LockoutDuration": "30 minutes",
					"ActionURL":       frontendURL + "/forgot-password",
				})
			},
		},
		{
			Name:    "security-new-device-login",
			Subject: "New Device Login to Your Account",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "New Device Login to Your Sentrix Account", "security-new-device-login", map[string]interface{}{
					"IPAddress": "192.168.1.105",
					"Browser":   "Chrome 121.0 on Linux",
					"OS":        "Arch Linux (x86_64)",
					"Timestamp": now,
					"Location":  "Internal Network",
					"ActionURL": frontendURL + "/settings",
				})
			},
		},
		{
			Name:    "security-critical-admin-action",
			Subject: "[ADMIN] Critical Action – User Deletion",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[ADMIN] Critical Action – User Deletion", "security-critical-admin-action", map[string]interface{}{
					"ActionType":  "User Account Deleted",
					"PerformedBy": "Admin User (sysadmin)",
					"Timestamp":   now,
					"Resource":    "User: bob.wilson@company.com (ID: USR-0047)",
					"Details":     "Soft-deleted user account and invalidated all active sessions.",
					"Changes":     "Account status changed from 'active' to 'deleted'. 3 active sessions terminated. 12 assigned tickets reassigned to unassigned pool.",
					"ActionURL":   frontendURL + "/admin/audit-log",
				})
			},
		},

		// ==================== REPORTS / DIGESTS (3) ====================
		{
			Name:    "daily-operations-digest",
			Subject: "Daily Operations Digest – Feb 14, 2026",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Daily Operations Digest – Feb 14, 2026", "daily-operations-digest", map[string]interface{}{
					"Date":              "February 14, 2026",
					"TotalAlerts":       "47",
					"CriticalAlerts":    "3",
					"ResolvedAlerts":    "41",
					"MTTR":              "18 min",
					"TicketsOpened":     "8",
					"TicketsClosed":     "6",
					"TicketsInProgress": "14",
					"TopNoisyDevices":   "1. edge-router-02 (12 alerts) 2. dist-switch-03 (8 alerts) 3. core-switch-01 (5 alerts)",
					"ActionURL":         frontendURL,
				})
			},
		},
		{
			Name:    "weekly-incident-review",
			Subject: "Weekly Incident Review – Feb 3–9, 2026",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Weekly Incident Review – Feb 3–9, 2026", "weekly-incident-review", map[string]interface{}{
					"WeekStart":         "Feb 3, 2026",
					"WeekEnd":           "Feb 9, 2026",
					"TotalIncidents":    "12",
					"Resolved":          "11",
					"AvgMTTR":           "34 min",
					"TopRootCauses":     "1. Hardware failure (4 incidents) 2. Configuration drift (3 incidents) 3. Software bug (2 incidents)",
					"RepeatOffenders":   "edge-router-02 (3 incidents this week, 8 this month – firmware issue), dist-switch-03 (2 incidents – bad SFP module)",
					"PreventionActions": "Scheduled firmware upgrade for edge-router-02 (MW-006). Ordered replacement SFP for dist-switch-03. Added threshold rule for early CPU warning.",
					"ActionURL":         frontendURL + "/incidents",
				})
			},
		},
		{
			Name:    "monthly-executive-summary",
			Subject: "Monthly Executive Summary – January 2026",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "Monthly Executive Summary – January 2026", "monthly-executive-summary", map[string]interface{}{
					"MonthName":         "January",
					"Year":              "2026",
					"Uptime":            "99.94%",
					"UptimeColor":       "#24a148",
					"SLACompliance":     "96.8%",
					"SLAColor":          "#24a148",
					"TotalTickets":      "89",
					"TotalAlerts":       "743",
					"VolumeVsLastMonth": "Alert volume down 12% vs December. Ticket volume up 5%. MTTR improved by 8 minutes (31 min → 23 min).",
					"CapacityWarnings":  "WAN link wan-router-01 averaging 78% utilization during peak hours (up from 65% last month). Consider bandwidth upgrade by Q2.",
					"KeyHighlights":     "Successfully completed 3 maintenance windows with zero unplanned downtime. Deployed new SNMP monitoring for 15 additional access switches. AI-powered alert correlation reduced duplicate tickets by 23%.",
					"ActionURL":         frontendURL + "/reports",
				})
			},
		},

		// ==================== COLLABORATION (3) ====================
		{
			Name:    "ticket-sla-warning",
			Subject: "[SLA WARNING] TKT-2089 Approaching Deadline",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "[SLA WARNING] TKT-2089 – Approaching Deadline", "ticket-sla-warning", map[string]interface{}{
					"TicketID":      "TKT-2089",
					"Title":         "Investigate intermittent packet loss on dist-switch-03",
					"Priority":      "high",
					"Status":        "in-progress",
					"TimeRemaining": "2 hours 15 minutes",
					"SLADeadline":   "Feb 14, 2026 18:00",
					"ActionURL":     frontendURL + "/tickets/TKT-2089",
				})
			},
		},
		{
			Name:    "runbook-published",
			Subject: "New Runbook Published – BGP Session Recovery",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "New Runbook Published – BGP Session Recovery", "runbook-published", map[string]interface{}{
					"RunbookTitle": "BGP Session Recovery Procedure",
					"Category":    "Network Troubleshooting",
					"Author":      "Jane Doe (Senior Engineer)",
					"StepCount":   "8 steps",
					"Description":  "Step-by-step procedure for diagnosing and recovering failed BGP sessions on edge routers. Includes verification commands, rollback procedures, and escalation criteria.",
					"ActionURL":   frontendURL + "/runbooks",
				})
			},
		},
		{
			Name:    "ticket-mention",
			Subject: "You Were Mentioned in TKT-2090",
			Send: func() error {
				return services.Email.SendNotification(toEmail, "Test User", "You Were Mentioned in TKT-2090", "ticket-mention", map[string]interface{}{
					"TicketID":       "TKT-2090",
					"Title":          "Core switch failover test results",
					"Priority":       "medium",
					"Status":         "in-progress",
					"CommentAuthor":  "Jane Doe",
					"CommentExcerpt": "@Test User – can you verify the spanning tree configuration on core-switch-02 before we proceed with the failover test tomorrow? The current priority values don't match our design doc.",
					"ActionURL":      frontendURL + "/tickets/TKT-2090",
				})
			},
		},
	}
}
