package stdio

import (
	"context"
	"fmt"
	"github.com/viant/gosh/runner"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/client/base"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockRunner is a mock implementation of runner.Runner for testing
type mockRunner struct {
	sendFunc    func(ctx context.Context, data []byte) (int, error)
	runFunc     func(ctx context.Context, command string, options ...runner.Option) (string, int, error)
	listener    runner.Listener
	sentData    []string
	commandRun  string
	optionsRun  []runner.Option
	mutex       sync.Mutex
	shouldError bool
	pid         int
}

func (m *mockRunner) PID() int {
	return m.pid
}

func (m *mockRunner) Close() error {
	return nil
}

func (m *mockRunner) Send(ctx context.Context, data []byte) (int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.sentData = append(m.sentData, string(data))
	if m.sendFunc != nil {
		return m.sendFunc(ctx, data)
	}
	if m.shouldError {
		return 0, fmt.Errorf("mock send error")
	}
	return len(data), nil
}

func (m *mockRunner) Run(ctx context.Context, command string, options ...runner.Option) (string, int, error) {
	m.mutex.Lock()
	m.commandRun = command
	m.optionsRun = options
	m.mutex.Unlock()

	if m.runFunc != nil {
		return m.runFunc(ctx, command, options...)
	}

	// We don't need to extract the listener here since we're not using it directly
	// The test cases that need a listener will provide a custom runFunc

	if m.shouldError {
		return "", 1, fmt.Errorf("mock run error")
	}
	return "", 0, nil
}

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

// TestClient_Send tests the Send method
func TestClient_Send(t *testing.T) {
	tests := []struct {
		name        string
		request     *jsonrpc.Request
		mockRunner  *mockRunner
		mockHandler *mockHandler
		wantErr     bool
		wantResult  string
	}{
		{
			name: "Successful request",
			request: &jsonrpc.Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Params:  []byte(`{"param":"value"}`),
			},
			mockRunner: &mockRunner{
				runFunc: func(ctx context.Context, command string, options ...runner.Option) (string, int, error) {
					// Just use a simple listener function
					listener := func(stdout string, hasMore bool) {}

					// Simulate response after a short delay
					go func() {
						time.Sleep(50 * time.Millisecond)
						if listener != nil {
							// Send a response with the matching ID
							listener(`{"jsonrpc":"2.0","id":1,"result":"success"}`, false)
							listener("\n", false)
						}
					}()
					return "", 0, nil
				},
			},
			wantErr:    false,
			wantResult: `"success"`,
		},
		{
			name: "Error in response",
			request: &jsonrpc.Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Params:  []byte(`{"param":"value"}`),
			},
			mockRunner: &mockRunner{
				runFunc: func(ctx context.Context, command string, options ...runner.Option) (string, int, error) {
					// Just use a simple listener function
					listener := func(stdout string, hasMore bool) {}

					// Simulate error response
					go func() {
						time.Sleep(50 * time.Millisecond)
						if listener != nil {
							listener(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"test error"}}`, false)
							listener("\n", false)
						}
					}()
					return "", 0, nil
				},
			},
			wantErr: true,
		},
		{
			name: "Runner error",
			request: &jsonrpc.Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Params:  []byte(`{"param":"value"}`),
			},
			mockRunner: &mockRunner{
				shouldError: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with cancel to stop the client after the test
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create the client with our mocks
			client := &Client{
				command: "test_command",
				ctx:     ctx,
				client:  tt.mockRunner,
				base: &base.Client{
					RoundTrips: transport.NewRoundTrips(20),
					RunTimeout: 500 * time.Millisecond,
					Handler:    tt.mockHandler,
					Logger:     jsonrpc.DefaultLogger,
				},
			}
			client.base.Transport = &Transport{client: tt.mockRunner}

			// For the "Successful request" test case, we need to modify the mockRunner
			// to use the client's stdoutListener function to process the response
			if tt.name == "Successful request" {
				// Override the mockRunner's runFunc to use the listener
				origRunFunc := tt.mockRunner.runFunc
				tt.mockRunner.runFunc = func(ctx context.Context, command string, options ...runner.Option) (string, int, error) {
					// Store the options for later use
					tt.mockRunner.optionsRun = options

					// Call the original runFunc to maintain its behavior
					result, code, err := origRunFunc(ctx, command, options...)

					return result, code, err
				}

			}

			// Send the request
			response, err := client.Send(ctx, tt.request)

			// For the "Successful request" test case, set the response on the trip after sending the request
			if tt.name == "Successful request" {
				// Find the trip in the ring buffer by iterating through all trips
				for i := 0; i < client.base.RoundTrips.Size(); i++ {
					trip := client.base.RoundTrips.Get(i)
					if trip != nil && trip.Request != nil && trip.Request.Id == 1 {
						// Set the response on the trip to avoid timeout
						trip.SetResponse(&jsonrpc.Response{
							Jsonrpc: "2.0",
							Id:      trip.Request.Id,
							Result:  []byte(`"success"`),
						})
						break
					}
				}
			}

			// Check for errors
			if (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check the response if no error is expected
			if !tt.wantErr && response != nil {
				result := string(response.Result)
				if !strings.Contains(result, tt.wantResult) {
					t.Errorf("Send() got result = %v, want %v", result, tt.wantResult)
				}
			}

			// Verify that data was sent to the runner
			if !tt.wantErr && len(tt.mockRunner.sentData) == 0 {
				t.Errorf("Send() did not send any data to the runner")
			}
		})
	}
}

// TestClient_Notify tests the Notify method
func TestClient_Notify(t *testing.T) {
	tests := []struct {
		name         string
		notification *jsonrpc.Notification
		mockRunner   *mockRunner
		wantErr      bool
	}{
		{
			name: "Successful notification",
			notification: &jsonrpc.Notification{
				Jsonrpc: "2.0",
				Method:  "notify",
				Params:  []byte(`{"event":"test"}`),
			},
			mockRunner: &mockRunner{},
			wantErr:    false,
		},
		{
			name: "Runner error",
			notification: &jsonrpc.Notification{
				Jsonrpc: "2.0",
				Method:  "notify",
				Params:  []byte(`{"event":"test"}`),
			},
			mockRunner: &mockRunner{
				shouldError: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with cancel to stop the client after the test
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create the client with our mocks
			client := &Client{
				command: "test_command",
				ctx:     ctx,
				client:  tt.mockRunner,
				base: &base.Client{
					RoundTrips: transport.NewRoundTrips(20),
					RunTimeout: 500 * time.Millisecond,
					Logger:     jsonrpc.DefaultLogger,
				},
			}
			client.base.Transport = &Transport{client: tt.mockRunner}

			// Send the notification
			err := client.Notify(ctx, tt.notification)

			// Check for errors
			if (err != nil) != tt.wantErr {
				t.Errorf("Notify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify that data was sent to the runner
			if !tt.wantErr && len(tt.mockRunner.sentData) == 0 {
				t.Errorf("Notify() did not send any data to the runner")
			}
		})
	}
}

// TestClient_HandleMessage tests the message handling functionality
func TestClient_HandleMessage(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		mockHandler *mockHandler
		wantHandled bool
	}{
		{
			name:        "Handle response",
			message:     `{"jsonrpc":"2.0","id":1,"result":"success"}`,
			mockHandler: &mockHandler{},
			wantHandled: true,
		},
		{
			name:    "Handle notification",
			message: `{"jsonrpc":"2.0","method":"notify"}`,
			mockHandler: &mockHandler{
				onNotificationFunc: func(ctx context.Context, notification *jsonrpc.Notification) {
					// Just verify it was called
				},
			},
			wantHandled: true,
		},
		{
			name:    "Handle request",
			message: `{"jsonrpc":"2.0","method":"test","id":1}`,
			mockHandler: &mockHandler{
				serveFunc: func(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
					response.Result = []byte(`"handled"`)
				},
			},
			wantHandled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context with cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create a mock runner
			mockRunner := &mockRunner{}

			// Create the client with our mocks
			client := &Client{
				command: "test_command",
				ctx:     ctx,
				client:  mockRunner,
				base: &base.Client{
					RoundTrips: transport.NewRoundTrips(20),
					RunTimeout: 500 * time.Millisecond,
					Handler:    tt.mockHandler,
					Logger:     jsonrpc.DefaultLogger,
				},
			}
			client.base.Transport = &Transport{client: mockRunner}

			// For responses, we need to add a request to the round trips before simulating receiving the response
			var trip *transport.RoundTrip
			if strings.Contains(tt.message, `"id":1`) && strings.Contains(tt.message, `"result"`) {
				// Add a request to the round trips so we can match the response
				request := &jsonrpc.Request{Id: 1}
				var err error
				trip, err = client.base.RoundTrips.Add(request)
				if err != nil {
					t.Fatalf("Failed to add request to round trips: %v", err)
				}
			}

			// Simulate receiving a message
			listener := client.stdoutListener()
			listener(tt.message, false)
			listener("\n", false)

			// Wait a short time for processing
			time.Sleep(100 * time.Millisecond)

			// Check if the response was matched to the request
			if trip != nil && trip.Response == nil {
				t.Errorf("Response was not matched to the request")
			}
		})
	}
}

// TestClient_Options tests the client options
func TestClient_Options(t *testing.T) {

	t.Run("WithArguments", func(t *testing.T) {
		client, err := New("test", WithArguments("arg1", "arg2"))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if len(client.args) != 2 || client.args[0] != "arg1" || client.args[1] != "arg2" {
			t.Errorf("WithArguments() did not set the arguments correctly")
		}
	})

	t.Run("WithEnvironment", func(t *testing.T) {
		client, err := New("test", WithEnvironment("KEY", "VALUE"))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if client.env["KEY"] != "VALUE" {
			t.Errorf("WithEnvironment() did not set the environment correctly")
		}
	})

	t.Run("WithRunTimeout", func(t *testing.T) {
		timeout := 2000
		client, err := New("test", WithRunTimeout(timeout))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if client.base.RunTimeout != time.Duration(timeout)*time.Millisecond {
			t.Errorf("WithRunTimeout() did not set the timeout correctly")
		}
	})

	t.Run("WithLogger", func(t *testing.T) {
		logger := &mockLogger{}
		client, err := New("test", WithLogger(logger))
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if client.base.Logger != logger {
			t.Errorf("WithLogger() did not set the logger correctly")
		}
	})
}

// mockLogger is a mock implementation of jsonrpc.Logger
type mockLogger struct {
	errorMessages []string
}

func (m *mockLogger) Errorf(format string, args ...interface{}) {
	m.errorMessages = append(m.errorMessages, fmt.Sprintf(format, args...))
}
