package stdio

import "io"

// Writer is a simple wrapper around io.Writer to implement the jsonrpc transport interface
type Writer struct {
	io.Writer
}

// Write implements the io.Writer interface for Writer
func (w *Writer) Write(p []byte) (n int, err error) {
	return w.Writer.Write(p)
}

// NewWriter creates a new Writer instance
func NewWriter(writer io.Writer) *Writer {
	return &Writer{
		Writer: writer,
	}
}
