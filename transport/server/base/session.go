package base

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type Session struct {
	Id           string `json:"id"`
	RoundTrips   *transport.RoundTrips
	Writer       io.Writer
	Handler      transport.Handler
	framer       FrameMessage
	RequestIdSeq uint64
	bufferSize   int
	events       []event
	err          error
	closed       int32
	sync.Mutex
	// sse enables SSE id injection and matching replay ids
	sse bool

	// Lifecycle metadata
	CreatedAt     time.Time
	LastSeen      time.Time
	DetachedAt    *time.Time
	State         SessionState
	WriterPresent bool

	// buffer overflow handling
	overflowPolicy OverflowPolicy
	overflowed     bool

	// writerGen increments on each writer (re)attachment to guard concurrent writers.
	writerGen uint64
}

// LastRequestID returns the most recently generated request id without mutating the underlying sequence.
// It is concurrency-safe and can be used to inspect the current sequence value.
func (s *Session) LastRequestID() jsonrpc.RequestId {
	return int(atomic.LoadUint64(&s.RequestIdSeq))
}

func (s *Session) NextRequestID() jsonrpc.RequestId {
	return int(atomic.AddUint64(&s.RequestIdSeq, 1))
}

type event struct {
	id   uint64
	data []byte
}

// SetError sets error
func (s *Session) SetError(err error) {
	s.err = err
}

// Error returns error
func (s *Session) Error() error {
	return s.err
}

func (s *Session) frameMessage(data []byte) []byte {
	if s.framer == nil {
		return data
	}
	return s.framer(data)
}

// SendError sends error
func (s *Session) SendError(ctx context.Context, error *jsonrpc.Error) {
	data, err := json.Marshal(error)
	if err != nil {
		fmt.Println(err)
		return
	}
	s.SendData(ctx, data)
}

// SendResponse sends response
func (s *Session) SendResponse(ctx context.Context, response *jsonrpc.Response) {
	if response.Error != nil {
		response.Result = nil
	}
	data, err := json.Marshal(response)
	if err != nil {
		return
	}
	s.SendData(ctx, data)
}

// SendRequest sends response
func (s *Session) SendRequest(ctx context.Context, request *jsonrpc.Request) {
	data, err := json.Marshal(request)
	if err != nil {
		fmt.Println(err)
		return
	}
	s.SendData(ctx, data)

}

func (s *Session) sendNotification(ctx context.Context, notification *jsonrpc.Notification) error {
	params, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	request := &jsonrpc.Request{
		Jsonrpc: jsonrpc.Version,
		Method:  notification.Method,
		Params:  params,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	s.SendData(ctx, data)
	return s.err
}

// SendData sends data
func (s *Session) SendData(ctx context.Context, data []byte) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	s.LastSeen = time.Now()
	framed := s.frameMessage(data)
	if s.sse {
		id := atomic.AddUint64(&s.RequestIdSeq, 1)
		prefix := []byte(fmt.Sprintf("id: %d\n", id))
		full := append(prefix, framed...)
		if s.Writer != nil {
			_, err := s.Writer.Write(full)
			if err != nil {
				s.SetError(err)
			}
		}
		if s.bufferSize > 0 {
			s.storeEvent(id, full)
		}
		return
	}
	if s.Writer != nil {
		_, err := s.Writer.Write(framed)
		if err != nil {
			s.SetError(err)
		}
	}
	if s.bufferSize > 0 {
		id := atomic.AddUint64(&s.RequestIdSeq, 1)
		s.storeEvent(id, framed)
	}
}

func (s *Session) storeEvent(id uint64, data []byte) {
	s.events = append(s.events, event{id: id, data: append([]byte(nil), data...)})
	if len(s.events) > s.bufferSize {
		// handle overflow
		if s.overflowPolicy == OverflowMark {
			s.overflowed = true
		}
		// drop oldest
		excess := len(s.events) - s.bufferSize
		s.events = s.events[excess:]
	}
}

// EventsAfter returns buffered framed messages with id greater than lastID.
func (s *Session) EventsAfter(lastID uint64) [][]byte {
	if lastID == 0 || len(s.events) == 0 {
		res := make([][]byte, len(s.events))
		for i, ev := range s.events {
			res[i] = ev.data
		}
		return res
	}
	var idx int
	// simple linear search as buffer small
	for idx < len(s.events) && s.events[idx].id <= lastID {
		idx++
	}
	if idx >= len(s.events) {
		return nil
	}
	res := make([][]byte, len(s.events)-idx)
	for i := idx; i < len(s.events); i++ {
		res[i-idx] = s.events[i].data
	}
	return res
}

func NewSession(ctx context.Context, id string, writer io.Writer, newHandler transport.NewHandler, options ...Option) *Session {
	if id == "" {
		id = uuid.New().String()
	}
	ret := &Session{
		Id:            id,
		Writer:        writer,
		RoundTrips:    transport.NewRoundTrips(20),
		CreatedAt:     time.Now(),
		LastSeen:      time.Now(),
		State:         SessionStateActive,
		WriterPresent: writer != nil,
	}
	ret.Handler = newHandler(ctx, NewTransport(ret.RoundTrips, ret.SendData, ret))
	for _, option := range options {
		option(ret)
	}
	return ret
}

// SessionState represents lifecycle state of a session.
type SessionState int

const (
	SessionStateActive SessionState = iota
	SessionStateDetached
	SessionStateClosed
)

// Touch updates LastSeen timestamp.
func (s *Session) Touch() {
	s.Mutex.Lock()
	s.LastSeen = time.Now()
	s.Mutex.Unlock()
}

// MarkDetached marks session as detached and records time.
func (s *Session) MarkDetached() {
	s.Mutex.Lock()
	now := time.Now()
	s.DetachedAt = &now
	s.State = SessionStateDetached
	s.WriterPresent = false
	s.Mutex.Unlock()
}

// MarkActiveWithWriter re-attaches a writer and marks session active.
func (s *Session) MarkActiveWithWriter(w io.Writer) {
	s.Mutex.Lock()
	s.Writer = w
	s.WriterPresent = w != nil
	s.State = SessionStateActive
	s.DetachedAt = nil
	s.LastSeen = time.Now()
	atomic.AddUint64(&s.writerGen, 1)
	s.Mutex.Unlock()
}

// WriterGeneration returns the current writer attachment generation.
func (s *Session) WriterGeneration() uint64 {
	return atomic.LoadUint64(&s.writerGen)
}
