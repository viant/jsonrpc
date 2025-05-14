package sse

type Option func(t *Handler)

func WithSseSessionLocation(location *Location) Option {
	// WithLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
	return func(t *Handler) {
		t.sseSessionIdLocation = location
	}
}

func WithStreamingSessionLocation(location *Location) Option {
	// WithLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
	return func(t *Handler) {
		t.streamingSessionIdLocation = location
	}
}
func WithMessageURI(messageURI string) Option {
	// WithMessageURI sets the message URI for the transport
	return func(t *Handler) {
		if t != nil {
			t.messageURI = messageURI
		}
	}
}

func WithSSEURI(sseURI string) Option {
	// WithSSEURI sets the SSE URI for the transport
	return func(t *Handler) {
		if t != nil {
			t.sseURI = sseURI
		}
	}
}
