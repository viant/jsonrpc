package jsonrpc

import (
	"fmt"
	"io"
	"os"
)

// Logger defines the interface for logging operations
type Logger interface {
	// Errorf logs an error message with formatting
	Errorf(format string, args ...interface{})
}

// StdLogger is a simple logger that writes to an io.Writer
type StdLogger struct {
	writer io.Writer
}

// Errorf implements Logger.Errorf by writing a formatted error message to the writer
func (l *StdLogger) Errorf(format string, args ...interface{}) {
	if l.writer != nil {
		fmt.Fprintf(l.writer, format+"\n", args...)
	}
}

// NewStdLogger creates a new StdLogger with the specified writer
// If writer is nil, os.Stderr is used as the default
func NewStdLogger(writer io.Writer) *StdLogger {
	if writer == nil {
		writer = os.Stderr
	}
	return &StdLogger{
		writer: writer,
	}
}

// DefaultLogger is the default logger instance that writes to os.Stderr
var DefaultLogger Logger = NewStdLogger(os.Stderr)
