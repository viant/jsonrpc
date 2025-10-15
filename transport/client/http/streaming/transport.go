package streaming

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
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
	defer t.Unlock()

	if t.endpoint == "" {
		return fmt.Errorf("transport is not initialised - endpoint is empty")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header[k] = v
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if sessionID := resp.Header.Get(mcpSessionHeaderKey); sessionID != "" {
		if t.c.sessionID == sessionID {

			go func() {
				err = t.c.openStream(ctx)
				if err != nil {

				}
			}()

		}

		t.c.sessionID = sessionID
	}

	if t.c.sessionID == "" {
		return fmt.Errorf("handshake missing %s header", mcpSessionHeaderKey)
	}

	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		if len(body) > 0 {
			t.c.base.HandleMessage(ctx, body)
		}
	default:
		return fmt.Errorf("invalid status code: %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
