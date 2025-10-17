package streamable

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Transport implements client side sender for the streaming HTTP transport. It
// expects that the endpoint supplied via handshake is capable of accepting a
// POST request with a JSON payload and will synchronously return any response
// payload.
type Transport struct {
	client   *http.Client
	headers  http.Header
	endpoint string
	host     string
	c        *Client
	sync.Mutex
}

func (t *Transport) setEndpoint(uri string) {
	t.endpoint = uri
}

// SendData forwards JSON-RPC message data to the server using HTTP POST.
func (t *Transport) SendData(ctx context.Context, data []byte) error {
	t.Lock()

	if t.endpoint == "" {
		return fmt.Errorf("transport is not initialised - endpoint is empty")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Per spec, client MUST declare it supports both JSON & SSE for POST
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, v := range t.headers {
		req.Header[k] = v
	}

	resp, err := t.client.Do(req)
	if err != nil {
		t.Unlock()
		return fmt.Errorf("failed to send request: %w", err)
	}
	// If server sent session id on handshake, capture it
	if sessionID := resp.Header.Get(t.c.sessionHeaderName); sessionID != "" {
		if t.c.sessionID == sessionID {

			go func() {
				_ = t.c.openStream(ctx)
			}()

		}

		t.c.sessionID = sessionID
		// Ensure subsequent message POSTs include the session id header
		t.headers.Set(t.c.sessionHeaderName, sessionID)
	}

	if t.c.sessionID == "" {
		t.Unlock()
		return fmt.Errorf("handshake missing %s header", t.c.sessionHeaderName)
	}

	// If server responded with SSE, consume stream and return
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		// Release the transport lock before consuming the stream to allow
		// re-entrant SendData calls (e.g. replies to server-initiated requests)
		t.Unlock()
		reader := bufio.NewReader(resp.Body)
		// consume stream inline; server should close stream after sending response
		t.c.consumeSSEPost(ctx, reader)
		_ = resp.Body.Close()
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		if len(body) > 0 {
			t.c.base.HandleMessage(ctx, body)
		}
	default:
		return fmt.Errorf("invalid status code: %d: %s", resp.StatusCode, string(body))
	}
	t.Unlock()
	return nil
}
