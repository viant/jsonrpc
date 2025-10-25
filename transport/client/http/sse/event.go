package sse

// Event represents a server-sent event (streaming) message.
type Event struct {
	ID    string
	Event string
	Data  string
}
