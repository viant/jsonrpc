package sse

import (
	"fmt"
	"net/http"
)

// Writer wraps the http.ResponseWriter to provide a custom Write method
type Writer struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

func (w *Writer) Write(p []byte) (int, error) {
	if w.flusher == nil {
		// Flush not supported
		return 0, fmt.Errorf("streaming not supported: %T does not support flushing", w.writer)
	}
	n, err := w.writer.Write(p)
	if err == nil {
		w.flusher.Flush() // ensure data is flushed to the client
	}
	return n, err
}

// NewWriter creates a new SSE writer
func NewWriter(writer http.ResponseWriter) *Writer {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		// Flushing not supported by this writer
		flusher = nil
	}
	return &Writer{
		writer:  writer,
		flusher: flusher,
	}
}
