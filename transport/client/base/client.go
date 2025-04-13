package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/base"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	Transport
	Handler transport.Handler
	*transport.RoundTrips
	RunTimeout time.Duration
	Listener   jsonrpc.Listener
	counter    uint64
	err        error
}

func (c *Client) Notify(ctx context.Context, request *jsonrpc.Notification) error {
	return c.sendRequest(ctx, &jsonrpc.Request{
		Jsonrpc: jsonrpc.Version,
		Method:  request.Method,
		Params:  request.Params,
	})
}

func (c *Client) SetError(err error) {
	c.err = err
}

func (c *Client) Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error) {
	request.Id = int(atomic.AddUint64(&c.counter, 1))
	trip, err := c.send(ctx, request)
	if err != nil {
		return nil, err // send error
	}
	err = trip.Wait(ctx, c.RunTimeout)
	if err != nil {
		return nil, err
	}
	return trip.Response, err
}

func (c *Client) HandleMessage(ctx context.Context, data []byte) {
	messageType := base.MessageType(data)
	message := &jsonrpc.Message{Type: messageType}
	if c.Listener != nil {
		defer c.Listener(message)
	}
	switch messageType {
	case jsonrpc.MessageTypeNotification:
		c.handleOnNotification(ctx, data, message)
		return
	case jsonrpc.MessageTypeRequest:
		c.handleRequest(ctx, data, message)
		return
	}
	c.handleResponse(ctx, data, message)
}

func (c *Client) handleResponse(ctx context.Context, data []byte, message *jsonrpc.Message) {
	response := &jsonrpc.Response{}
	err := json.Unmarshal(data, response)
	if err != nil {
		fmt.Printf("failed to parse response: %v", err)
		return
	}
	message.JsonRpcResponse = response
	trip, err := c.RoundTrips.Match(response.Id)
	if err != nil {
		fmt.Printf("failed to parse response: %v", err)
		return

	}
	trip.SetResponse(response)

}

func (c *Client) handleRequest(ctx context.Context, data []byte, message *jsonrpc.Message) {
	response := &jsonrpc.Response{}
	request := &jsonrpc.Request{}
	if err := json.Unmarshal(data, request); err != nil {
		c.handleError(ctx, jsonrpc.NewParsingError(fmt.Sprintf("failed to parse request: %v", err), data))
		return
	}
	c.Handler.Serve(ctx, request, response)
	message.JsonRpcRequest = request
	message.JsonRpcResponse = response
	if err := c.sendResponse(ctx, response); err != nil {
		c.handleError(ctx, jsonrpc.NewInternalError(nil, err.Error(), data))
	}
}

//func (c *Client) handleOnError(ctx context.Context, data []byte, message *jsonrpc.Message) {
//	anError := &jsonrpc.Error{}
//	err := json.Unmarshal(data, anError)
//	if err != nil {
//		fmt.Printf("failed to parse error: %v, %s", err, data)
//		return
//	}
//	message.JsonRpcError = anError
//	trip, err := c.RoundTrips.Match(anError.Id)
//	if err != nil {
//		fmt.Printf("failed to match request: %v, %s", anError.Id, data)
//	}
//	if trip != nil {
//		trip.SetError(anError)
//	}
//}

func (c *Client) handleOnNotification(ctx context.Context, data []byte, message *jsonrpc.Message) {
	notification := &jsonrpc.Notification{}
	err := json.Unmarshal(data, notification)
	if err != nil {
		fmt.Printf("failed to parse notification: %v, %s", err, data)
	}
	c.Handler.OnNotification(ctx, notification)
}

func (c *Client) handleError(ctx context.Context, error *jsonrpc.Error) {
	data, err := json.Marshal(error)
	if err == nil {
		err = c.SendData(ctx, append(data, '\n'))
	}
	if err != nil {
		fmt.Printf("failed to send error: %v\n", err)
	}
}

func (c *Client) send(ctx context.Context, request *jsonrpc.Request) (*transport.RoundTrip, error) {
	if c.err != nil {
		return nil, c.err
	}
	trip, err := c.RoundTrips.Add(request)
	if err != nil {
		return nil, err
	}
	err = c.sendRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	return trip, nil
}

func (c *Client) sendRequest(ctx context.Context, request *jsonrpc.Request) error {
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}
	if !strings.HasSuffix(buffer.String(), "\n") {
		buffer.WriteByte('\n')
	}
	if c.Listener != nil {
		c.Listener(&jsonrpc.Message{Type: jsonrpc.MessageTypeRequest, JsonRpcRequest: request})
	}
	err = c.SendData(ctx, buffer.Bytes())
	return err
}

func (c *Client) sendResponse(ctx context.Context, response *jsonrpc.Response) error {
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}
	if !strings.HasSuffix(buffer.String(), "\n") {
		buffer.WriteByte('\n')
	}
	if c.Listener != nil {
		c.Listener(&jsonrpc.Message{Type: jsonrpc.MessageTypeResponse, JsonRpcResponse: response})
	}
	err = c.SendData(ctx, buffer.Bytes())
	return err
}
