package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockEnhancedHandler is a simple mock implementation of transport.Handler for enhanced tests
type mockEnhancedHandler struct {
	serveFunc          func(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response)
	onNotificationFunc func(ctx context.Context, notification *jsonrpc.Notification)
	handleFunc         func(ctx context.Context, data []byte) ([]byte, error)
}

func (m *mockEnhancedHandler) Serve(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
	if m.serveFunc != nil {
		m.serveFunc(ctx, request, response)
		return
	}
	// Default implementation
	response.Result = []byte(`"ok"`)
}

func (m *mockEnhancedHandler) OnNotification(ctx context.Context, notification *jsonrpc.Notification) {
	if m.onNotificationFunc != nil {
		m.onNotificationFunc(ctx, notification)
	}
}

func (m *mockEnhancedHandler) Handle(ctx context.Context, data []byte) ([]byte, error) {
	if m.handleFunc != nil {
		return m.handleFunc(ctx, data)
	}
	return []byte(`{"result":"ok"}`), nil
}

// mockHandlerFactory creates a new mockEnhancedHandler for testing
func mockHandlerFactory(ctx context.Context, t transport.Transport) transport.Handler {
	return &mockEnhancedHandler{
		handleFunc: func(ctx context.Context, data []byte) ([]byte, error) {
			return []byte(`{"result":"ok"}`), nil
		},
	}
}

// TestCompleteMessageFlow tests a complete flow: SSE connection establishment and message handling
func TestCompleteMessageFlow(t *testing.T) {
	// Create a handler with our mock handler factory
	handler := New(mockHandlerFactory)

	// Step 1: Establish SSE connection
	sseReq := httptest.NewRequest(http.MethodGet, "/sse", nil)
	sseRecorder := httptest.NewRecorder()

	// Create a context with cancel to simulate client disconnection later
	ctx, cancel := context.WithCancel(context.Background())
	sseReq = sseReq.WithContext(ctx)

	// Start SSE connection in a goroutine since it blocks
	var sessionID string
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.ServeHTTP(sseRecorder, sseReq)
	}()

	// Wait a bit for the connection to be established
	time.Sleep(100 * time.Millisecond)

	// Extract session ID from the response
	responseBody := sseRecorder.Body.String()
	if !strings.Contains(responseBody, "event: endpoint") {
		t.Fatalf("Expected SSE connection to establish with endpoint event, got: %s", responseBody)
	}

	// Parse the session ID from the response
	// Format is: event: endpoint\ndata: /message?session_id=<session_id>\n
	parts := strings.Split(responseBody, "session_id=")
	if len(parts) < 2 {
		t.Fatalf("Could not find session_id in response: %s", responseBody)
	}
	sessionID = strings.TrimSpace(parts[1])

	// Step 2: Send a message using the session ID
	messageReq := httptest.NewRequest(http.MethodPost, "/message?session_id="+sessionID,
		bytes.NewBufferString(`{"jsonrpc":"2.0","method":"test","id":1}`))
	messageRecorder := httptest.NewRecorder()

	handler.ServeHTTP(messageRecorder, messageReq)

	// Check the response
	if messageRecorder.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, messageRecorder.Code)
	}

	// Verify the response content
	var response jsonrpc.Response
	if err := json.Unmarshal(messageRecorder.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response JSON: %v", err)
	}

	// Step 3: Clean up - cancel the context to close the SSE connection
	cancel()
	wg.Wait() // Wait for the SSE handler to complete

	// Verify the session was removed
	_, ok := handler.base.Sessions.Get(sessionID)
	if ok {
		t.Errorf("Expected session to be removed after connection closed")
	}
}

// TestSessionManagement tests session creation and deletion
func TestSessionManagement(t *testing.T) {
	// Create a handler with our mock handler factory
	handler := New(mockHandlerFactory)

	// Test session creation
	t.Run("Session Creation", func(t *testing.T) {
		// Initial session count should be 0
		initialCount := 0
		handler.base.Sessions.Range(func(key string, value *base.Session) bool {
			initialCount++
			return true
		})

		// Establish SSE connection
		sseReq := httptest.NewRequest(http.MethodGet, "/sse", nil)
		sseRecorder := httptest.NewRecorder()

		// Create a context with cancel
		ctx, cancel := context.WithCancel(context.Background())
		sseReq = sseReq.WithContext(ctx)

		// Start SSE connection in a goroutine
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler.ServeHTTP(sseRecorder, sseReq)
		}()

		// Wait for connection to establish
		time.Sleep(100 * time.Millisecond)

		// Count sessions - should be 1 more than initial
		newCount := 0
		handler.base.Sessions.Range(func(key string, value *base.Session) bool {
			newCount++
			return true
		})

		if newCount != initialCount+1 {
			t.Errorf("Expected session count to increase by 1, got initial=%d, new=%d", initialCount, newCount)
		}

		// Clean up
		cancel()
		wg.Wait()
	})

	// Test session deletion
	t.Run("Session Deletion", func(t *testing.T) {
		// Create a test session
		writer := NewWriter(httptest.NewRecorder())
		ctx := context.Background()
		session := base.NewSession(ctx, "", writer, mockHandlerFactory)
		handler.base.Sessions.Put(session.Id, session)

		// Verify session exists
		_, ok := handler.base.Sessions.Get(session.Id)
		if !ok {
			t.Fatalf("Session should exist after creation")
		}

		// Delete the session
		handler.base.Sessions.Delete(session.Id)

		// Verify session is gone
		_, ok = handler.base.Sessions.Get(session.Id)
		if ok {
			t.Errorf("Session should be deleted")
		}
	})
}

// TestErrorHandling tests various error scenarios
func TestErrorHandling(t *testing.T) {
	// Test cases for error handling
	tests := []struct {
		name           string
		url            string
		method         string
		body           string
		sessionID      string // If empty, no session will be created
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Invalid JSON",
			url:            "/message",
			method:         http.MethodPost,
			body:           `{"jsonrpc":"2.0",invalid}`,
			sessionID:      "test-session",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing Session ID",
			url:            "/message",
			method:         http.MethodPost,
			body:           `{"jsonrpc":"2.0","method":"test","id":1}`,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "failed to locate session",
		},
		{
			name:           "Session Not Found",
			url:            "/message?session_id=non-existent",
			method:         http.MethodPost,
			body:           `{"jsonrpc":"2.0","method":"test","id":1}`,
			expectedStatus: http.StatusNotFound,
			expectedError:  "session 'non-existent' not found",
		},
		{
			name:           "Method Not Allowed for SSE",
			url:            "/sse",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "Method Not Allowed for Message",
			url:            "/message",
			method:         http.MethodGet,
			sessionID:      "test-session",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a handler with our mock handler factory
			handler := New(mockHandlerFactory)

			// Create a session if needed
			if tt.sessionID != "" {
				writer := NewWriter(httptest.NewRecorder())
				ctx := context.Background()
				session := base.NewSession(ctx, tt.sessionID, writer, mockHandlerFactory)
				handler.base.Sessions.Put(session.Id, session)
			}

			// Create the request
			req := httptest.NewRequest(tt.method, tt.url, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(w, req)

			// Check status code
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			// Check error message if expected
			if tt.expectedError != "" && !strings.Contains(w.Body.String(), tt.expectedError) {
				t.Errorf("Expected error to contain %q, got %q", tt.expectedError, w.Body.String())
			}
		})
	}
}
