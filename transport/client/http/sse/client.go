package sse

import (
	"bufio"
	"context"
	"fmt"
	"github.com/viant/afs/url"
	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/client/base"
	"io"
	"net/http"
	stdurl "net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	stream           io.Reader
	handshakeTimeout time.Duration
	streamURL        string
	base             *base.Client
	done             chan bool
	transport        *Transport

	sessionID string

	// lastEventID tracks the last received SSE id for resumability
	lastEventID uint64

	// protocolVersion, if set, will be sent as MCP-Protocol-Version header
	// on all HTTP requests (GET handshake and POST messages).
	protocolVersion string

	// streamSessionParamName defines the query parameter name used to carry
	// session id on reconnect GET requests. Defaults to "Mcp-Session-Id" to
	// align with server default, but can be overridden via option.
	streamSessionParamName string
}

// sessionContext returns ctx enriched with MCP session id when available.
func (c *Client) sessionContext(ctx context.Context) context.Context {
	if c.sessionID == "" {
		return ctx
	}
	return context.WithValue(ctx, jsonrpc.SessionKey, c.sessionID)
}

func (c *Client) start(ctx context.Context) error {
	req, err := c.newStreamingRequest(ctx)
	if err != nil {
		return err
	}
	resp, err := c.transport.sseClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return jsonrpc.NewUnauthorizedError(resp.StatusCode, body)
		}
		_ = resp.Body.Close()
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}
	reader := bufio.NewReader(resp.Body)
	if err := c.handleHandshake(reader); err != nil {
		return err
	}
	go c.listenForMessages(ctx, reader)

	return nil

}

func (c *Client) Notify(ctx context.Context, request *jsonrpc.Notification) error {
	return c.base.Notify(c.sessionContext(ctx), request)
}

func (c *Client) Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error) {
	return c.base.Send(c.sessionContext(ctx), request)
}

// SessionID returns the current session id if known.
func (c *Client) SessionID() string { return c.sessionID }

func (c *Client) newStreamingRequest(ctx context.Context) (*http.Request, error) {
	// If session id is known, append as query param to support server-side session reuse
	urlStr := c.streamURL
	if c.sessionID != "" && c.streamSessionParamName != "" {
		if u, err := stdurl.Parse(urlStr); err == nil {
			q := u.Query()
			q.Set(c.streamSessionParamName, c.sessionID)
			u.RawQuery = q.Encode()
			urlStr = u.String()
		}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if c.protocolVersion != "" {
		req.Header.Set("MCP-Protocol-Version", c.protocolVersion)
	}
	if c.lastEventID > 0 {
		req.Header.Set("Last-Event-ID", fmt.Sprintf("%d", c.lastEventID))
	}
	return req, nil
}

func (c *Client) handleHandshake(reader *bufio.Reader) error {
	event, err := c.readWithTimeout(context.Background(), reader, c.handshakeTimeout)
	if err != nil {
		return err
	}
	switch event.Event {
	case "endpoint":
		c.transport.setEndpoint(event.Data)
		if event.Data == "" {
			return fmt.Errorf("endpoint event is empty")
		}

		// Attempt to extract session_id query parameter from the endpoint URI
		if u, err := stdurl.Parse(event.Data); err == nil {
			id := u.Query().Get("session_id")
			if id != "" {
				c.sessionID = id
			}
		}
		return nil
	default:
		return fmt.Errorf("unexpected event: %s", event.Event)
	}
}

func (c *Client) readWithTimeout(ctx context.Context, reader *bufio.Reader, timeout time.Duration) (*Event, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.read(ctx, reader)
}

func (c *Client) read(ctx context.Context, reader *bufio.Reader) (*Event, error) {
	var hasData, hasEvent bool
	event := &Event{}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				// Treat io.EOF as normal stream closure, but return error so caller can reconnect.
				if err == io.EOF {
					return event, nil
				}
				if err == io.ErrUnexpectedEOF {
					return nil, err
				}
				select {
				case <-c.done:
					return event, nil
				default:
					return nil, fmt.Errorf("SSE stream error: %v\n", err)
				}
			}

			line = strings.TrimRight(line, "\r\n")
			// Remove only newline markers
			if line == "" {
				// Empty line means end of event
				if hasData && hasEvent {
					return event, nil
				}
				continue
			}

			if strings.HasPrefix(line, "id:") {
				event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			} else if strings.HasPrefix(line, "event:") {
				event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				hasEvent = true
			} else if strings.HasPrefix(line, "data:") {
				event.Data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				hasData = true
			}
		}
	}
}

func (c *Client) listenForMessages(ctx context.Context, reader *bufio.Reader) {
	for {
		event, err := c.read(ctx, reader)
		if err != nil {
			// Attempt to seamlessly reconnect the SSE stream.
			go func() {
				_ = c.start(ctx)
			}()
			return
		}
		// Ignore empty events (can occur during reconnect or keep-alive comments).
		if event.Event == "" {
			continue
		}
		// Track last received event id for Last-Event-ID resumability
		if event.ID != "" {
			if v, err := strconv.ParseUint(strings.TrimSpace(event.ID), 10, 64); err == nil {
				c.lastEventID = v
			}
		}
		switch event.Event {
		case "message":
			c.base.HandleMessage(c.sessionContext(ctx), []byte(event.Data))
		default:
			// Unrecognised event – skip instead of propagating fatal error.
			continue
		}
	}
}

func New(ctx context.Context, streamURL string, options ...Option) (*Client, error) {
	schema := url.Scheme(streamURL, "http")
	host := url.Host(streamURL)
	client := &http.Client{}
	ret := &Client{
		streamURL:        streamURL,
		handshakeTimeout: time.Second * 30,
		done:             make(chan bool),
		base: &base.Client{
			RunTimeout: 15 * time.Minute,
			RoundTrips: transport.NewRoundTrips(100),
			Handler:    &base.Handler{},
			Logger:     jsonrpc.DefaultLogger,
		},
		transport: &Transport{
			messageClient: client,
			sseClient:     client,
			host:          fmt.Sprintf("%s://%s", schema, host),
			headers:       make(http.Header),
		},
	}
	// Default protocol version (can be overridden via option)
	if ret.protocolVersion == "" {
		ret.protocolVersion = "2025-06-18"
	}
	// Default streaming session param name aligns with server default
	if ret.streamSessionParamName == "" {
		ret.streamSessionParamName = "Mcp-Session-Id"
	}
	// Ensure POST requests include protocol version header by default
	if ret.protocolVersion != "" {
		ret.transport.headers.Set("MCP-Protocol-Version", ret.protocolVersion)
	}
	ret.transport.client = ret
	for _, opt := range options {
		opt(ret)
	}
	ret.base.Transport = ret.transport
	err := ret.start(ctx)
	return ret, err
}
