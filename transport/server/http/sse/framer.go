package sse

import (
	"fmt"
	"strings"
)

// frameSSE formats the data for SSE
func frameSSE(data []byte) []byte {
	expanded := fmt.Sprintf("event: message\ndata: %s\n\n", strings.TrimSpace(string(data)))
	return []byte(expanded)
}
