package base

import (
	"github.com/goccy/go-json"
	"github.com/viant/jsonrpc"
)

// MessageType returns message type
func MessageType(data []byte) jsonrpc.MessageType {
	probe := &probe{}
	_ = json.Unmarshal(data, probe)
	if probe.Error != nil {
		return jsonrpc.MessageTypeError
	}
	if probe.Id == nil {
		return jsonrpc.MessageTypeNotification
	}
	return jsonrpc.MessageTypeRequest
}

type probe struct {
	Id    jsonrpc.RequestId   `json:"id"`
	Error *jsonrpc.InnerError `json:"error" yaml:"error"`
}
