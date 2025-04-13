package jsonrpc

import "fmt"

// Error returns the error message
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
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
