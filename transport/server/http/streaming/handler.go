package streaming

import (
	"bytes"
	"context"
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
	defaultURI          = "/mcp"
	mcpSessionHeaderKey = "Mcp-Session-Id"
	ndjsonMime          = "application/x-ndjson"
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
// GET  (with Accept: application/x-ndjson & Mcp-Session-Id) – opens long-lived streaming connection.
// DELETE (with Mcp-Session-Id) – terminates session.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, h.URI) {
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
	sessionID := r.Header.Get(mcpSessionHeaderKey)
	if sessionID == "" {
		// handshake – create session
		h.initHandshake(w, r)
		return
	}
	// message for existing session
	h.handleMessage(w, r, sessionID)
}

func (h *Handler) handleGET(w http.ResponseWriter, r *http.Request) {
	// Accept header must indicate NDJSON stream
	if !acceptsNDJSON(r.Header) {
		http.Error(w, "unsupported Accept header – expecting application/x-ndjson", http.StatusNotAcceptable)
		return
	}

	sessionID := r.Header.Get(mcpSessionHeaderKey)
	if sessionID == "" {
		// Try query param fallback (for debug convenience)
		sessionID = r.URL.Query().Get(mcpSessionHeaderKey)
	}
	if sessionID == "" {
		http.Error(w, "missing Mcp-Session-Id header", http.StatusBadRequest)
		return
	}

	aSession, ok := h.base.Sessions.Get(sessionID)
	if !ok {
		http.Error(w, fmt.Sprintf("session '%s' not found", sessionID), http.StatusNotFound)
		return
	}

	// last event id support
	lastIDHeader := r.Header.Get("Last-Event-ID")
	var lastID uint64
	if lastIDHeader != "" {
		if v, err := strconv.ParseUint(strings.TrimSpace(lastIDHeader), 10, 64); err == nil {
			lastID = v
		}
	}

	// Prepare streaming response headers.
	w.Header().Set("Content-Type", ndjsonMime)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Inject writer that flushes every message.
	aSession.Writer = common.NewFlushWriter(w)

	// Flush catch-up events first.
	if lastID > 0 {
		msgs := aSession.EventsAfter(lastID)
		for _, msg := range msgs {
			_, _ = aSession.Writer.Write(msg)
		}
	}

	// Block until client closes.
	<-r.Context().Done()
	h.base.Sessions.Delete(sessionID)
}

func (h *Handler) handleDELETE(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get(mcpSessionHeaderKey)
	if sessionID == "" {
		http.Error(w, "missing Mcp-Session-Id header", http.StatusBadRequest)
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
	// apply buffering & framer
	base.WithEventBuffer(1024)(aSession)
	base.WithFramer(framerWithSession(aSession))(aSession)

	h.base.Sessions.Put(aSession.Id, aSession)
	w.Header().Set(mcpSessionHeaderKey, aSession.Id)
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

	buffer := bytes.Buffer{}
	h.base.HandleMessage(ctx, aSession, data, &buffer)

	if buffer.Len() == 0 { // notification
		w.WriteHeader(http.StatusAccepted)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buffer.Bytes())
}

// Helper – checks if Accept header contains application/x-ndjson
func acceptsNDJSON(hdr http.Header) bool {
	for _, v := range hdr.Values("Accept") {
		if strings.Contains(v, ndjsonMime) {
			return true
		}
	}
	return false
}

// New constructs Handler with default settings and provided options.
func New(newHandler transport.NewHandler, opts ...Option) *Handler {
	h := &Handler{
		newHandler: newHandler,
		Options: Options{
			URI:             defaultURI,
			SessionLocation: session.NewHeaderLocation(mcpSessionHeaderKey),
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
