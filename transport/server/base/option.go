package base

// Option represents option
type Option func(s *Session)

func WithFramer(framer FrameMessage) Option {
	return func(s *Session) {
		s.framer = framer
	}
}

// WithEventBuffer sets size of in-memory event buffer for session so that
// server can re-deliver messages on Last-Event-ID reconnect.
func WithEventBuffer(size int) Option {
	return func(s *Session) {
		if size > 0 {
			s.bufferSize = size
		}
	}
}

// WithSSE enables SSE id injection on each framed message and stores
// the same id for resumability (Last-Event-ID).
func WithSSE() Option {
	return func(s *Session) {
		s.sse = true
	}
}
