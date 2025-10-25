package streamable

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
)

// serverHandler implements transport.Handler with no-ops.
type serverHandler struct{}

func (h *serverHandler) Serve(_ context.Context, _ *jsonrpc.Request, _ *jsonrpc.Response) {}
func (h *serverHandler) OnNotification(_ context.Context, _ *jsonrpc.Notification)        {}

func TestStreamable_DetachReconnectAndCleanup(t *testing.T) {
	// Fast sweeper and grace for test
	opts := []Option{
		WithURI("/mcp-test"),
		WithCleanupInterval(50 * time.Millisecond),
		WithReconnectGrace(300 * time.Millisecond),
		WithIdleTTL(0),
		WithMaxLifetime(0),
		WithMaxEventBuffer(16),
		WithRemovalPolicy(base.RemovalAfterGrace),
	}

	h := New(func(ctx context.Context, tr transport.Transport) transport.Handler {
		return &serverHandler{}
	}, opts...)

	mux := http.NewServeMux()
	mux.Handle("/mcp-test", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Handshake: POST without session header
	resp, err := http.Post(srv.URL+"/mcp-test", "application/json", nil)
	if err != nil {
		t.Fatalf("handshake POST failed: %v", err)
	}
	_ = resp.Body.Close()
	sid := resp.Header.Get(defaultSessionHeaderKey)
	if sid == "" {
		t.Fatalf("missing session id header %s", defaultSessionHeaderKey)
	}

	// Attach streaming GET
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/mcp-test", nil)
	req.Header.Set("Accept", sseMime)
	req.Header.Set(defaultSessionHeaderKey, sid)
	getResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stream GET failed: %v", err)
	}

	// Give the server a moment to attach
	time.Sleep(50 * time.Millisecond)
	// Close to simulate disconnect
	_ = getResp.Body.Close()

	// Wait for handler to mark detached
	time.Sleep(60 * time.Millisecond)
	sess, ok := h.base.Sessions.Get(sid)
	if !ok {
		t.Fatalf("session not found after detach")
	}
	if sess.State != base.SessionStateDetached {
		t.Fatalf("expected detached state, got %v", sess.State)
	}

	// Reconnect within grace
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/mcp-test", nil)
	req2.Header.Set("Accept", sseMime)
	req2.Header.Set(defaultSessionHeaderKey, sid)
	getResp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("reconnect GET failed: %v", err)
	}
	// Allow reattach
	time.Sleep(50 * time.Millisecond)
	if sess.State != base.SessionStateActive {
		t.Fatalf("expected active state after reconnect, got %v", sess.State)
	}
	_ = getResp2.Body.Close()

	// Now wait for grace + cleanup to remove session
	time.Sleep(400 * time.Millisecond)
	if _, ok := h.base.Sessions.Get(sid); ok {
		t.Fatalf("expected session to be cleaned after grace")
	}
}

func TestStreamable_IdleTTLAndMaxLifetime(t *testing.T) {
	opts := []Option{
		WithURI("/mcp-test-ttl"),
		WithCleanupInterval(50 * time.Millisecond),
		WithReconnectGrace(0),
		WithIdleTTL(100 * time.Millisecond),
		WithMaxLifetime(200 * time.Millisecond),
		WithMaxEventBuffer(8),
		WithRemovalPolicy(base.RemovalAfterIdle),
	}
	h := New(func(ctx context.Context, tr transport.Transport) transport.Handler { return &serverHandler{} }, opts...)
	mux := http.NewServeMux()
	mux.Handle("/mcp-test-ttl", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/mcp-test-ttl", "application/json", nil)
	if err != nil {
		t.Fatalf("handshake POST failed: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	sid := resp.Header.Get(defaultSessionHeaderKey)
	if sid == "" {
		t.Fatalf("missing session id header %s", defaultSessionHeaderKey)
	}

	sess, ok := h.base.Sessions.Get(sid)
	if !ok {
		t.Fatalf("session not found after handshake")
	}

	// Force idle by backdating LastSeen
	sess.LastSeen = time.Now().Add(-2 * time.Second)
	time.Sleep(120 * time.Millisecond) // > IdleTTL and > CleanupInterval
	if _, ok := h.base.Sessions.Get(sid); ok {
		t.Fatalf("expected session removed due to IdleTTL")
	}

	// New session for MaxLifetime test
	resp2, err := http.Post(srv.URL+"/mcp-test-ttl", "application/json", nil)
	if err != nil {
		t.Fatalf("handshake POST failed: %v", err)
	}
	_ = resp2.Body.Close()
	sid2 := resp2.Header.Get(defaultSessionHeaderKey)
	if sid2 == "" {
		t.Fatalf("missing session id header")
	}
	sess2, ok := h.base.Sessions.Get(sid2)
	if !ok {
		t.Fatalf("session2 not found after handshake")
	}
	sess2.CreatedAt = time.Now().Add(-1 * time.Hour)

	time.Sleep(80 * time.Millisecond) // allow sweeper
	if _, ok := h.base.Sessions.Get(sid2); ok {
		t.Fatalf("expected session removed due to MaxLifetime")
	}
}
