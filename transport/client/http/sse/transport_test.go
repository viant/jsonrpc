package sse

import (
	"context"
	"errors"
	"fmt"
	"github.com/viant/jsonrpc"
	"io"
	"net/http"
	"strings"
	"testing"
)

type statusRoundTripper func(*http.Request) (*http.Response, error)

func (f statusRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type statusResponseBody struct {
	io.Reader
	closed bool
}

func (b *statusResponseBody) Close() error { b.closed = true; return nil }

func TestHTTPAuthorizationStatusRemainsDistinct(t *testing.T) {
	for _, status := range []int{401, 403, 500, 503} {
		for _, method := range []string{"GET", "POST", "reconnect"} {
			t.Run(fmt.Sprintf("%s/%d", method, status), func(t *testing.T) {
				body := &statusResponseBody{Reader: strings.NewReader("private response body")}
				client := &http.Client{Transport: statusRoundTripper(func(req *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: status, Header: make(http.Header), Body: body, Request: req}, nil
				})}
				transport := &Transport{sseClient: client, messageClient: client, endpoint: "http://fixture.invalid/mcp", headers: make(http.Header)}
				stream := &Client{streamURL: transport.endpoint, transport: transport}
				var err error
				if method == "GET" {
					err = stream.start(context.Background())
				} else if method == "reconnect" {
					_, err = stream.reconnect(context.Background())
				} else {
					err = transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0"}`))
				}
				if !body.closed {
					t.Fatal("rejected HTTP response body was not closed")
				}
				if status == 401 && method != "reconnect" {
					if !jsonrpc.IsUnauthorized(err) {
						t.Fatalf("401 lost challenge identity: %v", err)
					}
					return
				}
				var typed *jsonrpc.HTTPStatusError
				if !errors.As(err, &typed) || typed.HTTPStatusCode() != status {
					t.Fatalf("status lost: %v", err)
				}
				if jsonrpc.IsUnauthorized(err) || strings.Contains(err.Error(), "private response body") {
					t.Fatalf("non401 must not challenge or expose response: %v", err)
				}
			})
		}
	}
}
