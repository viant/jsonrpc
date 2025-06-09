package streaming

import (
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

// WithHandshakeTimeout overrides default handshake timeout.
func WithHandshakeTimeout(duration time.Duration) Option {
	return func(c *Client) {
		if duration <= 0 {
			return
		}
		c.handshakeTimeout = duration
	}
}
