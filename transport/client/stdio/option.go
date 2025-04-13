package stdio

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/scy/cred/secret"
	"time"
)

type Option func(c *Client)

// WithArguments is used to set the command line arguments for the base
func WithArguments(args ...string) Option {
	return func(c *Client) {
		c.args = args
	}
}

// WithEnvironment is used to set the environment for the base
func WithEnvironment(key, value string) Option {
	return func(c *Client) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		c.env[key] = value
	}
}

// WithSecret allows to inject a secret resource into the base
func WithSecret(resource secret.Resource) Option {
	return func(c *Client) {
		c.secret = resource // replace with actual secret resource initialization
	}
}

// WithTrips with trips
func WithTrips(trips *transport.RoundTrips) Option {
	return func(c *Client) {
		c.base.RoundTrips = trips
	}
}

// WithListener set listener on stdio base
func WithListener(listener jsonrpc.Listener) Option {
	return func(c *Client) {
		c.base.Listener = listener
	}
}

func WithRunTimeout(timeoutMs int) Option {
	return func(c *Client) {
		c.base.RunTimeout = time.Duration(timeoutMs) * time.Millisecond
	}
}

func WithHandler(handler transport.Handler) Option {
	return func(c *Client) {
		c.base.Handler = handler
	}
}
