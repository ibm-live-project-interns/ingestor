package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents a standard error code
type ErrorCode string

// Standard error codes - These MUST match what the UI expects
const (
	// ==========================================
	// Authentication errors (401)
	// ==========================================
	ErrCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrCodeTokenExpired       ErrorCode = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       ErrorCode = "TOKEN_INVALID"
	ErrCodeTokenMissing       ErrorCode = "TOKEN_MISSING"
	ErrCodeSessionExpired     ErrorCode = "SESSION_EXPIRED"
	ErrCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"

	// ==========================================
	// Authorization errors (403)
	// ==========================================
	ErrCodeForbidden        ErrorCode = "FORBIDDEN"
	ErrCodeInsufficientRole ErrorCode = "INSUFFICIENT_ROLE"
	ErrCodePermissionDenied ErrorCode = "PERMISSION_DENIED"
	ErrCodeAccessDenied     ErrorCode = "ACCESS_DENIED"

	// ==========================================
	// Client errors (4xx)
	// ==========================================
	ErrCodeBadRequest       ErrorCode = "BAD_REQUEST"
	ErrCodeNotFound         ErrorCode = "NOT_FOUND"
	ErrCodeConflict         ErrorCode = "CONFLICT"
	ErrCodeValidation       ErrorCode = "VALIDATION_ERROR"
	ErrCodeRateLimited      ErrorCode = "RATE_LIMITED"
	ErrCodePayloadTooLarge  ErrorCode = "PAYLOAD_TOO_LARGE"
	ErrCodeMethodNotAllowed ErrorCode = "METHOD_NOT_ALLOWED"
	ErrCodeUnsupportedMedia ErrorCode = "UNSUPPORTED_MEDIA_TYPE"

	// ==========================================
	// Server errors (5xx)
	// ==========================================
	ErrCodeInternal        ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceDown     ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout         ErrorCode = "TIMEOUT"
	ErrCodeUpstreamFailure ErrorCode = "UPSTREAM_FAILURE"
	ErrCodeDatabaseError   ErrorCode = "DATABASE_ERROR"
	ErrCodeConfigError     ErrorCode = "CONFIGURATION_ERROR"

	// ==========================================
	// AI Processing errors
	// ==========================================
	ErrCodeAIProcessing      ErrorCode = "AI_PROCESSING_ERROR"
	ErrCodeAIUnavailable     ErrorCode = "AI_UNAVAILABLE"
	ErrCodeAITimeout         ErrorCode = "AI_TIMEOUT"
	ErrCodeAIRateLimited     ErrorCode = "AI_RATE_LIMITED"
	ErrCodeAIInvalidResponse ErrorCode = "AI_INVALID_RESPONSE"

	// ==========================================
	// Business logic errors
	// ==========================================
	ErrCodeEventInvalid     ErrorCode = "INVALID_EVENT"
	ErrCodeRoutingFailed    ErrorCode = "ROUTING_FAILED"
	ErrCodeEnrichmentFailed ErrorCode = "ENRICHMENT_FAILED"
	ErrCodeAlertNotFound    ErrorCode = "ALERT_NOT_FOUND"
	ErrCodeTicketNotFound   ErrorCode = "TICKET_NOT_FOUND"
	ErrCodeDeviceNotFound   ErrorCode = "DEVICE_NOT_FOUND"
	ErrCodeDuplicateEntry   ErrorCode = "DUPLICATE_ENTRY"
	ErrCodeInvalidStatus    ErrorCode = "INVALID_STATUS"
	ErrCodeInvalidSeverity  ErrorCode = "INVALID_SEVERITY"
)

// APIError represents a standardized API error
type APIError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	HTTPStatus int       `json:"-"`
	Err        error     `json:"-"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *APIError) Unwrap() error {
	return e.Err
}

// WithDetails adds details to the error
func (e *APIError) WithDetails(details string) *APIError {
	e.Details = details
	return e
}

// WithError wraps an underlying error
func (e *APIError) WithError(err error) *APIError {
	e.Err = err
	return e
}

// ErrorResponse is the JSON structure returned to clients
type ErrorResponse struct {
	Error   ErrorCode `json:"error"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
}

