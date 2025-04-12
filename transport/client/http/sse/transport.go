package sse

import (
	"bytes"
	"context"
	"fmt"
	"github.com/viant/afs/url"
	"io"
	"net/http"
)

type transport struct {
	client   *http.Client
	host     string
	endpoint string
	headers  http.Header
}

// SendData sends data to the server
func (c *transport) SendData(ctx context.Context, data []byte) error {
	if c.endpoint == "" {
		return fmt.Errorf("transport is not initialized - endpoint is empty")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint,
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// set custom http headers
	for k, v := range c.headers {
		req.Header[k] = v
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted:
	default:
		return fmt.Errorf("invalid status code: %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (c *transport) setEndpoint(URI string) {
	c.endpoint = url.Join(c.host, URI)
}
