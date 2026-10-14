package base

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/viant/jsonrpc"
)

type concurrentRequestHandler struct{}

func (concurrentRequestHandler) Serve(_ context.Context, _ *jsonrpc.Request, response *jsonrpc.Response) {
	response.Result = []byte(`"ok"`)
}
func (concurrentRequestHandler) OnNotification(context.Context, *jsonrpc.Notification) {}

func TestConcurrentRequestIDsNeverRaceOrMoveSequenceBackward(t *testing.T) {
	endpoint := &Handler{}
	session := &Session{Handler: concurrentRequestHandler{}}
	const requests = 64
	start := make(chan struct{})
	var workers sync.WaitGroup
	for id := requests; id >= 1; id-- {
		workers.Add(1)
		go func(id int) {
			defer workers.Done()
			<-start
			var output bytes.Buffer
			endpoint.HandleMessage(context.Background(), session, []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"probe","params":{}}`, id)), &output)
			if output.Len() == 0 {
				t.Errorf("request %d had no response", id)
			}
		}(id)
	}
	close(start)
	workers.Wait()
	if got := session.LastRequestID(); got != requests {
		t.Fatalf("last request id = %v, want %d", got, requests)
	}
	if next := session.NextRequestID(); next != requests+1 {
		t.Fatalf("next request id = %v, want %d", next, requests+1)
	}
}
