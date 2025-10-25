package streamable

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/auth"
)

type noopSrv struct{}

func (n *noopSrv) Serve(ctx context.Context, req *jsonrpc.Request, resp *jsonrpc.Response) {}
func (n *noopSrv) OnNotification(ctx context.Context, notification *jsonrpc.Notification)  {}

func TestRehydrateOnHandshake_WithAuthCookie(t *testing.T) {
	store := auth.NewMemoryStore(14*24*time.Hour, 90*24*time.Hour, 30*time.Second)
	g := auth.NewGrant("user-123")
	if err := store.Put(context.Background(), g); err != nil {
		t.Fatalf("put grant: %v", err)
	}

	h := New(func(ctx context.Context, tr transport.Transport) transport.Handler { return &noopSrv{} },
		WithURI("/mcp-auth"),
		WithAuthStore(store),
		WithBFFAuthCookie(&BFFAuthCookie{Name: "BFF-Auth-Session", HttpOnly: true}),
		WithRehydrateOnHandshake(true),
	)
	mux := http.NewServeMux()
	mux.Handle("/mcp-auth", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// POST without session header but with BFF auth cookie should mint a new MCP session id in header
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp-auth", nil)
	req.AddCookie(&http.Cookie{Name: "BFF-Auth-Session", Value: g.ID})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	_ = resp.Body.Close()
	sid := resp.Header.Get(defaultSessionHeaderKey)
	if sid == "" {
		t.Fatalf("expected %s header to be set", defaultSessionHeaderKey)
	}
}
