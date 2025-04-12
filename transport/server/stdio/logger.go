package stdio

import (
	"io"
)

// Logger provides simple logging functionality
type Logger struct {
	writer io.Writer
}

// WriteString writes a string to the logger
func (l *Logger) WriteString(msg string) {
	if l.writer != nil {
		l.writer.Write([]byte(msg + "\n"))
	}
}
