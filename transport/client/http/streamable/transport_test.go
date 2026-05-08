package streamable

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type trackingReadCloser struct {
	io.Reader
	closed chan struct{}
	once   sync.Once
}

func (r *trackingReadCloser) Close() error {
	r.once.Do(func() {
		close(r.closed)
	})
	return nil
}

func TestTransportSendData_UnlocksAfterInvalidStatus(t *testing.T) {
	var calls int32
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			atomic.AddInt32(&calls, 1)
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("bad gateway")),
				Request:    req,
			}, nil
		}),
	}

	streamClient := &Client{
		sessionID:         "session-1",
		sessionHeaderName: "Mcp-Session-Id",
	}
	transport := &Transport{
		client:   client,
		headers:  make(http.Header),
		endpoint: "http://example.com/mcp",
		c:        streamClient,
	}

	err := transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0"}`))
	if err == nil || !strings.Contains(err.Error(), "invalid status code: 502") {
		t.Fatalf("expected invalid status code error, got %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0"}`))
	}()

	select {
	case err = <-errCh:
		if err == nil || !strings.Contains(err.Error(), "invalid status code: 502") {
			t.Fatalf("expected second invalid status code error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second SendData call blocked, transport mutex was not released")
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 requests, got %d", got)
	}
}

func TestTransportSendData_ClosesBodyWhenHandshakeHeaderMissing(t *testing.T) {
	bodyClosed := make(chan struct{})
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: &trackingReadCloser{
					Reader: strings.NewReader("ok"),
					closed: bodyClosed,
				},
				Request: req,
			}, nil
		}),
	}

	streamClient := &Client{
		sessionHeaderName: "Mcp-Session-Id",
	}
	transport := &Transport{
		client:   client,
		headers:  make(http.Header),
		endpoint: "http://example.com/mcp",
		c:        streamClient,
	}

	err := transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0"}`))
	if err == nil || !strings.Contains(err.Error(), "handshake missing Mcp-Session-Id header") {
		t.Fatalf("expected handshake missing error, got %v", err)
	}

	select {
	case <-bodyClosed:
	case <-time.After(time.Second):
		t.Fatal("response body was not closed on missing session header")
	}
}
