package stdio

import (
	"bufio"
	"context"
	"fmt"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
	"io"
	"os"
)

const sessionKey = "stdio"

// Server represents a server that handles incoming requests and responses
type Server struct {
	base   *base.Handler
	inout  io.ReadCloser
	reader *bufio.Reader

	errWriter io.Writer // Error writer for logging errors, defaults to os.Stderr
	logger    *Logger   // Custom logger for logging messages
	options   []base.Option
}

func (t *Server) ListenAndServe(ctx context.Context) error {
	// Ensure reader is initialized
	if t.reader == nil && t.inout != nil {
		t.reader = bufio.NewReader(t.inout)
	}

	// Ensure logger is initialized
	if t.logger == nil {
		t.logger = &Logger{writer: os.Stderr}
		if t.errWriter != nil {
			t.logger.writer = t.errWriter
		}
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := t.readLine(ctx)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		session, ok := t.base.Sessions.Get(sessionKey)
		if !ok {
			return fmt.Errorf("session not found")
		}
		t.base.HandleMessage(ctx, session, []byte(line))
	}
}

func (t *Server) readLine(ctx context.Context) (string, error) {
	if t.reader == nil {
		return "", fmt.Errorf("reader is not initialized")
	}
	readChan := make(chan string, 1)
	errChan := make(chan error, 1)
	// Use goroutine for non-blocking read
	go func() {
		line, err := t.reader.ReadString('\n')
		if line != "" && (err == nil || err == io.EOF) { // Handle the case where we read a line successfully
			readChan <- line
		} else if err != nil {
			errChan <- err
			return
		}
		readChan <- line
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-errChan:
		return "", err
	case line := <-readChan:
		return line, nil
	}
}

// New creates a new stdio transport instance with the provided handler and options
func New(ctx context.Context, newHandler transport.NewHandler, options ...Option) *Server {
	ret := &Server{
		base:      base.NewHandler(),
		inout:     os.Stdin,
		errWriter: os.Stderr,
	}
	for _, option := range options {
		option(ret)
	}
	aSession := base.NewSession(ctx, sessionKey, os.Stdout, newHandler, ret.options...)
	ret.base.Sessions.Put(sessionKey, aSession)
	// Apply all options
	for _, opt := range options {
		opt(ret)
	}

	// Initialize the reader if not already done by options
	if ret.reader == nil && ret.inout != nil {
		ret.reader = bufio.NewReader(ret.inout)
	}
	// Initialize the logger
	if ret.logger == nil {
		ret.logger = &Logger{writer: ret.errWriter}
	}
	return ret
}
