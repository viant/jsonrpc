package sse

import (
	"bytes"
	"context"
	"fmt"
	"github.com/viant/afs/url"
	"io"
	"net/http"
)

type Transport struct {
	sseClient     *http.Client
	messageClient *http.Client
	host          string
	endpoint      string
	headers       http.Header
	client        *Client
}

// SendData sends data to the server
func (t *Transport) SendData(ctx context.Context, data []byte) error {
	if t.endpoint == "" {
		return fmt.Errorf("transport is not initialized - endpoint is empty")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", t.endpoint,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// set custom http headers
	for k, v := range t.headers {
		req.Header[k] = v
	}
	resp, err := t.messageClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusOK:
		if len(body) > 0 {
			t.client.base.HandleMessage(ctx, body)
		}
	default:
		return fmt.Errorf("invalid status code: %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (t *Transport) setEndpoint(URI string) {
	t.endpoint = url.Join(t.host, URI)
}
