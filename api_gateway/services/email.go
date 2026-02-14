package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// EmailService handles email sending
type EmailService struct {
	client       *mail.Client
	fromAddr     string
	fromName     string
	templates    map[string]*template.Template
	templateDir  string
	frontendURL  string
	dashboardURL string
	docsURL      string
	appName      string
}

var Email *EmailService

// EmailData represents data for email templates (dynamic, supports any email type)
type EmailData struct {
	// Common fields
	AppName string
	Year    int
	Subject string

	// User info
	Username string

	// Dynamic URLs
	ActionURL    string // Generic action URL (verify, reset, etc.)
	DashboardURL string
	DocsURL      string

	// Legacy fields for backwards compatibility
	VerifyURL string
	ResetURL  string

	// Custom data for extensibility
	Custom map[string]interface{}
}

// InitEmailService initializes the email service
func InitEmailService() error {
	host := config.GetEnv("SMTP_HOST", "smtp.gmail.com")
	port := config.GetEnvInt("SMTP_PORT", 587)

	username := config.GetEnv("SMTP_USERNAME", "")
	password := config.GetEnv("SMTP_PASSWORD", "")
	fromAddr := config.GetEnv("SMTP_FROM", username)
	fromName := config.GetEnv("SMTP_FROM_NAME", "Sentrix")

	if username == "" || password == "" {
		return fmt.Errorf("SMTP_USERNAME and SMTP_PASSWORD are required")
	}

	frontendURL := config.GetEnv("FRONTEND_URL", "http://localhost:5173")
	appName := config.GetEnv("APP_NAME", "Sentrix")

	// Create mail client with best practices from go-mail docs
	client, err := mail.NewClient(
		host,
		mail.WithPort(port),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(username),
		mail.WithPassword(password),
		mail.WithTLSPolicy(mail.TLSMandatory),
		mail.WithTLSConfig(&tls.Config{
			InsecureSkipVerify: false,
			ServerName:         host,
		}),
		mail.WithTimeout(time.Duration(config.GetEnvInt("SMTP_TIMEOUT_SECONDS", 10))*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create email client: %w", err)
	}

	// Find template directory
	templateDir := config.GetEnv("EMAIL_TEMPLATE_DIR", "./templates")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		// Try relative to working directory
		templateDir = "ingestor/api_gateway/templates"
		if _, err := os.Stat(templateDir); os.IsNotExist(err) {
			return fmt.Errorf("templates directory not found")
		}
	}

	// Load email templates
	templates, err := loadTemplates(templateDir)
	if err != nil {
		return fmt.Errorf("failed to load email templates: %w", err)
	}

	logger.Info("Email service initialized with %d templates", len(templates))

	Email = &EmailService{
		client:       client,
		fromAddr:     fromAddr,
		fromName:     fromName,
		templates:    templates,
		templateDir:  templateDir,
		frontendURL:  frontendURL,
		dashboardURL: frontendURL,
		docsURL:      frontendURL + "/docs",
		appName:      appName,
	}

	return nil
}

