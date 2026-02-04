package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
	"github.com/ibm-live-project-interns/ingestor/shared/errors"
)

// ClientConfig holds HTTP client configuration
type ClientConfig struct {
	// Base URL for all requests
	BaseURL string
	// Request timeout
	Timeout time.Duration
	// Maximum retries for failed requests
	MaxRetries int
	// Initial retry delay (will be doubled for each retry)
	RetryDelay time.Duration
	// Custom headers to add to all requests
	Headers map[string]string
	// Skip SSL verification (not recommended for production)
	InsecureSkipVerify bool
}

// DefaultClientConfig returns sensible defaults
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		BaseURL:            "",
		Timeout:            time.Duration(config.GetEnvInt("HTTP_CLIENT_TIMEOUT_SECONDS", 30)) * time.Second,
		MaxRetries:         config.GetEnvInt("HTTP_CLIENT_MAX_RETRIES", 3),
		RetryDelay:         time.Duration(config.GetEnvInt("HTTP_CLIENT_RETRY_DELAY_MS", 1000)) * time.Millisecond,
		Headers:            make(map[string]string),
		InsecureSkipVerify: false,
	}
}

// Client is a reusable HTTP client with retry logic
type Client struct {
	config     ClientConfig
	httpClient *http.Client
}

// NewClient creates a new HTTP client
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// NewClientWithBaseURL creates a client with just a base URL
func NewClientWithBaseURL(baseURL string) *Client {
	cfg := DefaultClientConfig()
	cfg.BaseURL = baseURL
	return NewClient(cfg)
}

// Response wraps an HTTP response
type Response struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// JSON unmarshals the response body into the provided value
func (r *Response) JSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// IsSuccess returns true if the status code is 2xx
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// Request performs an HTTP request with retry logic
func (c *Client) Request(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*Response, error) {
	url := c.config.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, errors.NewInternal("failed to marshal request body").WithError(err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		// Create request
		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			return nil, errors.NewInternal("failed to create request").WithError(err)
		}

		// Set default headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		// Set config headers
		for k, v := range c.config.Headers {
			req.Header.Set(k, v)
		}

		// Set request-specific headers
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		// Make request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = errors.NewUpstreamFailure(url, err)

			// Retry on network errors
			if attempt < c.config.MaxRetries {
				time.Sleep(c.config.RetryDelay * time.Duration(attempt+1))
				continue
			}
			return nil, lastErr
		}
		defer resp.Body.Close()

		// Read body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errors.NewInternal("failed to read response body").WithError(err)
		}

		response := &Response{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
		}

		// Don't retry on client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return response, nil
		}

		// Retry on server errors (5xx)
		if resp.StatusCode >= 500 {
			lastErr = errors.NewUpstreamFailure(url, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody)))

			if attempt < c.config.MaxRetries {
				time.Sleep(c.config.RetryDelay * time.Duration(attempt+1))
				continue
			}
			return response, lastErr
		}

		return response, nil
	}

	return nil, lastErr
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, headers map[string]string) (*Response, error) {
	return c.Request(ctx, http.MethodGet, path, nil, headers)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body interface{}, headers map[string]string) (*Response, error) {
	return c.Request(ctx, http.MethodPost, path, body, headers)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body interface{}, headers map[string]string) (*Response, error) {
	return c.Request(ctx, http.MethodPut, path, body, headers)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string, headers map[string]string) (*Response, error) {
	return c.Request(ctx, http.MethodDelete, path, nil, headers)
}

// HealthCheck performs a health check on the specified path
func (c *Client) HealthCheck(ctx context.Context, path string) error {
	resp, err := c.Get(ctx, path, nil)
	if err != nil {
		return err
	}
	if !resp.IsSuccess() {
		return errors.NewServiceUnavailable(c.config.BaseURL)
	}
	return nil
}
