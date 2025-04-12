package stdio

import "io"

// Option represents a functional option for configuring the stdio transport
type Option func(*Server)

// WithReader sets the input reader
func WithReader(reader io.ReadCloser) Option {
	return func(t *Server) {
		t.inout = reader
		t.reader = nil // Reset reader so it will be initialized with new input
	}
}

// WithErrorWriter sets the error output writer
func WithErrorWriter(writer io.Writer) Option {
	return func(t *Server) {
		t.errWriter = writer
		if t.logger != nil {
			t.logger.writer = writer
		}
	}
}

// WithLogger sets the logger
func WithLogger(logger *Logger) Option {
	return func(t *Server) {
		t.logger = logger
	}
}
