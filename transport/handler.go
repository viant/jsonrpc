package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

type Handler interface {
	Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response)
	OnNotification(ctx context.Context, notification *jsonrpc.Notification)
}

// NewHandler is a function that creates a new Handler
type NewHandler func(ctx context.Context, transport Transport) Handler
