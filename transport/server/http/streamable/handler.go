package streamable

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
	"strconv"
	"strings"
)

// Default values following the MCP spec.
const (
	defaultURI = ""
	// default header name for session id; may be overridden via Options.SessionLocation
	defaultSessionHeaderKey = "Mcp-Session-Id"
	sseMime                 = "text/event-stream"
)

// Handler implements server-side of Streamable-HTTP transport (Model Context Protocol).
// Single endpoint (URI) is used for handshake, message exchange and streaming.
// Operation mode is distinguished by HTTP method and Accept header value.
type Handler struct {
	Options
	base       *base.Handler
	locator    session.Locator
	newHandler transport.NewHandler
	options    []base.Option
}

// ServeHTTP implements http.Handler.
// POST (no session header) – handshake creates a session, returns session id in header.
// POST (with Mcp-Session-Id) – JSON-RPC message for the session; response returned sync.
// GET  (with Accept: text/event-stream & Mcp-Session-Id) – opens long-lived streaming connection.
// DELETE (with Mcp-Session-Id) – terminates session.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.URI != "" && !strings.HasSuffix(r.URL.Path, h.URI) {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePOST(w, r)
	case http.MethodGet:
		h.handleGET(w, r)
	case http.MethodDelete:
		h.handleDELETE(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePOST(w http.ResponseWriter, r *http.Request) {
	// locate session using configured location (default: header)
	sessionID, _ := h.locator.Locate(h.SessionLocation, r)
	if sessionID == "" {
		// handshake – create session
		h.initHandshake(w, r)
		return
	}
	// message for existing session
	h.handleMessage(w, r, sessionID)
}

func (h *Handler) handleGET(w http.ResponseWriter, r *http.Request) {
	if !acceptsSSE(r.Header) {
		http.Error(w, "SSE not supported on this endpoint", http.StatusMethodNotAllowed)
		return
	}
	// locate session using configured location (default: header)
	sessionID, _ := h.locator.Locate(h.SessionLocation, r)
	if sessionID == "" {
		// Try query param fallback (for debug convenience)
		sessionID = r.URL.Query().Get(h.SessionLocation.Name)
	}
	if sessionID == "" {
		http.Error(w, fmt.Sprintf("missing %s", h.SessionLocation.Name), http.StatusBadRequest)
		return
	}

	aSession, ok := h.base.Sessions.Get(sessionID)
	if !ok {
		http.Error(w, fmt.Sprintf("session '%s' not found", sessionID), http.StatusNotFound)
		return
	}

	// last event id support (reserved; implemented in resumability step)
	_ = r.Header.Get("Last-Event-ID")

	// Prepare SSE response headers.
	w.Header().Set("Content-Type", sseMime)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Inject writer that flushes every message.
	aSession.Writer = common.NewFlushWriter(w)
	// Use SSE framer for this stream
	base.WithFramer(frameSSE)(aSession)
	base.WithEventBuffer(1024)(aSession)
	base.WithSSE()(aSession)

	// Support resumability: replay events after Last-Event-ID if provided
	if last := strings.TrimSpace(r.Header.Get("Last-Event-ID")); last != "" {
		if v, err := strconv.ParseUint(last, 10, 64); err == nil {
			if msgs := aSession.EventsAfter(v); len(msgs) > 0 {
				for _, m := range msgs {
					_, _ = aSession.Writer.Write(m)
				}
			}
		}
	}

	// Block until client closes.
	<-r.Context().Done()
	h.base.Sessions.Delete(sessionID)
}

func (h *Handler) handleDELETE(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := h.locator.Locate(h.SessionLocation, r)
	if sessionID == "" {
		http.Error(w, fmt.Sprintf("missing %s", h.SessionLocation.Name), http.StatusBadRequest)
		return
	}
	h.base.Sessions.Delete(sessionID)
	w.WriteHeader(http.StatusOK)
}

// initHandshake creates a new session and returns its id in response header.
func (h *Handler) initHandshake(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	//body, err := io.ReadAll(r.Body)
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusBadRequest)
	//}
	aSession := base.NewSession(ctx, "", io.Discard, h.newHandler)
	// apply buffering; framer will be configured when streaming begins
	base.WithEventBuffer(1024)(aSession)

	h.base.Sessions.Put(aSession.Id, aSession)
	// return session id at the configured location; for header we always set header
	// and use the configured header name
	if h.SessionLocation != nil && h.SessionLocation.Kind == "header" {
		w.Header().Set(h.SessionLocation.Name, aSession.Id)
	} else {
		// default to header if unspecified
		w.Header().Set(defaultSessionHeaderKey, aSession.Id)
	}
	h.handleMessage(w, r, aSession.Id)

	//w.WriteHeader(http.StatusCreated)
}

func (h *Handler) handleMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	aSession, ok := h.base.Sessions.Get(sessionID)
	if !ok {
		http.Error(w, fmt.Sprintf("session '%s' not found", sessionID), http.StatusNotFound)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	ctx := context.WithValue(r.Context(), jsonrpc.SessionKey, aSession)

	// If client accepts SSE, and this is a JSON-RPC request, stream via SSE.
	if acceptsSSE(r.Header) && isJSONRPCRequest(data) && hasID(data) {
		// Prepare SSE response and writer
		w.Header().Set("Content-Type", sseMime)
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		aSession.Writer = common.NewFlushWriter(w)
		base.WithFramer(frameSSE)(aSession)
		base.WithEventBuffer(1024)(aSession)
		base.WithSSE()(aSession)
		// Stream response and any further messages on this connection
		h.base.HandleMessage(ctx, aSession, data, nil)
		return
	}

	// Default: synchronous JSON response or 202 Accepted for notifications
	buffer := bytes.Buffer{}
	h.base.HandleMessage(ctx, aSession, data, &buffer)
	if buffer.Len() == 0 { // notification (no response)
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

// Helper – checks if Accept header contains text/event-stream
func acceptsSSE(hdr http.Header) bool {
	for _, v := range hdr.Values("Accept") {
		if strings.Contains(v, sseMime) {
			return true
		}
	}
	return false
}

// isJSONRPCRequest returns true if data looks like a JSON-RPC request (has method and optional id)
func isJSONRPCRequest(data []byte) bool {
	var tmp struct {
		Method string          `json:"method"`
		ID     json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return false
	}
	return tmp.Method != ""
}

// hasID returns true if the JSON has a non-null id field
func hasID(data []byte) bool {
	var tmp struct {
		ID *json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return false
	}
	return tmp.ID != nil
}

// New constructs Handler with default settings and provided options.
func New(newHandler transport.NewHandler, opts ...Option) *Handler {
	h := &Handler{
		newHandler: newHandler,
		Options: Options{
			URI:             defaultURI,
			SessionLocation: session.NewHeaderLocation(defaultSessionHeaderKey),
		},
		base: base.NewHandler(),
		options: []base.Option{
			base.WithFramer(frameJSON),
		},
	}
	for _, o := range opts {
		o(&h.Options)
	}
	return h
}
