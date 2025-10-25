package streamable

import (
	"github.com/viant/jsonrpc/transport/server/auth"
	"github.com/viant/jsonrpc/transport/server/base"
	"github.com/viant/jsonrpc/transport/server/http/session"
	"net/http"
	"time"
)

// Options exposes configurable attributes of the handler.
type Options struct {
	// URI of the endpoint (configurable; empty matches any path when handler is mounted on a specific route)
	URI string

	// SessionLocation defines where session id is transported (header or query param)
	SessionLocation *session.Location

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
}

// Option mutates Options.
type Option func(*Options)

// WithURI sets custom URI.
func WithURI(uri string) Option {
	return func(o *Options) { o.URI = uri }
}

// WithSessionLocation overrides default session location.
func WithSessionLocation(loc *session.Location) Option {
	return func(o *Options) { o.SessionLocation = loc }
}

// WithReconnectGrace sets the grace period during which a detached session is kept for reconnection.
func WithReconnectGrace(d time.Duration) Option { return func(o *Options) { o.ReconnectGrace = d } }

// WithIdleTTL sets the idle timeout for sessions.
func WithIdleTTL(d time.Duration) Option { return func(o *Options) { o.IdleTTL = d } }

// WithMaxLifetime sets the hard cap on session lifetime.
func WithMaxLifetime(d time.Duration) Option { return func(o *Options) { o.MaxLifetime = d } }

// WithCleanupInterval sets how often the cleanup sweeper runs.
func WithCleanupInterval(d time.Duration) Option { return func(o *Options) { o.CleanupInterval = d } }

// WithMaxEventBuffer sets the default event buffer size used for resumability.
func WithMaxEventBuffer(n int) Option { return func(o *Options) { o.MaxEventBuffer = n } }

// WithOnSessionClose registers a hook invoked when a session is finally closed.
func WithOnSessionClose(fn func(*base.Session)) Option {
	return func(o *Options) { o.OnSessionClose = fn }
}

// WithRemovalPolicy sets the session removal policy.
func WithRemovalPolicy(p base.RemovalPolicy) Option { return func(o *Options) { o.RemovalPolicy = p } }

// WithOverflowPolicy sets the event buffer overflow policy.
func WithOverflowPolicy(p base.OverflowPolicy) Option {
	return func(o *Options) { o.OverflowPolicy = p }
}

// WithSessionStore injects a custom SessionStore implementation.
func WithSessionStore(store base.SessionStore) Option { return func(o *Options) { o.Store = store } }

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

// WithBFFCookieSession enables cookie-based session id for BFF deployments.
func WithBFFCookieSession(c *BFFCookie) Option { return func(o *Options) { o.CookieSession = c } }

// WithCORSAllowedOrigins sets the allowed origins for CORS (must not be "*" when AllowCredentials is true).
func WithCORSAllowedOrigins(origins []string) Option {
	return func(o *Options) { o.AllowedOrigins = origins }
}

// WithCORSAllowCredentials toggles Access-Control-Allow-Credentials and credentialed requests.
func WithCORSAllowCredentials(v bool) Option { return func(o *Options) { o.AllowCredentials = v } }

// WithBFFCookieUseTopDomain enables auto setting cookie Domain to eTLD+1 derived from request host.
func WithBFFCookieUseTopDomain(v bool) Option { return func(o *Options) { o.CookieUseTopDomain = v } }

// BFFAuthCookie defines cookie attributes for the BFF auth session (opaque grant id).
type BFFAuthCookie struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool
	HttpOnly bool
	SameSite http.SameSite
	MaxAge   int
}

// WithAuthStore configures the durable store for BFF auth grants.
func WithAuthStore(store auth.Store) Option { return func(o *Options) { o.AuthStore = store } }

// WithBFFAuthCookie configures the cookie used to carry the BFF auth grant id.
func WithBFFAuthCookie(c *BFFAuthCookie) Option { return func(o *Options) { o.AuthCookie = c } }

// WithBFFAuthCookieUseTopDomain enables auto Domain=eTLD+1 for the auth cookie when Domain is empty.
func WithBFFAuthCookieUseTopDomain(v bool) Option {
	return func(o *Options) { o.AuthCookieUseTopDomain = v }
}

// WithRehydrateOnHandshake toggles using the auth cookie to mint a new MCP session when no session id is provided.
func WithRehydrateOnHandshake(v bool) Option { return func(o *Options) { o.RehydrateOnHandshake = v } }

// WithLogoutAllPath sets an optional path to revoke the BFF auth grant (logout all sessions).
func WithLogoutAllPath(path string) Option { return func(o *Options) { o.LogoutAllPath = path } }
