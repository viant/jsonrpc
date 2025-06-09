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
	return c.base.Notify(ctx, request)
}

func (c *Client) Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error) {
	return c.base.Send(ctx, request)
}

func (c *Client) newStreamingRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.streamURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
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
				if err == io.EOF {
					return event, nil
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

			if strings.HasPrefix(line, "event:") {
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
			c.base.SetError(err)
			return
		}
		switch event.Event {
		case "message":
			c.base.HandleMessage(ctx, []byte(event.Data))
		default:
			c.base.SetError(fmt.Errorf("unexpected event: %s", event.Event))
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
			RunTimeout: 5 * time.Minute,
			RoundTrips: transport.NewRoundTrips(100),
			Handler:    &base.Handler{},
			Logger:     jsonrpc.DefaultLogger,
		},
		transport: &Transport{
			messageClient: client,
			sseClient:     client,
			host:          fmt.Sprintf("%s://%s", schema, host),
		},
	}
	ret.transport.client = ret
	for _, opt := range options {
		opt(ret)
	}
	ret.base.Transport = ret.transport
	err := ret.start(ctx)
	return ret, err
}
