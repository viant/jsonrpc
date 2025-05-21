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
)

type Session struct {
	Id         string `json:"id"`
	RoundTrips *transport.RoundTrips
	Writer     io.Writer
	Handler    transport.Handler
	framer     FrameMessage
	err        error
	closed     int32
	sync.Mutex
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
	_, err := s.Writer.Write(s.frameMessage(data))
	if err != nil {
		s.SetError(err)
	}
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
	ret.Handler = newHandler(ctx, NewTransport(ret.RoundTrips, ret.SendData))
	for _, option := range options {
		option(ret)
	}
	return ret
}
