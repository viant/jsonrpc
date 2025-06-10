package base

import (
	"context"
	"encoding/json"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"time"
)

// Transport represents a Transport
type Transport struct {
	TripTimeout time.Duration
	tripper     *transport.RoundTrips
	sendData    func(ctx context.Context, data []byte)
	session     *Session
}

func (s *Transport) Notify(ctx context.Context, notification *jsonrpc.Notification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	s.sendData(ctx, data)
	return nil
}

func (s *Transport) NextRequestID() jsonrpc.RequestId {
	return s.session.NextRequestID()
}

func (s *Transport) Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error) {
	if request.Id == nil {
		request.Id = s.NextRequestID()
	}
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	s.sendData(ctx, data)
	roundTrip, err := s.tripper.Add(request)
	if err != nil {
		return nil, err
	}
	err = roundTrip.Wait(ctx, s.TripTimeout)
	if err != nil {
		return nil, err
	}
	return roundTrip.Response, err
}

// NewTransport creates a new Transport
func NewTransport(tripper *transport.RoundTrips, sendData func(ctx context.Context, data []byte), session *Session) *Transport {
	return &Transport{
		tripper:     tripper,
		sendData:    sendData,
		session:     session,
		TripTimeout: 5 * time.Minute,
	}
}
