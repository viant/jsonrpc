package sse

import (
	"bytes"
	"context"
	"fmt"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/server/base"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Handler represents a server-side newNandler for SSE and message transport.
type Handler struct {
	base                       *base.Handler
	messageURI                 string
	sseURI                     string
	sseSessionIdLocation       *Location // Optional sessionIdLocation for the transport, used for constructing full URIs
	streamingSessionIdLocation *Location // Optional sessionIdLocation for the transport, used for constructing full URIs

	locator    Locator
	newHandler transport.NewHandler
	options    []base.Option
}

// ServeHTTP implements the http.Handler interface.
func (s *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	if strings.HasSuffix(uri, s.sseURI) || r.Method == http.MethodGet {
		s.handleSSE(w, r)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		if sessionId, _ := s.locator.Locate(s.streamingSessionIdLocation, r); sessionId != "" {
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
	useStreaming := !strings.HasSuffix(r.URL.Path, s.messageURI)
	var sess *base.Session
	location := s.sseSessionIdLocation
	if useStreaming {
		location = s.streamingSessionIdLocation
	}
	sessionId, err := s.locator.Locate(location, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to locate session: %v", err), http.StatusBadRequest)
		return
	}

	if sessionId == "" {
		sess = base.NewSession(ctx, "", NewWriter(w), s.newHandler, s.options...)
	} else {
		var ok bool
		if sess, ok = s.base.Sessions.Get(sessionId); !ok {
			http.Error(w, fmt.Sprintf("session '%s' not found", sessionId), http.StatusNotFound)
			return
		}
	}
	buffer := bytes.Buffer{}
	s.base.HandleMessage(ctx, sess, data, &buffer)
	if useStreaming {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set(s.streamingSessionIdLocation.Name, sess.Id)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(buffer.Bytes()))
		return
	}
	w.WriteHeader(http.StatusAccepted)
	output := fmt.Sprintf("event: message\ndata: %s\n\n", buffer.String())
	sess.Writer.Write([]byte(output))
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

	writer := NewWriter(w) // Custom writer to handle the http.ResponseWriter
	ctx, cancel := context.WithCancel(r.Context())
	session, err := s.initSessionHandshake(ctx, writer)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to initialize session: %v", err), http.StatusInternalServerError)
		return
	}

	// Main event loop - this runs in the HTTP handler goroutine
	for {
		select {

		case <-r.Context().Done():
			s.base.Sessions.Delete(session.Id)
			cancel()
			return
		}
	}
}

// initSessionHandshake initializes a new session.
func (s *Handler) initSessionHandshake(ctx context.Context, writer *Writer) (*base.Session, error) {
	aSession := base.NewSession(ctx, "", writer, s.newHandler, s.options...)
	query := url.Values{}
	if err := s.locator.Set(s.sseSessionIdLocation, query, aSession.Id); err != nil {
		return nil, err
	}
	URI := s.messageURI + "?" + query.Encode()
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
		newHandler:                 newHandler,
		sseURI:                     "/sse",     // Default SSE URI
		messageURI:                 "/message", // Default message URI
		base:                       base.NewHandler(),
		sseSessionIdLocation:       NewQueryLocation("session_id"),
		streamingSessionIdLocation: NewQueryLocation("Mcp-Session-Id"),
		options: []base.Option{
			base.WithFramer(frameSSE),
		},
	}
	for _, opt := range options {
		opt(ret) // Apply each option to the transport instance
	}
	return ret
}
