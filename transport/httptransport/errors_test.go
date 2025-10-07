package httptransport

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		op      string
		code    string
		message string
		want    ErrorResponse
	}{
		{
			name:    "invalid cursor error",
			op:      "pull",
			code:    ErrCodeInvalidCursor,
			message: "cursor format invalid",
			want: ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrCodeInvalidCursor,
					Message: "cursor format invalid",
					Op:      "pull",
				},
			},
		},
		{
			name:    "auth required error",
			op:      "push",
			code:    ErrCodeAuthRequired,
			message: "missing authorization",
			want: ErrorResponse{
				Error: ErrorDetail{
					Code:    ErrCodeAuthRequired,
					Message: "missing authorization",
					Op:      "push",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewErrorResponse(tt.op, tt.code, tt.message)
			if got.Error.Code != tt.want.Error.Code {
				t.Errorf("NewErrorResponse() code = %v, want %v", got.Error.Code, tt.want.Error.Code)
			}
			if got.Error.Message != tt.want.Error.Message {
				t.Errorf("NewErrorResponse() message = %v, want %v", got.Error.Message, tt.want.Error.Message)
			}
			if got.Error.Op != tt.want.Error.Op {
				t.Errorf("NewErrorResponse() op = %v, want %v", got.Error.Op, tt.want.Error.Op)
			}
		})
	}
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{
			name: "invalid cursor - 400",
			code: ErrCodeInvalidCursor,
			want: http.StatusBadRequest,
		},
		{
			name: "invalid request - 400",
			code: ErrCodeInvalidRequest,
			want: http.StatusBadRequest,
		},
		{
			name: "invalid tenant - 400",
			code: ErrCodeInvalidTenant,
			want: http.StatusBadRequest,
		},
		{
			name: "invalid idempotency - 400",
			code: ErrCodeInvalidIdempotency,
			want: http.StatusBadRequest,
		},
		{
			name: "auth required - 401",
			code: ErrCodeAuthRequired,
			want: http.StatusUnauthorized,
		},
		{
			name: "not found - 404",
			code: ErrCodeNotFound,
			want: http.StatusNotFound,
		},
		{
			name: "conflict - 409",
			code: ErrCodeConflict,
			want: http.StatusConflict,
		},
		{
			name: "too large - 413",
			code: ErrCodeTooLarge,
			want: http.StatusRequestEntityTooLarge,
		},
		{
			name: "internal error - 500",
			code: ErrCodeInternal,
			want: http.StatusInternalServerError,
		},
		{
			name: "unknown code - 500",
			code: "UNKNOWN_ERROR",
			want: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTTPStatusFromCode(tt.code)
			if got != tt.want {
				t.Errorf("HTTPStatusFromCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorResponseJSONSerialization(t *testing.T) {
	resp := NewErrorResponse("pull", ErrCodeInvalidCursor, "invalid cursor format")

	// Serialize to JSON
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ErrorResponse: %v", err)
	}

	// Deserialize from JSON
	var decoded ErrorResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ErrorResponse: %v", err)
	}

	// Verify fields
	if decoded.Error.Code != ErrCodeInvalidCursor {
		t.Errorf("Deserialized code = %v, want %v", decoded.Error.Code, ErrCodeInvalidCursor)
	}
	if decoded.Error.Message != "invalid cursor format" {
		t.Errorf("Deserialized message = %v, want %v", decoded.Error.Message, "invalid cursor format")
	}
	if decoded.Error.Op != "pull" {
		t.Errorf("Deserialized op = %v, want %v", decoded.Error.Op, "pull")
	}
}

func TestRespondWithStructuredError(t *testing.T) {
	tests := []struct {
		name           string
		op             string
		code           string
		message        string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "invalid cursor error",
			op:             "pull",
			code:           ErrCodeInvalidCursor,
			message:        "cursor format invalid",
			expectedStatus: http.StatusBadRequest,
			expectedCode:   ErrCodeInvalidCursor,
		},
		{
			name:           "auth required error",
			op:             "push",
			code:           ErrCodeAuthRequired,
			message:        "missing token",
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   ErrCodeAuthRequired,
		},
		{
			name:           "internal error",
			op:             "subscribe",
			code:           ErrCodeInternal,
			message:        "database connection failed",
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   ErrCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request and response recorder
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			opts := DefaultServerOptions()

			// Call the function
			respondWithStructuredError(w, req, tt.op, tt.code, tt.message, opts)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("respondWithStructuredError() status = %v, want %v", w.Code, tt.expectedStatus)
			}

			// Check content type
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("respondWithStructuredError() Content-Type = %v, want application/json", contentType)
			}

			// Parse response body
			var resp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Verify error code
			if resp.Error.Code != tt.expectedCode {
				t.Errorf("respondWithStructuredError() error code = %v, want %v", resp.Error.Code, tt.expectedCode)
			}

			// Verify operation
			if resp.Error.Op != tt.op {
				t.Errorf("respondWithStructuredError() error op = %v, want %v", resp.Error.Op, tt.op)
			}

			// Verify message
			if resp.Error.Message != tt.message {
				t.Errorf("respondWithStructuredError() error message = %v, want %v", resp.Error.Message, tt.message)
			}
		})
	}
}

func TestErrorCodeConstants(t *testing.T) {
	// Verify all error code constants are properly defined
	codes := []string{
		ErrCodeInvalidCursor,
		ErrCodeInvalidRequest,
		ErrCodeAuthRequired,
		ErrCodeInvalidTenant,
		ErrCodeInvalidIdempotency,
		ErrCodeConflict,
		ErrCodeInternal,
		ErrCodeNotFound,
		ErrCodeTooLarge,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("Error code constant is empty")
		}
	}

	// Verify codes are unique
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("Duplicate error code: %v", code)
		}
		seen[code] = true
	}
}
