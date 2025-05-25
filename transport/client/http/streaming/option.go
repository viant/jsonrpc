package streaming

import (
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

// WithHandshakeTimeout overrides default handshake timeout.
func WithHandshakeTimeout(duration time.Duration) Option {
	return func(c *Client) {
		if duration <= 0 {
			return
		}
		c.handshakeTimeout = duration
	}
}
