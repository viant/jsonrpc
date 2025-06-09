package common

import (
	"fmt"
	"net/http"
)

// FlushWriter wraps http.ResponseWriter and flushes every write so data is
// pushed to the client immediately (required for streaming transports such as
// SSE and MCP NDJSON).
type FlushWriter struct {
	writer  http.ResponseWriter
	flusher http.Flusher
}

func (w *FlushWriter) Write(p []byte) (int, error) {
	if w.flusher == nil {
		return 0, fmt.Errorf("streaming not supported: %T does not support flushing", w.writer)
	}
	n, err := w.writer.Write(p)
	if err == nil && w.flusher != nil {
		w.flusher.Flush()
	}
	return n, err
}

// NewFlushWriter constructs a FlushWriter backed by given ResponseWriter.
func NewFlushWriter(rw http.ResponseWriter) *FlushWriter {
	flusher, ok := rw.(http.Flusher)
	if !ok {
		flusher = nil
	}
	return &FlushWriter{writer: rw, flusher: flusher}
}
