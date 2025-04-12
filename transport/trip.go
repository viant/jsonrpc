package transport

import (
	"context"
	"errors"
	"fmt"
	"github.com/viant/jsonrpc"
	"reflect"
	"sync/atomic"
	"time"
)

// RouteTrip represents a trip
type RouteTrip struct {
	Request  *jsonrpc.Request
	Response *jsonrpc.Response
	err      error
	done     chan struct{}
}

// NewRoundTrip creates a new round trip
func NewRoundTrip(request *jsonrpc.Request) *RouteTrip {
	return &RouteTrip{
		Request: request,
		done:    make(chan struct{}),
	}
}

// Error returns the error
func (t *RouteTrip) Error() error {
	return t.err
}

// Wait waits for the trip to finish
func (t *RouteTrip) Wait(ctx context.Context, timeout time.Duration) error {
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
func (t *RouteTrip) SetError(err error) {
	t.err = err
	close(t.done)
}

// SetResponse sets the response
func (t *RouteTrip) SetResponse(response *jsonrpc.Response) {
	t.Response = response
	close(t.done)
}

// RouteTrips represents a collection of trips
type RouteTrips struct {
	counter       uint64
	Ring          []*RouteTrip
	next          uint64
	capacity      int
	notifications chan *jsonrpc.Notification
	error         error
}

// CloseWithError closes trips with error
func (r *RouteTrips) CloseWithError(err error) {
	r.error = err
	r.Close()
}

// Close closes the trips
func (r *RouteTrips) Close() {
	close(r.notifications)
}

// Match matches a trip by id
func (r *RouteTrips) Match(id any) (*RouteTrip, error) {
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

// Notifications returns the notifications channel
func (r *RouteTrips) Notifications() chan *jsonrpc.Notification {
	return r.notifications
}

// Notify sends a notification
func (r *RouteTrips) Notify(notification *jsonrpc.Notification) error {
	if r.error != nil {
		return r.error
	}
	select {
	case r.notifications <- notification:
		return nil
	default:
		return fmt.Errorf("notification channel is full, dropping notification: %+v", notification)
	}
}

// Add adds a new trip
func (r *RouteTrips) Add(request *jsonrpc.Request) (*RouteTrip, error) {
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
func (r *RouteTrips) Get(index int) *RouteTrip {
	if index < 0 || index >= r.capacity {
		return nil
	}
	return r.Ring[int(r.counter)+index%r.capacity]
}

// Size returns the size of the trips
func (r *RouteTrips) Size() int {
	if int(r.counter) < r.capacity {
		return int(r.counter)
	}
	return r.capacity
}

// NewRoundTrips creates a new round trips
func NewRoundTrips(capacity int) *RouteTrips {
	return &RouteTrips{
		counter:       0,
		Ring:          make([]*RouteTrip, capacity),
		notifications: make(chan *jsonrpc.Notification, capacity),
		capacity:      capacity,
	}
}

func equals(id1 jsonrpc.RequestId, id2 any) bool {
	id1Type := reflect.TypeOf(id1)
	id2Type := reflect.TypeOf(id2)
	if id1Type.Kind() == id2Type.Kind() {
		return id1 == id2
	}
	if id1Type.Kind() == reflect.Uint64 {
		id1v := asInt(id1)
		id2v := asInt(id2)
		return id1v == id2v
	}
	return false
}

func asInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int8:
		return int(val)
	case int16:
		return int(val)
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint:
		return int(val)
	case uint8:
		return int(val)
	case uint16:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	case float32:
		return int(val)
	case float64:
		return int(val)
	}
	return -1
}
