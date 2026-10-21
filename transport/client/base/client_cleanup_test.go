package base

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
)

type sendDataFunc func(context.Context, []byte) error

func (f sendDataFunc) SendData(ctx context.Context, data []byte) error { return f(ctx, data) }

func waitSendResult(t *testing.T, result <-chan sendResult) sendResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatal("Send did not complete")
		return sendResult{}
	}
}

func reply(t *testing.T, client *Client, id any) {
	t.Helper()
	raw, err := json.Marshal(&jsonrpc.Response{Jsonrpc: jsonrpc.Version, Id: id, Result: json.RawMessage(`"ok"`)})
	if err != nil {
		t.Fatal(err)
	}
	client.HandleMessage(context.Background(), raw)
}

func TestClientSendFailureReleasesCapacity(t *testing.T) {
	want := errors.New("send failed")
	client := &Client{RoundTrips: transport.NewRoundTrips(1), Transport: sendDataFunc(func(context.Context, []byte) error { return want })}
	for i := 0; i < 5; i++ {
		if _, err := client.Send(context.Background(), &jsonrpc.Request{Method: "read"}); !errors.Is(err, want) {
			t.Fatalf("send %d: got %v, want original transport error", i, err)
		}
	}
	// Encoding errors occur after registration too.
	if _, err := client.Send(context.Background(), &jsonrpc.Request{Method: "read", Params: json.RawMessage(`{`)}); err == nil {
		t.Fatal("expected encoding error")
	}
	recorder := newRecordingTransport()
	client.Transport = recorder
	result := sendAsync(context.Background(), client, &jsonrpc.Request{Method: "recovered"})
	request := waitSentRequest(t, recorder)
	reply(t, client, request.Id)
	assertResponseResult(t, result, `"ok"`)
}

func TestClientWaitFailureReleasesCapacityAndIgnoresLateResponse(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			recorder := newRecordingTransport()
			client := &Client{RoundTrips: transport.NewRoundTrips(1), Transport: recorder}
			if mode == "timeout" {
				client.RunTimeout = 5 * time.Millisecond
			}
			var expired []any
			for i := 0; i < 5; i++ {
				ctx, cancel := context.WithCancel(context.Background())
				result := sendAsync(ctx, client, &jsonrpc.Request{Method: "read"})
				request := waitSentRequest(t, recorder)
				expired = append(expired, request.Id)
				if mode == "cancel" {
					cancel()
				}
				got := waitSendResult(t, result)
				cancel()
				if mode == "cancel" && !errors.Is(got.err, context.Canceled) || mode == "timeout" && (got.err == nil || got.err.Error() != "timeout") {
					t.Fatalf("unexpected %s result: %v", mode, got.err)
				}
				if got.response != nil {
					t.Fatal("failed wait returned a response")
				}
			}
			client.RunTimeout = 0
			result := sendAsync(context.Background(), client, &jsonrpc.Request{Method: "recovered"})
			request := waitSentRequest(t, recorder)
			for _, id := range expired {
				reply(t, client, id)
			}
			assertStillPending(t, result)
			reply(t, client, request.Id)
			assertResponseResult(t, result, `"ok"`)
		})
	}
}

func TestClientCancellationRacesResponse(t *testing.T) {
	recorder := newRecordingTransport()
	client := &Client{RoundTrips: transport.NewRoundTrips(1), Transport: recorder}
	for i := 0; i < 100; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		result := sendAsync(ctx, client, &jsonrpc.Request{Method: "read"})
		request := waitSentRequest(t, recorder)
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; cancel() }()
		go func() { defer wg.Done(); <-start; reply(t, client, request.Id) }()
		close(start)
		got := waitSendResult(t, result)
		wg.Wait()
		if got.err != nil && !errors.Is(got.err, context.Canceled) {
			t.Fatalf("unexpected race result: %v", got.err)
		}
		if got.err == nil && (got.response == nil || string(got.response.Result) != `"ok"`) {
			t.Fatalf("unexpected successful response: %v", got.response)
		}
	}
}

func TestClientSendFailureDoesNotRemoveReplacementTrip(t *testing.T) {
	trips := transport.NewRoundTrips(1)
	want := errors.New("failed after response matched")
	var replacement *transport.RoundTrip
	client := &Client{RoundTrips: trips, Transport: sendDataFunc(func(_ context.Context, raw []byte) error {
		var request jsonrpc.Request
		if err := json.Unmarshal(raw, &request); err != nil {
			return err
		}
		if _, err := trips.Match(request.Id); err != nil {
			return err
		}
		var err error
		replacement, err = trips.Add(&jsonrpc.Request{Id: request.Id, Method: "replacement"})
		if err != nil {
			return err
		}
		return want
	})}
	if _, err := client.Send(context.Background(), &jsonrpc.Request{Id: 7, Method: "original"}); !errors.Is(err, want) {
		t.Fatalf("got %v, want transport error", err)
	}
	matched, err := trips.Match(7)
	if err != nil || matched != replacement {
		t.Fatalf("old cleanup removed replacement: %v", err)
	}
}
