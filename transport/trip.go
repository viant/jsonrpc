package transport

import (
	"context"
	"errors"
	"fmt"
	"github.com/viant/jsonrpc"
	"sync/atomic"
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
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeout):
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
	counter  uint64
	Ring     []*RoundTrip
	next     uint64
	capacity int
	error    error
}

// CloseWithError closes trips with error
func (r *RoundTrips) CloseWithError(err error) {
	r.error = err
}

// Match matches a trip by id
func (r *RoundTrips) Match(id any) (*RoundTrip, error) {
	if r.error != nil {
		return nil, r.error
	}
	from := int(atomic.AddUint64(&r.next, 1) - 1)
	for i := from; i < r.capacity; i++ {
		if r.Ring[i] != nil && equals(r.Ring[i].Request.Id, id) {
			ret := r.Ring[i]
			r.Ring[i] = nil
			return ret, nil
		}
	}
	return nil, fmt.Errorf("trip not found")
}

// Add adds a new trip
func (r *RoundTrips) Add(request *jsonrpc.Request) (*RoundTrip, error) {
	if r.error != nil {
		return nil, r.error
	}
	from := int(atomic.AddUint64(&r.counter, 1) - 1)
	for i := from; i < r.capacity; i++ {
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
	if index < 0 || index >= r.capacity {
		return nil
	}
	return r.Ring[int(r.counter)+index%r.capacity]
}

// Size returns the size of the trips
func (r *RoundTrips) Size() int {
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
	id1Value, ok1 := jsonrpc.AsRequestIntId(id1)
	id2Value, ok2 := jsonrpc.AsRequestIntId(id2)
	if ok1 && ok2 {
		return id1Value == id2Value
	}
	return false
}
