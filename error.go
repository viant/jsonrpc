package jsonrpc

import (
	"encoding/json"
	"fmt"
)

// Error returns the error message
func (e *Error) Error() string {
	return fmt.Sprintf("code: %d, message: %s, data: %v", e.Code, e.Message, e.Data)
}

// NewParsingError creates a new parsing error
func NewParsingError(message string, data []byte) *Error {
	return NewError(ParseError, message, data)
}

// NewInternalError creates a new internal error
func NewInternalError(message string, data []byte) *Error {
	return NewError(InternalError, message, data)
}

// NewInvalidRequest creates a new invalid request error
func NewInvalidRequest(message string, data []byte) *Error {
	return NewError(InvalidRequest, message, data)
}

// NewInvalidParamsError creates a new invalid params error
func NewInvalidParamsError(message string, data []byte) *Error {
	return NewError(InvalidParams, message, data)
}

// NewMethodNotFound creates a new invalid request error
func NewMethodNotFound(message string, data []byte) *Error {
	return NewError(MethodNotFound, message, data)
}

// WithCause retains the originating local failure without changing the wire
// error. The original remains unchanged; received wire errors have no cause.
func (e *Error) WithCause(cause error) *Error {
	if e == nil {
		return nil
	}
	copy := *e
	copy.cause = cause
	return &copy
}
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// UnmarshalJSON discards local provenance even when a decoder reuses an Error.
func (e *Error) UnmarshalJSON(data []byte) error {
	type wireError Error
	if e != nil {
		e.cause = nil
	}
	return json.Unmarshal(data, (*wireError)(e))
}
