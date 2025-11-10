package sse

import (
	"github.com/viant/jsonrpc/transport/server/auth"
	"github.com/viant/jsonrpc/transport/server/base"
	"github.com/viant/jsonrpc/transport/server/http/session"
	"time"
)

type Option func(t *Options)

// WithSseSessionLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
func WithSseSessionLocation(location *session.Location) Option {
	return func(t *Options) {
		t.SessionLocation = location
	}
}

// WithStreamingSessionLocation sets the optional sessionIdLocation for the transport, used for constructing full URIs
func WithStreamingSessionLocation(location *session.Location) Option {
	return func(t *Options) {
		t.StreamingSessionLocation = location
	}
}

// WithMessageURI sets the message URI for the transport
func WithMessageURI(messageURI string) Option {
	// WithMessageURI sets the message URI for the transport
	return func(t *Options) {
		if t != nil {
			t.MessageURI = messageURI
		}
	}
}

// WithURI sets the SSE URI for the transport
func WithURI(sseURI string) Option {
	// WithURI sets the SSE URI for the transport
	return func(t *Options) {
		if t != nil {
			t.URI = sseURI
		}
	}
}

// WithReconnectGrace sets the grace period during which a detached session is kept for reconnection.
func WithReconnectGrace(d time.Duration) Option { return func(t *Options) { t.ReconnectGrace = d } }

// WithIdleTTL sets the idle timeout for sessions.
func WithIdleTTL(d time.Duration) Option { return func(t *Options) { t.IdleTTL = d } }

// WithMaxLifetime sets the hard cap on session lifetime.
func WithMaxLifetime(d time.Duration) Option { return func(t *Options) { t.MaxLifetime = d } }

// WithCleanupInterval sets how often the cleanup sweeper runs.
func WithCleanupInterval(d time.Duration) Option { return func(t *Options) { t.CleanupInterval = d } }

// WithMaxEventBuffer sets the default event buffer size used for resumability.
func WithMaxEventBuffer(n int) Option { return func(t *Options) { t.MaxEventBuffer = n } }

// WithOnSessionClose registers a hook invoked when a session is finally closed.
func WithOnSessionClose(fn func(*base.Session)) Option {
	return func(t *Options) { t.OnSessionClose = fn }
}

// WithRemovalPolicy sets the session removal policy.
func WithRemovalPolicy(p base.RemovalPolicy) Option { return func(t *Options) { t.RemovalPolicy = p } }

// WithOverflowPolicy sets the event buffer overflow policy.
func WithOverflowPolicy(p base.OverflowPolicy) Option {
	return func(t *Options) { t.OverflowPolicy = p }
}

// WithSessionStore injects a custom SessionStore implementation.
func WithSessionStore(store base.SessionStore) Option { return func(t *Options) { t.Store = store } }

// WithBFFCookieSession enables cookie-based session id for BFF deployments.
func WithBFFCookieSession(c *BFFCookie) Option { return func(t *Options) { t.CookieSession = c } }

// WithCORSAllowedOrigins sets the allowed origins for CORS (must not be "*" when AllowCredentials is true).
func WithCORSAllowedOrigins(origins []string) Option {
	return func(t *Options) { t.AllowedOrigins = origins }
}

// WithCORSAllowCredentials toggles Access-Control-Allow-Credentials and credentialed requests.
func WithCORSAllowCredentials(v bool) Option { return func(t *Options) { t.AllowCredentials = v } }

// WithBFFCookieUseTopDomain enables auto setting cookie Domain to eTLD+1 derived from request host.
func WithBFFCookieUseTopDomain(v bool) Option { return func(t *Options) { t.CookieUseTopDomain = v } }

// WithAuthStore configures the durable store for BFF auth grants.
func WithAuthStore(store auth.Store) Option { return func(t *Options) { t.AuthStore = store } }

// WithBFFAuthCookie configures the cookie used to carry the BFF auth grant id.
func WithBFFAuthCookie(c *BFFAuthCookie) Option { return func(t *Options) { t.AuthCookie = c } }

// WithBFFAuthCookieUseTopDomain enables auto Domain=eTLD+1 for the auth cookie when Domain is empty.
func WithBFFAuthCookieUseTopDomain(v bool) Option {
	return func(t *Options) { t.AuthCookieUseTopDomain = v }
}

// WithRehydrateOnHandshake toggles using the auth cookie to mint a new MCP session when no session id is provided.
func WithRehydrateOnHandshake(v bool) Option { return func(t *Options) { t.RehydrateOnHandshake = v } }

// WithLogoutAllPath sets an optional path to revoke the BFF auth grant (logout all sessions).
func WithLogoutAllPath(path string) Option { return func(t *Options) { t.LogoutAllPath = path } }

// WithKeepAliveInterval sets the interval for SSE keepalive frames on the GET stream.
// Set to 0 or negative to disable.
func WithKeepAliveInterval(d time.Duration) Option {
	return func(t *Options) { t.KeepAliveInterval = d }
}
