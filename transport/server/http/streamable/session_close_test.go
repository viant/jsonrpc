package streamable

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
)

func TestDeleteAndExpiryCloseSessionOnce(t *testing.T) {
	for _, expiry := range []bool{false, true} {
		name := "delete"
		if expiry {
			name = "expiry"
		}
		t.Run(name, func(t *testing.T) {
			var calls atomic.Int32
			closed := make(chan struct{}, 1)
			h := New(func(context.Context, transport.Transport) transport.Handler { return &serverHandler{} },
				WithURI("/mcp"), WithCleanupInterval(time.Millisecond), WithIdleTTL(time.Hour),
				WithOnSessionClose(func(*base.Session) { calls.Add(1); closed <- struct{}{} }))
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/mcp", nil))
			id := resp.Header().Get(defaultSessionHeaderKey)
			if id == "" {
				t.Fatal("missing session")
			}
			if expiry {
				sess, ok := h.base.Sessions.Get(id)
				if !ok {
					t.Fatal("new session missing before explicit aging")
				}
				sess.Lock()
				sess.LastSeen = time.Now().Add(-2 * time.Hour)
				sess.Unlock()
			} else {
				req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
				req.Header.Set(defaultSessionHeaderKey, id)
				h.ServeHTTP(httptest.NewRecorder(), req)
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("close hook missing")
			}
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodDelete, "/mcp", nil)
				req.Header.Set(defaultSessionHeaderKey, id)
				h.ServeHTTP(httptest.NewRecorder(), req)
			}
			if _, ok := h.base.Sessions.Get(id); ok {
				t.Fatal("session retained")
			}
			if calls.Load() != 1 {
				t.Fatalf("close calls=%d", calls.Load())
			}
		})
	}
}
