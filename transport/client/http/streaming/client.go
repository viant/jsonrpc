package streaming

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/viant/afs/url"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/client/base"
)

const (
	mcpSessionHeaderKey = "Mcp-Session-Id"
	ndjsonMime          = "application/x-ndjson"
)

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

	lastID uint64

	transport *Transport
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
	req.Header.Set("Accept", ndjsonMime)
	req.Header.Set(mcpSessionHeaderKey, c.sessionID)
	if c.lastID > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", c.lastID))
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
	go c.consume(ctx, reader)
	return nil
}

func (c *Client) consume(ctx context.Context, reader *bufio.Reader) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			// Network interruption may result in io.ErrUnexpectedEOF which indicates that
			// the connection was closed before a full frame was read. Treat it the same
			// way as io.EOF – terminate current stream reader without marking transport
			// as failed. The higher-level code may attempt reconnection if required.
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				c.base.SetError(err)
			}
			return
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}

		// attempt to parse extended frame with id
		var wrapper struct {
			ID   uint64          `json:"id"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil && wrapper.Data != nil {
			if wrapper.ID > 0 {
				c.lastID = wrapper.ID
			}
			c.base.HandleMessage(c.sessionContext(ctx), wrapper.Data)
			continue
		}
		// fallback – treat entire line as JSON-RPC message (no id)
		c.base.HandleMessage(c.sessionContext(ctx), []byte(trimmed))
	}
}

// New initialises Client and establishes streaming connection.
func New(ctx context.Context, endpointURL string, opts ...Option) (*Client, error) {
	schema := url.Scheme(endpointURL, "http")
	host := url.Host(endpointURL)

	httpClient := &http.Client{}

	c := &Client{
		endpointURL:      endpointURL,
		httpClient:       httpClient,
		handshakeTimeout: 30 * time.Second,
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

	c.transport.setEndpoint(c.endpointURL)

	return c, nil
}
