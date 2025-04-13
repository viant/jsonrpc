package base

import "context"

// Transport is a base transport interface
type Transport interface {
	SendData(ctx context.Context, data []byte) error
}
