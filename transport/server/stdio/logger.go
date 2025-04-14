package stdio

import (
	"fmt"
	"github.com/viant/jsonrpc"
	"io"
)

// Logger provides simple logging functionality
// Deprecated: Use jsonrpc.Logger interface instead
type Logger struct {
	writer io.Writer
	logger jsonrpc.Logger
}

// WriteString writes a string to the logger
func (l *Logger) WriteString(msg string) {
	if l.logger != nil {
		l.logger.Errorf("%s", msg)
	} else if l.writer != nil {
		l.writer.Write([]byte(msg + "\n"))
	}
}

// Errorf implements jsonrpc.Logger interface
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Errorf(format, args...)
	} else if l.writer != nil {
		fmt.Fprintf(l.writer, format+"\n", args...)
	}
}

// NewLogger creates a new Logger with the specified writer or logger
func NewLogger(writerOrLogger interface{}) *Logger {
	logger := &Logger{}

	switch v := writerOrLogger.(type) {
	case jsonrpc.Logger:
		logger.logger = v
	case io.Writer:
		logger.writer = v
	default:
		// Default to stderr
		logger.writer = io.Writer(nil)
	}

	return logger
}
