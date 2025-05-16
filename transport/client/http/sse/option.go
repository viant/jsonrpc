package sse

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"net/http"
	"time"
)

// Option is a function that configures the Client
type Option func(*Client)

// WithHttpClient sets the HTTP sseClient for the SSE sseClient
func WithHttpClient(client *http.Client) Option {
	return func(c *Client) {
		c.transport.sseClient = client
	}
}

// WithMessageHttpClient sets the message HTTP sseClient for the SSE sseClient
func WithMessageHttpClient(client *http.Client) Option {
	return func(c *Client) {
		c.transport.messageClient = client
	}
}

// WithHandshakeTimeout sets the handshake timeout for the SSE sseClient
func WithHandshakeTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.handshakeTimeout = timeout
	}
}

// WithTrips sets the trips for the SSE sseClient
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

// WithHandler sets the handler for the SSE sseClient
func WithHandler(handler transport.Handler) Option {
	return func(c *Client) {
		c.base.Handler = handler
	}
}

// WithRunTimeout sets the run timeout for the SSE sseClient
func WithRunTimeout(timeoutMs int) Option {
	return func(c *Client) {
		c.base.RunTimeout = time.Duration(timeoutMs) * time.Millisecond
	}
}

// WithLogger sets the log level for the SSE sseClient
func WithLogger(logger jsonrpc.Logger) Option {
	return func(c *Client) {
		c.base.Logger = logger
	}
}

// WithInterceptor sets a custom interceptor for the client
func WithInterceptor(interceptor transport.Interceptor) Option {
	return func(c *Client) {
		c.base.Interceptor = interceptor
	}
}
