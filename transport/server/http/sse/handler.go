package sse

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
	"net/url"
	"strconv"
	"strings"
	"time"
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
	if s.Options.LogoutAllPath != "" && strings.HasSuffix(uri, s.Options.LogoutAllPath) {
		s.handleLogoutAll(w, r)
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
	case http.MethodOptions:
		s.handleOPTIONS(w, r)
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
	if sessionId == "" && s.Options.RehydrateOnHandshake && s.Options.AuthStore != nil && s.Options.AuthCookie != nil {
		if authID := s.authCookieValue(r); authID != "" {
			if g, err := s.Options.AuthStore.Get(r.Context(), authID); err == nil && g != nil {
				_ = s.Options.AuthStore.Touch(r.Context(), authID, time.Now())
				if newID, err := s.Options.AuthStore.Rotate(r.Context(), authID, &authpkg.Grant{Subject: g.Subject, Scopes: g.Scopes, UAHash: g.UAHash, IPHint: g.IPHint, FamilyID: g.FamilyID}); err == nil && newID != "" {
					s.setAuthCookie(w, r, newID)
				} else {
					s.setAuthCookie(w, r, authID)
				}
			}
		}
	}
	if sessionId == "" && s.Options.CookieSession != nil {
		if ck, err := r.Cookie(s.Options.CookieSession.Name); err == nil {
			sessionId = ck.Value
		}
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

// handleLogoutAll revokes the BFF auth grant and clears the auth cookie.
func (s *Handler) handleLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Options.AuthStore == nil || s.Options.AuthCookie == nil {
		http.Error(w, "auth not configured", http.StatusBadRequest)
		return
	}
	authID := s.authCookieValue(r)
	if authID == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if g, err := s.Options.AuthStore.Get(r.Context(), authID); err == nil && g != nil {
		_ = s.Options.AuthStore.RevokeFamily(r.Context(), g.FamilyID)
	} else {
		_ = s.Options.AuthStore.Revoke(r.Context(), authID)
	}
	s.clearAuthCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Handler) authCookieValue(r *http.Request) string {
	if s.Options.AuthCookie == nil {
		return ""
	}
	if ck, err := r.Cookie(s.Options.AuthCookie.Name); err == nil {
		return ck.Value
	}
	return ""
}

func (s *Handler) setAuthCookie(w http.ResponseWriter, r *http.Request, id string) {
	if s.Options.AuthCookie == nil {
		return
	}
	domain := s.Options.AuthCookie.Domain
	if domain == "" && s.Options.AuthCookieUseTopDomain {
		if top, _ := common.TopDomain(common.ClientHost(r)); top != "" {
			domain = top
		}
	}
	ck := &http.Cookie{
		Name:     s.Options.AuthCookie.Name,
		Value:    id,
		Path:     s.Options.AuthCookie.Path,
		Domain:   domain,
		MaxAge:   s.Options.AuthCookie.MaxAge,
		Secure:   s.Options.AuthCookie.Secure,
		HttpOnly: s.Options.AuthCookie.HttpOnly,
		SameSite: s.Options.AuthCookie.SameSite,
	}
	if ck.Path == "" {
		ck.Path = "/"
	}
	http.SetCookie(w, ck)
}

func (s *Handler) clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	if s.Options.AuthCookie == nil {
		return
	}
	domain := s.Options.AuthCookie.Domain
	if domain == "" && s.Options.AuthCookieUseTopDomain {
		if top, _ := common.TopDomain(common.ClientHost(r)); top != "" {
			domain = top
		}
	}
	ck := &http.Cookie{
		Name:     s.Options.AuthCookie.Name,
		Value:    "",
		Path:     s.Options.AuthCookie.Path,
		Domain:   domain,
		MaxAge:   -1,
		Secure:   s.Options.AuthCookie.Secure,
		HttpOnly: s.Options.AuthCookie.HttpOnly,
		SameSite: s.Options.AuthCookie.SameSite,
	}
	if ck.Path == "" {
		ck.Path = "/"
	}
	http.SetCookie(w, ck)
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
	s.setCORSHeaders(w, r)

	writer := common.NewFlushWriter(w)
	ctx, cancelFun := context.WithCancel(r.Context())

	// Reuse existing session if provided via configured StreamingSessionLocation
	if s.StreamingSessionLocation != nil {
		sid, _ := s.locator.Locate(s.StreamingSessionLocation, r)
		if sid == "" && s.Options.CookieSession != nil {
			if ck, err := r.Cookie(s.Options.CookieSession.Name); err == nil {
				sid = ck.Value
			}
		}
		if sid != "" {
			if aSession, ok := s.base.Sessions.Get(sid); ok {
				// reattach writer and enable SSE framing/buffer
				aSession.MarkActiveWithWriter(writer)
				base.WithFramer(frameSSE)(aSession)
				base.WithEventBuffer(s.Options.MaxEventBuffer)(aSession)
				base.WithEventOverflowPolicy(s.Options.OverflowPolicy)(aSession)
				base.WithSSE()(aSession)

				// Resumability: replay after Last-Event-ID
				if last := strings.TrimSpace(r.Header.Get("Last-Event-ID")); last != "" {
					if v, err := strconv.ParseUint(last, 10, 64); err == nil {
						if msgs := aSession.EventsAfter(v); len(msgs) > 0 {
							for _, m := range msgs {
								_, _ = aSession.Writer.Write(m)
							}
						}
					}
				}

				// Optional keepalive with generation guard
				var stop chan struct{}
				if s.Options.KeepAliveInterval > 0 {
					gen := aSession.WriterGeneration()
					stop = make(chan struct{})
					go func(gen uint64) {
						ticker := time.NewTicker(s.Options.KeepAliveInterval)
						defer ticker.Stop()
						for {
							// stop if writer has been reattached
							if aSession.WriterGeneration() != gen {
								return
							}
							select {
							case <-ticker.C:
								if aSession.WriterGeneration() != gen {
									return
								}
								aSession.Touch()
								_, _ = aSession.Writer.Write([]byte(": keepalive\n\n"))
							case <-stop:
								return
							}
						}
					}(gen)
				}

				<-r.Context().Done()
				if stop != nil {
					close(stop)
				}
				// mark session detached for potential quick reconnect
				aSession.MarkDetached()
				aSession.Writer = nil
				cancelFun()
				return
			}
		}
	}

	// Fallback: create new session and perform SSE handshake
	aSession, err := s.initSessionHandshake(ctx, r, w, writer)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to initialize aSession: %v", err), http.StatusInternalServerError)
		cancelFun()
		return
	}

	// Keepalive while connection is open (generation guard)
	var stop chan struct{}
	if s.Options.KeepAliveInterval > 0 {
		gen := aSession.WriterGeneration()
		stop = make(chan struct{})
		go func(gen uint64) {
			ticker := time.NewTicker(s.Options.KeepAliveInterval)
			defer ticker.Stop()
			for {
				if aSession.WriterGeneration() != gen {
					return
				}
				select {
				case <-ticker.C:
					if aSession.WriterGeneration() != gen {
						return
					}
					aSession.Touch()
					_, _ = aSession.Writer.Write([]byte(": keepalive\n\n"))
				case <-stop:
					return
				}
			}
		}(gen)
	}

	<-r.Context().Done()
	if stop != nil {
		close(stop)
	}
	// mark session detached for potential quick reconnect
	aSession.MarkDetached()
	aSession.Writer = nil
	cancelFun()
}

