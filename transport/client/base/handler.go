package base

import (
	"context"
	"fmt"
	"github.com/viant/jsonrpc"
)

// Handler represents a default handler
type Handler struct{}

func (h *Handler) Serve(_ context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
	response.Id = request.Id
	response.Jsonrpc = request.Jsonrpc
	response.Error = jsonrpc.NewMethodNotFound(fmt.Sprintf("method %v not found", request.Method), nil)
}

func (h *Handler) OnNotification(_ context.Context, _ *jsonrpc.Notification) {
	//ignore
}
