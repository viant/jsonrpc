package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

// Notifier represents a notification handler
type Notifier interface {
	Notify(ctx context.Context, notification *jsonrpc.Notification) error
}
