package sse

import (
	"github.com/viant/jsonrpc/transport/server/auth"
	"github.com/viant/jsonrpc/transport/server/base"
	"github.com/viant/jsonrpc/transport/server/http/session"
	"net/http"
	"time"
)

// Options represents SSE options
type Options struct {
	MessageURI               string
	URI                      string
	SessionLocation          *session.Location // Optional sessionIdLocation for the transport, used for constructing full URIs
	StreamingSessionLocation *session.Location // Optional sessionIdLocation for the transport, used for constructing full URIs

	// Lifecycle controls
	ReconnectGrace  time.Duration
	IdleTTL         time.Duration
	MaxLifetime     time.Duration
	CleanupInterval time.Duration
	MaxEventBuffer  int
	OnSessionClose  func(*base.Session)
	RemovalPolicy   base.RemovalPolicy
	OverflowPolicy  base.OverflowPolicy
	// Optional custom session store (e.g., Redis-backed). Defaults to in-memory.
	Store base.SessionStore

	// BFF cookie-based session id (optional, disabled by default)
	CookieSession *BFFCookie
	// CORS settings for browsers
	AllowedOrigins   []string
	AllowCredentials bool

	// If true and CookieSession.Domain is empty, set cookie Domain to the request's top domain (eTLD+1).
	CookieUseTopDomain bool

	// BFF auth (server-held) grant
	AuthStore              auth.Store
	AuthCookie             *BFFAuthCookie
	AuthCookieUseTopDomain bool
	RehydrateOnHandshake   bool
	LogoutAllPath          string

	// KeepAliveInterval controls emission of SSE keepalive frames on the
	// long-lived GET stream. Zero or negative disables keepalives.
	KeepAliveInterval time.Duration
}

// BFFCookie defines cookie attributes used to carry the session id.
type BFFCookie struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
	MaxAge   int
}

// BFFAuthCookie defines cookie attributes used for the BFF auth session id (opaque grant id).
type BFFAuthCookie struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
	MaxAge   int
}
