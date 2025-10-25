package streamable

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	authpkg "github.com/viant/jsonrpc/transport/server/auth"
	"github.com/viant/jsonrpc/transport/server/base"
	"github.com/viant/jsonrpc/transport/server/http/common"
	"github.com/viant/jsonrpc/transport/server/http/session"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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
	if h.Options.LogoutAllPath != "" && strings.HasSuffix(r.URL.Path, h.Options.LogoutAllPath) {
		h.handleLogoutAll(w, r)
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePOST(w, r)
	case http.MethodGet:
		h.handleGET(w, r)
	case http.MethodOptions:
		h.handleOPTIONS(w, r)
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
		// Rehydrate MCP session using BFF auth cookie if configured
		if h.Options.RehydrateOnHandshake && h.Options.AuthStore != nil && h.Options.AuthCookie != nil {
			if authID := h.authCookieValue(r); authID != "" {
				if g, err := h.Options.AuthStore.Get(r.Context(), authID); err == nil && g != nil {
					// touch and rotate on use
					_ = h.Options.AuthStore.Touch(r.Context(), authID, time.Now())
					newID, err := h.Options.AuthStore.Rotate(r.Context(), authID, &authpkg.Grant{Subject: g.Subject, Scopes: g.Scopes, UAHash: g.UAHash, IPHint: g.IPHint, FamilyID: g.FamilyID})
					if err == nil && newID != "" {
						h.setAuthCookie(w, r, newID)
					} else {
						h.setAuthCookie(w, r, authID)
					}
				}
			}
		}
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
	// BFF cookie fallback
	if sessionID == "" && h.Options.CookieSession != nil {
		if ck, err := r.Cookie(h.Options.CookieSession.Name); err == nil {
			sessionID = ck.Value
		}
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
	h.setCORSHeaders(w, r)

	// Reattach writer that flushes every message and switch to SSE framing
	aSession.MarkActiveWithWriter(common.NewFlushWriter(w))
	base.WithFramer(frameSSE)(aSession)
	base.WithEventBuffer(h.Options.MaxEventBuffer)(aSession)
	base.WithEventOverflowPolicy(h.Options.OverflowPolicy)(aSession)
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

	// Block until client closes, then mark session detached for quick reconnect.
	<-r.Context().Done()
	aSession.MarkDetached()
	aSession.Writer = nil
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
	base.WithEventBuffer(h.Options.MaxEventBuffer)(aSession)
	base.WithEventOverflowPolicy(h.Options.OverflowPolicy)(aSession)

	h.base.Sessions.Put(aSession.Id, aSession)
	// return session id at the configured location; for header we always set header
	// and use the configured header name
	if h.SessionLocation != nil && h.SessionLocation.Kind == "header" {
		w.Header().Set(h.SessionLocation.Name, aSession.Id)
	} else {
		// default to header if unspecified
		w.Header().Set(defaultSessionHeaderKey, aSession.Id)
	}
	// do not set transport session cookies; MCP session id is header-only
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
		h.setCORSHeaders(w, r)
		aSession.MarkActiveWithWriter(common.NewFlushWriter(w))
		base.WithFramer(frameSSE)(aSession)
		base.WithEventBuffer(h.Options.MaxEventBuffer)(aSession)
		base.WithEventOverflowPolicy(h.Options.OverflowPolicy)(aSession)
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

// handleOPTIONS responds to CORS preflight requests when needed.
func (h *Handler) handleOPTIONS(w http.ResponseWriter, r *http.Request) {
	h.setCORSHeaders(w, r)
	if reqMethod := r.Header.Get("Access-Control-Request-Method"); reqMethod != "" {
		w.Header().Set("Access-Control-Allow-Methods", reqMethod)
	}
	if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
	}
	w.WriteHeader(http.StatusNoContent)
}

// setCORSHeaders sets Access-Control headers depending on options and request origin.
func (h *Handler) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if h.Options.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		for _, allowed := range h.Options.AllowedOrigins {
			if allowed == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				return
			}
		}
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// handleLogoutAll revokes the BFF auth grant and clears the auth cookie.
func (h *Handler) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Options.AuthStore == nil || h.Options.AuthCookie == nil {
		http.Error(w, "auth not configured", http.StatusBadRequest)
		return
	}
	authID := h.authCookieValue(r)
	if authID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if g, err := h.Options.AuthStore.Get(r.Context(), authID); err == nil && g != nil {
		_ = h.Options.AuthStore.RevokeFamily(r.Context(), g.FamilyID)
	} else {
		_ = h.Options.AuthStore.Revoke(r.Context(), authID)
	}
	h.clearAuthCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authCookieValue(r *http.Request) string {
	if h.Options.AuthCookie == nil {
		return ""
	}
	if ck, err := r.Cookie(h.Options.AuthCookie.Name); err == nil {
		return ck.Value
	}
	return ""
}