// ToResponse converts APIError to ErrorResponse
func (e *APIError) ToResponse() ErrorResponse {
	return ErrorResponse{
		Error:   e.Code,
		Message: e.Message,
		Details: e.Details,
	}
}

// ===== Error constructors =====

// NewBadRequest creates a bad request error
func NewBadRequest(message string) *APIError {
	return &APIError{
		Code:       ErrCodeBadRequest,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewUnauthorized creates an unauthorized error
func NewUnauthorized(message string) *APIError {
	return &APIError{
		Code:       ErrCodeUnauthorized,
		Message:    message,
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewForbidden creates a forbidden error
func NewForbidden(message string) *APIError {
	return &APIError{
		Code:       ErrCodeForbidden,
		Message:    message,
		HTTPStatus: http.StatusForbidden,
	}
}

// NewNotFound creates a not found error
func NewNotFound(resource string) *APIError {
	return &APIError{
		Code:       ErrCodeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		HTTPStatus: http.StatusNotFound,
	}
}

// NewValidation creates a validation error
func NewValidation(message string) *APIError {
	return &APIError{
		Code:       ErrCodeValidation,
		Message:    message,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewRateLimited creates a rate limited error
func NewRateLimited(retryAfter string) *APIError {
	return &APIError{
		Code:       ErrCodeRateLimited,
		Message:    "Rate limit exceeded",
		Details:    fmt.Sprintf("Retry after: %s", retryAfter),
		HTTPStatus: http.StatusTooManyRequests,
	}
}

// NewInternal creates an internal server error
func NewInternal(message string) *APIError {
	return &APIError{
		Code:       ErrCodeInternal,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
	}
}

// NewServiceUnavailable creates a service unavailable error
func NewServiceUnavailable(service string) *APIError {
	return &APIError{
		Code:       ErrCodeServiceDown,
		Message:    fmt.Sprintf("%s is currently unavailable", service),
		HTTPStatus: http.StatusServiceUnavailable,
	}
}

// NewTimeout creates a timeout error
func NewTimeout(operation string) *APIError {
	return &APIError{
		Code:       ErrCodeTimeout,
		Message:    fmt.Sprintf("%s timed out", operation),
		HTTPStatus: http.StatusGatewayTimeout,
	}
}

// NewUpstreamFailure creates an upstream failure error
func NewUpstreamFailure(service string, err error) *APIError {
	return &APIError{
		Code:       ErrCodeUpstreamFailure,
		Message:    fmt.Sprintf("Failed to reach %s", service),
		HTTPStatus: http.StatusBadGateway,
		Err:        err,
	}
}

// NewAIProcessingError creates an AI processing error
func NewAIProcessingError(message string, err error) *APIError {
	return &APIError{
		Code:       ErrCodeAIProcessing,
		Message:    message,
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

// NewDatabaseError creates a database error
func NewDatabaseError(operation string, err error) *APIError {
	return &APIError{
		Code:       ErrCodeDatabaseError,
		Message:    fmt.Sprintf("Database %s failed", operation),
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

// NewEventInvalid creates an invalid event error
func NewEventInvalid(reason string) *APIError {
	return &APIError{
		Code:       ErrCodeEventInvalid,
		Message:    reason,
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewRoutingFailed creates a routing failed error
func NewRoutingFailed(destination string, err error) *APIError {
	return &APIError{
		Code:       ErrCodeRoutingFailed,
		Message:    fmt.Sprintf("Failed to route to %s", destination),
		HTTPStatus: http.StatusBadGateway,
		Err:        err,
	}
}

// ==========================================
// Auth error constructors
// ==========================================

// NewTokenExpired creates a token expired error
func NewTokenExpired() *APIError {
	return &APIError{
		Code:       ErrCodeTokenExpired,
		Message:    "Token has expired",
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewTokenInvalid creates a token invalid error
func NewTokenInvalid() *APIError {
	return &APIError{
		Code:       ErrCodeTokenInvalid,
		Message:    "Token is invalid",
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewTokenMissing creates a token missing error
func NewTokenMissing() *APIError {
	return &APIError{
		Code:       ErrCodeTokenMissing,
		Message:    "Authorization token is required",
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewInvalidCredentials creates an invalid credentials error
func NewInvalidCredentials() *APIError {
	return &APIError{
		Code:       ErrCodeInvalidCredentials,
		Message:    "Invalid username or password",
		HTTPStatus: http.StatusUnauthorized,
	}
}

// ==========================================
// Authorization error constructors
// ==========================================

// NewInsufficientRole creates an insufficient role error
func NewInsufficientRole(required string) *APIError {
	return &APIError{
		Code:       ErrCodeInsufficientRole,
		Message:    fmt.Sprintf("Insufficient role: requires %s", required),
		HTTPStatus: http.StatusForbidden,
	}
}

// NewPermissionDenied creates a permission denied error
func NewPermissionDenied(permission string) *APIError {
	return &APIError{
		Code:       ErrCodePermissionDenied,
		Message:    fmt.Sprintf("Permission denied: %s required", permission),
		HTTPStatus: http.StatusForbidden,
	}
}

// ==========================================
// Resource error constructors
// ==========================================

// NewAlertNotFound creates an alert not found error
func NewAlertNotFound(id string) *APIError {
	return &APIError{
		Code:       ErrCodeAlertNotFound,
		Message:    fmt.Sprintf("Alert %s not found", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// NewTicketNotFound creates a ticket not found error
func NewTicketNotFound(id string) *APIError {
	return &APIError{
		Code:       ErrCodeTicketNotFound,
		Message:    fmt.Sprintf("Ticket %s not found", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// NewDeviceNotFound creates a device not found error
func NewDeviceNotFound(id string) *APIError {
	return &APIError{
		Code:       ErrCodeDeviceNotFound,
		Message:    fmt.Sprintf("Device %s not found", id),
		HTTPStatus: http.StatusNotFound,
	}
}

// ==========================================
// AI error constructors
// ==========================================

// NewAIUnavailable creates an AI unavailable error
func NewAIUnavailable() *APIError {
	return &APIError{
		Code:       ErrCodeAIUnavailable,
		Message:    "AI service is currently unavailable",
		HTTPStatus: http.StatusServiceUnavailable,
	}
}

// NewAITimeout creates an AI timeout error
func NewAITimeout() *APIError {
	return &APIError{
		Code:       ErrCodeAITimeout,
		Message:    "AI processing timed out",
		HTTPStatus: http.StatusGatewayTimeout,
	}
}

// NewAIRateLimited creates an AI rate limited error
func NewAIRateLimited() *APIError {
	return &APIError{
		Code:       ErrCodeAIRateLimited,
		Message:    "AI service rate limit exceeded",
		HTTPStatus: http.StatusTooManyRequests,
	}
}

// ==========================================
// Validation error constructors
// ==========================================

// NewInvalidSeverity creates an invalid severity error
func NewInvalidSeverity(value string) *APIError {
	return &APIError{
		Code:       ErrCodeInvalidSeverity,
		Message:    fmt.Sprintf("Invalid severity: %s. Must be critical, high, medium, low, or info", value),
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewInvalidStatus creates an invalid status error
func NewInvalidStatus(value string) *APIError {
	return &APIError{
		Code:       ErrCodeInvalidStatus,
		Message:    fmt.Sprintf("Invalid status: %s", value),
		HTTPStatus: http.StatusBadRequest,
	}
}

// NewDuplicateEntry creates a duplicate entry error
func NewDuplicateEntry(resource string) *APIError {
	return &APIError{
		Code:       ErrCodeDuplicateEntry,
		Message:    fmt.Sprintf("%s already exists", resource),
		HTTPStatus: http.StatusConflict,
	}
}
