package sse

import "github.com/viant/jsonrpc/transport/server/http/session"

// Options represents SSE options
type Options struct {
	MessageURI               string
	URI                      string
	SessionLocation          *session.Location // Optional sessionIdLocation for the transport, used for constructing full URIs
	StreamingSessionLocation *session.Location // Optional sessionIdLocation for the transport, used for constructing full URIs
}
