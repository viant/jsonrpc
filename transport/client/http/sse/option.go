package sse

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"net/http"
	"time"
)

// Option is a function that configures the Client
type Option func(*Client)

// WithClient sets the HTTP client for the SSE client
func WithClient(client *http.Client) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithHandshakeTimeout sets the handshake timeout for the SSE client
func WithHandshakeTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.handshakeTimeout = timeout
	}
}

// WithTrips sets the trips for the SSE client
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
