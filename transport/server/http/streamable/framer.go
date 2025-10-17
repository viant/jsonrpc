package streamable

// frameJSON appends a new line to ensure that each JSON message is written as a single line.
// The client side reader relies on a new line as a message delimiter.
// If the payload already contains a trailing new line the data is returned unmodified.
import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport/server/base"
)

// frameJSON is kept for compatibility with earlier code (id-less framing).
func frameJSON(data []byte) []byte {
	n := len(data)
	if n == 0 {
		return []byte("\n")
	}
	if data[n-1] == '\n' {
		return data
	}
	framed := make([]byte, n+1)
	copy(framed, data)
	framed[n] = '\n'
	return framed
}

// framerWithSession creates stateful framer that prepends incremental id to
// every JSON message so the stream can be resumed with Last-Event-ID.
func framerWithSession(s *base.Session) base.FrameMessage {
	return func(data []byte) []byte {

		requestID := s.NextRequestID()
		id, _ := jsonrpc.AsRequestIntId(requestID)
		// ensure data is trimmed to single line (no newline)
		payload := strings.TrimSpace(string(data))
		wrapper := struct {
			ID   uint64          `json:"id"`
			Data json.RawMessage `json:"data"`
		}{ID: uint64(id), Data: json.RawMessage(payload)}
		b, _ := json.Marshal(&wrapper)
		return append(b, '\n')
	}
}

// frameSSE formats the data using SSE event/format framing.
func frameSSE(data []byte) []byte {
	return []byte(fmt.Sprintf("event: message\ndata: %s\n\n", strings.TrimSpace(string(data))))
}
