package streamable

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

func TestConcurrentSessionResponseHeaders(t *testing.T) {
	client := &Client{sessionID: "session-1", sessionHeaderName: "Mcp-Session-Id", streamActive: true}
	transport := &Transport{c: client, endpoint: "http://example.test/mcp", headers: make(http.Header)}
	transport.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusAccepted, Header: http.Header{"Mcp-Session-Id": {"session-1"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})}
	client.transport = transport
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 200; i++ {
				if err := transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0","method":"ping"}`)); err != nil {
					t.Error(err)
					return
				}
				if client.SessionID() != "session-1" {
					t.Error("negotiated session lost")
					return
				}
				_ = client.sessionContext(context.Background())
			}
		}()
	}
	workers.Wait()
}
