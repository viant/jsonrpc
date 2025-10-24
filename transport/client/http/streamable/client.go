package streamable

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/viant/afs/url"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/client/base"
	"net/http/cookiejar"
	"sync"
)

const sseMime = "text/event-stream"

// Client implements streamable-http transport consumer (MCP 2025-03-26 spec).
// Handshake: POST /mcp -> obtains session id header.
// Stream    : GET  /mcp with same header and Accept: application/x-ndjson keeps receiving messages.
// Messages  : subsequent POST /mcp with header carry requests/notifications.
type Client struct {
	endpointURL string // /mcp endpoint
	base        *base.Client

	httpClient       *http.Client
	handshakeTimeout time.Duration

	sessionID string

	lastIDGet  uint64
	lastIDPost uint64

	transport *Transport

	// sessionHeaderName configures the HTTP header name carrying session id.
	// Defaults to "Mcp-Session-Id".
	sessionHeaderName string

	// protocolVersion, if set, will be sent as MCP-Protocol-Version header
	// on all HTTP requests (POST/GET) made by this client.
	protocolVersion string

	// streaming control
	streamMu     sync.Mutex
	streamActive bool
}

// sessionContext returns a context enriched with the current MCP session id. If
// no session id has been established yet it returns the original context.
func (c *Client) sessionContext(ctx context.Context) context.Context {
	if c.sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, jsonrpc.SessionKey, c.sessionID)
}

// Notify sends JSON-RPC notification.
func (c *Client) Notify(ctx context.Context, n *jsonrpc.Notification) error {
	return c.base.Notify(c.sessionContext(ctx), n)
}

// Send sends JSON-RPC request and waits for response.
func (c *Client) Send(ctx context.Context, r *jsonrpc.Request) (*jsonrpc.Response, error) {
	return c.base.Send(c.sessionContext(ctx), r)
}

func (c *Client) openStream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpointURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", sseMime)
	req.Header.Set(c.sessionHeaderName, c.sessionID)
	if c.protocolVersion != "" {
		req.Header.Set("MCP-Protocol-Version", c.protocolVersion)
	}
	if c.lastIDGet > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", c.lastIDGet))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return fmt.Errorf("stream invalid status: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	// Consume until the server closes or an error occurs; then return to caller
	c.consumeSSEGet(ctx, reader)
	_ = resp.Body.Close()
	return nil
}

// consumeSSE reads SSE frames and forwards JSON-RPC messages from "message" events.
// consumeSSEGet consumes events on the long-lived GET stream and updates lastIDGet.
func (c *Client) consumeSSEGet(ctx context.Context, reader *bufio.Reader) {
	for {
		evt, err := readSSE(ctx, reader)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				c.base.SetError(err)
			}
			return
		}
		if evt.ID != "" {
			if v, err := strconv.ParseUint(strings.TrimSpace(evt.ID), 10, 64); err == nil {
				c.lastIDGet = v
			}
		}
		if evt.Event != "message" || strings.TrimSpace(evt.Data) == "" {
			continue
		}
		c.base.HandleMessage(c.sessionContext(ctx), []byte(evt.Data))
	}
}

// consumeSSEPost consumes events on a POST-initiated SSE stream and updates lastIDPost.
func (c *Client) consumeSSEPost(ctx context.Context, reader *bufio.Reader) {
	for {
		evt, err := readSSE(ctx, reader)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				c.base.SetError(err)
			}
			return
		}
		if evt.ID != "" {
			if v, err := strconv.ParseUint(strings.TrimSpace(evt.ID), 10, 64); err == nil {
				c.lastIDPost = v
			}
		}
		if evt.Event != "message" || strings.TrimSpace(evt.Data) == "" {
			continue
		}
		c.base.HandleMessage(c.sessionContext(ctx), []byte(evt.Data))
	}
}

type sseEvent struct {
	ID    string
	Event string
	Data  string
}

// readSSE reads a single SSE event (terminated by blank line).
func readSSE(ctx context.Context, reader *bufio.Reader) (*sseEvent, error) {
	var hasData, hasEvent bool
	ev := &sseEvent{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return ev, io.EOF
				}
				return nil, err
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				if hasData || hasEvent {
					return ev, nil
				}
				continue
			}
			if strings.HasPrefix(line, "id:") {
				ev.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			} else if strings.HasPrefix(line, "event:") {
				ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				hasEvent = true
			} else if strings.HasPrefix(line, "data:") {
				ev.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				hasData = true
			}
		}
	}
}

// ensureStream starts a background reconnection loop for the GET SSE stream once a session id exists.
func (c *Client) ensureStream() {
	c.streamMu.Lock()
	if c.streamActive {
		c.streamMu.Unlock()
		return
	}
	c.streamActive = true
	c.streamMu.Unlock()

	go c.runStream()
}

func (c *Client) runStream() {
	// simple exponential backoff with cap
	backoff := 500 * time.Millisecond
	maxBackoff := 10 * time.Second
	for {
		// wait until session id is available
		if c.sessionID == "" {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		// Use a background context for long-lived stream
		ctx := context.Background()
		if err := c.openStream(ctx); err != nil {
			// stream couldn't be opened; back off and retry
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}
		// The stream ended gracefully; reset backoff and reconnect
		backoff = 500 * time.Millisecond
		// Loop to reconnect
	}
}

// New initialises Client and establishes streaming connection.
func New(ctx context.Context, endpointURL string, opts ...Option) (*Client, error) {
	schema := url.Scheme(endpointURL, "http")
	host := url.Host(endpointURL)

	// Default http.Client with cookie jar for auth session continuity, can be overridden via options
	jar, _ := cookiejar.New(nil)
	httpClient := &http.Client{Jar: jar}

	c := &Client{
		endpointURL:      endpointURL,
		httpClient:       httpClient,
		handshakeTimeout: 30 * time.Second,
	}
	c.sessionHeaderName = "Mcp-Session-Id"
	// Default protocol version (can be overridden via option)
	if c.protocolVersion == "" {
		c.protocolVersion = "2025-06-18"
	}

	// build transport
	c.transport = &Transport{
		client:  httpClient,
		headers: make(http.Header),
		host:    fmt.Sprintf("%s://%s", schema, host),
		c:       c,
	}

	c.base = &base.Client{
		RunTimeout: 15 * time.Minute,
		RoundTrips: transport.NewRoundTrips(100),
		Handler:    &base.Handler{},
		Logger:     jsonrpc.DefaultLogger,
	}
	c.base.Transport = c.transport

	for _, opt := range opts {
		opt(c)
	}

	// Ensure the transport uses the possibly overridden HTTP client
	c.transport.client = c.httpClient

	c.transport.setEndpoint(c.endpointURL)
	// Ensure POST requests include protocol version header by default
	if c.protocolVersion != "" {
		c.transport.headers.Set("MCP-Protocol-Version", c.protocolVersion)
	}

	return c, nil
}
