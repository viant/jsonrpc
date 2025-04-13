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
	anError := jsonrpc.NewMethodNotFound(request.Id, fmt.Sprintf("method %v not found", request.Method), nil)
	response.Error = anError
}

func (h *Handler) OnNotification(_ context.Context, _ *jsonrpc.Notification) {
	//ignore
}
