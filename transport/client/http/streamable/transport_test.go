package streamable

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	clientbase "github.com/viant/jsonrpc/transport/client/base"
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

func TestTransportSendData_PrefersHTTPStatusOverMissingHandshakeHeader(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("upstream unavailable")),
				Request:    req,
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
	if err == nil || !strings.Contains(err.Error(), "invalid status code: 503: upstream unavailable") {
		t.Fatalf("expected upstream status error, got %v", err)
	}
}

func TestTransportSendData_PostAcceptsJSONResponse(t *testing.T) {
	var acceptHeader string
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			acceptHeader = req.Header.Get("Accept")
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
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

	if err := transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0"}`)); err != nil {
		t.Fatalf("SendData() error: %v", err)
	}
	if acceptHeader != "application/json" {
		t.Fatalf("unexpected Accept header: %q", acceptHeader)
	}
}

func TestTransportSendData_AllowsConcurrentPostsWhileRequestInFlight(t *testing.T) {
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	var calls int32
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			call := atomic.AddInt32(&calls, 1)
			switch call {
			case 1:
				close(firstEntered)
				<-releaseFirst
			case 2:
				close(secondEntered)
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
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

	firstErr := make(chan error, 1)
	go func() {
		firstErr <- transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0","id":1}`))
	}()
	select {
	case <-firstEntered:
	case <-time.After(time.Second):
		t.Fatal("first request did not enter RoundTrip")
	}

	secondErr := make(chan error, 1)
	go func() {
		secondErr <- transport.SendData(context.Background(), []byte(`{"jsonrpc":"2.0","id":2}`))
	}()
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("second request blocked while first request was in flight")
	}

	close(releaseFirst)
	for i, errCh := range []chan error{firstErr, secondErr} {
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("SendData call %d returned error: %v", i+1, err)
			}
		case <-time.After(time.Second):
			t.Fatalf("SendData call %d did not finish", i+1)
		}
	}
}

func TestTransportSendData_ConsumesLargeSSEResponse(t *testing.T) {
	const sessionID = "session-1"
	const requestID = 1
	largeText := strings.Repeat("x", 70*1024)
	result := map[string]interface{}{
		"content": []map[string]string{{"type": "text", "text": largeText}},
	}
	resultData, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(result): %v", err)
	}
	responseData := []byte(`{"jsonrpc":"2.0","id":1,"result":` + string(resultData) + `}`)
	sseData := "event: message\ndata: " + string(responseData) + "\n\n"
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{sseMime},
				},
				Body:    io.NopCloser(strings.NewReader(sseData)),
				Request: req,
			}, nil
		}),
	}

	streamClient := &Client{
		sessionID:         sessionID,
		sessionHeaderName: "Mcp-Session-Id",
	}
	transportClient := &Transport{
		client:   client,
		headers:  make(http.Header),
		endpoint: "http://example.com/mcp",
		c:        streamClient,
	}
	streamClient.base = &clientbase.Client{
		RunTimeout: time.Second,
		RoundTrips: transport.NewRoundTrips(10),
		Handler:    &clientbase.Handler{},
		Logger:     jsonrpc.DefaultLogger,
		Transport:  transportClient,
	}

	response, err := streamClient.base.Send(context.Background(), &jsonrpc.Request{
		Id:      requestID,
		Jsonrpc: jsonrpc.Version,
		Method:  "tools/call",
		Params:  json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if response == nil {
		t.Fatal("Send() returned nil response")
	}
	if got, ok := jsonrpc.AsRequestIntId(response.Id); !ok || got != requestID {
		t.Fatalf("unexpected response id: %v", response.Id)
	}
	if len(response.Result) == 0 {
		t.Fatal("expected response result")
	}
}
