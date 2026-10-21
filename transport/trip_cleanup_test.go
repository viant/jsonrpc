package transport

import (
	"errors"
	"sync"
	"testing"

	"github.com/viant/jsonrpc"
)

func TestRoundTripsRemovePreservesReplacement(t *testing.T) {
	trips := NewRoundTrips(1)
	original, err := trips.Add(&jsonrpc.Request{Id: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := trips.Match(1); err != nil {
		t.Fatal(err)
	}
	replacement, err := trips.Add(&jsonrpc.Request{Id: 1})
	if err != nil {
		t.Fatal(err)
	}
	if trips.Remove(original) || trips.Remove(nil) {
		t.Fatal("removed an unowned trip")
	}
	trips.CloseWithError(errors.New("closed"))
	if !trips.Remove(replacement) {
		t.Fatal("close prevented cleanup")
	}
	if trips.Remove(replacement) {
		t.Fatal("cleanup was not idempotent")
	}
}

func TestRoundTripsRemoveRacesMatch(t *testing.T) {
	trips := NewRoundTrips(1)
	for i := 0; i < 100; i++ {
		trip, err := trips.Add(&jsonrpc.Request{Id: i})
		if err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var removed bool
		var matched *RoundTrip
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; removed = trips.Remove(trip) }()
		go func() { defer wg.Done(); <-start; matched, _ = trips.Match(i) }()
		close(start)
		wg.Wait()
		if removed == (matched != nil) {
			t.Fatal("cleanup and matching must have exactly one owner")
		}
		if matched != nil && matched != trip {
			t.Fatal("matched a different trip")
		}
	}
}
