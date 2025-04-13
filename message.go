package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// MessageType is an enumeration of the types of messages in the JSON-RPC protocol.
type MessageType string

const (
	MessageTypeRequest      MessageType = "request"
	MessageTypeNotification MessageType = "notification"
	MessageTypeResponse     MessageType = "response"
	MessageTypeError        MessageType = "error"
)

// Message is a wrapper around the different types of JSON-RPC messages (Request, Notification, Response, Error).
type Message struct {
	Type                MessageType
	JsonRpcRequest      *Request
	JsonRpcNotification *Notification
	JsonRpcResponse     *Response
	JsonRpcError        *Error
}

func (m *Message) Method() string {
	switch m.Type {
	case MessageTypeRequest:
		return m.JsonRpcRequest.Method
	default:
		return ""
	}
}

// MarshalJSON is a custom JSON marshaler for the Message type.
func (m *Message) MarshalJSON() ([]byte, error) {
	switch m.Type {
	case MessageTypeRequest:
		return json.Marshal(m.JsonRpcRequest)
	case MessageTypeNotification:
		return json.Marshal(m.JsonRpcNotification)
	case MessageTypeResponse:
		return json.Marshal(m.JsonRpcResponse)
	case MessageTypeError:
		return json.Marshal(m.JsonRpcError)
	default:
		return nil, errors.New("unknown message type, couldn't marshal")
	}
}

// NewNotificationMessage creates a new JSON-RPC message of type Notification.
func NewNotificationMessage(notification *Notification) *Message {
	return &Message{
		Type:                MessageTypeNotification,
		JsonRpcNotification: notification,
	}
}

// NewRequestMessage creates a new JSON-RPC message of type Request.
func NewRequestMessage(request *Request) *Message {
	return &Message{
		Type:           MessageTypeRequest,
		JsonRpcRequest: request,
	}
}

// NewResponseMessage creates a new JSON-RPC message of type Response.
func NewResponseMessage(response *Response) *Message {
	return &Message{
		Type:            MessageTypeResponse,
		JsonRpcResponse: response,
	}
}

// NewErrorMessage creates a new JSON-RPC message of type Error.
func NewErrorMessage(error *Error) *Message {
	return &Message{
		Type: MessageTypeError,
		JsonRpcError: &Error{
			Error:   error.Error,
			Id:      error.Id,
			Jsonrpc: error.Jsonrpc,
		},
	}
}

// NewError creates a new JSON-RPC error response.
func NewError(
	requestId RequestId, // The id of the request this error corresponds to
	inner InnerError,
) *Error {
	return &Error{
		Error:   inner,
		Id:      requestId, // Default to 0 for the id, this should be overridden by the caller
		Jsonrpc: Version,   // Use the current JSON-RPC version
	}
}

// NewInnerError creates a new InnerError instance to represent the error that occurred.
func NewInnerError(
	code int,
	message string,
	data interface{},
) InnerError {
	return InnerError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func NewRequest(method string, parameters interface{}) (*Request, error) {
	req := &Request{Jsonrpc: Version, Method: method}
	var err error
	req.Params, err = asParameters(method, parameters)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func asParameters(method string, parameters interface{}) (json.RawMessage, error) {
	switch actual := parameters.(type) {
	case string:
		return []byte(actual), nil
	case []byte:
		return actual, nil
	case json.RawMessage:
		return actual, nil
	default:
		data, err := json.Marshal(actual)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal jsonrpc request parameter: [method:%v, parameters: %+v] %w", method, parameters, err)
		}
		return data, nil
	}
}
