package sse

// Event represents a server-sent event (streaming) message.
type Event struct {
	Event string
	Data  string
}
