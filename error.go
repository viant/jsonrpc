package jsonrpc

// NewParsingError creates a new parsing error
func NewParsingError(id RequestId, err error, data []byte) *Error {
	return NewError(id, NewInnerError(ParseError, err.Error(), data))
}

// NewInternalError creates a new internal error
func NewInternalError(id RequestId, err error, data []byte) *Error {
	return NewError(id, NewInnerError(InternalError, err.Error(), data))
}

// NewInvalidRequest creates a new invalid request error
func NewInvalidRequest(id RequestId, err error, data []byte) *Error {
	return NewError(id, NewInnerError(InvalidRequest, err.Error(), data))
}

// NewInvalidParams creates a new invalid params error
func NewInvalidParams(id RequestId, err error, data []byte) *Error {
	return NewError(id, NewInnerError(InvalidParams, err.Error(), data))
}

// NewMethodNotFound creates a new invalid request error
func NewMethodNotFound(id RequestId, err error, data []byte) *Error {
	return NewError(id, NewInnerError(MethodNotFound, err.Error(), data))
}
