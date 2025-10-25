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

// WithProtocolVersion sets the MCP protocol version header (MCP-Protocol-Version)
// to be included on all HTTP requests made by the SSE client (handshake GET and message POSTs).
func WithProtocolVersion(version string) Option {
	return func(c *Client) {
		if version == "" {
			return
		}
		// Store for GET stream requests
		c.protocolVersion = version
		// Ensure POST requests include the header via transport headers
		if c.transport != nil {
			if c.transport.headers == nil {
				c.transport.headers = make(http.Header)
			}
			c.transport.headers.Set("MCP-Protocol-Version", version)
		}
	}
}

// WithSessionID sets an explicit session identifier which will be appended
// to streaming GET requests so the server can reuse the same session after
// reconnects. It does not bypass the SSE handshake used to obtain message
// endpoint URI.
func WithSessionID(id string) Option {
	return func(c *Client) {
		if id == "" {
			return
		}
		c.sessionID = id
	}
}

// WithStreamSessionParamName configures the query parameter name used on
// the streaming GET request to carry the session id (default: Mcp-Session-Id).
func WithStreamSessionParamName(name string) Option {
	return func(c *Client) {
		if name != "" {
			c.streamSessionParamName = name
		}
	}
}
