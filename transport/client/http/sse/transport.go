package sse

import (
	"bytes"
	"context"
	"fmt"
	"github.com/viant/afs/url"
	"io"
	"net/http"
	"sync"
)

type Transport struct {
	client   *http.Client
	host     string
	endpoint string
	headers  http.Header
	sync.Mutex
}

// SendData sends data to the server
func (c *Transport) SendData(ctx context.Context, data []byte) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.endpoint == "" {
		return fmt.Errorf("Transport is not initialized - endpoint is empty")
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

func (c *Transport) setEndpoint(URI string) {
	c.endpoint = url.Join(c.host, URI)
}
