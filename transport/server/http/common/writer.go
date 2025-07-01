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

func (w *FlushWriter) Write(p []byte) (n int, err error) {
	// A client can close the underlying connection at any time. Unfortunately, when that
	// happens net/http will panic inside the Write call (see golang/go#27529). To prevent
	// the panic from crashing the whole server we recover here and convert it to a normal
	// error that can be handled by the caller.
	defer func() {
		if r := recover(); r != nil {
			// Convert the recovered value into an error that indicates the write failed due
			// to a broken connection. We intentionally do not attempt to distinguish the
			// exact panic reason – the caller only needs to know the write did not succeed.
			// The panic value is included to aid diagnostics.
			switch x := r.(type) {
			case error:
				err = x
			default:
				err = fmt.Errorf("write failed: %v", x)
			}
		}
	}()

	if w.flusher == nil {
		return 0, fmt.Errorf("streaming not supported: %T does not support flushing", w.writer)
	}

	n, err = w.writer.Write(p)
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
