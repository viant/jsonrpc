package transport

import (
	"context"
	"github.com/viant/jsonrpc"
)

type Notifier interface {
	Notify(ctx context.Context, notification *jsonrpc.Notification) error
	Notification() chan *jsonrpc.Notification
}
