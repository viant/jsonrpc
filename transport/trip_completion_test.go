package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/viant/jsonrpc"
)

func TestRoundTripWaitPreservesDeliveredResponseAfterCancellation(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second} {
		trip := NewRoundTrip(&jsonrpc.Request{Id: 1})
		response := &jsonrpc.Response{Id: 1, Jsonrpc: jsonrpc.Version}
		trip.SetResponse(response)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := trip.Wait(ctx, timeout); err != nil || trip.Response != response {
			t.Fatalf("delivered response lost: response=%v error=%v", trip.Response, err)
		}
	}
}

func TestRoundTripWaitCancellationWithoutResponse(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Second} {
		trip := NewRoundTrip(&jsonrpc.Request{Id: 1})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := trip.Wait(ctx, timeout); !errors.Is(err, context.Canceled) || trip.Response != nil {
			t.Fatalf("pending request cancellation lost: response=%v error=%v", trip.Response, err)
		}
	}
}
