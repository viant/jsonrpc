package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

type Handler interface {
	Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) *jsonrpc.Error
	OnNotification(ctx context.Context, notification *jsonrpc.Notification) *jsonrpc.Error
	OnError(ctx context.Context, error *jsonrpc.Error) *jsonrpc.Error
}

// NewHandler is a function that creates a new Handler
type NewHandler func(ctx context.Context, transport Transport) Handler
