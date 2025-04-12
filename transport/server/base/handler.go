package base

import (
	"context"
	"github.com/goccy/go-json"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/internal/collection"
	"github.com/viant/jsonrpc/transport/client/base"
)

// Handler represents a jsonrpc endpoint
type Handler struct {
	Sessions *collection.SyncMap[string, *Session]
}

func (e *Handler) HandleRequest(ctx context.Context, session *Session, data []byte) {
	messageType := base.MessageType(data)
	switch messageType {
	case jsonrpc.MessageTypeRequest:
		request := &jsonrpc.Request{}
		if err := json.Unmarshal(data, request); err != nil {
			session.SendError(ctx, jsonrpc.NewParsingError(nil, err, data))
			return
		}
		response := &jsonrpc.Response{Id: request.Id, Jsonrpc: request.Jsonrpc}
		if err := session.Handler.Serve(ctx, request, response); err != nil {
			session.SendError(ctx, jsonrpc.NewInternalError(request.Id, err, data))
			return
		}
		session.SendResponse(ctx, response)
	case jsonrpc.MessageTypeNotification:
		notification := &jsonrpc.Notification{}
		if err := json.Unmarshal(data, notification); err != nil {
			session.SendError(ctx, jsonrpc.NewParsingError(nil, err, data))
			return
		}
		if err := session.Handler.OnNotification(ctx, notification); err != nil {
			session.SendError(ctx, jsonrpc.NewInternalError(nil, err, data))
		}
	case jsonrpc.MessageTypeError:
		enError := &jsonrpc.Error{}
		if err := json.Unmarshal(data, enError); err != nil {
			session.SendError(ctx, jsonrpc.NewParsingError(nil, err, data))
			return
		}
		if err := session.Handler.OnError(ctx, enError); err != nil {
			session.SendError(ctx, jsonrpc.NewInternalError(nil, err, data))
		}
	}
}

func NewHandler() *Handler {
	return &Handler{
		Sessions: collection.NewSyncMap[string, *Session](),
	}
}
