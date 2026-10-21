package transport

import (
	"context"
	"errors"
	"fmt"
	"github.com/viant/jsonrpc"
	"reflect"
	"sync"
	"time"
)

// RoundTrip represents a trip
type RoundTrip struct {
	Request  *jsonrpc.Request
	Response *jsonrpc.Response
	err      error
	done     chan struct{}
}

// NewRoundTrip creates a new round trip
func NewRoundTrip(request *jsonrpc.Request) *RoundTrip {
	return &RoundTrip{
		Request: request,
		done:    make(chan struct{}),
	}
}

// Wait waits for the trip to finish
func (t *RoundTrip) Wait(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.done:
			return t.err
		}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("timeout")
	case <-t.done:
		if t.err != nil {
			return t.err
		}
	}
	return nil
}

// SetError sets the error
func (t *RoundTrip) SetError(error *jsonrpc.Error) {
	t.Response = &jsonrpc.Response{Id: t.Request.Id, Jsonrpc: t.Request.Jsonrpc, Error: error}
	close(t.done)
}

// SetResponse sets the response
func (t *RoundTrip) SetResponse(response *jsonrpc.Response) {
	if response.Error != nil {
		response.Result = nil
	}
	t.Response = response
	close(t.done)
}

// RoundTrips represents a collection of trips
type RoundTrips struct {
	mu       sync.Mutex
	counter  uint64
	Ring     []*RoundTrip
	next     uint64
	capacity int
	error    error
}

// CloseWithError closes trips with error
func (r *RoundTrips) CloseWithError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.error = err
}

// Match matches a trip by id
func (r *RoundTrips) Match(id any) (*RoundTrip, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.error != nil {
		return nil, r.error
	}
	// Start scanning from a rotating index but wrap around to cover entire ring.
	start := 0
	if r.capacity > 0 {
		r.next++
		start = int(r.next-1) % r.capacity
	}
	for k := 0; k < r.capacity; k++ {
		i := (start + k) % r.capacity
		if r.Ring[i] != nil && equals(r.Ring[i].Request.Id, id) {
			ret := r.Ring[i]
			r.Ring[i] = nil
			return ret, nil
		}
	}
	return nil, fmt.Errorf("trip not found")
}

// Remove releases only the supplied trip. A response may already have matched
// it and its ID or slot may now belong to another request. Cleanup must never
// remove that replacement. It is safe to call Remove more than once or after
// CloseWithError; it does not complete the trip or change the response path.
func (r *RoundTrips) Remove(trip *RoundTrip) bool {
	if trip == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, pending := range r.Ring {
		if pending == trip {
			r.Ring[i] = nil
			return true
		}
	}
	return false
}

// Add adds a new trip
func (r *RoundTrips) Add(request *jsonrpc.Request) (*RoundTrip, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.error != nil {
		return nil, r.error
	}
	if r.capacity == 0 {
		return nil, fmt.Errorf("failed to add request, ring is full")
	}
	// A response is matched only by ID, so two active trips with the same ID
	// would make ownership ambiguous. Reject the second trip instead.
	for i := 0; i < r.capacity; i++ {
		if r.Ring[i] != nil && equals(r.Ring[i].Request.Id, request.Id) {
			return nil, fmt.Errorf("duplicate active request id: %v", request.Id)
		}
	}
	// Find next available slot starting at a rotating index and wrapping around.
	r.counter++
	start := int(r.counter-1) % r.capacity
	for k := 0; k < r.capacity; k++ {
		i := (start + k) % r.capacity
		if r.Ring[i] == nil {
			ret := NewRoundTrip(request)
			r.Ring[i] = ret
			return ret, nil
		}
	}
	return nil, fmt.Errorf("failed to add request, ring is full")
}

// Get returns the trip at the given index
func (r *RoundTrips) Get(index int) *RoundTrip {
	r.mu.Lock()
	defer r.mu.Unlock()
	if index < 0 || index >= r.capacity {
		return nil
	}
	if r.capacity == 0 {
		return nil
	}
	return r.Ring[index]
}

// Size returns the size of the trips
func (r *RoundTrips) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if int(r.counter) < r.capacity {
		return int(r.counter)
	}
	return r.capacity
}

// NewRoundTrips creates a new round trips
func NewRoundTrips(capacity int) *RoundTrips {
	return &RoundTrips{
		counter:  0,
		Ring:     make([]*RoundTrip, capacity),
		capacity: capacity,
	}
}

func equals(id1 jsonrpc.RequestId, id2 any) bool {
	id1Value, ok1 := numericRequestID(id1)
	id2Value, ok2 := numericRequestID(id2)
	if ok1 && ok2 {
		return id1Value == id2Value
	}
	return reflect.DeepEqual(id1, id2)
}

func numericRequestID(id any) (int, bool) {
	switch id.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		value, _ := jsonrpc.AsRequestIntId(id)
		return value, true
	default:
		return 0, false
	}
}
