package httptransport

import (
	"net/http"
)

// ErrorResponse wraps structured error information for HTTP responses
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail provides structured error information
type ErrorDetail struct {
	Code    string `json:"code"`    // e.g., INVALID_CURSOR, AUTH_REQUIRED
	Message string `json:"message"` // Human-readable message
	Op      string `json:"op"`      // Operation: push, pull, subscribe
}

// Error codes for consistent error handling
const (
	ErrCodeInvalidCursor      = "INVALID_CURSOR"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeAuthRequired       = "AUTH_REQUIRED"
	ErrCodeInvalidTenant      = "INVALID_TENANT"
	ErrCodeInvalidIdempotency = "INVALID_IDEMPOTENCY_KEY"
	ErrCodeConflict           = "CONFLICT"
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeTooLarge           = "REQUEST_TOO_LARGE"
)

// NewErrorResponse creates a structured error response
func NewErrorResponse(op, code, message string) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Op:      op,
		},
	}
}

// HTTPStatusFromCode maps error codes to HTTP status codes
func HTTPStatusFromCode(code string) int {
	switch code {
	case ErrCodeInvalidCursor, ErrCodeInvalidRequest, ErrCodeInvalidTenant, ErrCodeInvalidIdempotency:
		return http.StatusBadRequest
	case ErrCodeAuthRequired:
		return http.StatusUnauthorized
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeTooLarge:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

// respondWithStructuredError sends a structured error response
func respondWithStructuredError(w http.ResponseWriter, r *http.Request, op, code, message string, opts *ServerOptions) {
	resp := NewErrorResponse(op, code, message)
	status := HTTPStatusFromCode(code)
	respondWithJSON(w, r, status, resp, opts)
}
