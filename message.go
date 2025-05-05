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
)

// Message is a wrapper around the different types of JSON-RPC messages (Request, Notification, Response, Error).
type Message struct {
	Type                MessageType
	JsonRpcRequest      *Request
	JsonRpcNotification *Notification
	JsonRpcResponse     *Response
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

// NewError creates a new Error instance to represent the error that occurred.
func NewError(
	code int,
	message string,
	data interface{},
) *Error {

	var rawData []byte

	if data != nil {
		switch actual := data.(type) {
		case []byte:
			rawData = actual
		case string:
			rawData = []byte(actual)
		default:
			rawData, _ = json.Marshal(actual)
		}
	}

	return &Error{
		Code:    code,
		Message: message,
		Data:    rawData,
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
