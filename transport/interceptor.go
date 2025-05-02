package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

// Interceptor defines an interface for intercepting JSON-RPC requests and responses for specific methods
// It allows for method-level post-processing of responses and optionally sending additional requests
type Interceptor interface {
	// Intercept is called after a response is received for a specific method (even if it's an error)
	// It receives the context, original request, and the response
	// If it returns a non-nil request, that request will be sent as a follow-up
	// If it returns nil, no additional request will be sent
	Intercept(ctx context.Context, request *jsonrpc.Request, response *jsonrpc.Response) (*jsonrpc.Request, error)
}
