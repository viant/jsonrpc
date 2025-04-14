package sse

import (
	"bytes"
	"context"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockHandler is a simple mock implementation of transport.Handler
type mockHandler struct {
	handleFunc func(ctx context.Context, data []byte) ([]byte, error)
}

func (m *mockHandler) Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) error {
	// Simple implementation for testing
	return nil
}

func (m *mockHandler) OnNotification(ctx context.Context, notification *jsonrpc.Notification) error {
	// Simple implementation for testing
	return nil
}

func (m *mockHandler) OnError(ctx context.Context, err *jsonrpc.Error) error {
	// Simple implementation for testing
	return nil
}

func (m *mockHandler) Handle(ctx context.Context, data []byte) ([]byte, error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, data)
	}
	return []byte(`{"result":"ok"}`), nil
}

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		method         string
		body           string
		mockHandler    *mockHandler
		options        []Option
		expectedStatus int
		expectedBody   string
	}{
		{
			name:   "SSE endpoint - GET request",
			url:    "/sse",
			method: http.MethodGet,
			mockHandler: &mockHandler{
				handleFunc: func(ctx context.Context, data []byte) ([]byte, error) {
					return []byte(`{"result":"ok"}`), nil
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "event: endpoint\ndata: /message?session_id=",
		},
		{
			name:           "SSE endpoint - wrong method",
			url:            "/sse",
			method:         http.MethodPost,
			mockHandler:    &mockHandler{},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Message endpoint - without session",
			url:            "/message",
			method:         http.MethodPost,
			body:           `{"jsonrpc":"2.0","method":"test","id":1}`,
			mockHandler:    &mockHandler{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Unknown endpoint",
			url:            "/unknown",
			method:         http.MethodGet,
			mockHandler:    &mockHandler{},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new handler with the mock transport handler
			handler := New(tt.mockHandler, append(tt.options, WithMessageURI("/message"), WithSSEURI("/sse"))...)

			// Create a test request
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBufferString(tt.body))

			// Create a test response recorder
			w := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			// For successful SSE requests, check that the response contains the expected data
			if tt.expectedStatus == http.StatusOK && strings.HasSuffix(tt.url, "/sse") {
				if !strings.Contains(w.Body.String(), tt.expectedBody) {
					t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
				}

				// Check headers for SSE
				contentType := w.Header().Get("Content-Type")
				if contentType != "text/event-stream" {
					t.Errorf("Expected Content-Type header to be 'text/event-stream', got %q", contentType)
				}

				cacheControl := w.Header().Get("Cache-Control")
				if cacheControl != "no-cache" {
					t.Errorf("Expected Cache-Control header to be 'no-cache', got %q", cacheControl)
				}
			}
		})
	}
}

func TestHandler_handleMessage(t *testing.T) {
	// Create a session and add it to the handler
	mockHandler := &mockHandler{
		handleFunc: func(ctx context.Context, data []byte) ([]byte, error) {
			return []byte(`{"result":"message handled"}`), nil
		},
	}

	handler := New(mockHandler)

	// Create a test session
	writer := NewWriter(httptest.NewRecorder())
	session := base.NewSession("test-session", writer, mockHandler)
	handler.base.Sessions.Put(session.Id, session)

	// Create a test request with the session ID
	req := httptest.NewRequest(http.MethodPost, "/message?session_id="+session.Id, bytes.NewBufferString(`{"jsonrpc":"2.0","method":"test","id":1}`))
	w := httptest.NewRecorder()

	// Handle the message
	handler.handleMessage(w, req)

	// Check status code
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, w.Code)
	}
}

func TestFrameSSE(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple message",
			input:    `{"result":"test"}`,
			expected: "event: message\ndata: {\"result\":\"test\"}\n",
		},
		{
			name:     "Empty message",
			input:    ``,
			expected: "event: message\ndata: \n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := string(frameSSE([]byte(tc.input)))
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestWriter(t *testing.T) {
	t.Run("Writer with flusher", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := NewWriter(recorder)

		data := []byte("test data")
		n, err := writer.Write(data)

		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if n != len(data) {
			t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
		}

		if recorder.Body.String() != "test data" {
			t.Errorf("Expected body to be 'test data', got %q", recorder.Body.String())
		}
	})
}
