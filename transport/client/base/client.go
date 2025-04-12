package base

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"strings"
	"sync/atomic"
	"time"
)

type Client struct {
	Transport
	*transport.RouteTrips
	RunTimeout time.Duration

	Listener jsonrpc.Listener
	counter  uint64
	err      error
}

func (c *Client) Notification() chan *jsonrpc.Notification {
	return c.RouteTrips.Notifications()
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
	request.Id = atomic.AddUint64(&c.counter, 1)
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

func (c *Client) HandleMessage(ctx context.Context, data []byte) bool {
	messageType := MessageType(data)
	message := &jsonrpc.Message{Type: messageType}
	if c.Listener != nil {
		defer c.Listener(message)
	}
	switch messageType {
	case jsonrpc.MessageTypeNotification:
		notification := &jsonrpc.Notification{}
		err := json.Unmarshal(data, notification)
		if err != nil {
			c.handleError(ctx, jsonrpc.NewParsingError(nil, fmt.Errorf("failed to parse notification: %w", err), data))
			return true
		}
		message.JsonRpcNotification = notification
		if err = c.RouteTrips.Notify(notification); err != nil {
			c.handleError(ctx, jsonrpc.NewParsingError(nil, fmt.Errorf("failed send notification: %w", err), data))
		}
		return true
	case jsonrpc.MessageTypeError:
		enError := &jsonrpc.Error{}
		err := json.Unmarshal(data, enError)
		if err != nil {
			c.handleError(ctx, jsonrpc.NewParsingError(nil, fmt.Errorf("failed to parse error: %w", err), data))
			return true
		}
		message.JsonRpcError = enError
		return true
	}
	response := &jsonrpc.Response{}
	err := json.Unmarshal(data, response)
	if err != nil {
		c.handleError(ctx, jsonrpc.NewParsingError(nil, fmt.Errorf("failed to parse enError: %w", err), data))
		return true
	}
	message.JsonRpcResponse = response
	trip, err := c.RouteTrips.Match(response.Id)
	if err != nil {
		c.handleError(ctx, jsonrpc.NewInvalidRequest(nil, err, data))
	} else {
		trip.SetResponse(response)
	}
	return false
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

func (c *Client) send(ctx context.Context, request *jsonrpc.Request) (*transport.RouteTrip, error) {
	if c.err != nil {
		return nil, c.err
	}
	trip, err := c.RouteTrips.Add(request)
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
