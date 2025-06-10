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
	framed := s.frameMessage(data)
	_, err := s.Writer.Write(framed)
	if err != nil {
		s.SetError(err)
	}
	if s.bufferSize > 0 {
		id := atomic.AddUint64(&s.RequestIdSeq, 1)
		s.storeEvent(id, framed)
	}
}

func (s *Session) storeEvent(id uint64, data []byte) {
	s.events = append(s.events, event{id: id, data: append([]byte(nil), data...)})
	if len(s.events) > s.bufferSize {
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
		Id:         id,
		Writer:     writer,
		RoundTrips: transport.NewRoundTrips(20),
	}
	ret.Handler = newHandler(ctx, NewTransport(ret.RoundTrips, ret.SendData, ret))
	for _, option := range options {
		option(ret)
	}
	return ret
}
