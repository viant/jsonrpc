package streamable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
)

func TestPostSSERequestTermination(t *testing.T) {
	for _, tc := range []struct {
		name, event string
		abrupt      bool
		want        error
	}{
		{name: "empty EOF", want: io.EOF},
		{name: "notification EOF", event: `{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`, want: io.EOF},
		{name: "method-bearing matching ID EOF", event: `{"jsonrpc":"2.0","id":1,"method":"server/probe","result":{}}`, want: io.EOF},
		{name: "malformed matching ID EOF", event: `{"jsonrpc":"2.0","id":1,"error":"malformed"}`, want: io.EOF},
		{name: "unrelated response EOF", event: `{"jsonrpc":"2.0","id":2,"result":{}}`, want: io.EOF},
		{name: "truncated notification stream", event: `{"jsonrpc":"2.0","method":"notifications/tools/list_changed"}`, abrupt: true, want: io.ErrUnexpectedEOF},
		{name: "matching response EOF", event: `{"jsonrpc":"2.0","id":1,"result":{}}`},
		{name: "matching numeric response EOF", event: `{"jsonrpc":"2.0","id":1.0,"result":{}}`},
		{name: "matching error EOF", event: `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"unknown method"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ready := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var message struct {
					Method string `json:"method"`
				}
				_ = json.NewDecoder(r.Body).Decode(&message)
				if message.Method == "" {
					w.WriteHeader(http.StatusAccepted)
					return
				}
				w.Header().Set("Content-Type", sseMime)
				if tc.event != "" {
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", tc.event)
				}
				w.(http.Flusher).Flush()
				close(ready)
				if tc.abrupt {
					<-r.Context().Done()
				}
			}))
			defer server.Close()
			client, err := New(context.Background(), server.URL, WithStateless(), WithRunTimeout(0))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if tc.abrupt {
				go func() { <-ready; server.CloseClientConnections() }()
			}
			response, err := client.Send(ctx, &jsonrpc.Request{Jsonrpc: jsonrpc.Version, Id: 1, Method: "listen"})
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("Send error = %v, want %v", err, tc.want)
				}
				return
			}
			if err != nil || response == nil {
				t.Fatalf("Send response = %v, error = %v", response, err)
			}
		})
	}
}

func TestPostSSENotificationEOF(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Type", sseMime) }))
	defer server.Close()
	client, err := New(context.Background(), server.URL, WithStateless())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Notify(context.Background(), &jsonrpc.Notification{Method: "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
}

func TestPostSSECancellation(t *testing.T) {
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", sseMime)
		w.(http.Flusher).Flush()
		close(ready)
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := New(context.Background(), server.URL, WithStateless(), WithRunTimeout(0))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-ready; cancel() }()
	_, err = client.Send(ctx, &jsonrpc.Request{Jsonrpc: jsonrpc.Version, Id: 1, Method: "listen"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send error = %v", err)
	}
}

// The listener runs after base.HandleMessage has completed the registered
// round trip. Disconnect only then, to distinguish a known method outcome
// from a request whose response never arrived.
func TestPostSSECompletedResponseSurvivesTailFailure(t *testing.T) {
	for _, tail := range []string{"disconnect", "cancel"} {
		for _, result := range []string{"result", "error"} {
			t.Run(tail+"/"+result, func(t *testing.T) {
				payload := `{"jsonrpc":"2.0","id":1,"result":{"committed":true}}`
				if result == "error" {
					payload = `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method rejected"}}`
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", sseMime)
					fmt.Fprintf(w, "event: message\ndata: %s\n\ndata: unfinished tail", payload)
					w.(http.Flusher).Flush()
					<-r.Context().Done()
				}))
				defer server.Close()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				client, err := New(context.Background(), server.URL, WithStateless(), WithRunTimeout(0), WithListener(func(message *jsonrpc.Message) {
					if message.JsonRpcResponse == nil {
						return
					}
					if tail == "cancel" {
						cancel()
					} else {
						server.CloseClientConnections()
					}
				}))
				if err != nil {
					t.Fatal(err)
				}
				defer client.Close()
				response, err := client.Send(ctx, &jsonrpc.Request{Jsonrpc: jsonrpc.Version, Id: 1, Method: "mutate"})
				if err != nil || response == nil {
					t.Fatalf("completed outcome lost: response=%v error=%v", response, err)
				}
				if result == "result" && string(response.Result) != `{"committed":true}` {
					t.Fatalf("result lost: %s", response.Result)
				}
				if result == "error" && (response.Error == nil || response.Error.Code != -32601) {
					t.Fatalf("RPC error lost: %v", response)
				}
			})
		}
	}
}
