package stdio

import (
	"context"
	"fmt"
	"github.com/viant/gosh/runner"
)

type transport struct {
	client runner.Runner
}

// SendData sends data to the transport
func (c *transport) SendData(ctx context.Context, data []byte) error {
	if c.client == nil {
		return fmt.Errorf("transport is not initialized")
	}
	_, err := c.client.Send(ctx, data)
	return err
}
