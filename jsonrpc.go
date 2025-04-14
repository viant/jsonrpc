package jsonrpc

import (
	"encoding/json"
	"errors"
)

// RequestId is the type used to represent the id of a JSON-RPC request.
type RequestId any

// Error is used to provide additional information about the error that occurred.
type Error struct {
	// The error type that occurred.
	Code int `json:"code" yaml:"code" mapstructure:"code"`

	// Additional information about the error. The value of this member is defined by
	// the sender (e.g. detailed error information, nested errors etc.).
	Data interface{} `json:"data,omitempty" yaml:"data,omitempty" mapstructure:"data,omitempty"`

	// A short description of the error. The message SHOULD be limited to a concise
	// single sentence.
	Message string `json:"message" yaml:"message" mapstructure:"message"`
}

// Request represents a JSON-RPC request message.
type Request struct {
	// Id corresponds to the JSON schema field "id".
	Id RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	// Method corresponds to the JSON schema field "method".
	Method string `json:"method" yaml:"method" mapstructure:"method"`

	// Params corresponds to the JSON schema field "params".
	// It is stored as a []byte to enable efficient marshaling and unmarshaling into custom types later on in the protocol
	Params json.RawMessage `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params,omitempty"`
}

// UnmarshalJSON is a custom JSON unmarshaler for the Request type.
func (m *Request) UnmarshalJSON(data []byte) error {
	required := struct {
		Id      *RequestId       `json:"id" yaml:"id" mapstructure:"id"`
		Jsonrpc *string          `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Method  *string          `json:"method" yaml:"method" mapstructure:"method"`
		Params  *json.RawMessage `json:"params" yaml:"params" mapstructure:"params"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Id == nil {
		return errors.New("field id in Request: required")
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in Request: required")
	}
	if required.Method == nil {
		return errors.New("field method in Request: required")
	}
	if required.Params == nil {
		required.Params = new(json.RawMessage)
	}

	m.Id = *required.Id
	m.Jsonrpc = *required.Jsonrpc
	m.Method = *required.Method
	m.Params = *required.Params

	return nil
}

// Notification is a type representing a JSON-RPC notification message.
type Notification struct {
	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	// Method corresponds to the JSON schema field "method".
	Method string `json:"method" yaml:"method" mapstructure:"method"`

	// Params corresponds to the JSON schema field "params".
	// It is stored as a []byte to enable efficient marshaling and unmarshaling into custom types later on in the protocol
	Params json.RawMessage `json:"params,omitempty" yaml:"params,omitempty" mapstructure:"params,omitempty"`
}

// UnmarshalJSON is a custom JSON unmarshaler for the Notification type.
func (m *Notification) UnmarshalJSON(data []byte) error {
	required := struct {
		Jsonrpc *string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Method  *string `json:"method" yaml:"method" mapstructure:"method"`
		Id      *int64  `json:"id" yaml:"id" mapstructure:"id"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in Notifications: required")
	}
	if required.Method == nil {
		return errors.New("field method in Notifications: required")
	}
	if required.Id != nil {
		return errors.New("field id in Notifications: not allowed")
	}
	m.Jsonrpc = *required.Jsonrpc
	m.Method = *required.Method
	return nil
}

type Response struct {
	// Id corresponds to the JSON schema field "id".
	Id RequestId `json:"id" yaml:"id" mapstructure:"id"`

	// Jsonrpc corresponds to the JSON schema field "jsonrpc".
	Jsonrpc string `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`

	//Error
	Error *Error `json:"error" yaml:"error" mapstructure:"error"`

	// Result corresponds to the JSON schema field "result".
	Result json.RawMessage `json:"result,omitempty" yaml:"result" mapstructure:"result"`
}

// NewResponse creates a new Response instance with the specified id and data.
func NewResponse(id RequestId, data []byte) *Response {
	return &Response{
		Id:      id,      // Default to 0 for the id, this should be overridden by the caller
		Jsonrpc: Version, // Use the current JSON-RPC version
		Result:  data,    // Initialize with an empty raw message
	}
}

// UnmarshalJSON is a custom JSON unmarshaler for the Response type.
func (m *Response) UnmarshalJSON(data []byte) error {
	required := struct {
		Id      *RequestId       `json:"id" yaml:"id" mapstructure:"id"`
		Jsonrpc *string          `json:"jsonrpc" yaml:"jsonrpc" mapstructure:"jsonrpc"`
		Result  *json.RawMessage `json:"result" yaml:"result" mapstructure:"result"`
		Error   *Error           `json:"error" yaml:"error" mapstructure:"error"`
	}{}
	err := json.Unmarshal(data, &required)
	if err != nil {
		return err
	}
	if required.Id == nil {
		return errors.New("field id in Response: required")
	}
	if required.Jsonrpc == nil {
		return errors.New("field jsonrpc in Response: required")
	}
	m.Id = *required.Id
	m.Jsonrpc = *required.Jsonrpc
	if required.Result != nil {
		m.Result = *required.Result
	}
	m.Error = required.Error
	if required.Result == nil && required.Error == nil {
		return errors.New("field result in Response: required")
	}
	return err
}
