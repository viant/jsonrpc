package base

import (
	"context"
	"github.com/viant/jsonrpc"
)

// Handler represents a default handler
type Handler struct{}

func (h *Handler) Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) *jsonrpc.Error {
	response.Id = request.Id
	response.Jsonrpc = request.Jsonrpc
	return jsonrpc.NewMethodNotFound(request.Id, request.Method, nil)
}

func (h *Handler) OnNotification(ctx context.Context, notification *jsonrpc.Notification) *jsonrpc.Error {
	//ignore
	return nil
}

func (h *Handler) OnError(ctx context.Context, error *jsonrpc.Error) *jsonrpc.Error {
	//ignore
	return nil
}
