package sse

import "fmt"

// frameSSE formats the data for SSE
func frameSSE(data []byte) []byte {
	expanded := fmt.Sprintf("event: message\ndata: %s\n", string(data))
	return []byte(expanded)
}
