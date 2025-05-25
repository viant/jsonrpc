package streaming

import "github.com/viant/jsonrpc/transport/server/http/session"

// Options exposes configurable attributes of the handler.
type Options struct {
	// URI of the MCP endpoint (default: /mcp)
	URI string

	// SessionLocation defines where session id is transported (header or query param)
	SessionLocation *session.Location
}

// Option mutates Options.
type Option func(*Options)

// WithURI sets custom URI.
func WithURI(uri string) Option {
	return func(o *Options) { o.URI = uri }
}

// WithSessionLocation overrides default session location.
func WithSessionLocation(loc *session.Location) Option {
	return func(o *Options) { o.SessionLocation = loc }
}
