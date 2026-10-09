package base

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
)

type payloadLogCapture struct {
	entries   []string
	arguments []string
}

func (l *payloadLogCapture) Errorf(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
	l.arguments = append(l.arguments, fmt.Sprint(args...))
}

type payloadSink struct{}

func (*payloadSink) SendData(context.Context, []byte) error { return nil }

type payloadHandler struct{ requests, notifications int }

func (h *payloadHandler) Serve(_ context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
	h.requests++
	response.Id, response.Jsonrpc, response.Result = request.Id, request.Jsonrpc, []byte(`"ok"`)
}
func (h *payloadHandler) OnNotification(_ context.Context, _ *jsonrpc.Notification) {
	h.notifications++
}

func TestClientParserLogsExcludePayload(t *testing.T) {
	const marker = "AAECA/8=PRIVATE_ARTIFACT_PAYLOAD"
	const secretNumber = "1234567890123456789012345678901234567890"
	for _, tc := range []struct {
		kind, data    string
		notifications int
	}{
		{"request", `{"id":1,"method":"tool","jsonrpc":false,"params":{"artifact":"` + marker + `"}}`, 0},
		{"notification", `{"method":"tool","jsonrpc":false,"params":{"artifact":"` + marker + `"}}`, 1},
		{"response", `{"id":1,"jsonrpc":false,"result":"` + marker + `"}`, 0},
		{"response", `{"id":1,"jsonrpc":"2.0","error":{"code":` + secretNumber + `,"message":"` + marker + `"}}`, 0},
		{"notification", `{"method":"tool","jsonrpc":"2.0","params":"` + marker + `"` + "\x00", 1},
	} {
		t.Run(tc.kind+fmt.Sprint(len(tc.data)), func(t *testing.T) {
			logger, handler := &payloadLogCapture{}, &payloadHandler{}
			client := &Client{Logger: logger, Handler: handler, RoundTrips: transport.NewRoundTrips(10)}
			client.HandleMessage(context.Background(), []byte(tc.data))
			if len(logger.entries) != 1 {
				t.Fatalf("got %d log entries, want 1", len(logger.entries))
			}
			for _, value := range append(logger.entries, logger.arguments...) {
				if strings.Contains(value, marker) || strings.Contains(value, secretNumber) || strings.Contains(value, tc.data) {
					t.Fatal("parser logger received payload")
				}
			}
			if !strings.Contains(logger.entries[0], "failed to parse "+tc.kind) || !strings.Contains(logger.entries[0], fmt.Sprintf("message bytes: %d", len(tc.data))) {
				t.Fatalf("missing safe diagnostics: %s", logger.entries[0])
			}
			if handler.requests != 0 || handler.notifications != tc.notifications {
				t.Fatalf("parse error dispatch changed: %#v", handler)
			}
		})
	}
}

func TestClientValidMessagesStillDispatch(t *testing.T) {
	logger, handler := &payloadLogCapture{}, &payloadHandler{}
	client := &Client{Logger: logger, Handler: handler, Transport: &payloadSink{}, RoundTrips: transport.NewRoundTrips(10)}
	client.HandleMessage(context.Background(), []byte(`{"id":1,"method":"tool","jsonrpc":"2.0","params":{}}`))
	client.HandleMessage(context.Background(), []byte(`{"method":"event","jsonrpc":"2.0"}`))
	trip, err := client.RoundTrips.Add(&jsonrpc.Request{Id: 2})
	if err != nil {
		t.Fatal(err)
	}
	client.HandleMessage(context.Background(), []byte(`{"id":2,"jsonrpc":"2.0","result":"ok"}`))
	if handler.requests != 1 || handler.notifications != 1 || trip.Response == nil || string(trip.Response.Result) != `"ok"` || len(logger.entries) != 0 {
		t.Fatalf("valid dispatch changed: handler=%#v response=%#v logs=%v", handler, trip.Response, logger.entries)
	}
}
