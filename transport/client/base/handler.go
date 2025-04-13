package base

import (
	"context"
	"github.com/viant/jsonrpc"
)

// Handler represents a default handler
type Handler struct{}

func (h *Handler) Serve(_ context.Context, request *jsonrpc.Request, response *jsonrpc.Response) *jsonrpc.Error {
	response.Id = request.Id
	response.Jsonrpc = request.Jsonrpc
	return jsonrpc.NewMethodNotFound(request.Id, request.Method, nil)
}

func (h *Handler) OnNotification(_ context.Context, _ *jsonrpc.Notification) *jsonrpc.Error {
	//ignore
	return nil
}

func (h *Handler) OnError(_ context.Context, _ *jsonrpc.Error) *jsonrpc.Error {
	//ignore
	return nil
}