func (h *Handler) setAuthCookie(w http.ResponseWriter, r *http.Request, id string) {
	if h.Options.AuthCookie == nil {
		return
	}
	domain := h.Options.AuthCookie.Domain
	if domain == "" && h.Options.AuthCookieUseTopDomain {
		if top, _ := common.TopDomain(common.ClientHost(r)); top != "" {
			domain = top
		}
	}
	ck := &http.Cookie{
		Name:     h.Options.AuthCookie.Name,
		Value:    id,
		Path:     h.Options.AuthCookie.Path,
		Domain:   domain,
		MaxAge:   h.Options.AuthCookie.MaxAge,
		Secure:   h.Options.AuthCookie.Secure,
		HttpOnly: h.Options.AuthCookie.HttpOnly,
		SameSite: h.Options.AuthCookie.SameSite,
	}
	if ck.Path == "" {
		ck.Path = "/"
	}
	http.SetCookie(w, ck)
}

func (h *Handler) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	if h.Options.AuthCookie == nil {
		return
	}
	domain := h.Options.AuthCookie.Domain
	if domain == "" && h.Options.AuthCookieUseTopDomain {
		if top, _ := common.TopDomain(common.ClientHost(r)); top != "" {
			domain = top
		}
	}
	ck := &http.Cookie{
		Name:     h.Options.AuthCookie.Name,
		Value:    "",
		Path:     h.Options.AuthCookie.Path,
		Domain:   domain,
		MaxAge:   -1,
		Secure:   h.Options.AuthCookie.Secure,
		HttpOnly: h.Options.AuthCookie.HttpOnly,
		SameSite: h.Options.AuthCookie.SameSite,
	}
	if ck.Path == "" {
		ck.Path = "/"
	}
	http.SetCookie(w, ck)
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
			// Lifecycle defaults
			ReconnectGrace:       30 * time.Second,
			IdleTTL:              5 * time.Minute,
			MaxLifetime:          1 * time.Hour,
			CleanupInterval:      30 * time.Second,
			MaxEventBuffer:       1024,
			RemovalPolicy:        base.RemovalAfterGrace,
			RehydrateOnHandshake: true,
		},
		base: base.NewHandler(),
		options: []base.Option{
			base.WithFramer(frameJSON),
		},
	}
	for _, o := range opts {
		o(&h.Options)
	}
	// allow custom session store injection
	if h.Options.Store != nil {
		h.base.Sessions = h.Options.Store
	}
	// start cleanup sweeper if configured
	if h.Options.CleanupInterval > 0 {
		go h.runSweeper()
	}
	return h
}

// runSweeper periodically removes sessions based on lifecycle options.
func (h *Handler) runSweeper() {
	ticker := time.NewTicker(h.Options.CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var toDelete []string
		h.base.Sessions.Range(func(id string, sess *base.Session) bool {
			remove := false
			// Max lifetime
			if h.Options.MaxLifetime > 0 && now.Sub(sess.CreatedAt) > h.Options.MaxLifetime {
				remove = true
			}
			// Idle TTL
			if !remove && h.Options.IdleTTL > 0 && now.Sub(sess.LastSeen) > h.Options.IdleTTL {
				remove = true
			}
			// Policy-based detach handling
			if !remove {
				switch h.Options.RemovalPolicy {
				case base.RemovalOnDisconnect:
					if sess.State == base.SessionStateDetached {
						remove = true
					}
				case base.RemovalAfterGrace:
					if sess.State == base.SessionStateDetached && h.Options.ReconnectGrace > 0 && sess.DetachedAt != nil {
						if now.Sub(*sess.DetachedAt) > h.Options.ReconnectGrace {
							remove = true
						}
					}
				case base.RemovalAfterIdle:
					// already covered by IdleTTL; nothing extra
				case base.RemovalManual:
					// do nothing here
				}
			}
			if remove {
				toDelete = append(toDelete, id)
			}
			return true
		})
		for _, id := range toDelete {
			if h.Options.OnSessionClose != nil {
				if sess, ok := h.base.Sessions.Get(id); ok {
					func() {
						defer func() { _ = recover() }()
						h.Options.OnSessionClose(sess)
					}()
				}
			}
			h.base.Sessions.Delete(id)
		}
	}
}