// loadTemplates loads all email templates from the templates directory
// using template composition with a shared base template for shared CSS
func loadTemplates(dir string) (map[string]*template.Template, error) {
	templates := make(map[string]*template.Template)

	// Base template path with shared CSS/structure
	basePath := filepath.Join(dir, "base.html")

	templateFiles := []string{
		// Existing templates
		"verify-email.html",
		"reset-password.html",
		"welcome.html",
		"alert-notification.html",
		"ticket-notification.html",
		// Alert lifecycle
		"alert-acknowledged.html",
		"alert-resolved.html",
		"alert-dismissed.html",
		"alert-escalated.html",
		// User management
		"user-created-by-admin.html",
		"account-role-changed.html",
		"account-deactivated.html",
		"password-reset-by-admin.html",
		// SLA
		"sla-violation.html",
		"sla-weekly-report.html",
		"sla-monthly-report.html",
		// Maintenance
		"maintenance-upcoming.html",
		"maintenance-started.html",
		"maintenance-completed.html",
		// Escalation
		"escalation-triggered.html",
		"escalation-final.html",
		// On-call
		"oncall-rotation-reminder.html",
		"oncall-override-applied.html",
		"oncall-missed-alert.html",
		// Security
		"security-failed-logins.html",
		"security-account-locked.html",
		"security-new-device-login.html",
		"security-critical-admin-action.html",
		// Reports / digests
		"daily-operations-digest.html",
		"weekly-incident-review.html",
		"monthly-executive-summary.html",
		// Collaboration
		"ticket-sla-warning.html",
		"runbook-published.html",
		"ticket-mention.html",
	}

	for _, filename := range templateFiles {
		contentPath := filepath.Join(dir, filename)

		// Parse base template first, then content template
		// This allows content templates to use {{template "base" .}}
		tmpl, err := template.ParseFiles(basePath, contentPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", filename, err)
		}

		// Store without extension as key
		name := filename[:len(filename)-5] // Remove .html
		templates[name] = tmpl
	}

	return templates, nil
}

// SendVerificationEmail sends email verification link using template
func (e *EmailService) SendVerificationEmail(toEmail, username, verificationToken string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", e.frontendURL, verificationToken)

	data := e.baseEmailData()
	data.Username = username
	data.ActionURL = verifyURL
	data.VerifyURL = verifyURL // Backwards compatibility

	subject := fmt.Sprintf("Verify Your %s Account", e.appName)
	return e.sendTemplate(toEmail, subject, "verify-email", data)
}

// SendPasswordResetEmail sends password reset link using template
func (e *EmailService) SendPasswordResetEmail(toEmail, username, resetToken string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", e.frontendURL, resetToken)

	data := e.baseEmailData()
	data.Username = username
	data.ActionURL = resetURL
	data.ResetURL = resetURL // Backwards compatibility

	subject := fmt.Sprintf("Reset Your %s Password", e.appName)
	return e.sendTemplate(toEmail, subject, "reset-password", data)
}

// SendWelcomeEmail sends welcome email after verification using template
func (e *EmailService) SendWelcomeEmail(toEmail, username string) error {
	data := e.baseEmailData()
	data.Username = username
	data.DashboardURL = e.dashboardURL
	data.DocsURL = e.docsURL
	data.ActionURL = e.dashboardURL

	subject := fmt.Sprintf("Welcome to %s!", e.appName)
	return e.sendTemplate(toEmail, subject, "welcome", data)
}

// baseEmailData returns common email data with defaults
func (e *EmailService) baseEmailData() EmailData {
	return EmailData{
		AppName:      e.appName,
		Year:         time.Now().Year(),
		DashboardURL: e.dashboardURL,
		DocsURL:      e.docsURL,
		Custom:       make(map[string]interface{}),
	}
}

