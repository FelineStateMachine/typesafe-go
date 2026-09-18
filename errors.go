package typesafe

import (
	"fmt"
	"net/http"
)

// APIError describes a non-2xx HTTP response. Use errors.AsType to inspect it.
// Body and Header retain the server response and may contain sensitive data.
// Error deliberately excludes the body, credentials, and full URL.
type APIError struct {
	StatusCode int
	RequestID  string
	Header     http.Header
	Body       []byte
	Method     string
	URL        string
	Message    string
}

// Error returns an HTTP status summary without exposing response body contents.
func (e *APIError) Error() string {
	return fmt.Sprintf("typesafe: %s returned HTTP %d %s", e.Method, e.StatusCode, e.Message)
}

// ResponseError reports an invalid or oversized API response.
// RequestID can be used to correlate the failure with the API service.
type ResponseError struct {
	RequestID string
	Err       error
}

// Error returns a summary. Inspect Unwrap for the detailed decoding error.
func (e *ResponseError) Error() string {
	return "typesafe: invalid API response"
}

// Unwrap exposes the underlying response decoding or validation error.
func (e *ResponseError) Unwrap() error { return e.Err }
