package jsonrpc

import "fmt"

// Error returns the error message
func (e *InnerError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("code: %d, message: %s, data: %v", e.Code, e.Message, e.Data)
}

// NewParsingError creates a new parsing error
func NewParsingError(id RequestId, message string, data []byte) *Error {
	return NewError(id, NewInnerError(ParseError, message, data))
}

// NewInternalError creates a new internal error
func NewInternalError(id RequestId, message string, data []byte) *Error {
	return NewError(id, NewInnerError(InternalError, message, data))
}

// NewInvalidRequest creates a new invalid request error
func NewInvalidRequest(id RequestId, message string, data []byte) *Error {
	return NewError(id, NewInnerError(InvalidRequest, message, data))
}

// NewInvalidParamsError creates a new invalid params error
func NewInvalidParamsError(id RequestId, message string, data []byte) *Error {
	return NewError(id, NewInnerError(InvalidParams, message, data))
}

// NewMethodNotFound creates a new invalid request error
func NewMethodNotFound(id RequestId, message string, data []byte) *Error {
	return NewError(id, NewInnerError(MethodNotFound, message, data))
}
