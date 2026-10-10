package streamable

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/viant/jsonrpc"
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
	unlocked := false
	unlock := func() {
		if unlocked {
			return
		}
		t.Unlock()
		unlocked = true
	}
	defer unlock()

	if t.endpoint == "" {
		return fmt.Errorf("transport is not initialised - endpoint is empty")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Keep request/response POSTs synchronous. The long-lived GET stream still
	// carries server-initiated requests such as sampling, while the final
	// response for this request is delivered on this POST body.
	if t.c.stateless {
		req.Header.Set("Accept", "application/json, text/event-stream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	for k, v := range t.headers {
		req.Header[k] = append([]string(nil), v...)
	}
	if t.c.requestHeaderProvider != nil {
		if err := t.c.requestHeaderProvider(ctx, data, req.Header); err != nil {
			return fmt.Errorf("failed to build request headers: %w", err)
		}
	}
	unlock()

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return jsonrpc.NewUnauthorizedError(resp.StatusCode, body)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if isJSONRPCErrorResponse(data, body) {
			t.c.base.HandleMessage(ctx, body)
			return nil
		}
		return fmt.Errorf("invalid status code: %d: %s", resp.StatusCode, string(body))
	}

	// If server sent session id on handshake, capture it in stateful mode.
	if sessionID := resp.Header.Get(t.c.sessionHeaderName); !t.c.stateless && sessionID != "" {
		// Update known session id and ensure the GET stream is running
		t.Lock()
		t.c.setSessionID(sessionID)
		// Ensure subsequent message POSTs include the session id header
		t.headers.Set(t.c.sessionHeaderName, sessionID)
		t.Unlock()
		// Start long-lived GET stream (reconnection handled internally)
		t.c.ensureStream()
	}

	if !t.c.stateless && t.c.SessionID() == "" {
		_ = resp.Body.Close()
		return fmt.Errorf("handshake missing %s header", t.c.sessionHeaderName)
	}

	// If server responded with SSE, consume stream and return
	if ct := resp.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		// Release the transport lock before consuming the stream to allow
		// re-entrant SendData calls (e.g. replies to server-initiated requests)
		unlock()
		reader := bufio.NewReader(resp.Body)
		// consume stream inline; server should close stream after sending response
		t.c.consumeSSEPost(ctx, reader)
		_ = resp.Body.Close()
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if len(body) > 0 {
		t.c.base.HandleMessage(ctx, body)
	}
	return nil
}

func isJSONRPCErrorResponse(requestData, responseData []byte) bool {
	var request struct {
		ID json.RawMessage `json:"id"`
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Error   *jsonrpc.Error  `json:"error"`
	}
	if json.Unmarshal(requestData, &request) != nil || len(request.ID) == 0 {
		return false
	}
	if json.Unmarshal(responseData, &response) != nil || response.JSONRPC != jsonrpc.Version || response.Error == nil {
		return false
	}
	return bytes.Equal(bytes.TrimSpace(request.ID), bytes.TrimSpace(response.ID))
}
