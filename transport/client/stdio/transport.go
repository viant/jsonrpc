package stdio

import (
	"context"
	"fmt"
	"github.com/viant/gosh/runner"
	"sync"
)

type Transport struct {
	client runner.Runner
	sync.Mutex
}

// SendData sends data to the Transport
func (c *Transport) SendData(ctx context.Context, data []byte) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.client == nil {
		return fmt.Errorf("Transport is not initialized")
	}
	_, err := c.client.Send(ctx, data)
	return err
}
