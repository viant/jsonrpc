package sse

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"net/http"
	"time"
)

// Option is a function that configures the Client
type Option func(*Client)

// WithStreamingHTTPClient sets the HTTP streamingClient for the SSE streamingClient
func WithStreamingHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.transport.streamingClient = client
	}
}

func WithRPCHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.transport.rpcClient = client
	}
}

// WithHandshakeTimeout sets the handshake timeout for the SSE streamingClient
func WithHandshakeTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.handshakeTimeout = timeout
	}
}

// WithTrips sets the trips for the SSE streamingClient
func WithTrips(trips *transport.RoundTrips) Option {
	return func(c *Client) {
		c.base.RoundTrips = trips
	}
}

// WithListener set listener on http tips
func WithListener(listener jsonrpc.Listener) Option {
	return func(c *Client) {
		c.base.Listener = listener
	}
}

func WithHandler(handler transport.Handler) Option {
	return func(c *Client) {
		c.base.Handler = handler
	}
}

func WithRunTimeout(timeoutMs int) Option {
	return func(c *Client) {
		c.base.RunTimeout = time.Duration(timeoutMs) * time.Millisecond
	}
}

func WithLogger(logger jsonrpc.Logger) Option {
	return func(c *Client) {
		c.base.Logger = logger
	}
}
