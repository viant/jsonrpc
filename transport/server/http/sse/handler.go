package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
	"github.com/viant/jsonrpc/transport/server/http/common"
	"github.com/viant/jsonrpc/transport/server/http/session"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Handler represents a server-side newNandler for SSE and message transport.
type Handler struct {
	Options
	base       *base.Handler
	locator    session.Locator
	newHandler transport.NewHandler
	options    []base.Option
}

// ServeHTTP implements the http.Handler interface.
func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	if strings.HasSuffix(uri, s.URI) || r.Method == http.MethodGet {
		s.handleSSE(w, r)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if sessionId, _ := s.locator.Locate(s.StreamingSessionLocation, r); sessionId != "" {
			s.base.Sessions.Delete(sessionId)
			w.WriteHeader(http.StatusOK)
		}

	case http.MethodPost:
		s.handleMessage(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Handle message endpoint
}

// handleMessage handles incoming messages.
func (s *Handler) handleMessage(w http.ResponseWriter, r *http.Request) {
	var data []byte
	var err error
	if r.Body != nil {
		if data, err = io.ReadAll(r.Body); err != nil {
			http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
			return
		}
		r.Body.Close()
	}

	ctx := r.Context() // Use the request context for handling
	useStreaming := !strings.HasSuffix(r.URL.Path, s.MessageURI)
	var aSession *base.Session
	location := s.SessionLocation
	if useStreaming {
		location = s.StreamingSessionLocation
	}
	sessionId, err := s.locator.Locate(location, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to locate session: %v", err), http.StatusBadRequest)
		return
	}

	if sessionId == "" {
		aSession = base.NewSession(ctx, "", common.NewFlushWriter(w), s.newHandler, s.options...)
	} else {
		var ok bool
		if aSession, ok = s.base.Sessions.Get(sessionId); !ok {
			http.Error(w, fmt.Sprintf("session '%s' not found", sessionId), http.StatusNotFound)
			return
		}
	}
	buffer := bytes.Buffer{}
	ctx = context.WithValue(ctx, jsonrpc.SessionKey, aSession)
	s.base.HandleMessage(ctx, aSession, data, &buffer)

	if buffer.Len() == 0 { //notification no response
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if useStreaming { //forward compatibility
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(s.StreamingSessionLocation.Name, aSession.Id)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(buffer.Bytes()))
		return
	}

	w.WriteHeader(http.StatusAccepted)
	output := fmt.Sprintf("event: message\ndata: %s\n\n", buffer.String())
	aSession.Writer.Write([]byte(output))
}

func (s *Handler) isError(buffer bytes.Buffer) bool {
	jErr := jsonrpc.Response{}
	json.Unmarshal(buffer.Bytes(), &jErr)
	return jErr.Error != nil
}

// handleSSE handles Server-Sent Events (SSE).
func (s *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	writer := common.NewFlushWriter(w) // Custom writer to handle the http.ResponseWriter
	ctx, cancelFun := context.WithCancel(r.Context())
	aSession, err := s.initSessionHandshake(ctx, writer)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to initialize aSession: %v", err), http.StatusInternalServerError)
		cancelFun()
		return
	}

	// Main event loop - this runs in the HTTP handler goroutine
	for {
		select {

		case <-r.Context().Done():
			s.base.Sessions.Delete(aSession.Id)
			cancelFun()
			return
		}
	}
}

// initSessionHandshake initializes a new session.
func (s *Handler) initSessionHandshake(ctx context.Context, writer *common.FlushWriter) (*base.Session, error) {
	aSession := base.NewSession(ctx, "", writer, s.newHandler, s.options...)
	query := url.Values{}
	if err := s.locator.Set(s.SessionLocation, query, aSession.Id); err != nil {
		return nil, err
	}
	URI := s.MessageURI + "?" + query.Encode()
	payload := fmt.Sprintf("event: endpoint\ndata: %s\n\n", URI)
	if _, err := writer.Write([]byte(payload)); err != nil {
		return nil, err
	}
	s.base.Sessions.Put(aSession.Id, aSession)
	return aSession, nil
}

// New creates a new Handler instance with the provided options.
func New(newHandler transport.NewHandler, options ...Option) *Handler {
	ret := &Handler{
		newHandler: newHandler,
		Options: Options{
			URI:                      "/sse",     // Default SSE URI
			MessageURI:               "/message", // Default message URI
			SessionLocation:          session.NewQueryLocation("session_id"),
			StreamingSessionLocation: session.NewQueryLocation("Mcp-Session-Id"),
		},
		base: base.NewHandler(),
		options: []base.Option{
			base.WithFramer(frameSSE),
		},
	}
	for _, opt := range options {
		opt(&ret.Options) // Apply each option to the transport instance
	}
	return ret
}
