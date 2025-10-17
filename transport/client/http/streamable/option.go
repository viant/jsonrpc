package streamable

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"net/http"
	"time"
)

// Option mutates Client.
type Option func(*Client)

// WithHTTPClient allows custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
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
