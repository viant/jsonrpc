package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

type Handler interface {
	Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) error
	OnNotification(ctx context.Context, notification *jsonrpc.Notification) error
	OnError(ctx context.Context, error *jsonrpc.Error) error
}
