package jsonrpc

import (
	"errors"
	"fmt"
)

// UnauthorizedError represents an HTTP 401 Unauthorized error returned by a transport.
type UnauthorizedError struct {
	// StatusCode is the HTTP status code associated with the error (typically 401).
	StatusCode int
	// Body contains the raw response body, if available.
	Body []byte
}

// Error implements the error interface.
func (e *UnauthorizedError) Error() string {
	if len(e.Body) > 0 {
		return fmt.Sprintf("unauthorized (status %d): %s", e.StatusCode, string(e.Body))
	}
	return fmt.Sprintf("unauthorized (status %d)", e.StatusCode)
}

// NewUnauthorizedError constructs a new UnauthorizedError.
func NewUnauthorizedError(statusCode int, body []byte) *UnauthorizedError {
	return &UnauthorizedError{StatusCode: statusCode, Body: body}
}

// IsUnauthorized returns true if err is or wraps an UnauthorizedError.
func IsUnauthorized(err error) bool {
	var target *UnauthorizedError
	return errors.As(err, &target)
}

// HTTPStatusError preserves a rejected HTTP status without treating it as an
// OAuth challenge. Response bodies and URLs are deliberately not retained.
type HTTPStatusError struct{ StatusCode int }

func (e *HTTPStatusError) Error() string       { return fmt.Sprintf("invalid status code: %d", e.StatusCode) }
func (e *HTTPStatusError) HTTPStatusCode() int { return e.StatusCode }
func NewHTTPStatusError(statusCode int) *HTTPStatusError {
	return &HTTPStatusError{StatusCode: statusCode}
}