// sendTemplate renders a template and sends the email
func (e *EmailService) sendTemplate(toEmail, subject, templateName string, data EmailData) error {
	tmpl, ok := e.templates[templateName]
	if !ok {
		return fmt.Errorf("template %s not found", templateName)
	}

	// Add subject to data for title
	data.Subject = subject

	// Execute the base template (which includes the content block)
	var body bytes.Buffer
	if err := tmpl.ExecuteTemplate(&body, "base", data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return e.send(toEmail, subject, body.String())
}

// AlertEmailData holds the details needed for an alert notification email
type AlertEmailData struct {
	AlertID   string
	Title     string
	Severity  string
	Device    string
	SourceIP  string
	Category  string
	AISummary string
	Timestamp string
}

// SendAlertNotification sends an alert notification email to the given user
func (e *EmailService) SendAlertNotification(toEmail, username string, alert AlertEmailData) error {
	severityColors := map[string]string{
		"critical": "#da1e28",
		"high":     "#ff832b",
		"major":    "#ff832b",
		"medium":   "#f1c21b",
		"low":      "#24a148",
		"info":     "#0f62fe",
	}
	color := severityColors[alert.Severity]
	if color == "" {
		color = "#0f62fe"
	}

	data := e.baseEmailData()
	data.Username = username
	data.ActionURL = fmt.Sprintf("%s/alerts/%s", e.frontendURL, alert.AlertID)
	data.Custom = map[string]interface{}{
		"Title":         alert.Title,
		"Severity":      alert.Severity,
		"SeverityColor": color,
		"Device":        alert.Device,
		"SourceIP":      alert.SourceIP,
		"Category":      alert.Category,
		"AISummary":     alert.AISummary,
		"Timestamp":     alert.Timestamp,
	}

	subject := fmt.Sprintf("[%s] %s – %s", alert.Severity, alert.Device, alert.Title)
	return e.sendTemplate(toEmail, subject, "alert-notification", data)
}

// TicketEmailData holds the details needed for a ticket notification email
type TicketEmailData struct {
	TicketID      string
	Title         string
	Priority      string
	Status        string
	Assignee      string
	Category      string
	EventType     string // "Created", "Assigned", "Updated", "Commented"
	EventMessage  string // Human-readable event description
	Comment       string // Comment content (optional)
	CommentAuthor string // Comment author (optional)
}

// SendTicketNotification sends a ticket notification email to the given user
func (e *EmailService) SendTicketNotification(toEmail, username string, ticket TicketEmailData) error {
	priorityColors := map[string]string{
		"critical": "#da1e28",
		"high":     "#ff832b",
		"medium":   "#f1c21b",
		"low":      "#24a148",
	}
	color := priorityColors[ticket.Priority]
	if color == "" {
		color = "#0f62fe"
	}

	data := e.baseEmailData()
	data.Username = username
	data.ActionURL = fmt.Sprintf("%s/tickets/%s", e.frontendURL, ticket.TicketID)
	data.Custom = map[string]interface{}{
		"TicketID":      ticket.TicketID,
		"Title":         ticket.Title,
		"Priority":      ticket.Priority,
		"PriorityColor": color,
		"Status":        ticket.Status,
		"Assignee":      ticket.Assignee,
		"Category":      ticket.Category,
		"EventType":     ticket.EventType,
		"EventMessage":  ticket.EventMessage,
		"Comment":       ticket.Comment,
		"CommentAuthor": ticket.CommentAuthor,
	}

	subject := fmt.Sprintf("[Ticket %s] %s – %s", ticket.EventType, ticket.TicketID, ticket.Title)
	return e.sendTemplate(toEmail, subject, "ticket-notification", data)
}

// SendNotification is a generic method for sending any template-based notification.
// It accepts a template name, subject, recipient, and custom data map.
func (e *EmailService) SendNotification(toEmail, username, subject, templateName string, custom map[string]interface{}) error {
	data := e.baseEmailData()
	data.Username = username
	if actionURL, ok := custom["ActionURL"].(string); ok {
		data.ActionURL = actionURL
	} else {
		data.ActionURL = e.dashboardURL
	}
	data.Custom = custom
	return e.sendTemplate(toEmail, subject, templateName, data)
}

// BaseEmailData returns common email data (public, for test handler)
func (e *EmailService) BaseEmailData() EmailData {
	return e.baseEmailData()
}

// SendTemplatePublic is a public wrapper around sendTemplate for test/admin use
func (e *EmailService) SendTemplatePublic(toEmail, subject, templateName string, data EmailData) error {
	return e.sendTemplate(toEmail, subject, templateName, data)
}

// FrontendURL returns the configured frontend URL
func (e *EmailService) FrontendURL() string {
	return e.frontendURL
}

// send is the internal method to send emails
func (e *EmailService) send(toEmail, subject, htmlBody string) error {
	msg := mail.NewMsg()

	if err := msg.FromFormat(e.fromName, e.fromAddr); err != nil {
		return fmt.Errorf("failed to set From: %w", err)
	}

	if err := msg.To(toEmail); err != nil {
		return fmt.Errorf("failed to set To: %w", err)
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextHTML, htmlBody)

	// Send with retry
	if err := e.client.DialAndSend(msg); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
