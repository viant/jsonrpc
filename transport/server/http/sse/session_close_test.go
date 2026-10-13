package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
)

type closeTestHandler struct{}

func (*closeTestHandler) Serve(context.Context, *jsonrpc.Request, *jsonrpc.Response) {}
func (*closeTestHandler) OnNotification(context.Context, *jsonrpc.Notification)      {}
func TestDeleteCallsFinalSessionCloseOnce(t *testing.T) {
	var calls atomic.Int32
	h := New(func(context.Context, transport.Transport) transport.Handler { return &closeTestHandler{} }, WithURI("/sse"), WithMessageURI("/message"), WithIdleTTL(time.Hour), WithCleanupInterval(0), WithOnSessionClose(func(*base.Session) { calls.Add(1) }))
	h.base.Sessions.Put("test", &base.Session{Id: "test"})
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest(http.MethodDelete, "/message?Mcp-Session-Id=test", nil)
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	if calls.Load() != 1 {
		t.Fatalf("close calls=%d", calls.Load())
	}
}
