package sse

import "github.com/viant/jsonrpc/transport/server/http/session"

type Option func(t *Options)

// WithSseSessionLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
func WithSseSessionLocation(location *session.Location) Option {
	return func(t *Options) {
		t.SessionLocation = location
	}
}

// WithStreamingSessionLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
func WithStreamingSessionLocation(location *session.Location) Option {
	return func(t *Options) {
		t.StreamingSessionLocation = location
	}
}

// WithMessageURI sets the message URI for the transport
func WithMessageURI(messageURI string) Option {
	// WithMessageURI sets the message URI for the transport
	return func(t *Options) {
		if t != nil {
			t.MessageURI = messageURI
		}
	}
}

// WithURI sets the SSE URI for the transport
func WithURI(sseURI string) Option {
	// WithURI sets the SSE URI for the transport
	return func(t *Options) {
		if t != nil {
			t.URI = sseURI
		}
	}
}
