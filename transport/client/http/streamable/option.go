package streamable

import (
	"context"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"net/http"
	"time"
)

// RequestHeaderProvider derives per-message HTTP headers from the encoded
// JSON-RPC payload. Protocol layers can use this without coupling jsonrpc to
// protocol-specific routing headers.
type RequestHeaderProvider func(context.Context, []byte, http.Header) error

// Option mutates Client.
type Option func(*Client)

// WithHTTPClient allows custom http.Client for both SSE stream (GET) and
// JSON-RPC message (POST) requests.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
		// Also update the Transport's client so POST requests use the same
		// http.Client (e.g., with auth RoundTripper).
		if c.transport != nil {
			c.transport.client = client
		}
	}
}

// WithHandler sets the handler for the SSE sseClient
func WithHandler(handler transport.Handler) Option {
	return func(c *Client) {
		c.base.Handler = handler
	}
}

// WithListener sets a listener that observes low-level transport messages.
func WithListener(listener jsonrpc.Listener) Option {
	return func(c *Client) {
		c.base.Listener = listener
	}
}

// WithHandshakeTimeout overrides default handshake timeout.
func WithHandshakeTimeout(duration time.Duration) Option {
	return func(c *Client) {
		if duration <= 0 {
			return
		}
		c.handshakeTimeout = duration
	}
}

// WithSessionHeaderName sets a custom HTTP header name used to carry the
// session id. Defaults to "Mcp-Session-Id".
func WithSessionHeaderName(name string) Option {
	return func(c *Client) {
		if name != "" {
			c.sessionHeaderName = name
		}
	}
}

// WithProtocolVersion sets the MCP protocol version header (MCP-Protocol-Version)
// to be included on all HTTP requests made by the client (handshake, POSTs, and GET stream).
func WithProtocolVersion(version string) Option {
	return func(c *Client) {
		if version == "" {
			return
		}
		// Store for GET stream requests
		c.protocolVersion = version
		// Ensure POST requests include the header via transport headers
		if c.transport != nil && c.transport.headers != nil {
			c.transport.headers.Set("MCP-Protocol-Version", version)
		}
	}
}

// WithStateless enables independent POST-based Streamable HTTP requests. In
// this mode no session id or background GET stream is required.
func WithStateless() Option {
	return func(c *Client) {
		c.stateless = true
	}
}

// WithRequestHeaderProvider installs a hook invoked for each outgoing POST
// after static transport headers have been copied.
func WithRequestHeaderProvider(provider RequestHeaderProvider) Option {
	return func(c *Client) {
		c.requestHeaderProvider = provider
	}
}

// WithRunTimeout controls how long Send waits for a JSON-RPC response. A
// non-positive duration disables the transport timer and relies on context
// cancellation, which is useful for long-lived requests.
func WithRunTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.base.RunTimeout = timeout
	}
}

// WithSessionID sets an explicit session id for the client. When set, the
// client immediately applies the session header to POST requests and uses the
// same header on GET stream requests, allowing reconnects without a handshake.
// It also starts the background stream loop.
func WithSessionID(id string) Option {
	return func(c *Client) {
		if id == "" {
			return
		}
		c.setSessionID(id)
		if c.transport != nil && c.transport.headers != nil {
			// Ensure POSTs include the session header immediately
			if c.sessionHeaderName == "" {
				c.sessionHeaderName = "Mcp-Session-Id"
			}
			c.transport.headers.Set(c.sessionHeaderName, id)
		}
		// Ensure the stream is running
		c.ensureStream()
	}
}
