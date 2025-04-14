package stdio

import (
	"bufio"
	"bytes"
	"context"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"io"
	"strings"
	"testing"
	"time"
)

// mockHandler is a simple mock implementation of transport.Handler
type mockHandler struct {
	serveFunc          func(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response)
	onNotificationFunc func(ctx context.Context, notification *jsonrpc.Notification)
}

func (m *mockHandler) Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
	if m.serveFunc != nil {
		m.serveFunc(ctx, request, response)
		return
	}
	// Default implementation
	response.Result = []byte(`"ok"`)
}

func (m *mockHandler) OnNotification(ctx context.Context, notification *jsonrpc.Notification) {
	if m.onNotificationFunc != nil {
		m.onNotificationFunc(ctx, notification)
	}
}

// mockNewHandler creates a new mockHandler
func mockNewHandler(ctx context.Context, t transport.Transport) transport.Handler {
	return &mockHandler{}
}

// mockReadCloser is a mock implementation of io.ReadCloser
type mockReadCloser struct {
	reader io.Reader
	closed bool
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	return m.reader.Read(p)
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func newMockReadCloser(s string) *mockReadCloser {
	return &mockReadCloser{
		reader: strings.NewReader(s),
	}
}

// TestServer_ListenAndServe tests the ListenAndServe method
func TestServer_ListenAndServe(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		mockHandler *mockHandler
		wantErr     bool
		wantOutput  string
	}{
		{
			name:  "Valid JSON-RPC request",
			input: `{"jsonrpc":"2.0","method":"test","id":1}` + "\n",
			mockHandler: &mockHandler{
				serveFunc: func(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
					response.Result = []byte(`"test result"`)
				},
			},
			wantErr:    false,
			wantOutput: `{"id":1,"jsonrpc":"2.0","result":"test result"}`,
		},
		{
			name:  "Valid JSON-RPC notification",
			input: `{"jsonrpc":"2.0","method":"notify"}` + "\n",
			mockHandler: &mockHandler{
				onNotificationFunc: func(ctx context.Context, notification *jsonrpc.Notification) {
					// Just verify it was called
				},
			},
			wantErr:    false,
			wantOutput: "",
		},
		{
			name:       "Empty input",
			input:      "\n",
			wantErr:    false,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with cancel to stop the server after the test
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create input and output buffers
			input := newMockReadCloser(tt.input)
			output := &bytes.Buffer{}

			// Create the server with our mocks
			server := New(ctx, mockNewHandler,
				WithReader(input),
				WithErrorWriter(io.Discard), // Discard error output for tests
			)

			// Replace the stdout writer in the session with our buffer
			session, ok := server.base.Sessions.Get(sessionKey)
			if !ok {
				t.Fatalf("Session not found")
			}
			session.Writer = output

			// If we have a custom handler, replace the default one
			if tt.mockHandler != nil {
				session.Handler = tt.mockHandler
			}

			// Run the server in a goroutine
			errCh := make(chan error, 1)
			go func() {
				errCh <- server.ListenAndServe()
			}()

			// Wait a short time for processing
			time.Sleep(100 * time.Millisecond)

			// Cancel the context to stop the server
			cancel()

			// Check for errors
			var err error
			select {
			case err = <-errCh:
			case <-time.After(time.Second):
				t.Fatal("Server did not stop in time")
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("ListenAndServe() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check output if expected
			if tt.wantOutput != "" && !strings.Contains(output.String(), tt.wantOutput) {
				t.Errorf("Expected output to contain %q, got %q", tt.wantOutput, output.String())
			}
		})
	}
}

// TestServer_readLine tests the readLine method
func TestServer_readLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantLine string
		wantErr  bool
	}{
		{
			name:     "Simple line",
			input:    "test line\n",
			wantLine: "test line\n",
			wantErr:  false,
		},
		{
			name:     "Empty line",
			input:    "\n",
			wantLine: "\n",
			wantErr:  false,
		},
		{
			name:     "No newline",
			input:    "test line",
			wantLine: "test line",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			input := newMockReadCloser(tt.input)

			server := &Server{
				inout:  input,
				reader: bufio.NewReader(input),
				ctx:    ctx,
			}

			line, err := server.readLine(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("readLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if line != tt.wantLine {
				t.Errorf("readLine() got = %q, want %q", line, tt.wantLine)
			}
		})
	}
}

// TestServer_Options tests the server options
func TestServer_Options(t *testing.T) {
	t.Run("WithReader", func(t *testing.T) {
		input := newMockReadCloser("test")
		server := New(context.Background(), mockNewHandler, WithReader(input))
		if server.inout != input {
			t.Errorf("WithReader() did not set the input reader")
		}
	})

	t.Run("WithErrorWriter", func(t *testing.T) {
		errWriter := &bytes.Buffer{}
		server := New(context.Background(), mockNewHandler, WithErrorWriter(errWriter))
		if server.errWriter != errWriter {
			t.Errorf("WithErrorWriter() did not set the error writer")
		}
	})

	t.Run("WithLogger", func(t *testing.T) {
		logger := NewLogger(&bytes.Buffer{})
		server := New(context.Background(), mockNewHandler, WithLogger(logger))
		if server.logger != logger {
			t.Errorf("WithLogger() did not set the logger")
		}
	})
}
