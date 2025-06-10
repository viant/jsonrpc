package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

type Transport interface {
	Notifier
	Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error)
}

type Sequencer interface {
	NextRequestID() jsonrpc.RequestId
}
