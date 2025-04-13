package base

import (
	"github.com/goccy/go-json"
	"github.com/viant/jsonrpc"
)

// MessageType returns message type
func MessageType(data []byte) jsonrpc.MessageType {
	probe := &probe{}
	_ = json.Unmarshal(data, probe)
	if probe.Id == nil {
		return jsonrpc.MessageTypeNotification
	}
	if probe.Method != "" {
		return jsonrpc.MessageTypeRequest
	}
	return jsonrpc.MessageTypeResponse
}

type probe struct {
	Id     jsonrpc.RequestId `json:"id"`
	Error  *jsonrpc.Error    `json:"error" yaml:"error"`
	Method string            `json:"method" yaml:"method"`
}