// initSessionHandshake initializes a new session.
func (s *Handler) initSessionHandshake(ctx context.Context, r *http.Request, w http.ResponseWriter, writer *common.FlushWriter) (*base.Session, error) {
	aSession := base.NewSession(ctx, "", writer, s.newHandler, s.options...)
	// enable SSE id injection and buffering for resumability
	base.WithEventBuffer(s.Options.MaxEventBuffer)(aSession)
	base.WithEventOverflowPolicy(s.Options.OverflowPolicy)(aSession)
	base.WithSSE()(aSession)
	// do not set transport session cookies; MCP session id is header-only
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
			// Lifecycle defaults
			ReconnectGrace:    30 * time.Second,
			IdleTTL:           5 * time.Minute,
			MaxLifetime:       1 * time.Hour,
			CleanupInterval:   30 * time.Second,
			MaxEventBuffer:    1024,
			RemovalPolicy:     base.RemovalAfterGrace,
			KeepAliveInterval: 30 * time.Second,
		},
		base: base.NewHandler(),
		options: []base.Option{
			base.WithFramer(frameSSE),
		},
	}
	for _, opt := range options {
		opt(&ret.Options) // Apply each option to the transport instance
	}
	// allow custom session store injection
	if ret.Options.Store != nil {
		ret.base.Sessions = ret.Options.Store
	}
	// start cleanup sweeper if configured
	if ret.Options.CleanupInterval > 0 {
		go ret.runSweeper()
	}
	return ret
}

// handleOPTIONS responds to CORS preflight requests when needed.
func (s *Handler) handleOPTIONS(w http.ResponseWriter, r *http.Request) {
	s.setCORSHeaders(w, r)
	if reqMethod := r.Header.Get("Access-Control-Request-Method"); reqMethod != "" {
		w.Header().Set("Access-Control-Allow-Methods", reqMethod)
	}
	if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
	}
	w.WriteHeader(http.StatusNoContent)
}

// setCORSHeaders sets Access-Control headers depending on options and request origin.
func (s *Handler) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if s.Options.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		for _, allowed := range s.Options.AllowedOrigins {
			if allowed == origin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				return
			}
		}
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// runSweeper periodically removes sessions based on lifecycle options.
func (s *Handler) runSweeper() {
	ticker := time.NewTicker(s.Options.CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var toDelete []string
		s.base.Sessions.Range(func(id string, sess *base.Session) bool {
			remove := false
			// Max lifetime
			if s.Options.MaxLifetime > 0 && now.Sub(sess.CreatedAt) > s.Options.MaxLifetime {
				remove = true
			}
			// Idle TTL
			if !remove && s.Options.IdleTTL > 0 && now.Sub(sess.LastSeen) > s.Options.IdleTTL {
				remove = true
			}
			// Policy-based detach handling
			if !remove {
				switch s.Options.RemovalPolicy {
				case base.RemovalOnDisconnect:
					if sess.State == base.SessionStateDetached {
						remove = true
					}
				case base.RemovalAfterGrace:
					if sess.State == base.SessionStateDetached && s.Options.ReconnectGrace > 0 && sess.DetachedAt != nil {
						if now.Sub(*sess.DetachedAt) > s.Options.ReconnectGrace {
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
			if s.Options.OnSessionClose != nil {
				if sess, ok := s.base.Sessions.Get(id); ok {
					func() {
						defer func() { _ = recover() }()
						s.Options.OnSessionClose(sess)
					}()
				}
			}
			s.base.Sessions.Delete(id)
		}
	}
}
