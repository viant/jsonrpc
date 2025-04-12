package base

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"io"
)

type Session struct {
	Id      string `json:"id"`
	Writer  io.Writer
	Handler transport.Handler
	framer  FrameMessage
	err     error
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
		s.SetError(err)
		return
	}
	s.SendData(ctx, data)
}

// SendResponse sends response
func (s *Session) SendResponse(ctx context.Context, response *jsonrpc.Response) {
	data, err := json.Marshal(response)
	if err != nil {
		s.SetError(err)
		return
	}
	s.SendData(ctx, data)
}

// SendData sends data
func (s *Session) SendData(ctx context.Context, data []byte) {
	_, err := s.Writer.Write(s.frameMessage(data))
	if err != nil && err != io.EOF {
		s.SetError(err)
	}
}

func NewSession(id string, writer io.Writer, handler transport.Handler, options ...Option) *Session {
	if id == "" {
		id = uuid.New().String()
	}
	ret := &Session{
		Id:      id,
		Writer:  writer,
		Handler: handler,
	}
	for _, option := range options {
		option(ret)
	}
	return ret
}
