package base

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/internal/collection"
	"github.com/viant/jsonrpc/transport/base"
)

// Handler represents a jsonrpc endpoint
type Handler struct {
	Sessions *collection.SyncMap[string, *Session]
}

func (e *Handler) HandleMessage(ctx context.Context, session *Session, data []byte) {
	messageType := base.MessageType(data)
	switch messageType {
	case jsonrpc.MessageTypeRequest:
		request := &jsonrpc.Request{}
		if err := json.Unmarshal(data, request); err != nil {
			session.SendError(ctx, jsonrpc.NewParsingError(fmt.Sprintf("failed to parse: %v", err), data))
			return
		}
		response := &jsonrpc.Response{Id: request.Id, Jsonrpc: request.Jsonrpc}
		session.Handler.Serve(ctx, request, response)
		session.SendResponse(ctx, response)
	case jsonrpc.MessageTypeResponse:
		response := &jsonrpc.Response{}
		if err := json.Unmarshal(data, response); err != nil {
			fmt.Printf("failed to parse response: %v", err)
			return
		}
		aTrip, err := session.RoundTrips.Match(response.Id)
		if err != nil {
			return
		}
		aTrip.SetResponse(response)

		//TODO move fmt.Printf to a logger to expose to implementers
	case jsonrpc.MessageTypeNotification:
		notification := &jsonrpc.Notification{}
		if err := json.Unmarshal(data, notification); err != nil {
			fmt.Printf("failed to parse notification: %v", err)
			return
		}
		session.Handler.OnNotification(ctx, notification)
	}
}

func NewHandler() *Handler {
	return &Handler{
		Sessions: collection.NewSyncMap[string, *Session](),
	}
}
