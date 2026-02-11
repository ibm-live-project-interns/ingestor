package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
	"github.com/ibm-live-project-interns/ingestor/shared/httpclient"
	"github.com/ibm-live-project-interns/ingestor/shared/logger"
)

// RouteConfig holds routing configuration
type RouteConfig struct {
	// Routes maps event type/severity to destination URL
	Routes map[string]string
	// Default route for unmatched events
	DefaultRoute string
	// AI Core URL for AI processing
	AICoreURL string
	// API Gateway URL for storage
	APIGatewayURL string
	// Whether to send to AI Core first
	EnableAI bool
}

// DefaultRouteConfig returns default routing configuration
func DefaultRouteConfig() RouteConfig {
	return RouteConfig{
		Routes:        make(map[string]string),
		DefaultRoute:  config.GetEnv("DEFAULT_ROUTE_URL", ""),
		AICoreURL:     config.GetEnv("AI_CORE_URL", "http://ai-core:9000"),
		APIGatewayURL: config.GetEnv("API_GATEWAY_URL", "http://api-gateway:8080"),
		EnableAI:      config.GetEnvBool("ENABLE_AI_PROCESSING", true),
	}
}

// LoadRouteConfigFromFile loads route config from a JSON file
func LoadRouteConfigFromFile(path string) (RouteConfig, error) {
	cfg := DefaultRouteConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, errors.NewInternal(fmt.Sprintf("failed to read config file: %s", path)).WithError(err)
	}

	if err := json.Unmarshal(data, &cfg.Routes); err != nil {
		return cfg, errors.NewInternal("failed to parse route config").WithError(err)
	}

	return cfg, nil
}

// Router handles event routing
type Router struct {
	config    RouteConfig
	aiClient  *httpclient.Client
	apiClient *httpclient.Client
	clients   map[string]*httpclient.Client
	mu        sync.RWMutex
}

// NewRouter creates a new router
func NewRouter(cfg RouteConfig) *Router {
	r := &Router{
		config:  cfg,
		clients: make(map[string]*httpclient.Client),
	}

	if cfg.AICoreURL != "" {
		r.aiClient = httpclient.NewClientWithBaseURL(cfg.AICoreURL)
	}

	if cfg.APIGatewayURL != "" {
		r.apiClient = httpclient.NewClientWithBaseURL(cfg.APIGatewayURL)
	}

	return r
}

// RoutedEvent represents an event to be routed
type RoutedEvent struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	SourceHost string `json:"source_host,omitempty"`
	SourceIP   string `json:"source_ip,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	Category   string `json:"category,omitempty"`
	Severity   string `json:"severity,omitempty"`
}

// AIEnrichedEvent represents an event with AI analysis
type AIEnrichedEvent struct {
	RoutedEvent
	AISeverity          string `json:"ai_severity,omitempty"`
	AIExplanation       string `json:"ai_explanation,omitempty"`
	AIRecommendedAction string `json:"ai_recommended_action,omitempty"`
}

// RouteResult represents the result of routing
type RouteResult struct {
	Success     bool   `json:"success"`
	ForwardedTo string `json:"forwarded_to"`
	AIProcessed bool   `json:"ai_processed,omitempty"`
	Response    string `json:"response,omitempty"`
	Error       string `json:"error,omitempty"`
}

// Route routes an event to the appropriate destination
func (r *Router) Route(ctx context.Context, event RoutedEvent) (*RouteResult, error) {
	result := &RouteResult{}

	// Step 1: Send to AI Core if enabled
	if r.config.EnableAI && r.aiClient != nil {
		aiReq := map[string]string{
			"type":    event.Type,
			"message": event.Message,
		}

		logger.Debug("Routing event to AI Core: %s", r.config.AICoreURL)
		resp, err := r.aiClient.Post(ctx, "/events", aiReq, nil)

		if err != nil {
			logger.Warn("AI processing failed, continuing without AI: %v", err)
		} else if resp.IsSuccess() {
			result.AIProcessed = true

			var aiResp struct {
				Severity          string `json:"severity"`
				Explanation       string `json:"explanation"`
				RecommendedAction string `json:"recommended_action"`
			}

			if err := resp.JSON(&aiResp); err == nil {
				// Update event with AI analysis
				event.Severity = aiResp.Severity
			}
		}
	}

	// Step 2: Determine destination URL
	destURL := r.getDestination(event)
	if destURL == "" {
		return nil, errors.NewRoutingFailed("no destination", fmt.Errorf("no route for type: %s", event.Type))
	}

	// Step 3: Forward to destination
	logger.Debug("Forwarding event to: %s", destURL)
	client := r.getClient(destURL)
	resp, err := client.Post(ctx, "", event, nil)
	if err != nil {
		return nil, errors.NewRoutingFailed(destURL, err)
	}

	result.Success = resp.IsSuccess()
	result.ForwardedTo = destURL
	result.Response = string(resp.Body)

	if !resp.IsSuccess() {
		result.Error = fmt.Sprintf("destination returned status %d", resp.StatusCode)
	}

	return result, nil
}

// getDestination determines the destination URL for an event
func (r *Router) getDestination(event RoutedEvent) string {
	// Check by type/severity
	if url, ok := r.config.Routes[event.Type]; ok {
		return url
	}

	if url, ok := r.config.Routes[event.Severity]; ok {
		return url
	}

	// Use default route
	if r.config.DefaultRoute != "" {
		return r.config.DefaultRoute
	}

	// Fall back to API Gateway internal endpoint
	if r.config.APIGatewayURL != "" {
		return r.config.APIGatewayURL + "/api/internal/events"
	}

	return ""
}

// getClient gets or creates an HTTP client for a URL
func (r *Router) getClient(baseURL string) *httpclient.Client {
	r.mu.RLock()
	client, exists := r.clients[baseURL]
	r.mu.RUnlock()

	if exists {
		return client
	}

	// Double-check after acquiring write lock
	r.mu.Lock()
	defer r.mu.Unlock()

	if client, exists = r.clients[baseURL]; exists {
		return client
	}

	client = httpclient.NewClientWithBaseURL(baseURL)
	r.clients[baseURL] = client
	return client
}
